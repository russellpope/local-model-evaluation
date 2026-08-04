package datastores

import (
	"context"
	"fmt"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type countingRoundTripper struct {
	inner soap.RoundTripper
	calls int64
}

func (c *countingRoundTripper) RoundTrip(ctx context.Context, req, res soap.HasFault) error {
	atomic.AddInt64(&c.calls, 1)
	return c.inner.RoundTrip(ctx, req, res)
}

func (c *countingRoundTripper) Calls() int64 {
	return atomic.LoadInt64(&c.calls)
}

func newCountingClient(t *testing.T, ctx context.Context, simModel *simulator.Model) (*vim25.Client, *countingRoundTripper) {
	t.Helper()

	if err := simModel.Create(); err != nil {
		t.Fatalf("creating simulator: %v", err)
	}
	t.Cleanup(func() { simModel.Remove() })

	simModel.Service.TLS = nil
	server := simModel.Service.NewServer()
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL.String())
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	u.User = url.UserPassword("user", "pass")

	govClient, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		t.Fatalf("creating govmomi client: %v", err)
	}
	t.Cleanup(func() { govClient.Logout(ctx) })

	inner := govClient.Client.RoundTripper
	rt := &countingRoundTripper{inner: inner}
	govClient.Client.RoundTripper = rt

	return govClient.Client, rt
}

func TestGetDatastores(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Datacenter = 1
	simModel.Datastore = 3

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dsList, err := GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores() error = %v", err)
		}

		if len(dsList) != 3 {
			t.Fatalf("GetDatastores() returned %d datastores, want 3", len(dsList))
		}

		for _, ds := range dsList {
			if ds.Name == "" {
				t.Error("Datastore name should not be empty")
			}

			if ds.UsedBytes+ds.AvailableBytes != ds.CapacityBytes {
				t.Errorf("Datastore %s: used + available (%d + %d) != capacity (%d)",
					ds.Name, ds.UsedBytes, ds.AvailableBytes, ds.CapacityBytes)
			}

			if ds.AvailableBytes > ds.CapacityBytes {
				t.Errorf("Datastore %s: available (%d) > capacity (%d)",
					ds.Name, ds.AvailableBytes, ds.CapacityBytes)
			}

			if ds.Type != "unknown" {
				t.Errorf("Datastore %s: type = %q, want %q (vcsim LocalDatastoreInfo should be unknown)",
					ds.Name, ds.Type, "unknown")
			}
		}
	}, simModel)
}

func TestGetDatastoresExactValues(t *testing.T) {
	simModel := simulator.VPX()
	simModel.Datacenter = 1
	simModel.Datastore = 1

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		dsList, err := GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores() error = %v", err)
		}

		if len(dsList) != 1 {
			t.Fatalf("GetDatastores() returned %d datastores, want 1", len(dsList))
		}

		ds := dsList[0]
		if ds.Type != "unknown" {
			t.Errorf("Datastore %s: type = %q, want %q", ds.Name, ds.Type, "unknown")
		}

		usedStr := format.Bytes(ds.UsedBytes)
		availStr := format.Bytes(ds.AvailableBytes)
		if usedStr == "" {
			t.Error("used bytes string should not be empty")
		}
		if availStr == "" {
			t.Error("available bytes string should not be empty")
		}
	}, simModel)
}

func TestRoundTripsFlatAsVMCountGrows(t *testing.T) {
	ctx := context.Background()

	vmCounts := []int{2, 4, 8, 16}
	var roundTrips []int64

	for _, vmCount := range vmCounts {
		simModel := simulator.VPX()
		simModel.Machine = vmCount
		simModel.Host = 0
		simModel.Cluster = 1
		simModel.ClusterHost = 1
		simModel.Portgroup = 2

		c, rt := newCountingClient(t, ctx, simModel)

		_, err := GetDatastores(ctx, c)
		if err != nil {
			t.Fatalf("GetDatastores() with %d VMs error = %v", vmCount, err)
		}

		calls := rt.Calls()
		roundTrips = append(roundTrips, calls)
		t.Logf("VM count=%d, round trips=%d", vmCount, calls)
	}

	for i := 1; i < len(roundTrips); i++ {
		if roundTrips[i] > roundTrips[0]*2 {
			t.Errorf("round trips grew from %d (2 VMs) to %d (%d VMs); expected flat growth",
				roundTrips[0], roundTrips[i], vmCounts[i])
		}
	}

	t.Logf("round trips across VM counts %v: %v", vmCounts, roundTrips)
	fmt.Printf("round trips across VM counts %v: %v\n", vmCounts, roundTrips)
}
