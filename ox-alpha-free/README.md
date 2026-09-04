# ox-alpha-free — govmomi vSphere Inventory CLI

Eval workspace. `vsphere-inventory/` holds the model's submission, frozen as delivered.
Full audit: [`vsphere-inventory/REVIEW.md`](vsphere-inventory/REVIEW.md).

- Build prompt: [`govmomi-cli-eval-prompt.md`](govmomi-cli-eval-prompt.md)
- Audit prompt: [`govmomi-cli-audit-prompt.md`](govmomi-cli-audit-prompt.md)
- Audited 2026-08-21 · Go 1.26.7 darwin/arm64 · govmomi v0.44.0

## Verdict — PASS WITH CONCERNS, 25/30

0 Critical · 2 High · 7 Medium · 6 Low

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 4 | All 8 criteria met; phantom row, `pflag` dependency, `go 1.26.7` vs a stated 1.22+ bar |
| Integrity | 4 | No cheat of any kind; one mislabelled README transcript |
| Security | 4 | `insecure=false` default is test-guarded; no credential leakage; scanners clean |
| Performance | 4 | No N+1 anywhere; fleet-wide `config.storageDevice` fetch is the residual |
| Concurrency | 5 | Race-clean, zero goroutines, timeout probed rather than assumed |
| Quality | 4 | vet/staticcheck clean, real layering; `internal/cmd` at 0.0% coverage |

Zero Criticals is the headline. Only `claude-code-opus-4.7`, `gpt-5.5` (r1) and
`ornith-1.0-397B` have previously cleared an audit here without one.

## Position in the field

| Run | Score | Note |
|---|--:|---|
| claude-code-opus-4.7 | 30 | reference |
| gpt-5.5 | 29 | after r1 |
| ornith-1.0-397B | 28 | |
| ornith-1.0-35b | 25 | after 3 remediation rounds |
| **ox-alpha-free** | **25** | **first audit, no remediation** |
| qwen-agentworld-35b-a3b | 23 | after r2 |
| qwen3.8-27b-bf16 | 23 | best local first-audit before this |

It matches the best local arc's final score on its first pass.

## What was verified, not assumed

- **Criterion 4 is not a stub.** Injecting an iSCSI HBA, `ScsiTopology` and VMFS extent into
  the simulator makes the production `ListDatastores` return `iSCSI` for that datastore and
  `unknown` for the other. The HBA → LUN → extent traversal is real and reachable from
  production code, not decoration around a constant.
- **Criterion 6's standard-port-group half works.** Re-backing a VM's NIC onto `VM Network`
  makes the lookup return exactly that VM. The submitted suite only asserts the standard
  lookup returns *empty* — correct behaviour, missing proof.
- **Criterion 7's timeout is plumbed.** `VSPHERE_TIMEOUT=1ms` produces a wrapped
  `context deadline exceeded` and exit 1.
- **Anti-cheat surface is clean.** No `t.Skip`, no tautological assertions, no hardcoded
  values standing in for derived data, no forged evidence — there is no `build.log` or
  `PROGRESS.md`, so nothing was claimed and nothing could be forged. The datastores test goes
  past the spec and asserts `TYPE == "unknown"` exactly for the simulator instead of hiding
  behind set membership.

## The two High findings

**H-1 — phantom port-group row on every multi-host inventory.** In `vswitches.go:99-106` the
cross-host dedup `continue`s before `rows++`, so the "this switch has no port groups" fallback
fires spuriously and emits `vSwitch0 / - / -` alongside the real rows. Discriminating run:
1 host gives 2 real rows and 0 placeholders; 3 hosts gives 2 real rows **and** 1 placeholder.
The submitted `TestListSwitches` runs with `ClusterHost = 2` — with the bug active — and passes.
Its most specific assertion (`std[0].TotalPorts != 1536 || std[0].UsedPorts != 6`) resolves to
`std[0].PortGroup == "-"`, pinning exact values on the row that should not exist.

**H-2 — `make verify`'s port-group check cannot fail.** The Makefile selects a port group with
`awk 'NR>2 && $3 != "-"'`. `NR>2` skips the header *and* the first data row, so it always picks
`DC0_DVPG1`, which has no VMs attached, prints an empty table, and reports `verify OK`. The
spec's required acceptance gate for criterion 6 therefore asserts nothing. Reproduced twice,
deterministic.

## Notable Mediums

- README's "Run output sample", captioned *"From `make verify` against vcsim"*, is not
  `make verify` output: it shows `--portgroup DC0_DVPG0` with VM rows where the real target
  invokes `DC0_DVPG1` and gets none, and it drops the phantom row and two `DC0_DVPG*` rows with
  no elision marker. Every value shown is producible by the code, so this is a mislabelled and
  trimmed transcript rather than fabricated data.
- `github.com/spf13/pflag` is imported at `root.go:7` and sits in go.mod's direct block, outside
  the allowed set. Avoidable with `Flags().Changed`.
- `internal/cmd` and `main.go` have 0.0% test coverage.
- Criterion 4's *glue* (`classifyDatastoreInfo`, `lunTransportMap`) has no test, and criterion 3
  is unguarded — swapping `Committed` for `Uncommitted` survives the suite.

## Behavioural mutation battery

Runner `spine gate go mutate`, 26 hand-authored probes, every `find` literal extracted from the
tree by line range and asserted unique. Control green before and after; corpus verified
byte-identical.

- **kill rate (scorable): 11/22 = 50%** (excluded: 4 report-only, 0 NO-SITE, 0 BUILD-ERR)
- **kill rate (raw): 12/26 = 46%**

**14 survivors, 3 causes:** `internal/cmd` has zero test files (5); simulator fixtures cannot
distinguish the mutated value from the honest degrade the spec permits (7); report-only
lifecycle and security probes outside `cmd/` (2).

Field comparison — qwen3.8-27b-q8_0 28%, kat-coder-v2.5 26%, ornith-1.5-35b 46%, opus-4.7 retro
25%. This is the highest rate measured here on a full-size spec. The battery is a reporting
instrument with no pass threshold; the number is comparative signal only.

Full per-class verdict matrix: [`vsphere-inventory/REVIEW.md`](vsphere-inventory/REVIEW.md) §9.

## Reproducing the audit gates

```sh
cd vsphere-inventory
gofmt -l . && go build ./... && go vet ./...
go test ./... -race -count=1 -cover     # 48 tests, 0 fail, 0 skip
staticcheck ./... && govulncheck ./... && gosec ./...
make verify
```

Results as audited: gofmt/build/vet/staticcheck clean; govulncheck reports no reachable
vulnerabilities; gosec reports 3 G115 int→int32 conversions, all false positives on `int32` port
counts; coverage is `config` 95.5%, `inventory` 75.3%, `cmd` 0.0%, `main` 0.0%; `make verify`
exits green, subject to H-2.

## Limitations

No git history exists in the submission, so test-versus-implementation churn could not be
diffed; the integrity conclusion rests on reading the final state and mechanical probing. No
live vCenter was available, so the full fidelity of criterion 4 (real FC/iSCSI/NVMe against real
HBAs) and criterion 5 (real LACP and uplink state) is unverified against hardware. What is
settled is that the traversal runs end to end and returns the right protocol for an injected
iSCSI topology.

Not yet on the `docs/evals` board — no run record has been created for this workspace.
