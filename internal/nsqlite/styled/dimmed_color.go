package styled

import "github.com/fatih/color"

// DimmedColor returns a dimmed *color.Color to print secondary information.
func DimmedColor() *color.Color {
	return color.RGB(128, 128, 128)
}
