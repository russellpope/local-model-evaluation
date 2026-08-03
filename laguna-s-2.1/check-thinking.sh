#!/usr/bin/env bash
# Confirm laguna-s-2.1 is actually reasoning on the API path opencode uses.
# Enable Thinking is not persisted to disk by LM Studio — it lives with the
# loaded instance — so re-run this after ANY model reload or LM Studio restart.
# Expect reasoning_tokens > 0. A 0 means the run would be scored thinking-off.
curl -s http://localhost:1234/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poolside/laguna-s-2.1",
    "max_tokens": 1500, "temperature": 0.6, "top_p": 0.95, "top_k": 20,
    "messages": [{"role":"user","content":"Why is one Properties() call per VM over 500 VMs O(n) round-trips in govmomi?"}]
  }' | python3 -c '
import json,sys
d = json.load(sys.stdin)
n = d["usage"].get("completion_tokens_details",{}).get("reasoning_tokens", 0)
print(f"reasoning_tokens: {n}")
print("THINKING ON" if n > 0 else "THINKING OFF  <-- do not start the run")
'
