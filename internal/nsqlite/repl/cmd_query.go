package repl

import (
	"fmt"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
)

func cmdQuery(r *Repl, input string) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Format.Header = text.FormatDefault
	tw.Style().Color = table.ColorOptions{
		Header:       text.Colors{text.FgHiWhite, text.Bold},
		IndexColumn:  text.Colors{text.FgWhite},
		Row:          text.Colors{text.FgWhite},
		RowAlternate: text.Colors{text.FgWhite},
		Footer:       text.Colors{text.FgWhite},
	}

	res, err := r.clientInst.SendQuery(input, r.txId)
	if err != nil && res.Error == "" {
		tw.AppendHeader(table.Row{"Error"})
		tw.AppendRow(table.Row{err.Error()})
	}

	if res.Type == "error" {
		tw.AppendHeader(table.Row{"Error"})
		tw.AppendRow(table.Row{r.cleanError(res.Error)})
	}

	if res.Type == "ok" {
		tw.AppendHeader(table.Row{"OK"})
		tw.AppendRow(table.Row{"OK"})
	}

	if res.Type == "begin" {
		if res.TxId == "" {
			tw.AppendHeader(table.Row{"Error"})
			tw.AppendRow(table.Row{"No transaction ID returned"})
		}
		if res.TxId != "" {
			r.setTxId(res.TxId)
			tw.AppendHeader(table.Row{"OK"})
			tw.AppendRow(table.Row{"Transaction started"})
		}
	}

	if res.Type == "commit" {
		r.setTxId("")
		tw.AppendHeader(table.Row{"OK"})
		tw.AppendRow(table.Row{"Transaction committed"})
	}

	if res.Type == "rollback" {
		r.setTxId("")
		tw.AppendHeader(table.Row{"OK"})
		tw.AppendRow(table.Row{"Transaction rolled back"})
	}

	if res.Type == "write" {
		tw.AppendHeader(table.Row{"-", "Rows Affected", "Last Insert ID"})
		tw.AppendRow(table.Row{"OK", res.RowsAffected, res.LastInsertID})
	}

	if res.Type == "read" {
		header := table.Row{}
		for _, col := range res.Columns {
			header = append(header, col)
		}
		tw.AppendHeader(header)

		for _, value := range res.Values {
			tw.AppendRow(value)
		}
	}

	fmt.Println(tw.Render())
}
