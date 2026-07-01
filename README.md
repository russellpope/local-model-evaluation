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
| **Gemma 4 31B** (local) | ❌ FAIL | 16 / 30 | clean | **PASS** — 5 tests, 0 skip (precedence **vacuous**) | ❌ **`vswitches` crashes** (2 of 3 run) | 3 |

## Scorecard by dimension (1–5, auditor-assigned)

| Model | Accuracy | Integrity | Security | Performance | Concurrency | Quality | **Total** |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Claude Opus 4.7 | 5 | 5 | 5 | 5 | 5 | 5 | **30** |
| Qwen3-Coder-Next | 2 | 1 | 3 | 2 | 3 | 2 | **13** |
| Qwen3.6-35B-A3B | 1 | 1 | 4 | 3 | 4 | 2 | **15** |
| Gemma 4 12B | 1 | 2 | 2 | 1 | 3 | 1 | **10** |
| Qwen3.6-27B | 2 | 1 | 3 | 3 | 4 | 3 | **16** |
| orinth-1.0-35B | 2 | 2 | 4 | 2 | 4 | 2 | **16** |
| Gemma 4 31B | 2 | 1 | 3 | 4 | 4 | 2 | **16** |

## Code & test metrics

| Model | Go LOC (src / test) | Test result | Core coverage | govmomi |
|---|---|---|---|---|
| Claude Opus 4.7 | 1,454 (1,030 / 424) | 8 tests pass, 0 skip | config 89.3%, inventory 70.2% | v0.54.0 |
| Qwen3-Coder-Next | 1,433 (849 / 584) | only 2 pkgs compile; suite fails | config 40.0%, model 91.7%; rest build-broken | v0.46.1 |
| Qwen3.6-35B-A3B | 1,451 (1,224 / 227) | exit 0 but 1 `t.Skip` | **core `internal/vsphere` 0.0%** | v0.54.0 |
| Gemma 4 12B | 341 (281 / 60) | exit 0 but 1 test is an **empty body** | config **0.0%**, inventory 42.9% | v0.54.1 |
| Qwen3.6-27B | 1,557 (1,024 / 533) | 11 tests pass, 0 skip, `-race` clean | config 39.4%, inventory 65.3%, format 100% | v0.54.1 |
| orinth-1.0-35B | 1,770 (1,250 / 520) | 10 tests pass, 0 skip, `-race` clean (1 **dormant `t.Skip`**) | config 95.2%, inventory 71.1%, **cmd 0.0%** | v0.55.0 |
| Gemma 4 31B | 753 (621 / 132) | 5 tests pass, 0 skip (precedence **vacuous**, no vSwitch/portgroup test) | config 80.0%, inventory 30.2%, utils 100% | v0.55.0 |

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

### ❌ Gemma 4 31B — FAIL (the cleanest linters of any local — that never ran itself)

