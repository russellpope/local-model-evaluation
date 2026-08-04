#!/usr/bin/env bash
# Confirm laguna-s-2.1 is actually reasoning on the API path opencode uses.
# Re-run after ANY model reload, LM Studio restart, or server swap.
#
# Works against BOTH backends, because they report reasoning differently:
#   - LM Studio populates usage.completion_tokens_details.reasoning_tokens.
#   - llama-server (--jinja --reasoning-preserve) omits that field entirely
#     and returns the reasoning in choices[0].message.reasoning_content.
# Reading only reasoning_tokens gives a FALSE "THINKING OFF" on llama.cpp —
# the same accounting artifact that made opencode's tokens_reasoning read ~0.
# Verdict is therefore ON if EITHER signal is present.
curl -s http://localhost:1234/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poolside/laguna-s-2.1",
    "max_tokens": 1500, "temperature": 0.6, "top_p": 0.95, "top_k": 20,
    "messages": [{"role":"user","content":"Why is one Properties() call per VM over 500 VMs O(n) round-trips in govmomi?"}]
  }' | python3 -c '
import json,sys
d = json.load(sys.stdin)
if "error" in d:
    print("ERROR:", d["error"]); print("THINKING UNKNOWN  <-- server not answering"); sys.exit(1)
ch = d["choices"][0]
n  = (d.get("usage",{}).get("completion_tokens_details") or {}).get("reasoning_tokens") or 0
rc = ch.get("message",{}).get("reasoning_content") or ""
print(f"reasoning_tokens: {n}   (absent on llama-server — not a failure signal)")
print(f"reasoning_content chars: {len(rc)}")
print("THINKING ON" if (n > 0 or len(rc) > 0) else "THINKING OFF  <-- do not start the run")
'
