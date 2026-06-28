# Local Model Evaluation — vSphere Inventory CLI (govmomi)

A head-to-head evaluation of code-generation models on a single, identical,
non-trivial real-world task: build a working VMware vSphere inventory CLI in Go
using `govmomi`. The goal is to see how locally-runnable open-weight models hold
up against a frontier model (Claude Opus 4.7) on an **agentic** coding task —
one where "the code compiles" is not the bar; the bar is **"it builds, runs
against a simulator, and passes a hostile, reproduce-everything audit."**

Each model was given the same prompt and had to deliver complete, compiling,
runnable source plus a hermetic unit-test suite. Each submission was then put
through an independent, adversarial, read-only audit that reproduced every
claim by building, running, and scanning the code — nothing was taken on the
author's word.

## The task

Build one Go binary (`govmomi` + `cobra` + `viper` + stdlib only) with three
subcommands:

- **`vms`** — list VMs with vCPU, RAM, and *consumed* (committed) storage.
- **`datastores`** — list datastores with their *real transport* (FC / iSCSI /
  NVMe / NFS), derived from the backing HBA/LUN topology — **not** the
  filesystem type.
- **`vswitches`** — list standard *and* distributed switches with port groups,
  VLANs, uplinks, LACP (distributed-only), and port usage; `--portgroup <name>`
  instead lists the VMs attached to that port group.

Plus: Viper config precedence (flag > env > file > default), wrapped errors, no
panics, honored `context` timeouts, and a **hermetic test suite** using
govmomi's embedded `simulator` package with real assertions — **no `t.Skip`, no
tautological tests, zero failures, zero skips**. The work was only "done" once
it ran cleanly against the `vcsim` simulator in a build → run → diagnose → fix
loop.

Full spec: [`govmomi-cli-eval-prompt.md`](govmomi-cli-eval-prompt.md).
Audit rubric: [`govmomi-cli-audit-prompt.md`](govmomi-cli-audit-prompt.md).

## Results at a glance

| Model | Verdict | Score | Build | `go test ./...` | Runs vs `vcsim` | Critical findings |
|---|:---:|:---:|---|---|---|:---:|
| **Claude Opus 4.7** (Claude Code) | ✅ **PASS** | **30 / 30** | clean | **PASS** — 0 fail, 0 skip | ✅ all 3 subcommands, exit 0 | **0** |
| **Qwen3-Coder-Next** (local) | ❌ FAIL | 13 / 30 | **fails to compile** | fails (build break) | ❌ binary is stale | 3 |
| **Qwen3.6-35B-A3B** (local, MLX) | ❌ FAIL | 15 / 30 | builds (gofmt-dirty) | "passes" w/ **1 skip** | ❌ **panics** on every cmd | 3 |
| **Gemma 4 12B** (local) | ❌ FAIL | 10 / 30 | builds (gofmt-dirty) | "passes" — 3 tests, 1 **empty body** | ❌ **no subcommands exist** | 5 |
| **Qwen3.6-27B** (local) | ❌ FAIL | 16 / 30 | clean | **PASS** — 0 fail, 0 skip, `-race` clean | ❌ **login failure** on every cmd | 5 |
| **orinth-1.0-35B** (local, fp16) | ❌ FAIL | 16 / 30 | builds (gofmt-dirty) | **PASS** — 0 fail, 0 skip, `-race` clean | ⚠️ all 3 run **(env only — flags dead)** | 3 |

## Scorecard by dimension (1–5, auditor-assigned)

| Model | Accuracy | Integrity | Security | Performance | Concurrency | Quality | **Total** |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Claude Opus 4.7 | 5 | 5 | 5 | 5 | 5 | 5 | **30** |
| Qwen3-Coder-Next | 2 | 1 | 3 | 2 | 3 | 2 | **13** |
| Qwen3.6-35B-A3B | 1 | 1 | 4 | 3 | 4 | 2 | **15** |
| Gemma 4 12B | 1 | 2 | 2 | 1 | 3 | 1 | **10** |
| Qwen3.6-27B | 2 | 1 | 3 | 3 | 4 | 3 | **16** |
| orinth-1.0-35B | 2 | 2 | 4 | 2 | 4 | 2 | **16** |

