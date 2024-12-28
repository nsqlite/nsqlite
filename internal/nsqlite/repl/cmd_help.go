package repl

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
)

type dotCmd struct {
	name         string
	help         string
	autocomplete bool
}

func cmdHelpCommands() []dotCmd {
	return []dotCmd{
		{name: ".tables", help: "List all tables in the database", autocomplete: true},
		{name: ".clear", help: "Clear the terminal screen", autocomplete: true},
		{name: ".help", help: "Show the help message", autocomplete: true},
		{name: ".quit", help: "Exit the application", autocomplete: true},
		{name: ".exit", help: "Exit the application", autocomplete: true},
		{name: "CTRL+c", help: "Exit the application"},
	}
}

func cmdHelp() {
	fmt.Println("Available commands:")
	cmds := cmdHelpCommands()

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Format.Header = text.FormatDefault
	tw.Style().Color.Header = text.Colors{text.FgHiWhite, text.Bold}
	tw.AppendHeader(table.Row{"Command", "Description"})

	for _, cmd := range cmds {
		tw.AppendRow(table.Row{cmd.name, cmd.help})
	}

	fmt.Println(tw.Render())
}

func cmdHelpCompleter(line string) []string {
	suggestions := map[string]string{
		"SELECT ":               "Select data from a table",
		"SELECT * FROM ":        "Select all columns from a table",
		"SELECT COUNT(*) FROM ": "Count the number of rows in a table",
		"INSERT INTO ":          "Insert data into a table",
		"UPDATE":                "Update data in a table",
		"DELETE FROM ":          "Delete data from a table",
		"CREATE TABLE ":         "Create a new table",
		"DROP TABLE ":           "Drop a table",
		"ALTER TABLE ":          "Alter a table",
	}

	for _, cmd := range cmdHelpCommands() {
		if cmd.autocomplete {
			suggestions[cmd.name] = cmd.help
		}
	}

	results := []string{}
	for key := range suggestions {
		if strings.HasPrefix(strings.ToLower(key), strings.ToLower(line)) {
			results = append(results, key)
		}
	}

	return results
}
