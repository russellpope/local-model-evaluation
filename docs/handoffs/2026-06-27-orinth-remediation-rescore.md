# Handoff Reference — orinth-1.0-35b remediation re-score (2026-06-27)

Repo: `/Users/ldh/Projects/github.com/local-model-evaluation` (git root; each model
eval is a top-level dir). This doc lets a fresh session resume the **remediation
re-score** with zero prior context.

## Why (what's going on)

This repo is a head-to-head eval of code-gen models on one identical task: build a
govmomi vSphere inventory CLI in Go (`vms` / `datastores` / `vswitches`
subcommands). Each submission gets an adversarial, reproduce-everything audit
(rubric: `govmomi-cli-audit-prompt.md`; task spec: `govmomi-cli-eval-prompt.md`).

This session audited **orinth-1.0-35b-fp16** → **FAIL** (3 Critical, 3 High, 4
Medium, 5 Low). Full report committed at `orinth-1.0-35b-fp16/REVIEW.md`
(commit `86036fa`). It's the strongest *local* model (it's the only one whose
binary actually runs end-to-end) but still failed on flags, fabricated vswitches
USED, and silently-dropped standard switches.

**The experiment now in flight:** Russell had orinth *read its own REVIEW.md and
author a remediation prompt* (not ingest-and-act), and is running that prompt on
a **local branch `ornith-remediation-attempt`** (note the spelling: branch is
"ornith", the dir is "orinth"). When it finishes, **re-score the remediated
branch from scratch** as if it were a fresh untrusted submission.

## Current state (verify with git)

- Branch `ornith-remediation-attempt`, cut from `main` at **`86036fa`** — this is
  the **pre-remediation baseline**. As of this handoff, HEAD == baseline (no
  remediation committed yet). The remediation may or may not have landed when you
  resume — check `git -C <repo> log --oneline -5` on the branch.
- `main` holds the committed orinth submission + the README results table.
- Untracked: `eval-prompts-by-subcommand/` (4 per-subcommand eval prompts + index
  this session authored; never committed — leave them alone, do NOT let a
  `git add -A` sweep them into a remediation commit).
- `ruvector.db` (per-dir, ~1.5MB) is a build-harness artifact, gitignored.

## The task: re-score the remediation

1. See exactly what changed:
   ```
   git -C /Users/ldh/Projects/github.com/local-model-evaluation \
     diff 86036fa..ornith-remediation-attempt -- orinth-1.0-35b-fp16/govmomi-cli/
   ```
2. Re-run the FULL reproduce-everything pass on the branch tree (treat as fresh
   untrusted — new code can regress untouched parts). Recipe in the next section.
3. Write `orinth-1.0-35b-fp16/REVIEW-remediated.md` (same 11-section format as
   REVIEW.md) and a **before→after delta** row/annotation in the top-level
   `README.md`. Don't overwrite the original REVIEW.md (mirror the existing
   `claude-code-opus-4.7/REVIEW.md` + `REVIEW-opus-4.8.md` pattern).
4. **Ground truth = the original `REVIEW.md` findings + the eval spec.** The
   model's self-authored remediation prompt is ITS artifact, not the rubric — if
   its prompt narrowed a finding and the code therefore didn't fix it, that
   finding is still UNMET. Ignore any self-"re-score" the model produced.

### Probe these first (predicted likely-surviving failures)

The remediation prompt had high coverage but three substantive defects:

