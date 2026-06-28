# Handoff Reference — orinth-1.0-35b remediation **round 2** re-score (2026-06-28)

Repo: `/Users/ldh/Projects/github.com/local-model-evaluation` (git root; each model
eval is a top-level dir). This resumes the **round-2 remediation re-score** with
zero prior context. Builds on the round-1 handoff:
[`2026-06-27-orinth-remediation-rescore.md`](2026-06-27-orinth-remediation-rescore.md)
— read it for the full audit recipe, simulator-helper source, and env gotchas
(not all repeated here).

## The story so far (score progression)

A head-to-head eval of code-gen models on one govmomi vSphere inventory CLI task.
orinth-1.0-35b (strongest *local* model) has been through:

- **Original audit → FAIL, 16/30** — `orinth-1.0-35b-fp16/REVIEW.md` (committed `86036fa`).
- **Round-1 remediation → FAIL (improved), 20/30** — `orinth-1.0-35b-fp16/REVIEW-remediated.md`.
  Russell had the model author its own remediation prompt from REVIEW.md, then ran it.
- **Round-2 remediation → THIS re-score.** Same pattern: the model authored a
  round-2 prompt from REVIEW-remediated.md and ran it on the `remediate-round2` branch.

Your job: re-audit the round-2 state from scratch (fresh untrusted submission),
score it, and write `orinth-1.0-35b-fp16/REVIEW-remediated-r2.md` tracking the
16 → 20 → ?? progression. Ground truth = the **original `REVIEW.md` findings +
the eval spec** (`govmomi-cli-eval-prompt.md`), NOT the model's self-authored prompts.

## ⚠️ FIRST: sort out the git baseline (round-1 was never committed)

As of this handoff: branch `remediate-round2`, HEAD = `86036fa`, and the **round-1
remediation is uncommitted in the working tree** (it rode across the branch switch).
So once round-2 lands on top, the working tree = original + round-1 + round-2,
all mingled.

- **For scoring, that's fine** — score the *current state* vs the spec/original.
  Cumulative diff: `git -C <repo> diff 86036fa -- orinth-1.0-35b-fp16/govmomi-cli/`.
- **To isolate what round-2 changed**, you need a round-1 checkpoint commit. If one
  exists (check `git log --oneline -5` on the branch), diff round2 vs it. If round-1
  was never committed, you can only see cumulative — note that limitation; don't
  fabricate a round-1-vs-round-2 delta you can't compute.
