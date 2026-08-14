#!/usr/bin/env python3
"""Read-only run sampler — GET /slots every 30s to a CSV.

NEVER POSTs. A POST to a busy llama-server evicts the slot's cached context.

This is what distinguishes "the model stalled" from "the operator blocked it":
a long unbroken run of is_processing=False with no context growth is a stall
signature (permission dialog, dead session), while is_processing=False with a
jump in n_prompt_tokens between samples is just a completed turn.

Yields active-vs-wall time directly: active_s = 30 * (rows with is_processing).
"""
import csv
import json
import sys
import time
import urllib.request

URL = "http://127.0.0.1:1234/slots"
OUT = sys.argv[1] if len(sys.argv) > 1 else "sampler.csv"
INTERVAL = 30

FIELDS = [
    "ts", "elapsed_s", "is_processing", "n_ctx", "n_prompt_tokens",
    "n_prompt_tokens_cache", "n_prompt_tokens_processed", "n_decoded",
    "speculative", "err",
]

start = time.time()
with open(OUT, "a", newline="", buffering=1) as fh:
    w = csv.DictWriter(fh, fieldnames=FIELDS)
    if fh.tell() == 0:
        w.writeheader()
    while True:
        row = {k: "" for k in FIELDS}
        row["ts"] = time.strftime("%Y-%m-%d %H:%M:%S")
        row["elapsed_s"] = int(time.time() - start)
        try:
            with urllib.request.urlopen(URL, timeout=20) as r:
                s = json.load(r)[0]
            row["is_processing"] = s.get("is_processing")
            row["n_ctx"] = s.get("n_ctx")
            row["n_prompt_tokens"] = s.get("n_prompt_tokens")
            row["n_prompt_tokens_cache"] = s.get("n_prompt_tokens_cache")
            row["n_prompt_tokens_processed"] = s.get("n_prompt_tokens_processed")
            nt = s.get("next_token") or {}
            if isinstance(nt, list):
                nt = nt[0] if nt else {}
            row["n_decoded"] = nt.get("n_decoded")
            row["speculative"] = s.get("speculative")
        except Exception as e:  # server down, restarting, or busy-timeout
            row["err"] = type(e).__name__
        w.writerow(row)
        time.sleep(INTERVAL)
