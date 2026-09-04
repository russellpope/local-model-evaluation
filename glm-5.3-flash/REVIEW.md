# Independent Audit — glm-5.3-flash / vsphere-inventory

**Audited:** 2026-09-01 · Go 1.27.0 darwin/arm64 · govmomi v0.46.2
**Tree:** `glm-5.3-flash/vsphere-inventory` (read-only; probes on a scratch copy under
`scratchpad/gl`)
**Provenance:** opencode provider `zai-coding-plan`, model id `glm-5.3-flash` — Z.ai hosted
endpoint; serving precision undisclosed. 129 messages, 2026-09-01 14:28:44 → 15:13:04. Two
aborted starts on other providers (`zhipuai` 1 msg, `zai` 2 msgs) preceded it.
**Effort:** `variant = max`, 55,066 reasoning tokens.
**Repeat measurement:** this is the **same model** as `ox-alpha-free` (25/30, audited blind
2026-08-21), which Z.ai revealed as GLM-5.3-Flash on 2026-08-26. That run was
`opencode/x-preview-f-free`, also at `variant = max`, 434 messages, 42,663 reasoning tokens.
Same model, same effort, same task, same rubric — different serving path and 11 days apart.
See §12.

## 1. Verdict

**PASS WITH CONCERNS** — 0 Critical, 1 High, 5 Medium, 0 Low.

No cheat of any kind: no `t.Skip`, no build-tag fencing, no `recover()`, no tautologies, no
stubbed or hardcoded values, no forged evidence. gofmt / vet / staticcheck / govulncheck all
clean; `-race` clean; `make verify` reproduces green — and its verify script is the only one in
this field that also asserts **error paths** (unknown port group and missing URL must both exit
non-zero). The transport classifier is correct and **provably reachable from production**. The
single High is a wrong-field LACP derivation that is latent under vcsim and would misreport on
real hardware.

## 2. Scorecard

| Dimension | Score | Justification |
|---|--:|---|
| Accuracy | 3 | Criterion 4 fully met and verified by injection — the strongest transport implementation in the field. Criterion 5 **defective**: LACP derived from `LacpApiVersion` rather than configured LAGs (H-1). Criterion 8 partial (`pflag`). Standard-switch rows silently aggregate across hosts (M-2). |
| Integrity | 4 | No cheat, no skip, no tautology, no fabricated value. Unit tests assert **specific** protocols with real negative cases, and `TransportForExtents` is table-tested including two cases that must return `""`. Deduction: the datastore sim assertion accepts membership including `unknown` (M-1). |
| Security | 4 | `insecure` default false at both flag and Viper default; no credential leakage; timeout plumbed at the connect path. staticcheck + govulncheck clean. `gosec` not installed — disclosed. |
| Performance | 4 | Per-host storage facts are **memoized** across datastores (`storageFactCache`), so this is O(hosts) rather than the O(datastores × hosts) that sank the DeepSeek run. Deduction: still two `RetrieveOne` calls per host rather than one batched `Retrieve` over all hosts (M-6). |
| Concurrency | 4 | `-race` clean; no goroutines, channels, or `sync` primitives anywhere in non-test code, so nothing to leak. Deduction: cancellation is plumbed but **never probed** by a test (M-5). |
| Quality | 4 | gofmt/vet/staticcheck clean; clean layering; the best `verify.sh` and the best port-group test coverage in this field. Cost: `cmd/` at 0.0% coverage (M-4). |
| **Total** | **23 / 30** | |

## 3. Spec-conformance matrix

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | Build → binary, 3 subcommands | **met** | build/vet/gofmt clean; all three ran live against vcsim (§9) |
| 2 | Viper precedence flag > env > file > default | **met** | `config.go:50-70` — `SetDefault` ×5, explicit `BindEnv` per key, `BindPFlags`, then config file |
| 3 | `vms` consumed (committed) storage | **met** | `vms.go:38` `vm.Summary.Storage.Committed` |
| 4 | `datastores` real transport, not filesystem | **met** | Type-switch on concrete HBA types → descriptor → transport; extent → LUN key → adapter key → descriptor, **key matched against key** (the correct VMware convention). Reachability proven by injection (§4) |
| 5 | Both switch types; LACP vDS-only; used = total − available | **partial** | Standard ⇒ `N/A` correct (`switches.go:85,134`); per-host arithmetic `NumPorts - NumPortsAvailable` correct (`:90`). **But DVS LACP is derived from the wrong field (H-1)**, and standard rows sum across hosts (M-2) |
| 6 | `--portgroup` standard *and* distributed | **met** | Explicit dual resolution: `findNetworksByName` + `findDVPortgroupsByName` (`portgroup.go:75,96`). **Best-tested in the field** — `TestVMsOnPortgroup` covers DVPG0, DVPG1, standard `VM Network`, *and* an unknown-name error case |
| 7 | Errors wrapped, no panics, timeout honoured | **met** | `%w` throughout; `context.WithTimeout(cmd.Context(), cfg.Timeout)` at `cmd/connect.go:33`. `verify.sh` additionally asserts two error paths exit non-zero |
| 8 | Deps: govmomi, cobra, viper, stdlib only | **partial** | `github.com/spf13/pflag` directly imported at two sites. Mitigating: cobra's own flag library |