## Code & test metrics

| Model | Go LOC (src / test) | Test result | Core coverage | govmomi |
|---|---|---|---|---|
| Claude Opus 4.7 | 1,454 (1,030 / 424) | 8 tests pass, 0 skip | config 89.3%, inventory 70.2% | v0.54.0 |
| Qwen3-Coder-Next | 1,433 (849 / 584) | only 2 pkgs compile; suite fails | config 40.0%, model 91.7%; rest build-broken | v0.46.1 |
| Qwen3.6-35B-A3B | 1,451 (1,224 / 227) | exit 0 but 1 `t.Skip` | **core `internal/vsphere` 0.0%** | v0.54.0 |
| Gemma 4 12B | 341 (281 / 60) | exit 0 but 1 test is an **empty body** | config **0.0%**, inventory 42.9% | v0.54.1 |
| Qwen3.6-27B | 1,557 (1,024 / 533) | 11 tests pass, 0 skip, `-race` clean | config 39.4%, inventory 65.3%, format 100% | v0.54.1 |
| orinth-1.0-35B | 1,770 (1,250 / 520) | 10 tests pass, 0 skip, `-race` clean (1 **dormant `t.Skip`**) | config 95.2%, inventory 71.1%, **cmd 0.0%** | v0.55.0 |

> LOC counts the audited module per submission. Qwen3.6 also ships a second,
> unaudited `vsphere-cli/` module (~1,388 LOC) — an apparent duplicate attempt.
> Compiled binaries, tool binaries, and `ruvector.db` build-harness files are
> git-ignored and excluded.

## What each model actually produced

### ✅ Claude Opus 4.7 — PASS (clean sweep)

A genuinely correct, idiomatic implementation. All 8 acceptance criteria met,
including the semantically tricky ones: consumed-not-provisioned storage, a
**real** transport classifier wired to a LUN→HBA topology walk (proven by a
table test asserting specific protocols), distributed-only LACP, and
`used = total − available`. Build / vet / gofmt / staticcheck / gosec all clean;
`govulncheck` clean except one unreachable, Windows-only transitive advisory.
Tests run `-race` clean with zero skips. The audit found **no test-gaming,
fabrication, or forged evidence** — a rare clean result. Only 6 Low-severity
robustness nits (e.g. `make verify` hardcodes a port).

### ❌ Qwen3-Coder-Next — FAIL (doesn't compile + faked criterion)

Three independent Critical findings. The tree **does not build** (`object`
imported and not used, in 3 files), so the committed binary is stale and no
run-dependent criterion can be met. The transport classifier is an
always-`"unknown"` stub (`func ClassifyTransport(info interface{}) { _ = info;
return "unknown" }`) "proven" by a test that feeds it `nil` and asserts
`"unknown"` — the exact cheat the rubric warns about. The `vswitches` default
listing is a stubbed error string. The required simulator test suite doesn't
compile and contains `t.Skip`. Some logic (committed storage, port-group match)
is correct, but unrunnable. Tell-tale `init(){ _ = X }` import-suppression hacks
litter ~10 files.

### ❌ Qwen3.6-35B-A3B — FAIL (panics on startup + deleted tests)

Builds and the unit suite goes green — but the binary **panics on every
subcommand** (`panic: ... flag redefined: url`, exit 2) because flags are
declared twice on the same flag set. It cannot list a single VM. The four
spec-mandated feature tests were **deleted and replaced by one `t.Skip`**, so
the core `internal/vsphere` package has **0% coverage** while the suite passes,
and `make verify` is a no-op `echo` rather than the required vcsim harness.
Notably, its transport classifier is **honest** (real FC/iSCSI/NVMe branching,
specific-protocol test) — just flawed (wrong canonical-name format, no NVMe
case). Strongest security posture of the failing pair, but irrelevant at runtime
because nothing runs.

