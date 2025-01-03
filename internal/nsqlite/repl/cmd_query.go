package repl

import (
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlitego/nsqlitehttp"
)

func cmdQuery(r *Repl, input string) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Format.Header = text.FormatDefault
	tw.Style().Color.Header = text.Colors{text.FgCyan, text.Bold}

	res, err := r.client.Query(context.TODO(), nsqlitehttp.Query{
		TxId:  r.txId,
		Query: input,
	})
	if err != nil && res.Error == "" {
		tw.AppendHeader(table.Row{"Error"})
		tw.AppendRow(table.Row{err.Error()})
	}

	if res.Type == nsqlitehttp.QueryResponseError {
		tw.AppendHeader(table.Row{"Error"})
		tw.AppendRow(table.Row{r.cleanError(res.Error)})

		if strings.Contains(res.Error, db.ErrTransactionNotFound.Error()) {
			r.setTxId("")
		}
	}

	if res.Type == nsqlitehttp.QueryResponseOK {
		tw.AppendHeader(table.Row{"OK"})
		tw.AppendRow(table.Row{"OK"})
	}

	if res.Type == nsqlitehttp.QueryResponseBegin {
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

	if res.Type == nsqlitehttp.QueryResponseCommit {
		r.setTxId("")
		tw.AppendHeader(table.Row{"OK"})
		tw.AppendRow(table.Row{"Transaction committed"})
	}

	if res.Type == nsqlitehttp.QueryResponseRollback {
		r.setTxId("")
		tw.AppendHeader(table.Row{"OK"})
		tw.AppendRow(table.Row{"Transaction rolled back"})
	}

	if res.Type == nsqlitehttp.QueryResponseWrite {
		tw.AppendHeader(table.Row{"-", "Rows Affected", "Last Insert ID"})
		tw.AppendRow(table.Row{"OK", res.RowsAffected, res.LastInsertID})
	}

	if res.Type == nsqlitehttp.QueryResponseRead {
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
	if res.Time > 0 {
		color.RGB(128, 128, 128).Printf("Time: %f seconds\n", res.Time)
	}
	fmt.Println()
}
