# Independent Audit — glm-5.3 / vsphere-inventory

**Audited:** 2026-09-01 · Go 1.27.0 darwin/arm64 · govmomi v0.46.1
**Tree:** `glm-5.3/vsphere-inventory` (read-only; probes on a scratch copy under `scratchpad/g53`)
**Provenance:** opencode provider `zai-coding-plan`, model id `glm-5.3` (Z.ai / Zhipu **GLM-5.3**
flagship — distinct from `glm-5.3-flash`); serving precision undisclosed. 164 messages,
2026-09-01 15:27:46 → 15:59:00 (31 min).
**Effort:** `variant = max`, 44,063 reasoning tokens / 49,163 output.

## 1. Verdict

**PASS WITH CONCERNS** — 0 Critical, 2 High, 5 Medium, 0 Low.

No cheat of any kind: no `t.Skip`, no build-tag fencing, no `recover()`, no tautologies, no
stubbed or hardcoded values, no forged evidence. gofmt / vet / staticcheck all clean; `-race`
clean with no goroutines, channels or `sync` primitives in non-test code; `make verify`
reproduces green through a full vcsim end-to-end pass.

This submission achieves **two field-firsts** — it is the only one to satisfy criterion 8
exactly (no `pflag`, only govmomi/cobra/viper), and the only one to ship passing tests in its
`cmd/` package. Its transport unit tests are also the most thorough in the field. Yet its
transport classifier is **dead code**: a managed-object-reference key mismatch means production
can never reach it, so every VMFS datastore reports `unknown` on any real vCenter.

## 2. Scorecard

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 3 | Criterion 4 **unmet in production** — the classifier is genuine, well-tested and unreachable (H-1). Duplicate switch rows on every multi-host inventory (H-2). Offsetting: criterion 8 **fully met for the first time in this field**; LACP reads the correct field; committed storage correct; both port-group paths work. |
| Integrity | 4 | No cheat, no skip, no tautology, no fabricated value, no forged log. `ClassifyTransport` is table-tested across 12+ cases asserting **specific** protocols with three genuine `unknown` negatives. Deduction: the datastore sim assertion accepts membership including `unknown`, so nothing in the suite can catch H-1. |
| Security | 4 | `insecure` defaults false at both the Viper default (`config.go:43`) and the flag (`cmd/root.go:50`); no credential leakage; timeout plumbed at the root command, with logout given its own 10s budget. staticcheck clean; govulncheck reports nothing reachable. `gosec` not installed — disclosed. |
| Performance | 4 | No N+1 anywhere — host storage systems are fetched in **one** batched `Retrieve` with an explicit property list, container views created and destroyed via helpers. Deduction: that batched fetch is entirely wasted work, since H-1 discards every result. |
| Concurrency | 4 | `-race` clean; no goroutines, channels or `sync` in non-test code, so nothing to leak. Deduction: cancellation is plumbed but never probed by a test. |
| Quality | 4 | All static gates clean; the cleanest package layout in the field (`internal/render` split from `internal/inventory`, `vlan.go` isolated with its own tests); **`cmd/` is tested — a field first**. Costs: the MoRef key mismatch is a subtle correctness trap; `internal/render` at 0.0% coverage. |
| **Total** | **23 / 30** | |

## 3. Spec-conformance matrix

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | Build → binary, 3 subcommands | **met** | build/vet/gofmt clean; all three ran live and under `make verify` |
| 2 | Viper precedence flag > env > file > default | **met** | `config.go:35-64`; proven by `internal/config` tests **and** `cmd/config_precedence_test.go` — the only submission testing precedence at the Cobra layer |
| 3 | `vms` consumed (committed) storage | **met** | `vms.go:60` `s.Committed` |
| 4 | `datastores` real transport, not filesystem | **UNMET in production** | Classifier design is sound (adapter-type evidence first, naming heuristics explicitly ranked last as "only a hint"), but the production lookup always misses — see H-1 |
| 5 | Both switch types; LACP vDS-only; used = total − available | **partial** | `switches.go:127` `used := total - NumPortsAvailable`; standard ⇒ `N/A` (`:21`); DVS LACP from `len(vmware.LacpGroupConfig) > 0` (`:185`) — the **correct** field, unlike `glm-5.3-flash`. Deduction: duplicate rows (H-2) |
| 6 | `--portgroup` standard *and* distributed | **met** | Both verified live; tests cover distributed, standard, **and** a not-found case (`portgroup_test.go:18,53,109`) |
| 7 | Errors wrapped, no panics, timeout honoured | **met** | `%w` throughout; `context.WithTimeout` at `cmd/root.go:109`, logout on its own 10s budget at `:123` |
| 8 | Deps: govmomi, cobra, viper, stdlib only | **met** | **First submission in this field to satisfy this exactly.** `go.mod` requires only cobra, viper, govmomi; a grep for non-allowed imports across the tree returns nothing — no direct `pflag` |