- Recommended (if not already done): commit round-1 as a checkpoint *before* round-2,
  explicit paths only (never `git add -A` — it'd sweep `docs/` + `eval-prompts-by-subcommand/`):
  `git add orinth-1.0-35b-fp16/govmomi-cli/ orinth-1.0-35b-fp16/REVIEW-remediated.md && git commit -m "round-1 remediation checkpoint (16->20)"`

## What round 2 MUST fix (the round-1 survivors — probe these first)

From `REVIEW-remediated.md`. Verify each against the round-2 code + a live sim run:

- **C1 (Critical) — flags still silently dropped.** Round 1 moved registration to
  `init()` so flags *parse* and show in `--help`, but `bindFlags` (`cmd/root.go:~74`)
  looks up `cmd.PersistentFlags().Lookup(...)` on the **subcommand** (empty set) → the
  binding no-ops → env/default win, `--url` silently ignored. **Live test:** env=`http://wrong:1/sdk`
  + `--url <good>` → must connect to `<good>`, not `wrong:1`. Fix is `cmd.Flags()` /
  `rootCmd.PersistentFlags()`. Also: the precedence unit test (`config_test.go`
  `TestBindPFlagEndToEnd`) binds its OWN flagset, so it passes while the real wiring is
  broken — round 2's test must drive the actual command, or it's still green-masking.
- **C2-distributed (High) — distributed USED still == TOTAL.** Standard switches got
  real used-ports in round 1 (live `vSwitch0` = 1536/6 ✓), but distributed leaves
  `AvailablePorts` unset → `Used = Total − 0 = Total` (live `DVS0` = 1/1 ✗), and
  `switches_test.go` exempts distributed from the strict check. **Live test:** the `DVS0`
  rows in `vswitches` output — USED must not equal TOTAL (or be `N/A`).
- **N1 (High, latent) — positional batch association.** `ListVMsByPortGroup`
  (`vms.go:~135-173`) keys results by input-ref *index* (`vmRefs[i].Value`), but
  `pc.Retrieve` doesn't guarantee return order → mis-associates VM↔NIC on a real
  vCenter. Sim-masked. Fix = key by each returned object's `.Self.Value`. Code-read,
  not reproducible on vcsim.
- **N2 (Medium) — duplicate standard-switch rows.** Each host's identical
  `vSwitch0/Management Network` + `VM Network` emit as separate rows with no HOST
  column (live: 4 identical rows on a 4-host VPX). Fix = HOST column or dedup.
- **M1-residual / Low** — port-group test should assert the exact VM set + cover a
  standard PG; replace the `context.TODO()` in `dsInfoFromMo`/`hostHBAsForDatastore`
  (`datastores.go:~83`); the HBA walk is O(ds×hosts).

## What round 1 already FIXED — must NOT regress (treat as fresh, re-verify)

C3 (standard switches list via `config.network`), H1 (real HBA discovery), H2
(timeout threaded through retrieval), H3 (`pc.Retrieve` batching), M2 (staticcheck
clean), M3 (Makefile + README), L1–L4 (gofmt clean, dup formatters gone). New code
can regress these — re-run the full pass, don't assume.

## Audit recipe (condensed — full detail in the round-1 doc)

From `orinth-1.0-35b-fp16/govmomi-cli/` on the branch:
```
gofmt -l .        # empty
go build ./...    # 0
go vet ./...      # 0
go test ./... -race -count=1 -cover
staticcheck ./... ; gosec -quiet ./... ; govulncheck ./...   # tools in /Users/ldh/go/bin
```
Then build + drive against the embedded simulator **with flags** (the C1 test):
- Stand up the sim via the throwaway `simhelper` module (govmomi v0.55 dropped the
  `vcsim` binary — `go run …/vcsim` fails; use `simulator.Model.Service.NewServer()`).
  Full source in the round-1 handoff doc. It prints `http://user:pass@127.0.0.1:PORT/sdk`;
  strip `user:pass@` for `VSPHERE_URL`.
- `gcli vms --url <u> --username user --password pass --insecure` etc.
- `gcli --help` (all 6 flags), `vswitches` (BOTH std+dist; check DVS USED≠TOTAL),
  `vswitches --portgroup DC0_DVPG0` (→16 VMs).
- C1 probe: `VSPHERE_URL=http://wrong:1/sdk gcli vms --url <good> …` → must hit `<good>`.
- **Kill the sim when done** (`kill $(cat sim.pid)`); a stray sim + local models can OOM.

## Deliverables for this round

- `orinth-1.0-35b-fp16/REVIEW-remediated-r2.md` — same finding-by-finding format,
  with a 16 → 20 → N scorecard column. Preserve `REVIEW.md` + `REVIEW-remediated.md`.
- A before→after row/annotation in the top-level `README.md` (orinth progression).
- Update the file-memory verdict note + offer Open Brain writeback.
- Do NOT commit unless Russell asks; if asked, explicit paths only.

## Env gotchas (the expensive-to-rediscover ones)

- **GateGuard** fact-forcing hook gates the **first Bash** and **every Edit/Write**
  (incl. each new file): present the 4 facts in plain text, then **retry the identical
  call**. Budget for it on every write. Disable only if it blocks: `ECC_GATEGUARD=off`.
- **fish resets cwd** after a backgrounded command — use `git -C <repo>` + absolute paths.
- govmomi **v0.55 has no vcsim binary** — use the embedded `simulator` helper (round-1 doc).
- Toolchain: go1.26.4 darwin/arm64; staticcheck/gosec/govulncheck in `/Users/ldh/go/bin`.

## LM Studio / opencode harness state (configured this session — for context, not action)

The serving stack is now tuned (not part of the audit, but explains the eval setup):
- LM Studio: Ornith-1.0-35B **bf16** GGUF, n_ctx 262144 (256K), n_parallel=1,
  Flash Attention on, prompt cache on. Inference: temp 0.6 / top_p 0.95 / top_k 20,
  repeat-penalty 1.0. Reasoning (`<think>`) verified working + parsed.
- opencode (`~/.config/opencode/opencode.json`): `provider.lmstudio.models.ornith-1.0-35b`
  now sets `options{temperature:0.6, top_p:0.95, top_k:20}` + `limit{context:262144,
  output:65536}`. Verified in the LM Studio request log: opencode sends those params
  and reasoning lands in `reasoning_content` (no `<think>` leakage). Backup at
  `opencode.json.bak-2026-06-27`.

## After the audit: instrumentation exploration (Russell wants this next)

Once the round-2 re-score is delivered, the next thing to explore is **live
memory/KV instrumentation** of the local LLM serving stack. Full details in the
auto-memory note `local-llm-memory-instrumentation.md`. The seed:
- LM Studio's model backend is a `node` process (~70 GB RSS):
  `ps -axo pid,rss,comm | grep node | sort -k2 -n | tail -1`.
- `vmmap -summary <pid>` already showed: weights = `mapped file` (~64.6 GB, mmap'd
  GGUF, clean), KV+compute = `MALLOC_LARGE` (grows with context), `IOAccelerator`
  ≈ 8 MB (UMA no-copy — Metal wraps host pages, no separate VRAM).
- **Live demo to run:** `vmmap -summary <pid>` before → load a big context (opencode
  to ~70K tokens) → `vmmap -summary <pid>` after; watch `MALLOC_LARGE` resident
  climb (KV cache) while `mapped file` stays flat.
- Tools: `mactop` (installed), `footprint`, Instruments Metal trace; `llama-server
  --metrics` → Prometheus/Grafana if moving off LM Studio.
- Possible build: a prompt-cache **churn monitor** (per-turn prefill tokens +
  cache hit/miss, correlated with subagent handoffs) — parse `~/.lmstudio/server-logs/`;
  prototype in Python first, Rust+ratatui only if it sticks.

## Working preferences

- **Cost is not a concern** (subscription) — don't pause for usage warnings.
- Auto-memory: `/Users/ldh/.claude/projects/-Users-ldh-Projects-github-com-local-model-evaluation/memory/`
  (`MEMORY.md` index + `orinth-1-0-35b-verdict.md` has the full 16→20 history + survivor list).