- **G1 — distributed USED likely still == TOTAL.** Its C2 fix says "leave
  AvailablePorts zero if unavailable" for distributed switches → `UsedPorts =
  Total − 0 = Total`, re-creating the original bug on the only rows vcsim shows
  (`DVS0`). It also references a non-existent `pg.Config.UsedPorts` field. **Check
  the distributed rows specifically**, not just the (newly-added) standard rows.
- **G2 — C1 flag fix likely ships test-unverified.** Its M1 fixes `switches_test`
  and `portgroup_test` but **omits** fixing `config_test.go`'s precedence test,
  which uses `v.Set()` instead of a real cobra flag. So even a correct flags-in-
  `init()` fix may have no test that would catch a C1 regression. **Check whether
  `config_test` actually drives a `--url` flag through the command.**
- **G3 — H2 timeout proof is unfalsifiable.** Its verification uses
  `VSPHERE_TIMEOUT=1ns` which fails at *connect* (identical output whether or not
  retrieval is covered). Re-score H2 with a **deterministic cancelled-context
  test**: pass an already-cancelled `ctx` to `ListVMs`/etc. and assert it returns
  a context error.

Prediction to confirm/refute: vms, datastores (honest `unknown`), standard-switch
listing, and `--portgroup` likely genuinely fixed; distributed USED and the
precedence test likely still open → likely still short of a clean PASS.

## Reproduce-everything recipe

From `orinth-1.0-35b-fp16/govmomi-cli/` on the branch:
```
gofmt -l .                       # expect empty
go build ./...                   # exit 0
go vet ./...                     # exit 0
go test ./... -race -count=1 -cover
staticcheck ./... ; gosec -quiet ./... ; govulncheck ./...   # tools in $(go env GOPATH)/bin
```
Then drive the binary against a live simulator **with flags** (this is the C1
check — env-vars-only would mask broken flags):
```
go build -o /tmp/gcli .
/tmp/gcli vms --url <simurl> --username user --password pass --insecure
/tmp/gcli datastores --url ... ; /tmp/gcli vswitches --url ...   # vswitches: BOTH std+dist rows, USED != TOTAL
/tmp/gcli vswitches --portgroup DC0_DVPG0 --url ...
/tmp/gcli --help                 # all 6 flags must appear
```

### Standing up the simulator (IMPORTANT gotcha)

govmomi **v0.55.0 split the `vcsim` binary out of the module** — `go run
github.com/vmware/govmomi/vcsim` (and `@v0.55.0`) both fail with "does not contain
package …/vcsim". The hermetic unit tests use the embedded `simulator` package and
work fine via plain `go test`. For the manual live run, stand up a server with a
tiny throwaway module (this worked this session):

`/<scratch>/simhelper/go.mod`:
```
module simhelper
go 1.23
require github.com/vmware/govmomi v0.55.0
```
`/<scratch>/simhelper/main.go`:
```go
package main
import ("fmt";"os";"github.com/vmware/govmomi/simulator")
func main(){
  m:=simulator.VPX(); m.Machine=8; m.Datastore=3; m.Portgroup=3
  if err:=m.Create(); err!=nil { fmt.Fprintln(os.Stderr,err); os.Exit(1) }
  m.Service.TLS=nil
  s:=m.Service.NewServer()
  fmt.Println(s.URL.String())   // http://user:pass@127.0.0.1:PORT/sdk
  select{}
}
```
`go mod tidy && go run .` prints the URL; strip the `user:pass@` for `VSPHERE_URL`
(pass user/pass separately). Kill it when done — local models + a lingering sim
can OOM the laptop.

To prove the C3 root cause (standard switches), a probe that mirrors the audited
code's exact call confirmed it: `RetrieveOne(hostRef, ["networkInfo"], &mo.HostNetworkSystem)`
→ `InvalidProperty`; the correct `RetrieveOne(hostRef, ["config.network"], &mo.HostSystem)`
returns 1 vSwitch + 2 PGs/host. Re-use that style of probe if needed.

## Env gotchas (avoid rediscovering)

- **GateGuard fact-forcing hook** intercepts the **first `Bash`** and **every
  `Edit`/`Write`** (incl. each new file). It returns an error asking for facts;
  present the 4 facts in plain text, then **retry the identical call** and it
  passes. Budget for this on every file write. Disable only if it blocks setup:
  `ECC_GATEGUARD=off` or add `pre:bash:gateguard-fact-force` /
  `pre:edit-write:gateguard-fact-force` to `ECC_DISABLED_HOOKS`.
- **fish shell resets cwd** after a backgrounded command ("Shell cwd was reset
  to …"). Don't rely on a persisted `cd`; use `git -C <repo>` and absolute paths.
- Scratchpad for temp files this session:
  `/private/tmp/claude-501/-Users-ldh-Projects-github-com-local-model-evaluation-orinth-1-0-35b-fp16/55fe55d8-1b68-45f1-b68f-6cd7d594b29a/scratchpad`
  (a fresh session gets its own — just use any scratch dir, not `/tmp` directly).
- Toolchain: go1.26.4 darwin/arm64. staticcheck/gosec/govulncheck already
  installed in `$(go env GOPATH)/bin` (= `/Users/ldh/go/bin`).

## Decisions made this session (don't relitigate)

- Split eval prompts: **self-contained** per-subcommand specs (each repeats shared
  scaffolding), `vswitches` **split into listing + portgroup** (4 total), **reuse
  the existing shared audit rubric** scoped per run. Faithful-slice principle: no
  model-specific hints injected, so each stays a valid eval. Files live in
  `eval-prompts-by-subcommand/` (untracked; commit with explicit paths if at all).
- README results: orinth recorded as **16/30 FAIL**, tying Qwen3.6-27B; it's the
  first local that actually runs end-to-end.

## Working preferences

- **Cost is not a concern** (subscription billing) — don't pause for usage warnings.
- Auto-memory: `/Users/ldh/.claude/projects/-Users-ldh-Projects-github-com-local-model-evaluation/memory/`
  — see `MEMORY.md` index and `orinth-1-0-35b-verdict.md` (has the full verdict +
  this remediation plan). Per-model verdicts also live in Open Brain.
- Record durable verdicts/decisions to the file-memory verdict notes (per-model)
  and update `MEMORY.md`. Open Brain writeback lands as pending review.