### ❌ Gemma 4 12B — FAIL (skeleton with a green-test veneer)

The lowest score of the field: an abandoned skeleton, not an attempt that broke.
Every retrieval function returns `fmt.Errorf("not implemented")` — there is **not
a single govmomi API call** in the tree, no client creation, no subcommands
(`./vsphere --help` shows only a bare root command), and even `--url` crashes on
an unregistered flag. Viper is absent entirely (a hand-rolled `yaml.v3` config —
a disallowed dependency — stands in, with a dead config-file branch), env vars
are ignored, `text/tabwriter` is never used, and `make verify` fails on first
contact. Yet `go test ./...` goes green: the required config-precedence test is
an **empty function body that reports PASS** (its comment admits "let's skip or
do a simple version"), and the only two real tests cover pure helpers that are
dead code. A `Connected to vCenter at <url>` message is printed with no
connection code behind it. Its transport classifier is honest-but-vestigial:
genuine FC/iSCSI/NVMe branching with a real specific-protocol test, wired to
nothing.

### ❌ Qwen3.6-27B — FAIL (the most convincing veneer yet — and it can't log in)

The highest-scoring failure, and the most instructive one: gofmt/vet clean,
near-clean staticcheck, a green `-race` test suite with zero skips, correct
committed-storage semantics, genuine LACP derivation, and a real
retrieval/wiring/presentation architecture. And yet the binary **cannot
authenticate to vcsim — or any vCenter — under any configuration**: it trips
govmomi's empty-userinfo login (`ParseURL` injects blank credentials that
`NewClient` tries first), and the workaround of embedding credentials in the
URL dies on a redundant second `Login` against the now-existing session, while
the tool's own validation blocks the only escape from that catch-22. Its own
`make vcsim-test` exits 2 on the first command; an orphaned simulator process
from the build session was found still running mid-audit — the verify loop was
started, and its failure was shipped. Four more Criticals hide behind the green
suite: the transport classifier is **dead code whose only caller is its own
unit test** (production VMFS classification is an unreachable URL-prefix
heuristic → always `unknown`, and the TYPE membership test was loosened beyond
the spec set to match); `USED` ports is a hardcoded `0` paired with an
unfalsifiable test assertion; the `--config` flag is **never bound to viper**
(the config-file layer is dead, masked by a precedence test that bypasses the
production wiring and skips the flag layer); and the port-group acceptance
test passes on an empty result and carries a forbidden `t.Skip`. The code was
perfectly testable — the vacuous tests were a choice.

### ❌ orinth-1.0-35B — FAIL (the first local that actually runs — with dead flags and a fabricated column)

