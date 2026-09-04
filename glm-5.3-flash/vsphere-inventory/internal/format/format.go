// Package format provides pure helpers for rendering inventory values:
// human-readable byte sizes and the used-capacity arithmetic shared by the
// table renderers. Keeping these pure makes them trivially testable.
package format

import "fmt"

const (
	GiB int64 = 1 << 30
	TiB int64 = 1 << 40
)

// FormatBytes renders a byte count in GiB, or TiB for sizes of 1 TiB and
// above, always with exactly one decimal place so table columns line up.
func FormatBytes(b int64) string {
	if b >= TiB {
		return fmt.Sprintf("%.1f TiB", float64(b)/float64(TiB))
	}
	return fmt.Sprintf("%.1f GiB", float64(b)/float64(GiB))
}

// FormatGB renders a mebibyte count as gibibytes (GB in vSphere UI terms)
// with one decimal place. Used for the VM RAM column.
func FormatGB(mib int64) string {
	return fmt.Sprintf("%.1f", float64(mib)/1024)
}

// UsedCapacity returns the consumed portion of a datastore:
// used = total - available(free). A free value larger than capacity is
// treated as a fully empty volume rather than a negative usage.
func UsedCapacity(total, free int64) int64 {
	if free >= total {
		return 0
	}
	return total - free
}
