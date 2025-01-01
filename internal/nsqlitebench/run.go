package nsqlitebench

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/nsqlite/nsqlite/internal/version"
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

	tmpDir, err := os.MkdirTemp("", "nsqbench_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	mattnDb, err := createMattnDriver(tmpDir)
	if err != nil {
		return fmt.Errorf("error opening mattn/go-sqlite3 db: %w", err)
	}
	defer mattnDb.Close()

	nsqliteDb, err := createNsqliteDriver(tmpDir)
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
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Format.Header = text.FormatDefault
	tw.Style().Color.Header = text.Colors{text.FgCyan, text.Bold}
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