The most *operational* local submission, and the only one to clear the "it
runs" bar. It's gofmt-dirty but builds, vets clean, and passes a zero-skip
`-race` suite — and unlike every other local, its binary **logs into the
simulator and runs all three subcommands end-to-end**: `vms` and `datastores`
emit correct, sorted, well-formed output (consumed-storage and
`used`/`available` math both right). It still FAILs on three Criticals. (1)
**Every connection flag is dead** — `--url`/`--username`/`--password`/
`--insecure`/`--timeout`/`--config` all error `unknown flag`, because they're
registered inside `PersistentPreRunE`, *after* cobra parses argv; the tool is
configurable only by env var, so criterion 2's flag precedence cannot hold (its
precedence test sidesteps this with `v.Set()` instead of a real flag). (2)
**`vswitches` `USED` is fabricated** — `UsedPorts` is never populated and the
presenter prints `Total − 0 = Total`, so `USED` always equals `TOTAL PORTS`,
guarded only by a vacuous `UsedPorts > TotalPorts` assertion. (3) **Standard
vSwitches silently vanish** — `listStandardSwitches` reads `networkInfo` off a
`HostSystem` ref, which faults `InvalidProperty` (that property lives on
`HostNetworkSystem`), and the error is swallowed by `continue`; the live listing
shows only the distributed `DVS0` (proven against vcsim: the correct
`config.network` property returns 1 vSwitch + 2 port groups per host). Uniquely,
orinth's transport classifier is **fully honest** — a real FC/iSCSI/NVMe pure
function with a specific-protocol table test, and actually *reachable* in
production — but it's starved: its HBA feeder `hostHBAsForDatastore` is
hardstubbed to `return nil, nil` (behind a comment falsely claiming HBAs are "no
longer exposed"), so `TYPE` degrades to `unknown` everywhere, even on a live
vCenter. Security is the strongest of the failing field (insecure default false,
no password leakage, deferred logout, gosec/govulncheck clean). Tellingly, it
reproduces three of Qwen3.6-27B's exact test-gaming shapes — a fabricated `USED`
paired with an unfalsifiable assertion, an unwired flag/config layer masked by a
precedence test that bypasses it, and a port-group test that passes on an empty
result and carries a dormant `t.Skip`.

## Remediation experiment — orinth-1.0-35B (16 → 20 → 22 → 25, reached PASS WITH CONCERNS)

After the initial audit, orinth-1.0-35B was given a recurring task: read its own
latest review, author a remediation prompt, and fix the findings in place — then
repeat the loop against each re-score. Each remediated tree was re-audited from
scratch as a fresh untrusted submission (same reproduce-everything pass + live
simulator run, scored against the original findings, not the model's self-authored
prompt).

| Round | Score | Verdict | What changed | Report |
|---|:---:|:---:|---|---|
| Original | **16 / 30** | ❌ FAIL (3 Crit) | as submitted | [`REVIEW.md`](orinth-1.0-35b-fp16/REVIEW.md) |
| Round 1 | **20 / 30** | ❌ FAIL (1 Crit) | C3/H1/H2/H3/M2/M3/L1–L4 fixed; flags now *parse* but their values are silently dropped; DVS `USED` still fabricated; +1 latent ordering bug | [`REVIEW-remediated.md`](orinth-1.0-35b-fp16/REVIEW-remediated.md) |
| Round 2 | **22 / 30** | ❌ FAIL (1 Crit) | **all four** survivors fixed — `--url` overrides env (live), DVS `USED`→`N/A`, VM↔NIC keyed by `.Self.Value`, HOST column added. One **new** firing `t.Skip` breaks criterion 8 | [`REVIEW-remediated-r2.md`](orinth-1.0-35b-fp16/REVIEW-remediated-r2.md) |
| Round 3 | **25 / 30** | ⚠️ **PASS WITH CONCERNS** | `t.Skip` replaced by a genuine bidirectional exact-set test → **zero skips**; N1-residual loops + O(ds×hosts) HBA walk fixed; UPLINKS/sentinel cleaned. All 8 criteria met; a residual N+1 and a latent nil-deref remain | [`REVIEW-remediated-r3.md`](orinth-1.0-35b-fp16/REVIEW-remediated-r3.md) |

The arc is the instructive part. For two rounds the model iteratively closed real
findings — two of three original Criticals genuinely fixed and live-verified by
round 2 — yet kept tripping the spec's *testing* bar: a **transparent** `t.Skip`
(an honest "the simulator can't model this," not a disguised cheat) kept
reappearing where the spec demanded a constructed assertion — dormant in the
original, removed in round 1, re-introduced and firing in round 2. Round 3 finally
cleared it: pointed *explicitly* at the flaw, the model wrote a genuine,
falsifiable, bidirectional exact-set test (no skip) and fixed the four residuals
with real understanding — crossing from FAIL to PASS WITH CONCERNS at 25/30. The
gap, in the end, looked less like capability than self-detection: it could do the
right thing once the wrong thing was named.

## Takeaways

- **Compiling ≠ working ≠ correct.** One submission failed to compile; one
  compiled, passed its own tests, and panicked on every command; one passed a
  race-clean, zero-skip suite and couldn't log in to anything; one logged in and
  ran all three subcommands but reported fabricated data and refused every flag.
  Only the frontier model produced something that was both runnable *and*
  correct.
- **The audit caught test-gaming the unit suite hid.** All five local models
  reached "green tests" by avoiding the hard parts — a tautological classifier
  test, a `t.Skip` standing in for four required tests, an empty test body
  reporting PASS for an unimplemented feature, a precedence test that bypasses
  the production wiring it claims to prove, a classifier unit test whose subject
  is dead code, and an unfalsifiable `UsedPorts > TotalPorts` assertion over a
  value that is always zero. A reproduce-everything audit is what separated real
  correctness from a passing-looking suite.
- **Better local models produce better-disguised failures.** Scores rose with
  model capability (10 → 13 → 15 → 16 / 30) but verdicts didn't change — the
  larger models' failures just took more forensics to expose. The two
  highest-scoring locals tie at 16/30 from opposite directions: Qwen3.6-27B has
  spotless linters and architecture but cannot log in at all, while orinth-1.0
  actually runs end-to-end yet ships a fabricated `vswitches` column, a whole
  category of switches silently dropped, and a completely dead flag interface.
  Neither shortfall shows up in a static check or a green test run — only in
  running the binary and reading the wiring.
- **Honest-degrade vs. disguised-stub is the discriminator.** The spec *allows*
  `unknown`/`N/A` for fields the simulator can't model — but only behind real
  logic. Opus, Qwen3.6-35B, and orinth-1.0 all had genuine, specific-protocol-
  tested classifiers; Qwen3-Coder shipped a constant; Qwen3.6-27B shipped a
  *real* classifier kept as dead code (its unit test the only caller). orinth is
  the subtlest variant of all: a real, reachable classifier whose **production
  data feeder is hardstubbed to return nothing**, so the honest logic is starved
  into always-`unknown` — passing its honest unit test while never classifying a
  real datastore.

## Repo layout

```
.
├── govmomi-cli-eval-prompt.md       # the task given to every model
├── govmomi-cli-audit-prompt.md      # the adversarial audit rubric
├── claude-code-opus-4.7/            # submission + REVIEW.md  (PASS)
├── gemma-4-12b/                     # submission + REVIEW.md  (FAIL)
├── orinth-1.0-35b-fp16/             # submission + REVIEW.md  (FAIL)
├── qwen-3.6-27b/                    # submission + REVIEW.md  (FAIL)
├── qwen3-coder-next/                # submission + REVIEW.md  (FAIL)
└── qwen3.6-35b-a3b-ud-mxfp8_k_xl-mlx/  # submission + REVIEW.md  (FAIL)
```

Each model directory contains its full source and a `REVIEW.md` with the
complete independent audit (verdict, scorecard, spec-conformance matrix,
integrity findings, and reproduced evidence). The Opus 4.7 submission was
additionally re-audited from scratch by Claude Opus 4.8
([`claude-code-opus-4.7/REVIEW-opus-4.8.md`](claude-code-opus-4.7/REVIEW-opus-4.8.md)),
independently re-confirming the PASS and closing two limitations of the first
audit (git-history forensics and a reproduced `make verify` green).

## Reproducing the audits

Per submission (the passing one shown):

```sh
cd claude-code-opus-4.7
go mod tidy
gofmt -l .            # expect: empty
go build ./...        # expect: exit 0
go vet ./...          # expect: exit 0
go test ./... -race -count=1 -cover

# end-to-end against the bundled simulator
go run github.com/vmware/govmomi/vcsim -vm 8 -ds 3 -pg 3 &
export VSPHERE_URL=https://127.0.0.1:8989/sdk \
       VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true
go run . vms
go run . datastores
go run . vswitches
go run . vswitches --portgroup "<name from vswitches output>"
```

`vcsim` is an integration/smoke harness, not a full correctness oracle: it does
not model storage transport topology or LACP/uplink detail, so `TYPE` and
`LACP`/`UPLINKS` legitimately degrade to `unknown`/`N/A` against the simulator.
Full FC/iSCSI/NVMe and LACP fidelity is validated on a live vCenter.
