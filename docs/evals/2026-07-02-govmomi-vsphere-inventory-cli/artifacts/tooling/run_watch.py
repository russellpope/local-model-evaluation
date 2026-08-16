#!/usr/bin/env python3
"""Live behaviour tail for a local-model eval run driven through opencode.

The server log tells you the model is *generating*. This tells you what it is
actually *doing* — reasoning cadence, tool calls, context depth, compaction, and
whether it has quietly stopped making progress.

Reads the opencode store READ-ONLY. Never writes, never locks.

  python3 run_watch.py                    # follow the newest session
  python3 run_watch.py --session ses_xxx  # follow a specific session
  python3 run_watch.py --replay           # dump history first, then follow
  python3 run_watch.py --full             # don't truncate reasoning/text

Why the stall columns exist: three consecutive runs in this field lost hours to a
permission stall, and the prior watchdog could catch a dead server but was blind to
"busy forever, no progress". Tool-call age is the signal that distinguishes them —
a healthy run emits a tool call every few minutes; a livelocked one generates
tokens indefinitely while the tool-call clock stops.
"""

import argparse
import json
import os
import sqlite3
import sys
import time

DB = os.path.expanduser("~/.local/share/opencode/opencode.db")

# tool-call age (seconds) before we start shouting
WARN_IDLE = 600     # 10 min — unusual but survivable
ALERT_IDLE = 1800   # 30 min — historically always a real stall

C = {
    "dim": "\033[2m", "red": "\033[31m", "grn": "\033[32m", "yel": "\033[33m",
    "blu": "\033[34m", "mag": "\033[35m", "cyn": "\033[36m", "bold": "\033[1m",
    "rst": "\033[0m",
}


def color(s, c, enabled):
    return f"{C[c]}{s}{C['rst']}" if enabled else s


def connect():
    if not os.path.exists(DB):
        sys.exit(f"opencode store not found at {DB}")
    return sqlite3.connect(f"file:{DB}?mode=ro", uri=True)


def newest_session(con):
    row = con.execute(
        "select session_id, max(time_created) from part group by session_id "
        "order by 2 desc limit 1"
    ).fetchone()
    if not row:
        sys.exit("no sessions in the store")
    return row[0]


def title_of(con, sid):
    row = con.execute("select title from session where id=?", (sid,)).fetchone()
    return row[0] if row else "(untitled)"


def flat(s, limit):
    s = " ".join((s or "").split())
    if limit and len(s) > limit:
        s = s[:limit] + "…"
    return s


def render(row, st, args, tty):
    pid, created, data = row
    try:
        d = json.loads(data)
    except Exception:
        return None
    typ = d.get("type")
    ts = time.strftime("%H:%M:%S", time.localtime(created / 1000))
    lim = 0 if args.full else 130

    if typ == "step-finish":
        tot = (d.get("tokens") or {}).get("total")
        if tot:
            st["depth"] = tot
        return None

    if typ == "compaction":
        st["compactions"] += 1
        bar = "=" * 72
        depth_s = color(format(st["depth"], ","), "bold", tty)
        note = ("  Historically ~5h at this depth. Not the model's fault; "
                "record as a run condition.")
        return (
            f"\n{color(bar, 'red', tty)}\n"
            f"{color('  *** COMPACTION FIRED ***', 'red', tty)}  at depth "
            f"{depth_s} tokens — expect a full cold reprocess (cache read/write 0).\n"
            f"{color(note, 'dim', tty)}\n"
            f"{color(bar, 'red', tty)}\n"
        )

    if typ == "reasoning":
        st["reason_n"] += 1
        txt = d.get("text") or ""
        st["reason_chars"] += len(txt)
        if args.quiet:
            return None
        return (f"{color(ts,'dim',tty)} {color('REASON','mag',tty)} "
                f"{color(f'[{len(txt):>6,}c]','dim',tty)} {flat(txt, lim)}")

    if typ == "text":
        st["text_n"] += 1
        return (f"{color(ts,'dim',tty)} {color('SAY   ','cyn',tty)} "
                f"{' '*9}{flat(d.get('text'), lim)}")

    if typ == "tool":
        st["tool_n"] += 1
        st["last_tool"] = created / 1000
        name = d.get("tool") or "?"
        st["tools"][name] = st["tools"].get(name, 0) + 1
        inp = ((d.get("state") or {}).get("input")) or {}
        detail = (inp.get("command") or inp.get("filePath") or inp.get("pattern")
                  or inp.get("description") or "")
        if name in ("write", "edit"):
            st["files"] += 1
            hue = "grn"
        elif name == "todowrite":
            hue = "yel"
        else:
            hue = "blu"
        label = color(format(name.upper(), "<6"), hue, tty)
        seq = color("#" + format(st["tool_n"], "<4"), "dim", tty)
        return f"{color(ts,'dim',tty)} {label} {seq} {flat(detail, lim)}"

    return None


