#!/usr/bin/env python3
"""Read GGUF KV metadata from a local file or a remote URL via HTTP Range.

Downloads only the header bytes (a few MB), never tensor data.
Usage: gguf_head.py <url-or-path> [key-substring ...]
"""
import struct
import sys
import urllib.request

CHUNK = 4 * 1024 * 1024


class Src:
    def __init__(self, loc):
        self.loc = loc
        self.remote = loc.startswith("http")
        self.buf = b""
        self.pos = 0
        if not self.remote:
            self.fh = open(loc, "rb")

    def _ensure(self, upto):
        while len(self.buf) < upto:
            start = len(self.buf)
            end = start + CHUNK - 1
            if self.remote:
                req = urllib.request.Request(
                    self.loc, headers={"Range": f"bytes={start}-{end}"}
                )
                with urllib.request.urlopen(req, timeout=120) as r:
                    data = r.read()
            else:
                self.fh.seek(start)
                data = self.fh.read(CHUNK)
            if not data:
                raise EOFError("source exhausted")
            self.buf += data

    def read(self, n):
        self._ensure(self.pos + n)
        out = self.buf[self.pos:self.pos + n]
        self.pos += n
        return out

    def u32(self):
        return struct.unpack("<I", self.read(4))[0]

    def u64(self):
        return struct.unpack("<Q", self.read(8))[0]


# GGUF value type ids
T_U8, T_I8, T_U16, T_I16, T_U32, T_I32, T_F32, T_BOOL, T_STR, T_ARR, T_U64, T_I64, T_F64 = range(13)
FIXED = {
    T_U8: ("<B", 1), T_I8: ("<b", 1), T_U16: ("<H", 2), T_I16: ("<h", 2),
    T_U32: ("<I", 4), T_I32: ("<i", 4), T_F32: ("<f", 4), T_BOOL: ("<?", 1),
    T_U64: ("<Q", 8), T_I64: ("<q", 8), T_F64: ("<d", 8),
}


def read_val(s, t):
    if t in FIXED:
        fmt, n = FIXED[t]
        return struct.unpack(fmt, s.read(n))[0]
    if t == T_STR:
        return s.read(s.u64()).decode("utf-8", "replace")
    if t == T_ARR:
        et = s.u32()
        n = s.u64()
        if et == T_STR:
            # strings can be huge (tokenizer vocab) — record length, sample a few
            vals = []
            for i in range(n):
                v = s.read(s.u64()).decode("utf-8", "replace")
                if i < 8:
                    vals.append(v)
            return f"<array str len={n} first={vals}>"
        if et in FIXED:
            fmt, sz = FIXED[et]
            raw = s.read(sz * n)
            vals = [struct.unpack_from(fmt, raw, i * sz)[0] for i in range(min(n, 16))]
            return f"<array type={et} len={n} first={vals}>"
        raise ValueError(f"nested array type {et}")
    raise ValueError(f"unknown value type {t}")


def main():
    loc = sys.argv[1]
    filters = [a.lower() for a in sys.argv[2:]]
    s = Src(loc)
    magic = s.read(4)
    if magic != b"GGUF":
        print(f"NOT A GGUF: magic={magic!r}")
        sys.exit(2)
    ver = s.u32()
    n_tensors = s.u64()
    n_kv = s.u64()
    print(f"gguf_version={ver} n_tensors={n_tensors} n_kv={n_kv}")
    print("-" * 70)
    for _ in range(n_kv):
        key = s.read(s.u64()).decode("utf-8", "replace")
        t = s.u32()
        val = read_val(s, t)
        if filters and not any(f in key.lower() for f in filters):
            continue
        sval = str(val)
        if len(sval) > 300:
            sval = sval[:300] + f"... [{len(sval)} chars]"
        print(f"{key} = {sval}")
    print("-" * 70)
    print(f"header bytes read: {s.pos/1e6:.2f} MB")


if __name__ == "__main__":
    main()
