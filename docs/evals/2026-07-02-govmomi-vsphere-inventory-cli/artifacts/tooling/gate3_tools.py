#!/usr/bin/env python3
"""Gate 2/3: chat template accepted + a REAL two-turn tool round-trip.

/props advertises chat_format "Content-only" even when tool parsing works, so
only an actual round-trip is evidence. Turn 1 must produce a parsed tool_call;
turn 2 must consume the tool result and answer from it.
"""
import json
import urllib.request

URL = "http://127.0.0.1:1234/v1/chat/completions"

TOOLS = [{
    "type": "function",
    "function": {
        "name": "get_datastore_capacity",
        "description": "Return the capacity of a vSphere datastore in bytes.",
        "parameters": {
            "type": "object",
            "properties": {
                "datastore": {"type": "string", "description": "Datastore name"},
            },
            "required": ["datastore"],
        },
    },
}]


def post(payload):
    req = urllib.request.Request(
        URL, data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=600) as r:
        return json.load(r)


msgs = [{
    "role": "user",
    "content": "What is the capacity of datastore LocalDS_0? Use the tool, then state the value in GiB.",
}]

print("=== TURN 1 — expect a parsed tool_call ===")
r1 = post({"model": "qwen3.8-27b-bf16", "messages": msgs, "tools": TOOLS,
           "max_tokens": 2048})
m1 = r1["choices"][0]["message"]
tc = m1.get("tool_calls")
reasoning = m1.get("reasoning_content") or ""
print("finish_reason :", r1["choices"][0].get("finish_reason"))
print("tool_calls    :", json.dumps(tc, indent=2) if tc else "*** NONE — GATE 3 FAIL ***")
print(f"reasoning_content: {len(reasoning)} chars")
if reasoning:
    print("  first 300:", reasoning[:300].replace("\n", " "))
u = r1.get("usage", {})
print("usage         :", u)
t1 = r1.get("timings", {})
if t1:
    print(f"timings       : prompt {t1.get('prompt_per_second',0):.1f} t/s | "
          f"decode {t1.get('predicted_per_second',0):.1f} t/s")

if not tc:
    raise SystemExit("GATE 3 FAILED at turn 1 — no tool call parsed")

# --- turn 2: feed the tool result back -------------------------------------
call = tc[0]
args = json.loads(call["function"]["arguments"] or "{}")
print("\nparsed arguments:", args)

msgs.append({k: v for k, v in m1.items() if k in ("role", "content", "tool_calls", "reasoning_content")})
msgs.append({
    "role": "tool",
    "tool_call_id": call.get("id", "call_0"),
    "content": json.dumps({"datastore": args.get("datastore"), "capacity_bytes": 3221225472}),
})

print("\n=== TURN 2 — expect the tool result consumed ===")
r2 = post({"model": "qwen3.8-27b-bf16", "messages": msgs, "tools": TOOLS,
           "max_tokens": 2048})
m2 = r2["choices"][0]["message"]
content = m2.get("content") or ""
reasoning2 = m2.get("reasoning_content") or ""
print("finish_reason :", r2["choices"][0].get("finish_reason"))
print(f"reasoning_content: {len(reasoning2)} chars")
print("content       :", content[:600])
t2 = r2.get("timings", {})
if t2:
    print(f"timings       : prompt {t2.get('prompt_per_second',0):.1f} t/s | "
          f"decode {t2.get('predicted_per_second',0):.1f} t/s")

ok = "3" in content and ("GiB" in content or "GB" in content)
print("\nRESULT:", "GATE 2/3 PASS — round-trip closed" if ok
      else "*** turn 2 did not clearly use the tool value — inspect above ***")
