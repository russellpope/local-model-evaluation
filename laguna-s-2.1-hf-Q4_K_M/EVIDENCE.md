# Evidence: Test & vcsim Output

## go vet

```
(no output — clean)
```

## gofmt

```
(no output — all files formatted)
```

## go test ./...

```
?   github.com/local-model-evaluation/vsphere-inventory-cli/cmd/vcsim        [no test files]
?   github.com/local-model-evaluation/vsphere-inventory-cli/cmd/vsphere-inventory [no test files]
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/client  (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/config  (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/datastores (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/format  (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/transport (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/vms     (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/vswitch 2.371s
```

## make verify (vcsim integration)

```
go vet ./...
go test ./...
?   github.com/local-model-evaluation/vsphere-inventory-cli/cmd/vcsim        [no test files]
?   github.com/local-model-evaluation/vsphere-inventory-cli/cmd/vsphere-inventory [no test files]
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/client  (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/config  (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/datastores (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/format  (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/transport (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/vms     (cached)
ok  github.com/local-model-evaluation/vsphere-inventory-cli/internal/vswitch (cached)
=== Building vcsim ===
=== Starting vcsim simulator ===
vcsim PID: 40810
=== Waiting for vcsim to be ready ===
vcsim is listening on https://user:pass@127.0.0.1:8989/sdk
username: user
password: pass
insecure: true

vcsim is ready
=== Building binary ===
=== Running vms subcommand ===
NAME            VCPU  RAM       STORAGE
DC0_C0_RP0_VM0  1     32.0 MiB  0.0 MiB
DC0_C0_RP0_VM1  1     32.0 MiB  0.0 MiB
DC0_C0_RP0_VM2  1     32.0 MiB  0.0 MiB
DC0_C0_RP0_VM3  1     32.0 MiB  0.0 MiB
DC0_C0_RP0_VM4  1     32.0 MiB  0.0 MiB
DC0_C0_RP0_VM5  1     32.0 MiB  0.0 MiB
DC0_C0_RP0_VM6  1     32.0 MiB  0.0 MiB
DC0_C0_RP0_VM7  1     32.0 MiB  0.0 MiB
=== Running datastores subcommand ===
NAME       TYPE     USED      AVAILABLE
LocalDS_0  unknown  80.0 GiB  2.9 TiB
LocalDS_1  unknown  0.0 MiB   3.0 TiB
LocalDS_2  unknown  0.0 MiB   3.0 TiB
=== Running vswitches subcommand ===
SWITCH    SWITCH TYPE  PORTGROUP           VLAN    UPLINKS                          LACP  PORTS  USED
DVS0      distributed  DC0_DVPG0           N/A     N/A                              N/A   1      1
DVS0      distributed  DC0_DVPG1           N/A     N/A                              N/A   1      1
DVS0      distributed  DC0_DVPG2           N/A     N/A                              N/A   1      1
DVS0      distributed  DVS0-DVUplinks-8    0-4094  N/A                              N/A   1      1
vSwitch0  standard     Management Network  N/A     key-vim.host.PhysicalNic-vmnic0  N/A   1536   6
vSwitch0  standard     VM Network          N/A     key-vim.host.PhysicalNic-vmnic0  N/A   1536   6
=== Discovering portgroup name ===
Using portgroup: DC0_DVPG0
=== Running vswitches --portgroup DC0_DVPG0 ===
NAME
DC0_C0_RP0_VM0
DC0_C0_RP0_VM1
DC0_C0_RP0_VM2
DC0_C0_RP0_VM3
DC0_C0_RP0_VM4
DC0_C0_RP0_VM5
DC0_C0_RP0_VM6
DC0_C0_RP0_VM7
=== Stopping vcsim ===

Shutting down vcsim...
=== All checks passed ===
```
