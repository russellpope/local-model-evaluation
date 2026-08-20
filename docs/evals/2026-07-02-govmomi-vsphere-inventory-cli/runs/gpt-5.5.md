---
name: gpt-5.5
created: 2026-07-08
model: gpt-5.5 (OpenAI GPT-5.5 — frontier hosted/API model, NOT a local open-weight submission like the qwen/gemma/ornith runs; driven from a superpowers subagent-driven-development TDD plan, exact harness not recorded by user)
stage: rescored
score: 29 / 30
---

# Run — gpt-5.5

## Wire

Submission landed complete as **untracked working-tree files** in `gpt-5.5/` (`git ls-files`
empty — only the eval prompt is committed, so no git-history test-churn forensic is possible).
13 Go files in a flat `package main` layout: 8 source (`main`, `config`, `client`, `commands`,
`inventory`, `output`, `format`, `types`), 4 tests (`config_test`, `format_test`,
`transport_test`, `inventory_simulator_test`), and a `//go:build ignore` standalone `vcsim.go`
runner. `go.mod` module `github.com/local-model-evaluation/gpt55-vsphere-inventory`, `go 1.23.0`;
built locally with go1.26.4. `Makefile` (`verify` target), `README.md`, built binary in `bin/`,
and cached `.cache/verify-*.txt` outputs present. Driving artifact
`docs/superpowers/plans/2026-07-09-vsphere-inventory-cli.md` is a superpowers TDD plan (Task 1 =
"write failing tests"); its checkboxes are mostly unticked (abandoned checklist, not incomplete
code) and it asserts **nothing** about completion — no PROGRESS.md / build.log / GATE GREEN to
reconcile. `go build ./...` and `go vet ./...` clean on arrival. **"gpt-5.5" is OpenAI's
GPT-5.5** — a frontier hosted/API model, not a local open-weight submission like the
qwen/gemma/ornith runs (so no quantization applies); the exact driving harness was not recorded
by the user.

## Audit

**PASS WITH CONCERNS, 26/30 — zero Criticals; the strongest FIRST-PASS in the field.** One
fresh-context adversarial subagent plus independent orchestrator reproduction agree: this is a
genuinely honest implementation with **no cheat, no gamed test, no fabricated/hardcoded data, no
forged evidence.** Everything reproduces green — `go build`/`go vet` clean, `gofmt -l` flags only
vendored `.cache/` (no project source), `go test ./... -race` passes **8 tests, 0 skips**
(58.6% cover), and `make verify` exits 0 driving all three subcommands + `--portgroup` against
vcsim; my fresh e2e outputs are **byte-identical** to the author's cached `.cache/verify-*.txt`.

The lineage's recurring cheats are all absent. The **transport classifier is an HONEST DEGRADE,
not the disguised stub** the rubric warns about: `ClassifyTransportDescriptor` (inventory.go:316)
is a real pure function with true FC/iSCSI/NVMe/NFS branching, and its unit test
(transport_test.go) asserts **specific protocols**, not membership-including-`unknown`. Storage is
committed not provisioned (`summary.storage.committed`, inventory.go:31/38); both switch types are
enumerated with real API port values (live `1536/6`, `used = total − NumPortsAvailable`); LACP is
hardcoded `N/A` for standard only and derived for distributed; the port-group test is a genuine
**bidirectional exact-set** assertion (cardinality + exact sorted names from the deterministic VPX
model). Deps = only govmomi/cobra/viper.