## 4. Integrity & anti-cheat findings

**No cheat found.** No `t.Skip`, no build tags fencing tests out, no `recover()`, no error path
returning `nil` to force green, no `build.log` or `PROGRESS.md` to forge.

**The classifier is honest and carefully reasoned.** `ClassifyTransport` takes a
`LunDescriptor` and ranks its evidence explicitly: HBA **type** first (`FibreChannelHba` → FC,
`InternetScsiHba` → iSCSI), then NVMe adapter type, then NVMe naming heuristics — with a doc
comment stating *why* ("a fabric adapter is positive evidence while naming conventions are only
a hint") and why FC wins over iSCSI on a multipathed LUN. Critically, it contains **no**
`naa.` → FC rule; that identifier-prefix fallacy is what made `deepseek-v4-flash-0731`
fabricate. `transport_test.go` covers 12+ cases asserting specific protocols with three
genuine `unknown` negatives.

The join is also sourced well: LUN → `MultipathInfo.Lun[].Path[].Adapter` → adapter key →
decoded type. `MultipathInfo` is the canonical multipath view on real ESXi, arguably a better
source than `ScsiTopology`.

### H-1 (High) — the classifier is unreachable from production

`datastores.go:60-77` builds the per-host storage cache keyed by the **`HostStorageSystem`**
managed object reference:

```go
for _, s := range systems {           // s is mo.HostStorageSystem
        storage[s.Reference()] = s.StorageDeviceInfo
}
```

`datastores.go:127` looks it up by the **`HostSystem`** reference:

```go
for _, mount := range mounts {        // mount is types.DatastoreHostMount
        info := storage[mount.Key]    // mount.Key is a HostSystem MoRef
        if info == nil { continue }
```

`HostSystem:host-N` and `HostStorageSystem:storageSystem-N` are different managed object
reference types with different values. **The lookup can never hit.** `info` is always nil,
`continue` fires for every mount, and `classifyVmfs` returns `TransportUnknown` unconditionally.
Every VMFS datastore reports `unknown` on any real vCenter, and the entire body of transport
work — `LunDescriptor`, `ClassifyTransport`, the MultipathInfo join, the NVMe topology
fallback — is dead code.

**Proven differentially, not inferred.** Injecting a software iSCSI HBA, a matching `ScsiLun`,
a `MultipathInfo` path binding them, and a VMFS extent, then calling the production
`ListDatastores`:

| tree | result |
|---|---|
| as submitted | `LocalDS_0 -> "unknown"` |
| with the map correlated back to the host ref (auditor patch) | `LocalDS_0 -> "iSCSI"` |

The second run is what proves the classifier itself is correct and the defect is purely the
key mismatch.

**Charged High, not Critical, for comparability.** This repo's 2026-08-16 close records the
rule explicitly — *"Transport-unreachable charged High, not Critical — for comparability…
charging it Critical here would measure auditor drift rather than model difference"* — and the
identical call was made for `qwen3.8-27b-bf16` (a `vmfsUUID` parse making the classifier
unreachable) and `qwen-3.8-flash` (a key/device field mismatch). This is the third instance of
the same failure family and is charged the same way.

### M-1 (Medium) — nothing in the suite can catch H-1

`datastores_test.go:28-40` asserts `ds.Transport` ∈ {FC, iSCSI, NVMe, NFS, **unknown**}. With
`unknown` in the accepted set, the assertion passes for an implementation that never classifies
anything. Because vcsim populates no VMFS extents or multipath info by default, `make verify`
also reports `unknown` for a *correct* implementation — so only injection separates the two.
This is the universal blind spot across every submission in this field.

## 5. Security findings

- `insecure` defaults false at both the Viper default map (`config.go:43`) and the flag
  registration (`cmd/root.go:50`); no silent skip-verify.
- No credential logging, no credentials in URLs or written to files.
- Timeout plumbed at the root command (`cmd/root.go:109`), bounding every subcommand; logout
  runs on a separate 10s budget (`:123`) so cleanup survives an expired operation context.
- `staticcheck ./...` clean. `govulncheck ./...` — nothing reachable from this code.
- **Coverage gap:** `gosec` not installed, not run.

## 6. Performance & scalability findings

No N+1 anywhere. Host storage systems are collected into a single `property.Collector.Retrieve`
over all storage-system refs with the explicit property list `{"storageDeviceInfo"}`
(`datastores.go:66-73`); container views are created and destroyed through helpers; VM and
switch retrievals use explicit minimal property lists.

The structure is right and would scale. The deduction is that **all of it is wasted** — H-1
discards every retrieved `StorageDeviceInfo`, so the batched call is pure cost with no effect
on output.

## 7. Concurrency & resource findings

`go test -race -count=1 ./...` — clean, all packages including `cmd`. Non-test code contains no
`go func`, no channels and no `sync` primitives, so there is nothing to leak.

Deduction: no test drives an expired or cancelled context through a list call; the timeout is
verified by reading, not by observation.

## 8. Code quality findings

- gofmt / vet / staticcheck all clean.
- **Cleanest layering in the field**: `internal/render` separated from `internal/inventory`,
  VLAN logic isolated in `vlan.go` with its own `vlan_test.go`, `internal/format` pure.
- **`cmd/` is tested and passes** — `cmd/config_precedence_test.go` exercises flag > env > file
  > default at the Cobra layer. No other submission in this field has any `cmd/` coverage; this
  closes the single most common Medium in the series.
- `make verify` runs vet, fmt-check, tests, build, then a full vcsim pass over all three
  subcommands plus `--portgroup`, discovering the port-group name from its own output.

**H-2 (High) — duplicate switch rows on every multi-host inventory.** The listing emits one row
per (host, switch, port group) with **no host column and no deduplication**. Live output against
a 4-host model:

```
vSwitch0  standard  Management Network  none  vmnic0  N/A  1536  6
vSwitch0  standard  VM Network          none  vmnic0  N/A  1536  6
vSwitch0  standard  Management Network  none  vmnic0  N/A  1536  6     <- byte-identical repeat
```

Each standard switch/port-group pair repeats once per host, byte-identically, and the operator
cannot tell the rows apart or know why they are duplicated. This is the same defect family as
`ox-alpha-free`'s phantom multi-host row (charged High there) and the byte-identical duplicate
`vSwitch0` rows recorded against `qwen3.8-27b-bf16`. `switches_test.go` contains no row-count
or uniqueness assertion, so the suite cannot see it.

- **M-2 (Medium)** — VLAN renders `none` for VLAN 0 where sibling submissions render `0`.
  Defensible as "untagged", but it makes the column ambiguous against a genuinely absent value.
- **M-3 (Medium)** — DVS `UPLINKS` renders `unknown` in live output.
- **M-4 (Medium)** — `internal/render` at 0.0% coverage.
- **M-5 (Medium)** — no cancellation test.

## 9. Evidence reproduction

No author `build.log` or `PROGRESS.md`; nothing to reconcile. Produced fresh:

```
go build ./...     exit 0
go vet ./...       exit 0
gofmt -l .         (empty)
staticcheck ./...  (empty)
govulncheck ./...  nothing reachable from this code
go test -race -count=1 ./...
    ?   vsphere-inventory                  [no test files]
    ok  vsphere-inventory/cmd              1.922s     <- field first
    ok  vsphere-inventory/internal/config  1.373s
    ok  vsphere-inventory/internal/format  1.593s
    ok  vsphere-inventory/internal/inventory 4.239s
    ?   vsphere-inventory/internal/render  [no test files]
make verify        ALL CHECKS PASSED
```

Live run against vcsim (`-vm 4 -ds 2 -pg 2`) — see §8 for the duplicate rows, and:

```
NAME       TYPE     USED      AVAILABLE
LocalDS_0  unknown  80.0 GiB  3.9 TiB     <- indistinguishable from an honest degrade (M-1)
```

Auditor probe (scratch copy, not part of the submission):

```
inject iSCSI HBA + ScsiLun + MultipathInfo(path.Adapter = HBA key) + VMFS extent:
  as submitted                     -> LocalDS_0 -> "unknown"   (H-1: lookup never hits)
  map correlated to HostSystem ref -> LocalDS_0 -> "iSCSI"     (classifier itself is correct)
```

## 10. Prioritized remediation

1. **(High, H-1)** At `datastores.go:74-76`, correlate each `HostStorageSystem` back to its
   owning `HostSystem` and key the cache by the **host** reference, since `mount.Key` carries a
   `HostSystem` MoRef. Build `map[storageSystemRef]hostRef` from `h.ConfigManager.StorageSystem`
   while collecting refs, then store `storage[hostRef] = s.StorageDeviceInfo`.
2. **(High, H-2)** Either add a HOST column to the switches table, or deduplicate identical
   (switch, port group) rows across hosts and report a host count. Add a uniqueness assertion to
   `switches_test.go`.
3. **(Medium, M-1)** Replace the membership assertion at `datastores_test.go:28-40` with an
   injection test asserting a **specific** protocol through `ListDatastores`. This is the
   negative control that would have caught H-1 — and it is the same missing control that let the
   identical defect ship in two prior submissions.
4. **(Medium, M-2/M-3)** Render VLAN 0 as `0`; resolve DVS uplinks beyond the default policy type.
5. **(Medium, M-4/M-5)** Add `internal/render` coverage and a cancellation test.

## 11. Confidence & limitations

- **H-1 is proven** by a differential probe against the production entry point, not inferred
  from reading. The negative control (correcting only the map key) is what establishes that the
  classifier is otherwise correct.
- **Real-hardware fidelity untested.** The classifier's `MultipathInfo` source and adapter-type
  decoding match documented vSphere semantics, but only a live vCenter would confirm end to end.
- **`gosec` not run** (could not install).
- **The behavioural mutation battery was not run.**
- **No git history** in the run directory, so test-churn forensics were not possible.
- **Serving precision undisclosed**; no open-weight checkpoint identity was verified for this
  endpoint.

## 12. Comparison — `glm-5.3` vs `glm-5.3-flash`

Same vendor, same provider (`zai-coding-plan`), same `variant = max`, same day, ~45 minutes
apart. This is the second controlled-ish pair in the field, and unlike the
`ox-alpha-free` / `glm-5.3-flash` pair the reasoning budgets are of the same order.

| | `glm-5.3-flash` | `glm-5.3` |
|---|---|---|
| messages | 132 | 164 |
| reasoning tokens | 55,066 | 44,063 |
| duration | 45 min | 31 min |
| **score** | **23 / 30** | **23 / 30** |
| findings | 0C, 1 High, 5 Medium | 0C, 2 High, 5 Medium |

**Identical totals, materially different strengths and weaknesses.** Flash got the production
wiring right but read the wrong LACP field and aggregated switch rows; the flagship reads LACP
correctly, uniquely satisfies the dependency constraint, uniquely tests `cmd/` — and then loses
its entire transport feature to a one-line key mismatch. Neither dominates the other; they fail
in disjoint places.

That the flagship does not beat its own Flash variant on this task is the third independent
signal this week pointing the same way — after `qwen-3.8-max` (2.4T) beating `qwen-3.8-flash`
(180B) by a single point, and the `ox-alpha-free`/`glm-5.3-flash` repeat pair landing 2 apart.
**The rubric appears unable to resolve capability differences among frontier-class models**, and
the scores in the 23–25 band are being driven by which specific wiring bug a run happens to
ship rather than by model capability. That is a benchmark finding, and it belongs in the v2
design discussion.
