# qwen3.8-27b-q8_0 — behavioural mutation battery

**Date:** 2026-08-15
**Tree under test:** `qwen3.8-27b-q8_0/vsphere-inventory` (copied to a private scratch tree; the
submission itself was never mutated — verified byte-identical afterwards)
**Runner:** `~/.claude/skills/model-eval/scripts/mutate.py`
**Spec:** `docs/evals/2026-07-02-govmomi-vsphere-inventory-cli/mutation-specs/qwen3.8-27b-q8_0.json`
(22 probes, 4 marked `report_only`)
**Suite as found:** 13 tests, all passing, zero `t.Skip`; `cmd/` and `main.go` report
**"no test files"**.

## Scalars

- **kill rate (scorable): 5/18 = 28%** (excluded: 4 report-only, 0 no-site, 0 build-err)
- **kill rate (raw): 6/22 = 27%** (excluded: 0 no-site, 0 build-err)

## Distinct-cause summary

**16 survivors, 3 structural causes + 3 singletons:** `cmd/` has zero test files (4);
spec-prescribed set-membership / upper-bound assertions that any legal constant satisfies (6);
report-only lifecycle+security probes (3); plus one unreachable code path, one 1-element
fixture, and one unasserted matching rule.

## Controls

| Control | Command | Result |
|---|---|---|
| Negative, before | `go build ./... && go test ./...` on pristine tree | **GREEN** (exit 0) |
| Negative, after | `go test -count=1 ./...` after full battery | **GREEN** (exit 0) |
| Tree integrity, after | `diff -r` vs runner `.pristine` and vs original submission | **identical** (only `bin/` extra in submission) |
| Patch landing, all 22 | `diff -u` pristine vs mutated per probe | **22/22 landed** (evidence below) |
| Positive control 1 | `C1-invoke-clf` — classifier short-circuited to `FC` | **KILLED** ✔ |
| Positive control 2 | `C7b-units-gib` — GiB divisor halved | **KILLED** ✔ |

Both positive controls went red, so the harness is not silently green. Patch-landing evidence
(a real `diff -u` hunk per probe, plus an assertion that the `find` literal count went 1 → 0)
is in the run's scratch directory (`q8-mutation/patch-evidence.txt`, alongside `battery-run.txt`
and the per-probe mutated files under `q8-mutation/patch-evidence/`); every `find` string was extracted from the tree's
source by line range rather than retyped, and asserted to occur exactly once in its file.
There were **no `NO-SITE` and no `BUILD-ERR` rows** — all 22 probes are valid results.

## Verdict matrix

| ID | Class | Result | Behaviour broken |
|---|---|---|---|
| C1-invoke-clf | 1 invocation | **KILLED** | `ClassifyTransport` returns constant `FC` *(positive control)* |
| C1b-chain-stub | 1 invocation | SURVIVED | whole HBA→LUN→extent traversal bypassed; every VMFS datastore reports `unknown` |
| C2-wire-bindflag | 2 wiring | **KILLED** | viper `BindPFlag` deleted: `--flag` no longer beats env |
| C3-flag-portgroup | 3 flag honoured | SURVIVED | `--portgroup` value ignored; always prints the switch listing |
| C4-col-lacp-gone | 4 column presence | SURVIVED | LACP column deleted from header and every row |
| C5-col-order-ds | 5 column order | SURVIVED | datastores USED/AVAILABLE values swapped under unchanged headers |
| C6-sort-vms | 6 ordering | **KILLED** | documented VM name sort reversed |
| C6b-sort-ds | 6 ordering | SURVIVED | documented datastore name sort reversed |
| C7-units-ram | 7 units/labels | SURVIVED | RAM off by 1024× (MiB shifted as KiB), label unchanged |
| C7b-units-gib | 7 units/labels | **KILLED** | GiB divisor halved *(positive control)* |
| C8-tls-forced | 8 security default | SURVIVED\* `[report-only]` | TLS verification unconditionally skipped |
| C8b-insecure-dflt | 8 security default | **KILLED\*** `[report-only]` | built-in `insecure` default flipped to `true` |
| C9-life-logout | 9 lifecycle | SURVIVED\* `[report-only]` | session `Logout` removed from cleanup |
| C9b-life-view | 9 lifecycle | SURVIVED\* `[report-only]` | container-view `Destroy` removed (view leak per call) |
| C10-err-degrade | 10 error path | **KILLED** | unknown port group returns empty list instead of erroring |
| C10b-pg-prefix | 10 error path | SURVIVED | port-group matching loosened from exact to prefix |
| C11-ports-arith | port arithmetic | SURVIVED | standard vSwitch USED reports FREE ports (inverted) |
| C11b-dvs-ports | port arithmetic | SURVIVED | distributed port-group USED inflated by one fabricated port |
| C12-props-ds | property selection | SURVIVED | datastore `host` mounts no longer fetched; transport chain starved |
| C12b-props-vm | property selection | SURVIVED | STORAGE reports `Uncommitted` (provisioned side) instead of `Committed` |
| C13-lacp-dvs | LACP branching | SURVIVED | DVS with no LACP policy reports `N/A` instead of `disabled` |
| C13b-lacp-std | LACP branching | SURVIVED | standard vSwitch port groups report LACP `enabled` (LACP is DVS-only) |

`*` = report-only, excluded from the scorable denominator.

## Worst survivor

**`C12b-props-vm` — `info.Committed = m.Summary.Storage.Committed` → `.Uncommitted`.**

