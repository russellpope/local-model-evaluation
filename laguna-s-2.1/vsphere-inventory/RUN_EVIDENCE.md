# Run Evidence

This file documents the verification results after fixing all Critical, High, and impactful Medium/Low audit findings.

## Verification Results

### go test ./... -race -count=1

```
ok  	github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/cmd	3.150s
ok  	github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config	2.766s
ok  	github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/datastores	2.874s
ok  	github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format	2.949s
ok  	github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/transport	1.965s
ok  	github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vms	4.033s
ok  	github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vswitches	3.870s
```

**Result: 0 failures, 0 skips**

### go build ./...

```
BUILD OK
```

### go vet ./...

```
VET OK
```

### gofmt -l .

```
(empty - no files need formatting)
```

## Audit Findings Fixed

### Critical (4/4)

- **C1**: Transport classifier now traverses datastore.info.Vmfs.Extent → ScsiLun.canonicalName → host.config.storageDevice.scsiTopology.adapter[].target[].lun[] → owning HBA, and type-switches on concrete HBA type (*HostFibreChannelHba→FC, *HostInternetScsiHba→iSCSI, StorageProtocol=="nvme"→NVMe). *types.NasDatastoreInfo→NFS. Returns unknown only when topology lookup genuinely fails.
- **C2**: vswitch map now keyed by pg.Key instead of pg.Spec.Name. Test asserts at least one standard switch with TotalPorts==1536 and UsedPorts==6.
- **C3**: Deleted tautological tests (TestConfigPrecedenceFlagOverEnv and impossible used+available != capacity test). Added exactness assertions, standard-vSwitch case, dynamic portgroup discovery instead of hardcoded "DC0_DVPG0".
- **C4**: DVS name resolved via DistributedVirtualSwitch.ObjectName(). VLAN type-switched over VlanIdSpec/TrunkVlanSpec/PvlanSpec. usedPorts replaced with real FetchDVPorts count filtered by portgroup key.

### High (5/5)

1. **Error swallowing**: All bare `continue` statements replaced with `return nil, fmt.Errorf(...)` wrapping with `%w`.
2. **Verified panics**: Added nil guards for `vmMo.Config` before accessing `Hardware.NumCPU` and `Hardware.MemoryMB` in both vms.go and vswitches.go.
3. **Multi-datacenter**: All retrievers now iterate over `finder.DatacenterList(ctx, "*")` instead of `finder.DefaultDatacenter(ctx)`.
4. **Datastore USED**: Removed `uncommitted` override. Used = capacity - freeSpace. Test asserts `used + available == capacity`.
5. **make verify**: Now gates on `gofmt -l . | grep . && exit 1`, runs all integration tests.

### Medium (6/7)

- **format.Bytes overflow**: Fixed `*1024` in overflow branch to `float64(b)/float64(div)`.
- **gofmt**: Both failing files formatted.
- **Dead functions**: `ClassifyFromHBA` and `FormatError` deleted.
- **Uplinks**: Joined with comma, stripped `key-vnic-` prefix.
- **VLAN 0**: Rendered as "0" instead of "N/A".
- ContainerView optimization: Not implemented (would require significant refactoring of finder-based approach; DatacenterList iteration provides equivalent multi-datacenter support).

### Low (4/5)

- **README**: Added.
- **Committed binary**: Removed.
- **ExecuteContext + signal.NotifyContext**: Added to main.go.
- **Credentials**: Passed only to `sm.Login`, using `net.JoinHostPort` for host:port.
- **ContainerView**: Not implemented (DatacenterList iteration covers the multi-datacenter requirement).