Concerns are spec-fidelity and test-coverage, not dishonesty. **H1 (High):** criterion 4's
live-vCenter transport derivation is effectively unimplemented — `datastoreTransport`
(inventory.go:309-314) feeds the classifier `fmt.Sprintf("%T %s", ds.Info, ds.Summary.Type)`, i.e.
the datastore's own Go type name (`*types.VmfsDatastoreInfo`) + filesystem string (`"VMFS"`), which
can **never** contain a block-transport token; verified against govmomi source that transport lives
in `VmfsDatastoreInfo.Vmfs` (extent→LUN→HBA), never the type name — so FC/iSCSI/NVMe return
`unknown` on both vcsim *and* real vCenter (only NFS classifies). Honest degrade (truthful
`unknown`, disclosed in README), so **not Critical** — but the required HBA/LUN traversal simply
doesn't exist. **M1 (Medium):** the standard `--portgroup` path has zero automated coverage (the
exact-set test only exercises a *distributed* PG); I manually verified `"VM Network"` resolves
(empty — no VMs on it in-sim) but `"Management Network"` errors `not found` though it's listed by
`vswitches`, because `networkRefsByName` resolves only `Network`/DVPG MOs. **M2 (Medium):** the sim
datastore-TYPE test asserts `validTransport()` which **accepts `unknown`** — the very
membership-including-unknown pattern the rubric flags; would pass an always-`unknown` stub,
mitigated only by the separate pure-fn test. Lows: dead `retrieveRefs` (staticcheck U1000),
environmental `GO-2026-5856` crypto/tls (toolchain go1.26.4→1.26.5, not author code), lenient
`parseVLAN` test helper. Raw report: `gpt-5.5/REVIEW.md`.

## Score

**26 / 30** — Accuracy 4, Integrity 4, Security 5, Performance 4, Concurrency 5, Quality 4.
Findings: **Critical 0, High 1, Medium 2, Low 3.** Accuracy 4: committed storage, LACP
distributed-only, used=total−available, both switch types, viper precedence, units, and sort all
correct; criteria 4 and 6 Partial (live transport derivation missing; standard `--portgroup`
untested). Integrity 4: no cheats, reproducible green, genuine exact-set port-group test and
specific-protocol classifier test — docked because the sim datastore test is
membership-including-`unknown` and the standard PG path is unproven. Security 5: `insecure`
defaults false, password redacted via `redactedURL`, logout deferred with `context.Background()`
on all paths, timeout plumbed through, no injection. Performance 4: textbook single
`ContainerView`+`Retrieve` with minimal property lists, views `Destroy()`'d, no N+1 — minor full-
`config` over-fetch in the port-group lookup. Concurrency 5: no goroutines in app code, `-race`
clean. Quality 4: gofmt/vet/staticcheck clean but one dead function; clean
retrieval/command/presentation separation, `%w`-wrapped errors throughout.

## Compare

**gpt-5.5 is a frontier hosted model (OpenAI GPT-5.5), so the apt peer is the other frontier
entry — claude-code-opus-4.7 (Claude, 30/30 reference) — not the local open-weight field.** Two
frontier models on the same task: **Claude 30/30 vs GPT-5.5 26/30.** GPT-5.5 did **not** match the
Claude reference — the 4-point gap is a real High spec gap (live-vCenter HBA/LUN transport
derivation never wired, H1), the untested standard `--portgroup` half (M1), and the
membership-including-`unknown` sim test (M2) — spec fidelity and coverage, not integrity (both are
cheat-free and fully reproducible).

**After round-1 remediation (26 → 29), GPT-5.5 is the highest-scoring non-reference run in the
field** — above ornith-1.0-397B's post-3-round 28, one point under the Claude reference's 30. The
round-1 gap-to-Claude was the unwired live transport derivation (H1); the self-prompted remediation
genuinely closed it (real HBA→LUN→topology traversal, `ContainerView` throughout, no relocated
cheat), so what remains between it and 30 is a single fleet-scale over-fetch (Performance 4). Against
the **local open-weight field**, every local model started in FAIL and remediated up (ornith-1.0-397B
22→28, ornith-1.0-35b 16→25, qwen-agentworld 16→23, gemma-4-31b →22, qwen3.6-35b →21) or stayed FAIL
(qwen-3.6-27b 16, qwen3-coder-next 13, gemma-4-12b 10); GPT-5.5 opened at 26 first-pass and reached
29 in one round — but as a frontier model that's expected and not a like-for-like comparison. The
lineage-consistent result: its self-prompted round genuinely fixed what the audit flagged, with **no
relocated dishonesty** (contrast qwen3.6/qwen-agentworld, which relocated a cheat each pass) — the
same clean-remediation signature ornith-1.0-397B showed.

## Remediate

