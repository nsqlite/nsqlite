package nsqlitebench

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nsqlite/nsqlite/internal/nsqlite/styled"
	"github.com/nsqlite/nsqlite/internal/version"
	"github.com/peterh/liner"
)

// benchmarkResult stores the outcome of a benchmark.
type benchmarkResult struct {
	Name        string
	Duration    time.Duration
	TotalReads  uint64
	TotalWrites uint64
}

// Run executes benchmarks for two SQLite drivers and prints the results.
func Run(ctx context.Context) error {
	fmt.Println(version.BenchVersion())
	fmt.Println()

	tmpDir, err := os.MkdirTemp("", "nsqlitebench_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	sqliteDBPath := path.Join(tmpDir, "/benchmark.sqlite")
	fmt.Printf("The temporary SQLite database to be benchmarked will be stored in %s\n", sqliteDBPath)

	nsqliteDSN := "http://localhost:9876"
	fmt.Printf("The NSQLite server to be benchmarked is %s\n", color.RedString(nsqliteDSN))

	fmt.Println()
	color.Red("Make sure the NSQLite server is not important, as the benchmark will make changes to the database.")
	fmt.Println()

	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)

	for {
		prompt, err := line.Prompt(`Enter "start" to start the benchmark, or press CTRL+C to exit: `)
		if err != nil {
			if err == liner.ErrPromptAborted {
				fmt.Println("CTRL+C pressed, exiting...")
				return nil
			}
			return err
		}
		if prompt == "start" {
			break
		}
	}

	mattnDb, err := createMattnDriver(sqliteDBPath)
	if err != nil {
		return fmt.Errorf("error opening mattn/go-sqlite3 db: %w", err)
	}
	defer mattnDb.Close()

	nsqliteDb, err := createNsqliteDriver(nsqliteDSN)
	if err != nil {
		return fmt.Errorf("error opening nsqlite/nsqlitego db: %w", err)
	}
	defer nsqliteDb.Close()

	fmt.Println("\n--- Benchmarks for mattn/go-sqlite3 ---")
	mattnResults, err := runBenchmark(mattnDb, getMattnConfig())
	if err != nil {
		return fmt.Errorf("error benchmarking mattn/go-sqlite3: %w", err)
	}
	printResults(mattnResults)

	fmt.Println("\n--- Benchmarks for nsqlite/nsqlitego ---")
	nsqliteResults, err := runBenchmark(nsqliteDb, getNsqliteConfig())
	if err != nil {
		return fmt.Errorf("error benchmarking nsqlite/nsqlitego: %w", err)
	}
	printResults(nsqliteResults)

	return nil
}

func printResults(results []benchmarkResult) {
	tw := styled.NewTableWriter()
	tw.AppendHeader(table.Row{"Name", "Reads", "Writes", "Duration"})

	for _, r := range results {
		tw.AppendRow(table.Row{r.Name, r.TotalReads, r.TotalWrites, r.Duration})
	}

	fmt.Println(tw.Render())
}

// runBenchmark executes all benchmarks, and returns results.
//
// It recreates the schema before each benchmark.
func runBenchmark(db *sql.DB, cfg benchmarksConfig) ([]benchmarkResult, error) {
	if err := recreateSchema(db); err != nil {
		return nil, err
	}

	benchs := []func(*sql.DB, benchmarksConfig) (benchmarkResult, error){
		runBenchmarkSimple,
		runBenchmarkComplex,
		runBenchmarkMany,
		runBenchmarkLarge,
	}

	var results []benchmarkResult

	for _, bench := range benchs {
		if err := recreateSchema(db); err != nil {
			return nil, err
		}

		res, err := bench(db, cfg)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}

	return results, nil
}
