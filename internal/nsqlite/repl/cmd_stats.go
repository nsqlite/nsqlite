package repl

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/nsqlite/nsqlite/internal/util/numutil"
)

func cmdStats(r *Repl, statsQty int) {
	stats, err := r.client.GetStats(context.Background())
	if err != nil {
		fmt.Println("Failed to get stats:", err)
		return
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Format.Header = text.FormatDefault
	tw.Style().Color.Header = text.Colors{text.FgCyan, text.Bold}
	tw.Style().Color.Footer = text.Colors{text.FgCyan, text.Bold}

	tw.AppendHeader(table.Row{"Minute (UTC)", "Reads", "Writes", "Begins", "Commits", "Rollbacks", "Requests"})

	rows := []table.Row{}
	for i, stat := range stats.Stats {
		if i >= statsQty {
			break
		}

		minute, err := time.Parse(time.RFC3339, stat.Minute)
		if err != nil {
			continue
		}

		rows = append(rows, table.Row{
			minute.Format("2006-01-02 15:04"),
			numutil.IntWithCommas(stat.Reads),
			numutil.IntWithCommas(stat.Writes),
			numutil.IntWithCommas(stat.Begins),
			numutil.IntWithCommas(stat.Commits),
			numutil.IntWithCommas(stat.Rollbacks),
			numutil.IntWithCommas(stat.HTTPRequests),
		})
	}
	slices.Reverse(rows)
	tw.AppendRows(rows)

	tw.AppendFooter(table.Row{
		"Total",
		numutil.IntWithCommas(stats.Totals.Reads),
		numutil.IntWithCommas(stats.Totals.Writes),
		numutil.IntWithCommas(stats.Totals.Begins),
		numutil.IntWithCommas(stats.Totals.Commits),
		numutil.IntWithCommas(stats.Totals.Rollbacks),
		numutil.IntWithCommas(stats.Totals.HTTPRequests),
	})

	fmt.Println(tw.Render())
	color.RGB(128, 128, 128).Printf("Showing the last 5 minutes of stats\n")
	color.RGB(128, 128, 128).Printf("Uptime: %s\n", stats.Uptime)
	fmt.Println()
}