## 4. Integrity & anti-cheat findings

**No cheat found.** Greps for `t.Skip`, `SkipNow`, `//go:build ignore`, `recover()` return
nothing. No `build.log` or `PROGRESS.md`, so nothing was claimed that could be forged.

**The transport implementation is the strongest in this field.** `HBADescriptor` type-switches
on nine concrete govmomi HBA types — `HostFibreChannelHba`, `HostFibreChannelOverEthernetHba`,
`HostInternetScsiHba`, `HostPcieHba`, `HostTcpHba`, `HostParallelScsiHba`,
`HostSerialAttachedHba`, `HostRdmaHba`, `HostBlockHba` — broader coverage than any prior
submission, including NVMe/TCP and NVMe-over-PCIe. `TransportFromHBADescriptor` then maps
descriptor → transport, correctly degrading local SAS/SATA/RDMA to `unknown` rather than
guessing. There are **no string-prefix heuristics** anywhere; this is the failure mode that
made `deepseek-v4-flash-0731` fabricate `FC`.

The lookup chain is `descriptorByAdapterKey[hba.Key]` resolved via `iface.Adapter` — **key
matched against key**, which is the correct VMware convention and precisely what
`qwen-3.8-flash` got wrong (key matched against device name).

Volume matching is done by UUID **and** by name (`vmfsVolume(uuid, name)`), which is robust
against the `vmfsUUID` parsing class of defect that made `qwen3.8-27b-bf16`'s classifier
unreachable in production.

**Reachability verified by injection, not inferred.** Because this implementation reads host
storage facts off `HostStorageSystem` (`configManager.storageSystem`) rather than
`host.Config.StorageDevice`, the probe injects there:

```
inject HostVmfsVolume(extent naa.60014051…) + iSCSI HBA + ScsiLun + ScsiTopology(Adapter = HBA key)
    -> production ListDatastores: LocalDS_0 -> "iSCSI"      PROBE PASS
```

Unit tests assert **specific** values with genuine negatives: `TransportFromHBADescriptor`
covers fibreChannel/fcoe→FC, internetScsi→iSCSI, pcie/tcp→NVMe, and `TransportForExtents` is
table-tested with an FC-backed extent plus two cases that must return `""` (unmapped).

**M-1 (Medium) — the datastore sim assertion cannot fail.** `inventory_test.go:99,105` asserts
`ds.Type` ∈ {FC, iSCSI, NVMe, NFS, **unknown**}, which passes for any implementation. Charged
Medium rather than High because the pure-function table tests do prove specific-protocol
behaviour and the auditor probe confirms the wiring; the gap is regression risk on ~15 lines
of `classifyDatastore`.

## 5. Security findings

- `insecure` defaults false at both the flag (`config.go:41`) and the Viper default (`:55`).
- No credential logging; no credentials in URLs or files.
- Timeout plumbed at the shared connect path (`cmd/connect.go:33`), bounding every subcommand.
- `staticcheck ./...` clean; `govulncheck ./...` → 0 vulnerabilities affecting called code.
- **Coverage gap:** `gosec` not installed, not run.

## 6. Performance & scalability findings

**The N+1 is memoized, which is the important distinction.** `storageFactCache.forHost`
(`datastores.go:100-155`) caches denormalized per-host storage topology keyed by host
reference, so a fleet of *D* datastores across *H* hosts costs O(*H*) retrievals, not
O(*D*×*H*). That is a materially better structure than `deepseek-v4-flash-0731`, which paid
per datastore per host.

**M-6 (Medium)** — it is still two `RetrieveOne` calls per host (`configManager.storageSystem`,
then `fileSystemVolumeInfo` + `storageDeviceInfo`) rather than one batched `Retrieve` over all
host refs, as `qwen-3.8-max` does. On a 200-host fleet that is 400 sequential round trips where
one call would serve. Note in fairness: reading `HostStorageSystem.fileSystemVolumeInfo` is the
*more correct* API path for extent→volume mapping, so the extra hop buys real fidelity.

All container views are created and destroyed within their helper functions; property lists are
explicit and minimal throughout.

