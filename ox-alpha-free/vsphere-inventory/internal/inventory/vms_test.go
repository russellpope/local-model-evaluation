package inventory

import (
	"context"
	"testing"
)

func TestListVMs(t *testing.T) {
	ctx := context.Background()
	c, done := newTestClient(t)
	defer done()

	vms, err := ListVMs(ctx, c)
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}

	const wantCount = 3 // Model.Machine = 3, one resource pool
	if len(vms) != wantCount {
		t.Fatalf("ListVMs() returned %d VMs, want %d: %v", len(vms), wantCount, vms)
	}

	for i, vm := range vms {
		if i > 0 && vms[i-1].Name >= vm.Name {
			t.Errorf("ListVMs() not sorted by name at index %d: %q after %q", i, vm.Name, vms[i-1].Name)
		}
		if vm.Name == "" || vm.Name == "unknown" {
			t.Errorf("VM %d has no usable name", i)
		}
		if vm.VCPU <= 0 {
			t.Errorf("VM %q has %d vCPUs, want > 0", vm.Name, vm.VCPU)
		}
		if vm.RAMGB <= 0 {
			t.Errorf("VM %q has %g GB RAM, want > 0", vm.Name, vm.RAMGB)
		}
		if vm.StorageBytes < 0 {
			t.Errorf("VM %q has negative committed storage %d", vm.Name, vm.StorageBytes)
		}
	}

	if vms[0].Name != "DC0_C0_RP1_VM0" {
		t.Errorf("first VM name = %q, want %q (simulator naming)", vms[0].Name, "DC0_C0_RP1_VM0")
	}
}
