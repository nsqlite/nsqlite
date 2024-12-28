package repl

import "fmt"

func cmdHelp() {
	fmt.Println("Available commands:")
	fmt.Println(".tables - List all tables in the database")
	fmt.Println(".clear  - Clear the terminal screen")
	fmt.Println(".help   - Show this help message")
	fmt.Println(".quit   - Exit the application")
	fmt.Println(".exit   - Exit the application")
	fmt.Println("CTRL+c  - Exit the application")
}
