package datastores

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/transport"
)

func TestGetDatastores(t *testing.T) {
	model := simulator.VPX()
	model.Datastore = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		results, err := GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores: %v", err)
		}

		if len(results) != model.Datastore {
			t.Errorf("expected %d datastores, got %d", model.Datastore, len(results))
		}

		validTypes := map[transport.Transport]bool{
			transport.TransportFC:      true,
			transport.TransportISCSI:   true,
			transport.TransportNVMe:    true,
			transport.TransportNFS:     true,
			transport.TransportUnknown: true,
		}

		for _, ds := range results {
			if ds.Name == "" {
				t.Error("datastore has empty name")
			}
			if !validTypes[ds.Type] {
				t.Errorf("datastore %q has invalid type %q", ds.Name, ds.Type)
			}
			if strings.HasPrefix(ds.Name, "Local") && ds.Type == transport.TransportNVMe {
				t.Errorf("local datastore %q should not report NVMe transport, got %q", ds.Name, ds.Type)
			}
			if ds.Capacity < 0 {
				t.Errorf("datastore %q has negative capacity: %d", ds.Name, ds.Capacity)
			}
			if ds.Free < 0 {
				t.Errorf("datastore %q has negative free: %d", ds.Name, ds.Free)
			}
			if ds.Free > ds.Capacity {
				t.Errorf("datastore %q has free > capacity: %d > %d", ds.Name, ds.Free, ds.Capacity)
			}
			expectedUsed := ds.Capacity - ds.Free
			if ds.Used != expectedUsed {
				t.Errorf("datastore %q used = %d, want %d (capacity - free)", ds.Name, ds.Used, expectedUsed)
			}
		}

		for i := 1; i < len(results); i++ {
			if results[i-1].Name > results[i].Name {
				t.Error("datastores are not sorted by name")
			}
		}
	}, model)
}

func TestClassifyTargetTransport(t *testing.T) {
	tests := []struct {
		name      string
		transport types.BaseHostTargetTransport
		expected  transport.Transport
	}{
		{"FibreChannel", &types.HostFibreChannelTargetTransport{}, transport.TransportFC},
		{"iSCSI", &types.HostInternetScsiTargetTransport{}, transport.TransportISCSI},
		{"PCIe", &types.HostPcieTargetTransport{}, transport.TransportNVMe},
		{"BlockAdapter", &types.HostBlockAdapterTargetTransport{}, transport.TransportUnknown},
		{"nil", nil, transport.TransportUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyTargetTransport(tt.transport)
			if result != tt.expected {
				t.Errorf("classifyTargetTransport(%T) = %q, want %q", tt.transport, result, tt.expected)
			}
		})
	}
}

func TestGetDatastoresDefault(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		results, err := GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores: %v", err)
		}

		if len(results) < 1 {
			t.Error("expected at least 1 datastore, got 0")
		}

		for _, ds := range results {
			if ds.Name == "" {
				t.Error("datastore has empty name")
			}
			if ds.Free > ds.Capacity {
				t.Errorf("datastore %q has free > capacity", ds.Name)
			}
		}
	})
}
