// Package format implements the pure presentation helpers shared by the
// inventory commands: human-readable byte sizes and the used-capacity math.
package format

import "fmt"

const (
	gib = 1 << 30
	tib = 1 << 40
)

// HumanBytes renders a byte count using GiB/TiB units with one decimal
// place. Values below 1 GiB render as "x.x GiB"; values of 1 TiB or more
// render as "x.x TiB".
func HumanBytes(b int64) string {
	switch {
	case b >= tib:
		return fmt.Sprintf("%.1f TiB", float64(b)/tib)
	default:
		return fmt.Sprintf("%.1f GiB", float64(b)/gib)
	}
}

// HumanGB renders a quantity expressed in MB (as the vSphere API reports
// memory) as GB with one decimal place.
func HumanGB(mb int64) string {
	return fmt.Sprintf("%.1f GB", float64(mb)/1024)
}

// UsedBytes returns the consumed portion of a capacity given the total and
// the available amount: used = total - available.
func UsedBytes(total, available int64) int64 {
	return total - available
}