One token. It changes every row of the `vms` table and it inverts **DoD criterion 3**
("`vms` reports consumed (committed) storage, not provisioned"). Measured against the tree's
own fixture: `committed = 234` bytes vs `uncommitted = 10737418240` — the STORAGE column goes
from `0.0 GiB` to `10.0 GiB` per VM, a 7-order-of-magnitude change in the exact quantity the
criterion exists to pin. It survives inside the **tested** package (`internal/inventory`, 61%
covered) with a simulator test that reads that very field — `TestListVMs` asserts only
`vm.Committed >= 0`. The suite looks like it covers criterion 3 and cannot see it break.

Runner-up: `C5-col-order-ds` (USED/AVAILABLE swap — 30 GiB vs 994 GiB in the fixture), but its
cause is the generic "`cmd/` has zero tests".

## Step 0 — requirements attack

### Survivors charged to the instrument (author wrote what it was asked to write)

- **C13, C13b, C11b** — prompt line 139 prescribes "`used ports ≤ total ports`, and
  `LACP ∈ enabled/disabled/N/A`". Both are satisfied by *any* legal constant. A fabricated
  `+1` on used ports and a standard vSwitch falsely claiming LACP `enabled` both pass the
  prescribed assertion verbatim. **Not chargeable to the model.**
- **C1b, C12-props-ds** — prompt lines 178–183 explicitly relieve the author of proving the
  transport chain against the simulator ("those fields must still render without error,
  degrading to `unknown`/`N/A`; the transport classifier's own logic is proven by its dedicated
  pure-function test"). The author in fact went *beyond* the prescription — it asserts
  `ds.Type == unknown` exactly, not mere set membership — and a stub returning `unknown` still
  passes, because that is what the instrument defines as correct here. **Instrument.**

### Survivors that are the author's own doing

- **C3, C4, C5, C7** — `cmd/` has no test file at all. The printers are already written as
  `printSwitches(w io.Writer, …)` / `printDatastores(w io.Writer, …)`; a `bytes.Buffer` golden
  test was available at zero architectural cost and was not written.
- **C6b** — the shared fixture sets `m.Datastore = 1`, so the prompt's "Sort rows by datastore
  name" (line 85) is unassertable by construction. Fixture-strength, author's choice.
- **C10b** — nothing in the prompt discourages asserting that port-group matching is exact.
- **C12b** — *jointly charged*, see contradiction (c) below.

### Spec contradictions (surfaced with proposed resolutions, not silently resolved)

**(a) DoD criterion 5 "used ports = total − available" vs distributed switches.**
A DVS exposes no `numPortsAvailable`; the tree derives DVS used ports from
`FetchDVPorts(Connected)`. *Proposed resolution:* read criterion 5's arithmetic as scoped to
**standard** vSwitches, where `numPorts`/`numPortsAvailable` exist; for a DVS, "used" must mean
actually-connected ports. The tree's implementation matches that reading and should not be
marked down for it.

**(b) Line 65 "RAM — configured memory, shown in GB" vs line 112 "consistent units (GiB/TiB)".**
*Proposed resolution:* line 112 governs (it is the formatting rule, and the required
byte-formatting test targets GiB/TiB). The tree uses GiB. Fix the prompt's line 65 to say GiB.

**(c) Test prescription 1 ("storage ≥ 0", line 133) vs DoD criterion 3 ("consumed, not
provisioned", line 166).** The prescribed minimum assertion is mathematically incapable of
discriminating committed from provisioned, yet criterion 3 is a stated acceptance bar — the
instrument asks for a test that cannot prove the criterion it grades. This is exactly the gap
`C12b` walks through. *Proposed resolution:* strengthen the prescription to require an
assertion that discriminates the two (the standard fixture already makes them differ by 7
orders of magnitude, so this is cheap and honest). Charge `C12b` **jointly**: the instrument set
a weak bar, but the author had both an explicit obligation (criterion 3) and the means.

**(d) Line 137 "`TYPE ∈ FC/iSCSI/NVMe/NFS/unknown`" vs DoD criterion 4 "real transport, not
filesystem type".** Membership that includes `unknown` cannot distinguish a working classifier
from a stub. *Proposed resolution:* keep the permissive simulator assertion (it is the honest
bar there per lines 178–183) but add a required **pure-function test over the chain assembly**
— a table test feeding synthetic `mo.Datastore` + host-storage inputs to `resolveTransport` —
which is fully hermetic and would kill `C1b` and `C12-props-ds`. This is the single highest-value
change to the instrument.

Note on a known instrument hazard: the prompt's `used ports ≤ total ports` bar is the *correct*
prescription and must not be tightened to "non-zero PORTS" — an earlier run in this series had
fabrication induced by exactly that impossible-honest demand.

## Collateral finding (outside the battery's remit, discovered by it)

Two probes (`C11-ports-arith`, `C13b-lacp-std`) mutate code that **never executes**. I verified
against the tree's own fixture that `ListSwitches` returns **only** `DVS0` — no standard
vSwitch, ever — while the simulator host `DC0_C0_H0` really does carry
`vSwitch0` (`numPorts=1536`, `numPortsAvailable=1530`) with port groups `VM Network` and
`Management Network`. Two independent defects in `listStandardSwitches` cause this:

1. it discovers standard switches by scanning `host.network` for refs of type
   `HostVirtualSwitch` / `HostPortGroup`, but `host.network` returns only `Network` and
   `DistributedVirtualPortgroup` refs — `HostVirtualSwitch` is a *data object*, not a managed
   object, so it can never appear there (true on a live vCenter too, not a simulator artifact);
2. `asRefs` does not handle `types.ArrayOfManagedObjectReference`, which is the concrete type
   the property collector actually returns for `host.network`, so it yields `nil` regardless.

The suite stays green because `TestListSwitches` asserts only that *at least one* switch exists
and that a distributed one was seen — it never asserts a standard switch appears, so the
"covers both standard and distributed" half of DoD criterion 5 is untested and, in fact,
unimplemented in effect. Flagging for the ground-truth pass to confirm independently.