**Round 1 is self-prompted, eval-prompt-as-instrument.** The user had GPT-5.5 read its own
independent `gpt-5.5/REVIEW.md` and the model authored + ran its own remediation prompt (the prompt
itself was not saved to the workspace). Per convention the audit report was left **unpatched** — any
unaddressed finding is signal. Target set from REVIEW.md §10: H1 (real HBA/LUN transport
derivation), M1 (standard-portgroup coverage + not-found-vs-empty), M2 (strengthen TYPE proof), and
the Lows (delete `retrieveRefs`, tighten `parseVLAN`).

**Process note (baseline-commit slip, recovered):** the pre-remediation baseline was never committed
(the whole tree is untracked), and remediation overwrote the source in place — so the convention's
`baseline → remediated` git diff was unavailable. Recovered it: only **3 files changed**
(`inventory.go`, `inventory_simulator_test.go`, `transport_test.go` — confirmed via mtimes; the other
10 retain their Jul-8 mtimes), and the auditor held all three verbatim from the round-1 read, so the
two changed test files were reconstructed and diffed to run the anti-test-weakening forensic anyway.
No remediation branch (untracked working tree).

## Rescore

**Round 1 reached PASS WITH CONCERNS at 29 / 30 (arc 26 → 29)** — the highest non-reference score in
the field, in a single self-prompted round. Fresh cold audit of the remediated tree (one
fresh-context subagent + independent orchestrator reproduction; raw report:
`gpt-5.5/REVIEW-remediated-r1.md`). **No relocated cheat, no test-weakening** — the reconstructed
baseline diff shows every test change is additive or strengthening: the bidirectional exact-set
port-group assertion survives verbatim (moved into an `assertVMNames` helper), `parseVLAN` was
tightened to validate 0–4095 ranges (not loosened), no `t.Skip`, 0 skips.

Both Highs/Mediums genuinely closed. **H1 FIXED (Accuracy 4→5):** live transport derivation is
really implemented and really wired — `ListDatastores` now retrieves each host's
`config.storageDevice` (gated on `containsVMFS`) and threads it through
`datastoreTransport → vmfsTransport → vmfsExtentTransport → (lunKeyByCanonicalName /
adapterKeyByLUNKey / hbaDescriptor) → ClassifyTransportDescriptor` (inventory.go:48-76, 152-166,
363-460). Verified against govmomi v0.52.0 that this yields FC (`HostFibreChannelHba`), iSCSI
(`HostInternetScsiHba`), and NVMe (via the `StorageProtocol` field, since no `HostNvmeHba` type
exists) on a live vCenter, and degrades to `unknown` on vcsim with no crash (`make verify` green,
sim datastores still honest `unknown`). Confirmed `vmfsTransport` has a real production caller — NOT
the qwen-3.6-27b dead-code-classifier pattern (staticcheck now 0 findings). **M1 FIXED
(Integrity/coverage):** `networkRefsByName` resolves standard host portgroups (inventory.go:290-303);
new `TestListVMsByStandardPortGroupWithSimulator` asserts positive exact-set on "VM Network",
empty-not-error on "Management Network", and real not-found on a bogus name. **M2 closed:** real TYPE
proof migrated to `TestVMFSTransportFromHostStorage` (drives the production chain with specific
FC/iSCSI/NVMe assertions + negatives); the sim `validTransport` still accepts `unknown` but asserting
non-unknown in a *simulator* test is impossible, so this is resolved-in-the-right-place.
**Dead code removed** (`retrieveRefs` + `property` import gone; staticcheck U1000 cleared → Quality
4→5). gofmt/vet/staticcheck clean, `-race` clean, coverage 58.6% → **63.8%**.

**Rescore: Accuracy 5, Integrity 5, Security 5, Performance 4, Concurrency 5, Quality 5 = 29/30.**
Findings: **Critical 0, High 0, Medium 1, Low 2.** The sole residual (and the reason it stays
PASS-WITH-CONCERNS, not clean PASS) is a new Medium the fix introduced: `ListDatastores` now
over-fetches full `config.storageDevice` for **every host in the fleet** whenever any datastore is
VMFS (almost always) — a single bulk PropertyCollector call (not N+1, and the derivation loops are
in-memory over already-retrieved data), but `config.storageDevice` is a heavy property, so the scope
is over-broad. Lows: environmental `GO-2026-5856` crypto/tls (toolchain, not author code), and the
new transport derivation's O(ds×extent×host×lun) in-memory cost at extreme scale. Full arc: 26 → 29.
