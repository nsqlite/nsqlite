package numutil

import "fmt"

// IntWithCommas returns a string representation of an integer with commas.
//
// Example:
//
//	12345 -> "12,345"
func IntWithCommas(i int) string {
	if i < 0 {
		return "-" + IntWithCommas(-i)
	}
	if i < 1000 {
		return fmt.Sprintf("%d", i)
	}
	return IntWithCommas(i/1000) + "," + fmt.Sprintf("%03d", i%1000)
}