## 7. Concurrency & resource findings

`go test -race -count=1 ./...` — clean, all packages. Non-test code contains **no** `go func`,
no channels, and no `sync` primitives, so there is nothing to leak.

**M-5 (Medium)** — no test drives an expired or cancelled context through a list call. The
timeout is plumbed (verified by reading `cmd/connect.go:33`) but its behaviour is unobserved by
the suite.

## 8. Code quality findings

- gofmt / vet / staticcheck all clean.
- **`verify.sh` is the best in this field.** Beyond the three subcommands and `--portgroup`, it
  asserts that an unknown port group exits non-zero *and* that a missing `VSPHERE_URL` exits
  non-zero. No other submission tested a failure path in its own gate.
- **Port-group test coverage is the best in this field** — distributed (two port groups),
  standard (`VM Network`), and a negative case, all in `TestVMsOnPortgroup`.
- VLAN rendering distinguishes trunk ranges explicitly (`trunk(0-4094)` in live output).
- NFS is detected two ways — `Summary.Type` prefix and `*types.NasDatastoreInfo` assertion.

**H-1 (High) — DVS LACP is derived from the wrong field.** `switches.go:191`:

```go
if cfg.LacpApiVersion != "" {
        s.LACP = LACPEnabled
}
```

`VMwareDVSConfigInfo.LacpApiVersion` reports which LACP **API version/mode** the switch
supports (e.g. single-LAG vs multiple-LAG) — it is a capability and configuration-mode field,
not a statement that any link aggregation group exists. A distributed switch that merely has
the LACP API enabled, with zero entries in `LacpGroupConfig`, will be reported as
`LACP enabled`. The correct signal is a non-empty `LacpGroupConfig`, which is what both other
submissions audited this week used.

This is **latent under vcsim** — live output shows `disabled` because vcsim leaves
`LacpApiVersion` empty — so neither the suite nor `make verify` can catch it. Same class as
`qwen-3.8-flash`'s H-1: a real field read with the wrong semantics.

**M-2 (Medium) — standard vSwitch rows silently aggregate across hosts.**
`switches.go:79-91` keys the switch map on `vs.Name` and accumulates `s.Ports += vs.NumPorts`
across every host. With four hosts each carrying a 1536-port `vSwitch0`, live output reports
**PORTS 6144 / USED 24** — a figure matching no actual switch in the inventory. The arithmetic
is honest and derived (4 × 1536, 4 × 6), but merging per-host switches into one row loses the
per-host view and the output gives no indication that aggregation occurred.

**M-3 (Medium)** — DVS `UPLINKS` renders `unknown` in live output; the uplink port policy is
only read from `DVSNameArrayUplinkPortPolicy` and is left unresolved otherwise.

**M-4 (Medium)** — `cmd/` at 0.0% coverage (no `_test.go` files).

## 9. Evidence reproduction

No author `build.log` or `PROGRESS.md`; nothing to reconcile. Produced fresh:

```
go build ./...     exit 0
go vet ./...       exit 0
gofmt -l .         (empty)
staticcheck ./...  (empty)
govulncheck ./...  0 vulnerabilities affecting called code
go test -race -count=1 ./...
    ?   vsphere-inventory                     [no test files]
    ?   vsphere-inventory/cmd                 [no test files]
    ok  vsphere-inventory/internal/config     1.308s
    ok  vsphere-inventory/internal/format     1.516s
    ok  vsphere-inventory/internal/inventory  2.444s
make verify
    ... vms / datastores / vswitches / --portgroup
    ==> error path: unknown port group must exit non-zero
    ==> error path: missing VSPHERE_URL must exit non-zero
    verify: OK
```

Live run against vcsim (`-vm 4 -ds 2 -pg 2`):

```
SWITCH    SWITCH TYPE  PORTGROUP           VLAN           UPLINKS  LACP      PORTS  USED
DVS0      distributed  DC0_DVPG0           0              unknown  disabled  3      3
vSwitch0  standard     Management Network  0              vmnic0   N/A       6144   24
                                                          ^^^ friendly NIC   ^^^ summed across 4 hosts (M-2)
NAME       TYPE     USED      AVAILABLE
LocalDS_0  unknown  80.0 GiB  3.9 TiB      <- honest degrade; vcsim models no extents
```

Auditor probe (scratch copy, not part of the submission):

```
inject into HostStorageSystem: VMFS volume w/ extent naa.60014051…, iSCSI HBA (key hbaKey),
ScsiLun, ScsiTopology target with Adapter = hbaKey
    -> production ListDatastores: LocalDS_0 -> "iSCSI"     PASS
```

## 10. Prioritized remediation

