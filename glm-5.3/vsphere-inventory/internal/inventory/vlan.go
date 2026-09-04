package inventory

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/vim25/types"
)

// VLAN rendering notes:
//   - "none"                 - untagged traffic (VLAN id 0)
//   - "N"                    - a single VLAN id
//   - "N-M" / "N-M,O"        - trunk ranges
//   - "private(N)"           - private VLAN port group

const (
	vlanTrunkAll   = "0-4095"
	vlanNone       = "none"
	maxVLAN        = 4095
	trunkMarker    = 4095 // standard vSwitch "all VLANs" id
	privatePrefix  = "private("
	privateSuffix  = ")"
	rangeSeparator = ","
)

// FormatStandardVLAN renders the VLAN column value for a standard (host)
// port group spec. VlanId 0 means untagged; 4095 means all VLANs pass
// through (VGT trunk).
func FormatStandardVLAN(id int32) string {
	switch {
	case id == 0:
		return vlanNone
	case id == trunkMarker:
		return vlanTrunkAll
	default:
		return strconv.FormatInt(int64(id), 10)
	}
}

// FormatDistributedVLAN renders the VLAN column value for a distributed
// port group's default port setting. Trunk and private-VLAN specs render
// their range or type rather than a single id.
func FormatDistributedVLAN(setting types.BaseDVPortSetting) string {
	if setting == nil {
		return vlanNone
	}
	spec, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || spec.Vlan == nil {
		return vlanNone
	}
	switch vlan := spec.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		if vlan.Inherited {
			return vlanNone
		}
		return FormatStandardVLAN(vlan.VlanId)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		parts := make([]string, 0, len(vlan.VlanId))
		for _, r := range vlan.VlanId {
			if r.Start == r.End {
				parts = append(parts, strconv.FormatInt(int64(r.Start), 10))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		if len(parts) == 0 {
			return vlanNone
		}
		return strings.Join(parts, rangeSeparator)
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("%s%d%s", privatePrefix, vlan.PvlanId, privateSuffix)
	default:
		return "unknown"
	}
}

// ParseVLAN validates a rendered VLAN column value and returns the set of
// VLAN ids it covers. It accepts the exact shapes produced by the formatters
// above; anything else is an error.
func ParseVLAN(s string) ([]int, error) {
	switch {
	case s == "" || s == vlanNone || s == "unknown":
		return nil, nil
	case strings.HasPrefix(s, privatePrefix) && strings.HasSuffix(s, privateSuffix):
		idStr := strings.TrimSuffix(strings.TrimPrefix(s, privatePrefix), privateSuffix)
		id, err := parseVLANID(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid private vlan %q: %w", s, err)
		}
		return []int{id}, nil
	}

	var ids []int
	for _, part := range strings.Split(s, rangeSeparator) {
		lo, hi, found := strings.Cut(part, "-")
		if !found {
			id, err := parseVLANID(part)
			if err != nil {
				return nil, fmt.Errorf("invalid vlan %q: %w", s, err)
			}
			ids = append(ids, id)
			continue
		}
		start, err := parseVLANID(lo)
		if err != nil {
			return nil, fmt.Errorf("invalid vlan range %q: %w", s, err)
		}
		end, err := parseVLANID(hi)
		if err != nil {
			return nil, fmt.Errorf("invalid vlan range %q: %w", s, err)
		}
		if start > end {
			return nil, fmt.Errorf("invalid vlan range %q: start %d > end %d", s, start, end)
		}
		for id := start; id <= end; id++ {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func parseVLANID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", s)
	}
	if id < 0 || id > maxVLAN {
		return 0, fmt.Errorf("%d outside 0..%d", id, maxVLAN)
	}
	return id, nil
}