def status(st, tty):
    idle = time.time() - st["last_tool"] if st["last_tool"] else 0
    if idle >= ALERT_IDLE:
        hue, tag = "red", "STALLED?"
    elif idle >= WARN_IDLE:
        hue, tag = "yel", "idle"
    else:
        hue, tag = "grn", "ok"
    top = sorted(st["tools"].items(), key=lambda kv: -kv[1])[:3]
    tops = " ".join(f"{k}:{v}" for k, v in top)
    depth_s = color(format(st["depth"], ">7,"), "bold", tty)
    idle_s = color(format(idle / 60, ">5.1f") + "m", hue, tty)
    return (
        f"{color('  -- ','dim',tty)}"
        f"depth {depth_s}  "
        f"tools {st['tool_n']:<4} files {st['files']:<3} "
        f"reason {st['reason_n']}/{st['reason_chars']:,}c  "
        f"compact {st['compactions']}  "
        f"last-tool {idle_s} {color(tag, hue, tty)}  {color(tops,'dim',tty)}"
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--session")
    ap.add_argument("--interval", type=float, default=3.0)
    ap.add_argument("--replay", action="store_true", help="print history before following")
    ap.add_argument("--full", action="store_true", help="do not truncate")
    ap.add_argument("--quiet", action="store_true", help="suppress reasoning bodies")
    ap.add_argument("--status-every", type=float, default=60.0)
    args = ap.parse_args()

    tty = sys.stdout.isatty()
    con = connect()
    sid = args.session or newest_session(con)

    st = {"depth": 0, "tool_n": 0, "files": 0, "reason_n": 0, "reason_chars": 0,
          "text_n": 0, "compactions": 0, "last_tool": 0.0, "tools": {}}

    print(color(f"watching {sid}", "bold", tty))
    print(color(f"  {title_of(con, sid)}", "dim", tty))
    print(color(f"  store {DB} (read-only)", "dim", tty))
    print()

    if args.replay:
        cursor = 0
    else:
        row = con.execute(
            "select max(time_created) from part where session_id=?", (sid,)
        ).fetchone()
        cursor = (row[0] or 0)
        # prime counters from history so the status line is accurate
        for r in con.execute(
            "select id,time_created,data from part where session_id=? order by time_created", (sid,)
        ):
            render(r, st, argparse.Namespace(full=False, quiet=True), False)

    last_status = 0.0
    try:
        while True:
            rows = con.execute(
                "select id,time_created,data from part "
                "where session_id=? and time_created > ? order by time_created",
                (sid, cursor),
            ).fetchall()
            for r in rows:
                cursor = max(cursor, r[1])
                line = render(r, st, args, tty)
                if line:
                    print(line, flush=True)
            now = time.time()
            if now - last_status >= args.status_every:
                print(status(st, tty), flush=True)
                last_status = now
            time.sleep(args.interval)
    except KeyboardInterrupt:
        print("\n" + status(st, tty))
        print(color("stopped", "dim", tty))


if __name__ == "__main__":
    main()
