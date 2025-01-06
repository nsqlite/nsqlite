package styled

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// NewTableWriter returns a new table.Writer with the custom
// styles for NSQLite CLI.
func NewTableWriter() table.Writer {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Format.Header = text.FormatDefault
	tw.Style().Color.Header = text.Colors{text.FgCyan, text.Bold}
	tw.Style().Color.Footer = text.Colors{text.FgCyan, text.Bold}

	return tw
}
