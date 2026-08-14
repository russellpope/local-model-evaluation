# Behavioural Mutation Battery — govmomi-inventory (laguna-s-2.1)

Date: 2026-08-14
Tree under mutation: `.../scratchpad/mutate/govmomi-inventory` (disposable copy; verified byte-identical to `.../scratchpad/frozen/govmomi-inventory` before and after the run)
Runner: `~/.claude/skills/model-eval/scripts/mutate.py`
Spec: `.../scratchpad/mutate/spec.json` (38 probes) · Raw log: `battery.log`
Toolchain: go1.26.5 darwin/arm64

Baseline re-confirmed green in this session before mutating: `go build ./...` OK, `go vet ./...` OK, `go test ./...` → `config` ok, `format` ok, `inventory` ok; `.` and `cmd` report **[no test files]**.

## Spec validation (pre-flight)

Every probe was validated mechanically before the battery ran, via
`verify_spec.py`: for each entry it counts occurrences of the `find` literal,
applies the replacement, **re-reads the file off disk** to confirm the patched
text is present and the `find` count decremented, then restores and asserts the
restore. Result: **38 probes, 0 problems** — no NO-SITE rows, and every
SURVIVED row below is a mutation proven to have landed on disk, not a patch
that silently failed to apply.

Two probes match 3 identical sites (`TMO-ctx-inert`, `LIFE-logout`); the runner
replaces only the first, which is `runVMs` in both cases. That is intended.

## Verdict matrix

| id | file | result |
|---|---|---|
| OUT-hdr-ds | cmd/root.go | **SURVIVED** — neg-ctl (must-survive) |
| OUT-units-fmt | format/format.go | **KILLED** — neg-ctl (must-kill) |
| OUT-swrow-vlan | cmd/root.go | SURVIVED |
| ARITH-ram-mb | inventory/vms.go | SURVIVED |
| ARITH-vm-storage | inventory/vms.go | SURVIVED |
| ARITH-ds-used | inventory/datastores.go | SURVIVED |
| PRED-ds-nfs | inventory/datastores.go | KILLED |
| PRED-pg-match | inventory/vswitches.go | SURVIVED |
| PRED-dvs-pgkey | inventory/vswitches.go | SURVIVED |
| ERR-suppress-std | inventory/vswitches.go | SURVIVED |
| ERR-suppress-dvs | inventory/vswitches.go | SURVIVED |
| ERR-prop-break | inventory/vswitches.go | KILLED |
| FAB-lacp-std | inventory/vswitches.go | SURVIVED |
| FAB-lacp-dvs | inventory/vswitches.go | SURVIVED |
| FAB-dvs-vlan | inventory/vswitches.go | SURVIVED |
| FAB-dvs-used | inventory/vswitches.go | KILLED |
| FAB-dvs-ports | inventory/vswitches.go | **BUILD-ERR** (excluded, disclosed below) |
| FAB-dvs-total | inventory/vswitches.go | **BUILD-ERR** (excluded, disclosed below) |
| FAB-dvs-total2 | inventory/vswitches.go | SURVIVED |
| ARITH-sw-used | inventory/vswitches.go | SURVIVED |
| FAB-transport | inventory/datastores.go | SURVIVED |
| TMO-ctx-inert | cmd/root.go | SURVIVED |
| TMO-cfg-inert | config/config.go | SURVIVED |
| TMO-default | config/config.go | KILLED |
| SORT-vms | inventory/vms.go | SURVIVED |
| SORT-ds | inventory/datastores.go | SURVIVED |
| SORT-sw-drop | inventory/vswitches.go | KILLED |
| SORT-sw-invert | inventory/vswitches.go | SURVIVED |
| SORT-sw-name | inventory/vswitches.go | SURVIVED |
| BND-fmt-lt | format/format.go | KILLED |
| BND-fmt-exp | format/format.go | KILLED |
| BND-vlan-trunk | inventory/vswitches.go | SURVIVED |
| BND-vcpu-floor | inventory/vms.go | SURVIVED |
| SEC-insecure-def | config/config.go | KILLED\* [report-only] |
| SEC-tls-forced | cmd/root.go | SURVIVED\* [report-only] |
| SEC-cred-override | cmd/root.go | SURVIVED\* [report-only] |
| LIFE-logout | cmd/root.go | SURVIVED\* [report-only] |
| LIFE-view-destroy | inventory/vswitches.go | SURVIVED\* [report-only] |

## Rates

```
kill rate (raw):      9/36 = 25%   (excluded: 0 no-site, 2 build-err)
kill rate (scorable): 8/31 = 26%   (excluded: 5 report-only, 0 no-site, 2 build-err)
```

