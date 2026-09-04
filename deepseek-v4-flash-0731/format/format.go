// Package format renders byte quantities as human-readable GiB/TiB figures
// with a single decimal place, as required by the CLI's output contract.
package format

import "fmt"

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
	TiB = 1024 * GiB
)

// Bytes renders a byte count in GiB (or TiB once the count reaches one TiB)
// with one decimal place, e.g. "12.3 GiB" or "1.5 TiB".
func Bytes(n int64) string {
	if n >= TiB {
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(TiB))
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
}

// GigabytesFromMiB converts a MiB count to a GiB figure with one decimal
// place, e.g. 4096 -> "4.0". Used for the vms RAM column.
func GigabytesFromMiB(miB int64) string {
	return fmt.Sprintf("%.1f", float64(miB)*float64(MiB)/float64(GiB))
}