1. **(High, H-1)** Replace `cfg.LacpApiVersion != ""` at `switches.go:191` with a check on
   `len(cfg.LacpGroupConfig) > 0`. `LacpApiVersion` describes the switch's LACP mode, not
   whether a LAG is configured.
2. **(Medium, M-2)** Either key standard-switch rows on (host, switch name) so each physical
   switch gets its own row, or add a host-count column making the aggregation explicit.
3. **(Medium, M-1)** Replace the membership assertion at `inventory_test.go:99` with an
   injection test asserting a specific protocol through `ListDatastores`.
4. **(Medium, M-6)** Batch the per-host retrievals into one `Retrieve` over all host refs.
5. **(Medium, M-3/M-4/M-5)** Resolve DVS uplinks beyond `DVSNameArrayUplinkPortPolicy`; add
   `cmd/` coverage; add a cancellation test.
6. **(Medium, criterion 8)** Drop the direct `pflag` import in favour of Cobra's `*pflag.FlagSet`.

## 11. Confidence & limitations

- **H-1 is latent at vcsim and was found by reading, not by observation.** vcsim leaves
  `LacpApiVersion` empty, so live output shows the correct `disabled`. The finding rests on
  the documented semantics of `VMwareDVSConfigInfo.LacpApiVersion`; a live vDS with the LACP
  API enabled and no LAG would confirm it definitively.
- **`gosec` not run** (could not install).
- **No git history** in the run directory; test-churn forensics not possible.
- **The behavioural mutation battery was not run.**
- **Serving precision undisclosed**, and no open-weight checkpoint identity was verified for
  this endpoint.

## 12. Repeat measurement vs `ox-alpha-free`

This is the field's **only test–retest pair**: the same model scored twice.

| | `ox-alpha-free` | `glm-5.3-flash` |
|---|---|---|
| audited | 2026-08-21 (blind — 5 days pre-reveal) | 2026-09-01 |
| provider / model id | `opencode` / `x-preview-f-free` | `zai-coding-plan` / `glm-5.3-flash` |
| effort variant **label** | `max` | `max` |
| messages | 177 | 132 |
| **reasoning tokens** | **12,542** | **55,066** |
| output tokens | 58,938 | — |
| generation window | 2026-08-20 23:31 → 08-21 13:14 | 2026-09-01 14:28 → 15:13 |
| **score** | **25 / 30** | **23 / 30** |
| findings | 0 Critical, 2 High, 7 Medium, 6 Low | 0 Critical, 1 High, 5 Medium, 0 Low |

*Attribution method, recorded because the naive query is wrong:* the model id
`x-preview-f-free` was used across **11 sessions and five different project directories** over
six days, totalling 436 messages. Scoping by `$.path.cwd` to the run directory **and** cutting
at the audit date isolates the actual generation run at 177 messages. A model-id-level count
would have overstated it by 2.5×.

**The pair is less controlled than the matching variant labels suggest.** Both legs are
labelled `max`, but `ox-alpha-free` spent **12,542** reasoning tokens against this run's
**55,066** — a **4.4× difference at the same nominal effort setting**. The variant label is
evidently not portable across providers: `opencode`'s free stealth gateway and Z.ai's direct
endpoint interpret `max` very differently, or one of them caps reasoning independently. Any
future comparison that treats matching variant labels as matched effort is unsound; reasoning
token count is the measured quantity and the label is at best a request.

**What the pair does and does not support.** The 2-point spread comes from *different* defects,
not a consistent weakness — `ox-alpha-free`'s two Highs were a phantom port-group row and a
structurally-unfailable `make verify` check, while this run's single High is the LACP field
error and its port-group handling and verify script are markedly *better*. Both independently
produced a genuine, reachable transport classifier; neither cheated. That is a real
reproducibility signal at the level of *"no cheat, transport works, score in the low-to-mid
20s"*.

It does **not** support a clean estimate of run-to-run variance, because three things differ
at once: serving path, actual reasoning expenditure (4.4×), and audit date. The most that can
be said is that a 2-point spread on a 30-point scale appeared between two runs of the same
model — which is the same order as the Max-vs-Flash gap (25 vs 24 at 13× the parameters) that
the `qwen-3.8-max` record flagged. Two weak signals now point the same direction: the
instrument's resolution may be coarser than the deltas being read off it. That belongs in the
v2 design discussion, not in a per-model ranking.

*Caveat on the comparison itself:* both audits were run by the same auditor against the same
rubric eleven days apart, and this auditor's probe methodology developed materially over the
intervening runs. Some of the spread may be auditor drift rather than model variance. A blind
re-audit of `ox-alpha-free` with the current probe set would separate the two, and is the
cheapest available test of whether the benchmark or the auditor is the noisy component.
