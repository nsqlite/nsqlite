package nsqlitebench

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type benchmarkLargeConfig struct {
	insertXUsers     int
	insertYBytes     int
	insertGoroutines int
}

// runBenchmarkLarge inserts X users with Y Bytes of content and then queries
// all of them in single query.
func runBenchmarkLarge(
	db *sql.DB, fullConfig benchmarksConfig,
) (benchmarkResult, error) {
	conf := fullConfig.benchmarkLargeConfig
	start := time.Now()
	var totalReads uint64 = 0
	var totalWrites uint64 = 0

	wg := sync.WaitGroup{}
	wgch := make(chan bool, conf.insertGoroutines)
	errChan := make(chan error, conf.insertXUsers)

	email := strings.Repeat("Y", conf.insertYBytes)
	for range conf.insertXUsers {
		wg.Add(1)
		wgch <- true

		go func() {
			defer func() {
				wg.Done()
				<-wgch
			}()

			res, err := db.Exec(
				"INSERT INTO users (created, email, active) VALUES (?, ?, ?)",
				time.Now().Unix(), email, 1,
			)
			if err != nil {
				errChan <- err
				return
			}

			rowsAffected, err := res.RowsAffected()
			if err != nil {
				errChan <- err
				return
			}

			atomic.AddUint64(&totalWrites, uint64(rowsAffected))
		}()
	}

	wg.Wait()
	close(wgch)
	close(errChan)

	for e := range errChan {
		if e != nil {
			return benchmarkResult{}, fmt.Errorf("error when inserting: %w", e)
		}
	}

	rows, err := db.Query(
		"SELECT id, created, email, active FROM users ORDER BY id",
	)
	if err != nil {
		return benchmarkResult{}, fmt.Errorf("error when querying: %w", err)
	}

	for rows.Next() {
		var id, created, active int
		var email string
		err = rows.Scan(&id, &created, &email, &active)
		if err != nil {
			return benchmarkResult{}, fmt.Errorf("error when scanning: %w", err)
		}

		atomic.AddUint64(&totalReads, 1)
	}

	return benchmarkResult{
		Name:        "Simple",
		Duration:    time.Since(start),
		TotalReads:  totalReads,
		TotalWrites: totalWrites,
	}, nil
}
