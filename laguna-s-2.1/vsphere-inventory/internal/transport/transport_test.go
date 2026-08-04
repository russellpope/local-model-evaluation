package transport

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyHBA(t *testing.T) {
	tests := []struct {
		name string
		hba  types.BaseHostHostBusAdapter
		want string
	}{
		{
			name: "FibreChannelHba",
			hba: &types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.FibreChannelHba-vmhba1",
				},
			},
			want: "FC",
		},
		{
			name: "FibreChannelOverEthernetHba",
			hba: &types.HostFibreChannelOverEthernetHba{
				HostFibreChannelHba: types.HostFibreChannelHba{
					HostHostBusAdapter: types.HostHostBusAdapter{
						Key: "key-vim.host.FibreChannelOverEthernetHba-vmhba2",
					},
				},
			},
			want: "FC",
		},
		{
			name: "InternetScsiHba",
			hba: &types.HostInternetScsiHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.InternetScsiHba-vmhba3",
				},
			},
			want: "iSCSI",
		},
		{
			name: "ParallelScsiHba (unknown)",
			hba: &types.HostParallelScsiHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.ParallelScsiHba-vmhba0",
				},
			},
			want: "unknown",
		},
		{
			name: "BlockHba (unknown)",
			hba: &types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.BlockHba-vmhba4",
				},
			},
			want: "unknown",
		},
		{
			name: "BlockHba with nvme protocol",
			hba: &types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:             "key-vim.host.BlockHba-vmhba5",
					StorageProtocol: "nvme",
				},
			},
			want: "NVMe",
		},
		{
			name: "BlockHba with scsi protocol",
			hba: &types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:             "key-vim.host.BlockHba-vmhba6",
					StorageProtocol: "scsi",
				},
			},
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHBA(tt.hba)
			if got != tt.want {
				t.Errorf("classifyHBA() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyByScsiTopology(t *testing.T) {
	fcHba := &types.HostFibreChannelHba{
		HostHostBusAdapter: types.HostHostBusAdapter{
			Key: "key-vim.host.FibreChannelHba-vmhba1",
		},
	}
	iscsiHba := &types.HostInternetScsiHba{
		HostHostBusAdapter: types.HostHostBusAdapter{
			Key: "key-vim.host.InternetScsiHba-vmhba2",
		},
	}
	nvmeHba := &types.HostBlockHba{
		HostHostBusAdapter: types.HostHostBusAdapter{
			Key:             "key-vim.host.BlockHba-vmhba3",
			StorageProtocol: "nvme",
		},
	}
	parallelScsiHba := &types.HostParallelScsiHba{
		HostHostBusAdapter: types.HostHostBusAdapter{
			Key: "key-vim.host.ParallelScsiHba-vmhba0",
		},
	}

	lunKey := "key-vim.host.ScsiLun-0"
	lun := &types.ScsiLun{
		HostDevice: types.HostDevice{},
		Key:        lunKey,
	}

	tests := []struct {
		name       string
		hostMo     mo.HostSystem
		lun        *types.ScsiLun
		wantType   string
		wantReason string
	}{
		{
			name: "FC via SCSI topology",
			hostMo: mo.HostSystem{
				ManagedEntity: mo.ManagedEntity{Name: "host1"},
				Config: &types.HostConfigInfo{
					StorageDevice: &types.HostStorageDeviceInfo{
						HostBusAdapter: []types.BaseHostHostBusAdapter{fcHba},
						ScsiTopology: &types.HostScsiTopology{
							Adapter: []types.HostScsiTopologyInterface{
								{
									Adapter: "key-vim.host.FibreChannelHba-vmhba1",
									Target: []types.HostScsiTopologyTarget{
										{
											Lun: []types.HostScsiTopologyLun{
												{ScsiLun: lunKey},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			lun:        lun,
			wantType:   "FC",
			wantReason: "",
		},
		{
			name: "iSCSI via SCSI topology",
			hostMo: mo.HostSystem{
				ManagedEntity: mo.ManagedEntity{Name: "host2"},
				Config: &types.HostConfigInfo{
					StorageDevice: &types.HostStorageDeviceInfo{
						HostBusAdapter: []types.BaseHostHostBusAdapter{iscsiHba},
						ScsiTopology: &types.HostScsiTopology{
							Adapter: []types.HostScsiTopologyInterface{
								{
									Adapter: "key-vim.host.InternetScsiHba-vmhba2",
									Target: []types.HostScsiTopologyTarget{
										{
											Lun: []types.HostScsiTopologyLun{
												{ScsiLun: lunKey},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			lun:        lun,
			wantType:   "iSCSI",
			wantReason: "",
		},
		{
			name: "NVMe via SCSI topology",
			hostMo: mo.HostSystem{
				ManagedEntity: mo.ManagedEntity{Name: "host3"},
				Config: &types.HostConfigInfo{
					StorageDevice: &types.HostStorageDeviceInfo{
						HostBusAdapter: []types.BaseHostHostBusAdapter{nvmeHba},
						ScsiTopology: &types.HostScsiTopology{
							Adapter: []types.HostScsiTopologyInterface{
								{
									Adapter: "key-vim.host.BlockHba-vmhba3",
									Target: []types.HostScsiTopologyTarget{
										{
											Lun: []types.HostScsiTopologyLun{
												{ScsiLun: lunKey},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			lun:        lun,
			wantType:   "NVMe",
			wantReason: "",
		},
		{
			name: "parallel SCSI via SCSI topology (unknown)",
			hostMo: mo.HostSystem{
				ManagedEntity: mo.ManagedEntity{Name: "host4"},
				Config: &types.HostConfigInfo{
					StorageDevice: &types.HostStorageDeviceInfo{
						HostBusAdapter: []types.BaseHostHostBusAdapter{parallelScsiHba},
						ScsiTopology: &types.HostScsiTopology{
							Adapter: []types.HostScsiTopologyInterface{
								{
									Adapter: "key-vim.host.ParallelScsiHba-vmhba0",
									Target: []types.HostScsiTopologyTarget{
										{
											Lun: []types.HostScsiTopologyLun{
												{ScsiLun: lunKey},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			lun:        lun,
			wantType:   "unknown",
			wantReason: "",
		},
		{
			name: "no SCSI topology (degraded)",
			hostMo: mo.HostSystem{
				ManagedEntity: mo.ManagedEntity{Name: "host5"},
				Config: &types.HostConfigInfo{
					StorageDevice: &types.HostStorageDeviceInfo{},
				},
			},
			lun:        lun,
			wantType:   "unknown",
			wantReason: "no SCSI topology on host host5",
		},
		{
			name: "LUN not found in topology (degraded)",
			hostMo: mo.HostSystem{
				ManagedEntity: mo.ManagedEntity{Name: "host6"},
				Config: &types.HostConfigInfo{
					StorageDevice: &types.HostStorageDeviceInfo{
						HostBusAdapter: []types.BaseHostHostBusAdapter{fcHba},
						ScsiTopology: &types.HostScsiTopology{
							Adapter: []types.HostScsiTopologyInterface{
								{
									Adapter: "key-vim.host.FibreChannelHba-vmhba1",
									Target: []types.HostScsiTopologyTarget{
										{
											Lun: []types.HostScsiTopologyLun{
												{ScsiLun: "different-key"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			lun:        lun,
			wantType:   "unknown",
			wantReason: "could not find HBA for LUN " + lunKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := classifyByScsiTopology(tt.hostMo, tt.lun)
			if err != nil {
				t.Fatalf("classifyByScsiTopology() error = %v", err)
			}
			if result.Type != tt.wantType {
				t.Errorf("classifyByScsiTopology() type = %q, want %q", result.Type, tt.wantType)
			}
			if tt.wantReason != "" && result.Reason != tt.wantReason {
				t.Errorf("classifyByScsiTopology() reason = %q, want %q", result.Reason, tt.wantReason)
			}
		})
	}
}

func TestClassifyDatastore(t *testing.T) {
	tests := []struct {
		name     string
		info     types.BaseDatastoreInfo
		wantType string
	}{
		{
			name:     "NasDatastoreInfo -> NFS",
			info:     &types.NasDatastoreInfo{},
			wantType: "NFS",
		},
		{
			name:     "LocalDatastoreInfo -> unknown",
			info:     &types.LocalDatastoreInfo{},
			wantType: "unknown",
		},
		{
			name:     "VsanDatastoreInfo -> unknown",
			info:     &types.VsanDatastoreInfo{},
			wantType: "unknown",
		},
		{
			name:     "VmfsDatastoreInfo with nil Vmfs -> unknown",
			info:     &types.VmfsDatastoreInfo{},
			wantType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsMo := mo.Datastore{
				Info: tt.info,
			}
			result, err := ClassifyDatastore(context.TODO(), nil, dsMo)
			if err != nil {
				t.Fatalf("ClassifyDatastore() error = %v", err)
			}
			if result.Type != tt.wantType {
				t.Errorf("ClassifyDatastore() type = %q, want %q", result.Type, tt.wantType)
			}
		})
	}
}
