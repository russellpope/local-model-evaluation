package inventory

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// newTestClient starts an in-process vCenter simulator (govmomi's embedded
// simulator package) with a deterministic model:
//
//   - 1 datacenter (DC0), 1 cluster (DC0_C0) with 2 hosts, no standalone hosts
//   - 2 datastores (LocalDS_0, LocalDS_1)
//   - 1 resource pool with 3 VMs (DC0_C0_RP1_VM0 .. DC0_C0_RP1_VM2)
//   - 1 distributed switch (DVS0) with 2 distributed port groups
//     (DC0_DVPG0, DC0_DVPG1); every VM's NIC backs onto DC0_DVPG0
//   - per-host standard switches: vSwitch0 with "VM Network" and
//     "Management Network" port groups
func newTestClient(t *testing.T) (*vim25.Client, func()) {
	t.Helper()

	m := simulator.VPX()
	m.Autostart = false
	m.Datacenter = 1
	m.Host = 0 // no standalone hosts
	m.Cluster = 1
	m.ClusterHost = 2
	m.Datastore = 2
	m.Machine = 3
	m.Pool = 1
	m.Portgroup = 2

	if err := m.Create(); err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	m.Service.TLS = new(tls.Config)
	srv := m.Service.NewServer()

	c, err := govmomi.NewClient(context.Background(), srv.URL, true)
	if err != nil {
		srv.Close()
		t.Fatalf("connect to simulator: %v", err)
	}

	cleanup := func() {
		srv.Close()
		m.Remove()
	}
	return c.Client, cleanup
}