## BUILD-ERR disclosure

Two probes broke compilation and are excluded from both denominators. Both were
the *same* probe intent — a fabricated but self-consistent DVS port count —
defeated by Go's unused-variable rule:

- `FAB-dvs-ports`: replacing `TotalPorts: totalPorts` with a literal left the
  `totalPorts` variable declared and unused.
- `FAB-dvs-total`: replacing `totalPorts = int(dvsConfig.NumPorts)` with a
  literal left `dvsConfig` declared and unused.
- `FAB-dvs-total2` (third attempt) widened the replacement to also drop the
  binding (`if _, ok := ...`), compiled, and **SURVIVED**.

The retries are disclosed rather than folded in, because the surviving third
attempt is the load-bearing result: `FAB-dvs-used` (the only KILLED probe in the
fabricated-constant class) died solely because `UsedPorts: 3 > TotalPorts: 0`
tripped a `used <= total` inequality — the simulator reports DVS `NumPorts` as 0.
Observed failure text: `switch DVS0 used ports 3 > total ports 0`. Once the
fabricated pair is internally consistent (`FAB-dvs-total2`, 128/0), the suite
sees nothing. The suite checks that port numbers are *plausible*, never that they
are *real*.

## Distinct-cause summary

**27 survivors, 8 distinct causes — but three account for 19 of them:**

1. **`cmd/` has zero test files** (6): OUT-hdr-ds, OUT-swrow-vlan, TMO-ctx-inert,
   SEC-tls-forced, SEC-cred-override, LIFE-logout. The entire CLI layer —
   output formatting, timeout plumbing, TLS, credentials, session lifecycle — is
   unobserved.
2. **`TestListVMs` / `TestListDatastores` never call `ListVMs` / `ListDatastores`** (6):
   ARITH-ram-mb, ARITH-vm-storage, ARITH-ds-used, SORT-vms, SORT-ds, BND-vcpu-floor.
   Both tests re-implement the retrieval and the unit arithmetic inline against the
   simulator and assert on their own copy. The production functions they are named
   for are never executed, so their arithmetic can be arbitrarily wrong.
3. **`ListSwitches` is called, but asserted only for plausibility** (7):
   FAB-lacp-std, FAB-lacp-dvs, FAB-dvs-vlan, FAB-dvs-total2, ARITH-sw-used,
   BND-vlan-trunk, LIFE-view-destroy. Assertions are set-membership
   (`LACP ∈ {enabled,disabled,N/A}`), non-negativity, and `used <= total` — every
   one satisfiable by a fabricated constant.
4. **`TestFindVMsByPortGroup` discards its result** (2, `_ = vms`): PRED-pg-match,
   PRED-dvs-pgkey. Both port-group matching predicates can be inverted freely.
5. **Sortedness asserted with the same comparator the sort uses** (2):
   SORT-sw-invert, SORT-sw-name. `switchLess` orders the slice *and* checks it —
   inverting it keeps the assertion tautologically true.
6. **No test exercises any error path** (2): ERR-suppress-std, ERR-suppress-dvs.
7. **Dead code** (1): FAB-transport. `ClassifyTransportFromDevice` is exported and
   called by nothing — not production, not tests.
8. **Masked by a fallback** (1): TMO-cfg-inert. `LoadConfig` discarding the
   configured timeout is hidden by its own `if cfg.Timeout == 0 { 60s }` default,
   and no test sets a non-default timeout — an inert `--timeout` flag.

Where the 9 kills live matters as much as the count: **5 of 9 are in `format` and
`config`**, the two leaf utility packages — and `format.HumanReadable` is dead in
production (`cmd` formats sizes with `%.1f` inline and never calls it). Of the 4
inventory kills, 3 are incidental rather than value assertions: `ERR-prop-break`
(a forced `InvalidProperty` hard-fails the run), `FAB-dvs-used` (zero-total
inequality, see above), `SORT-sw-drop` (ordering). Only `PRED-ds-nfs` is a real
value check, and it covers a two-line helper that returns `"unknown"` for
everything except NFS.

## Negative controls

| control | probe | prediction | observed | outcome |
|---|---|---|---|---|
| Must kill | `OUT-units-fmt` — `format` unit labels `KiB→KB`; `format_test.go` asserts exact strings for 9 inputs | KILLED | KILLED | Harness sound — mutations do reach the suite and turn it red |
| Must survive | `OUT-hdr-ds` — datastore table header `GiB→GB` in `cmd/root.go`, a package with no test files | SURVIVED | SURVIVED | Harness not trivially killing everything |

Both controls landed as predicted, so the 25% / 26% figures are trustworthy in
both directions.
