<!--
Round-1 remediation prompt — SELF-PROMPTED.
Authored by the model under test (Laguna S 2.1) after being asked to read its own
independent REVIEW.md. Captured here verbatim from the opencode session store
(ses_0364c4719ffeABfcSLIXg6aanW, 2026-08-03 15:17:06) as the prompt driving
remediation pass 1. Not edited by the auditor (eval-prompt-as-instrument: gaps are
signal, and this prompt becomes part of the post-run audit rubric).

Provenance note: the model was given the FULL REVIEW.md, including its section 11
prioritized-remediation list — the same conditions as ornith-1.0-397B pass 1,
gpt-5.5 round 1, and qwen3.6-35b passes 1-3. A findings-only variant of the review
(REVIEW-findings-only-r1.md, section 11 excised) was prepared but NOT used; it is
retained only as a possible contrast arm for a later round.
-->

You are an expert Go engineer tasked with fixing a vSphere inventory CLI that failed an independent audit. The audit found 4 Critical, 6 High, 10 Medium, 6 Low issues. Your job is to fix all Critical and High findings, plus the most impactful Medium/Low items.
Submission location
laguna-s-2.1/vsphere-inventory/
Critical Findings (must fix)
C1 — Transport classifier is a disguised stub. internal/transport/transport.go is an identity map — it returns DeviceType unchanged. Its only caller passes dsMo.Summary.Type (which returns VMFS/NFS/NFS41/vsan/VVOL/OTHER — never FC/iSCSI/NVMe). Fix: traverse datastore.info.Vmfs.Extent → ScsiLun.canonicalName → host.config.storageDevice.scsiTopology.adapter[].target[].lun[] → owning HBA, and switch on its concrete type (*types.HostFibreChannelHba→FC, *types.HostInternetScsiHba→iSCSI, StorageProtocol=="nvme"→NVMe); *types.NasDatastoreInfo→NFS. Return unknown only when the topology lookup genuinely fails.
C2 — Every standard vSwitch is silently dropped. vswitches.go:65 keys a map by pg.Spec.Name but looks up using vsw.Portgroup (which holds keys like key-vim.host.PortGroup-VM Network). Fix: key the map by pg.Key and assert in TestGetSwitches that at least one row has Type == "standard" with TotalPorts == 1536 / UsedPorts == 6.
C3 — Test suite is not load-bearing. Four mutations (hardcode Type="unknown", read uncommitted instead of committed, delete standard-vSwitch block, hardcode VCPU/RAM) each leave go test ./... green. Delete the two tautological tests (TestConfigPrecedenceFlagOverEnv at integration_test.go:213-223 and the impossible used+available != capacity test) and replace with real ones. Add exactness assertions (spec:141 "returns exactly that set"), add a standard-portgroup case, stop hardcoding "DC0_DVPG0".
C4 — Fabricated constants. vswitches.go:132-141 hardcodes vlanID="N/A", usedPorts=0, Name="N/A". Fix: resolve Config.DistributedVirtualSwitch to the DVS name; type-switch Vlan over VlanIdSpec/TrunkVlanSpec/PvlanSpec; replace usedPorts := 0 with a real FetchDVPorts count.
High Findings (must fix)
1. Error swallowing — four bare continues at vswitches.go:54,117,129,188 discard API failures. Wrap with %w and surface with non-zero exit.
2. Verified panics — vms.go:52-53 and vswitches.go:210-211 read vmMo.Config.Hardware.NumCPU with no nil guard on Config. Add guard + regression test.
3. Multi-datacenter unusable — all retrievers call finder.DefaultDatacenter. Replace with DatacenterList iteration or root-folder ContainerView.
4. Datastore USED reports provisioned — datastores.go:52-55 overrides used with summary.uncommitted. Delete the override; tighten test to exact used + available == capacity.
5. make verify doesn't verify — it never starts vcsim, never invokes the binary, never passes --portgroup. Fix to start vcsim, poll for ready, run all three subcommands plus a discovered --portgroup, trap teardown, and gate on gofmt -l . | grep . && exit 1.
Medium/Low (fix most impactful)
- Replace all six per-object .Properties() loops with ContainerView + PropertyCollector (H1)
- Fix format.Bytes overflow bug (*1024 in overflow branch) and its reverse-engineered test expectation
- gofmt -w the two failing files
- Delete dead functions (ClassifyFromHBA, FormatError, unused netMo)
- Join all uplinks and strip key prefix
- Render VLAN 0 as 0, not N/A
- Add README and run-evidence deliverables
- Remove committed binary
- ExecuteContext + signal.NotifyContext
- Pass credentials only to sm.Login, use net.JoinHostPort
Verification
After fixes, go test ./... -race -count=1 must pass with 0 failures, 0 skips. go build ./... and go vet ./... must be clean. gofmt -l . must return empty. The mutations M1-M4 must now cause test failures.
