package repl

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nsqlite/nsqlite/internal/nsqlite/styled"
)

type dotCmd struct {
	name         string
	help         string
	autocomplete bool
}

func cmdHelpCommands() []dotCmd {
	return []dotCmd{
		{name: ".tables", help: "List all tables in the database", autocomplete: true},
		{name: ".stats", help: "Shows the server stats of last 5 minutes", autocomplete: true},
		{name: ".stats N", help: "Shows the server stats of last N minutes", autocomplete: false},
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

	tw := styled.NewTableWriter()
	tw.AppendHeader(table.Row{"Command", "Description"})

	for _, cmd := range cmds {
		tw.AppendRow(table.Row{cmd.name, cmd.help})
	}

	fmt.Println(tw.Render())
}

func cmdHelpCompleter(line string) []string {
	suggestions := []string{
		"SELECT ",
		"SELECT * FROM ",
		"SELECT COUNT(*) FROM ",
		"INSERT INTO ",
		"UPDATE",
		"DELETE FROM ",
		"CREATE TABLE ",
		"DROP TABLE ",
		"ALTER TABLE ",
	}

	for _, cmd := range cmdHelpCommands() {
		if cmd.autocomplete {
			suggestions = append(suggestions, cmd.name)
		}
	}

	results := []string{}
	for _, suggestion := range suggestions {
		if strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(line)) {
			results = append(results, suggestion)
		}
	}

	return results
}