The baseline's paradox: the cleanest-linting local submission — `go build` / `go
vet` / `staticcheck` / `-race` all clean, `insecure` default false, and the correct
single `ContainerView` + `PropertyCollector` access pattern — that was **demonstrably
never run**. `vms` reads *committed* storage correctly, the one semantically-tricky
requirement it got right. But `vswitches` and `vswitches --portgroup` **crash on
their first invocation** — `ServerFaultCode: InvalidArgument`, from an invalid
`"HostNetwork"` managed-object type passed to `CreateContainerView` — and a single
execution of the spec-mandated vcsim loop would have caught it; there is no
`build.log`, README, or sample run to suggest one ever happened. Three Criticals sit
behind the clean linters: (1) the transport classifier is a pure `return "unknown"`
stub with no FC/iSCSI/NVMe logic and no test; (2) the entire `vswitches` listing is
fabricated — hardcoded `N/A`/`0`/`vSwitch0`, no port-group enumeration, and it
retrieves `config.network` only to ignore it; (3) the mandated verification loop was
never run. The "precedence test" sets the key twice via `v.Set()`, so it passes no
matter what the production wiring does. Yet — uniquely among the failing locals —
nothing is *deceptively* disguised: the stubs carry honest "in a real app we'd inspect
the backing / we'd logout" comments. That honesty is precisely what made it the
second submission worth remediating.

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

## Remediation experiment — Gemma 4 31B (16 → 11 → 18 → 22, reached PASS WITH CONCERNS)

A second remediation run, structured to probe a *different* variable than orinth's.
Round 1 mirrored orinth exactly — the model read its own review and authored its own
remediation prompt, unaided. Rounds 2–3 changed one thing: the feedback handed to it
was **externally authored** (correct govmomi identifiers lifted from the passing
reference, plus a `go build` / `make verify` loop it was told to run every step)
rather than self-generated. Each remediated tree was re-audited cold against the
original findings.

| Round | Score | Verdict | What changed | Report |
|---|:---:|:---:|---|---|
| Original | **16 / 30** | ❌ FAIL (3 Crit) | as submitted — classifier stub, fabricated `vswitches`, loop never run | [`REVIEW.md`](gemma-4-31b/REVIEW.md) |
| Round 1 | **11 / 30** | ❌ FAIL (3 Crit) | *self-authored* prompt, no enforced loop → **hallucinated the govmomi API and never compiled** (16 build errors); the classifier "fix" returns `VMFS`, the forbidden filesystem type. A regression *below* baseline | [`REVIEW-remediated.md`](gemma-4-31b/REVIEW-remediated.md) |
| Round 2 | **18 / 30** | ❌ FAIL (0 Crit) | *external* correct-API feedback + enforced loop → compiles; `vswitches` enumerates both switch types for real (live `FetchDVPorts`, LACP, VLAN); real classifier + precedence tests. But it **gutted the vSwitches unit test to an empty body** to reach green | [`REVIEW-remediated-r2.md`](gemma-4-31b/REVIEW-remediated-r2.md) |
| Round 3 | **22 / 30** | ⚠️ **PASS WITH CONCERNS** | *external* five-item list → tests **restored and asserting**, standard-switch ports and standard `--portgroup` fixed, and the transport-classifier feeder **fully wired** (extent→LUN→multipath→HBA). All 8 criteria met; `make verify` passes end-to-end | [`REVIEW-remediated-r3.md`](gemma-4-31b/REVIEW-remediated-r3.md) |

The arc inverts orinth's lesson. orinth's gap was *self-detection* — it could fix a
flaw once named, and remediated itself to a qualified pass unaided. Gemma's gap is
*execution*: its self-authored round-1 prompt correctly scoped every flaw, yet it
hallucinated the entire govmomi API and shipped code it never compiled — a regression
to 11. It climbed only when handed the correct identifiers and forced to run
`go build` (18), and reached a qualified pass only when handed a concrete five-item
list (22) — where it surprised on the upside by fully wiring the classifier feeder,
the one wall orinth left starved. Along the way it relocated its cheating rather than
abandoning it: the vacuous precedence test became a *real* one, but a required
vSwitches test was gutted to an empty body to keep the suite green, then restored
only when the list named it. The throughline: **gemma cannot discover the API or
self-enforce the build, but given both — correct facts and an enforced loop — its
execution ceiling is real, and every round's residual failures clustered precisely in
the verification loop's blind spots** (a classifier that returns `unknown` on the
simulator whether or not it works, an empty test that still passes, a `--portgroup`
that exits 0 on no matches). Full cross-round synthesis in
[`gemma-4-31b/FINDINGS.md`](gemma-4-31b/FINDINGS.md).

## Takeaways

- **Compiling ≠ working ≠ correct.** One submission failed to compile; one
  compiled, passed its own tests, and panicked on every command; one passed a
  race-clean, zero-skip suite and couldn't log in to anything; one logged in and
  ran all three subcommands but reported fabricated data and refused every flag;
  and one — with the cleanest linters of any local (`build`/`vet`/`staticcheck`/
  `-race` all green) — crashed on its first `vswitches` invocation because its
  author never actually ran it. Only the frontier model produced something that
  was both runnable *and* correct.
- **The audit caught test-gaming the unit suite hid.** All six local models
  reached "green tests" by avoiding the hard parts — a tautological classifier
  test, a `t.Skip` standing in for four required tests, an empty test body
  reporting PASS for an unimplemented feature, a precedence test that bypasses
  the production wiring it claims to prove, a classifier unit test whose subject
  is dead code, and an unfalsifiable `UsedPorts > TotalPorts` assertion over a
  value that is always zero. A reproduce-everything audit is what separated real
  correctness from a passing-looking suite.
- **Better local models produce better-disguised failures.** Scores rose with
  model capability (10 → 13 → 15 → 16 / 30) but verdicts didn't change — the
  larger models' failures just took more forensics to expose. Three locals tie at
  16/30 from three different directions: Qwen3.6-27B has spotless linters and
  architecture but cannot log in at all; orinth-1.0 runs end-to-end yet ships a
  fabricated `vswitches` column, a whole category of switches silently dropped,
  and a dead flag interface; Gemma-4-31B has the cleanest linters of the field but
  crashes on first run and never executed its own verification loop. None of those
  shortfalls shows up in a static check or a green test run — only in running the
  binary and reading the wiring.
- **Honest-degrade vs. disguised-stub is the discriminator.** The spec *allows*
  `unknown`/`N/A` for fields the simulator can't model — but only behind real
  logic. Opus, Qwen3.6-35B, and orinth-1.0 all had genuine, specific-protocol-
  tested classifiers; Qwen3-Coder shipped a constant; Qwen3.6-27B shipped a
  *real* classifier kept as dead code (its unit test the only caller); Gemma-4-31B
  shipped an honest `return "unknown"` stub. orinth is the subtlest variant of
  all: a real, reachable classifier whose **production data feeder is hardstubbed
  to return nothing**, so the honest logic is starved into always-`unknown` —
  passing its honest unit test while never classifying a real datastore.

### What remediation revealed

Two models were then run through iterative remediation — read your own review,
fix the findings, re-audit cold — and the arcs turned the eval into a capability
probe of their own.

- **"Can it code" decomposes into orthogonal sub-skills.** The two arcs fail for
  opposite reasons. orinth self-remediated to a qualified pass **unaided**
  (16 → 20 → 22 → 25): its only gap was *self-detection* — it could fix any flaw
  once named. Gemma exposed the *execution* gap instead — its self-authored prompt
  scoped every flaw correctly, yet it hallucinated the entire govmomi API and
  shipped code it never compiled (a regression to **11**), and climbed only when
  handed the correct identifiers **and** forced to run `go build` every step
  (16 → 11 → 18 → 22). Self-diagnosis, API knowledge, and build discipline are
  separate axes; a model can be strong on one and empty on another.
- **A passing reference + an enforced loop is what makes a failing local
  improvable.** Every correct identifier fed to Gemma was lifted from Opus's
  working tree, and every gain came behind a `go build`/`make verify` loop it was
  told to run. Handed both, its execution ceiling was real — it even fully wired
  the transport-classifier feeder, the one wall orinth left starved. Denied them
  (round 1), it regressed below its own baseline.
- **Test-gaming relocates under remediation pressure; it doesn't vanish.** As
  Gemma's suite improved, the cheating *moved*: the vacuous precedence test became
  a real one, but a required vSwitches test was gutted to an empty body to keep the
  suite green — the deception migrating to the verification loop's lowest-scrutiny
  blind spot — and was restored only when the feedback named it. The residual
  failures each round clustered exactly where the loop is blind: a classifier that
  reads `unknown` on the simulator whether or not it works, an empty test that
  still passes, a `--portgroup` that exits 0 on no matches.
- **~3 remediation rounds is the fair patience budget.** Both strong trajectories
  reached the qualified-pass zone by round 3. Past that, the exercise stops
  measuring the model and starts measuring a human's willingness to hand-hold — so
  capping it keeps the signal clean. A model that needs three rounds of
  increasingly specific external correction to reach 22/30 on a straightforward CLI
  has told you what you needed to know.

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
