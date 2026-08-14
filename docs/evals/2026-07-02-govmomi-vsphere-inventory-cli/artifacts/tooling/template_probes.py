#!/usr/bin/env python3
"""Two template probes, re-run for THIS model. Conclusions are template-specific
and do NOT carry over from KAT-Coder.

PROBE A — preserve semantics: does preserving reasoning add anything WITHIN a
turn, or only across completed user turns? Tool responses render as pseudo-user
turns on some templates, which is what made the flag a no-op on KAT.

PROBE B — prompt-cache stability: with preserve OFF, does a new user message
strip prior thinking out of the MIDDLE of the prompt? If so the cached prefix
stops matching and every operator follow-up costs a full context reprocess.

Read-only: /apply-template and /tokenize only. No generation, no POST to a busy
slot's completion endpoint.
"""
import json
import urllib.request

BASE = "http://127.0.0.1:1234"


def post(path, payload):
    req = urllib.request.Request(
        BASE + path, data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=120) as r:
        return json.load(r)


def render(messages, preserve):
    body = {"messages": messages}
    if preserve is not None:
        body["chat_template_kwargs"] = {"preserve_thinking": preserve}
    return post("/apply-template", body)["prompt"]


def toks(prompt):
    return post("/tokenize", {"content": prompt})["tokens"]


def lcp(a, b):
    n = 0
    for x, y in zip(a, b):
        if x != y:
            break
        n += 1
    return n


# A tool-call sequence inside ONE user turn, then a second user turn.
BASE_MSGS = [
    {"role": "system", "content": "You are a Go engineer."},
    {"role": "user", "content": "List the datastores."},
    {"role": "assistant", "reasoning_content": "REASON_ONE: I should call the list tool first.",
     "content": "", "tool_calls": [{"id": "c1", "type": "function",
      "function": {"name": "list_datastores", "arguments": "{}"}}]},
    {"role": "tool", "tool_call_id": "c1", "content": "[\"LocalDS_0\"]"},
    {"role": "assistant", "reasoning_content": "REASON_TWO: now fetch capacity.",
     "content": "", "tool_calls": [{"id": "c2", "type": "function",
      "function": {"name": "get_capacity", "arguments": "{\"ds\":\"LocalDS_0\"}"}}]},
    {"role": "tool", "tool_call_id": "c2", "content": "{\"bytes\":3221225472}"},
    {"role": "assistant", "reasoning_content": "REASON_THREE: convert and answer.",
     "content": "LocalDS_0 is 3 GiB."},
]
FOLLOWUP = {"role": "user", "content": "Now show the hosts."}

print("=" * 72)
print("PROBE A — how many reasoning blocks survive rendering")
print("=" * 72)
for label, preserve in (("preserve OFF", False), ("preserve ON", True), ("template default", None)):
    p = render(BASE_MSGS, preserve)
    kept = [n for n in ("REASON_ONE", "REASON_TWO", "REASON_THREE") if n in p]
    print(f"{label:18s} think-blocks={p.count('<think>'):2d}  "
          f"tokens={len(toks(p)):5d}  kept={kept}")

print()
print("=" * 72)
print("PROBE B — prompt-cache stability across a new user turn")
print("=" * 72)
for label, preserve in (("preserve OFF", False), ("preserve ON", True)):
    before = toks(render(BASE_MSGS, preserve))
    after = toks(render(BASE_MSGS + [FOLLOWUP], preserve))
    shared = lcp(before, after)
    pct = 100.0 * shared / len(before) if before else 0.0
    verdict = "append-only" if pct > 99 else "PREFIX BROKEN — full reprocess"
    print(f"{label:14s} before={len(before):5d} after={len(after):5d} "
          f"shared_prefix={shared:5d} ({pct:5.1f}% of before)  -> {verdict}")
