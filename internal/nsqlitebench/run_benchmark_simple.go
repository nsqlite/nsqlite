package nsqlitebench

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nsqlite/nsqlite/internal/nsqlitebench/benchbar"
)

type benchmarkSimpleConfig struct {
	insertXUsers     int
	insertGoroutines int
}

// runBenchmarkSimple inserts X users and then queries all of them in single
// query.
func runBenchmarkSimple(
	db *sql.DB, fullConfig benchmarksConfig,
) (benchmarkResult, error) {
	conf := fullConfig.benchmarkSimpleConfig
	start := time.Now()
	var totalReads uint64 = 0
	var totalWrites uint64 = 0

	wg := sync.WaitGroup{}
	wgch := make(chan bool, conf.insertGoroutines)
	errChan := make(chan error, conf.insertXUsers)
	bar := benchbar.NewBar(
		fmt.Sprintf("Inserting %d users", conf.insertXUsers), conf.insertXUsers,
	)

	for idx := range conf.insertXUsers {
		wg.Add(1)
		wgch <- true

		go func() {
			defer func() {
				wg.Done()
				<-wgch
			}()

			res, err := db.Exec(
				"INSERT INTO users (created, email, active) VALUES (?, ?, ?)",
				time.Now().Unix(), fmt.Sprintf("user%d@example.com", idx), 1,
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

			bar.Inc()
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

	bar.Finish()
	bar = benchbar.NewBar("Reading users", 1)

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

	bar.Finish()
	return benchmarkResult{
		Name:        "Simple",
		Duration:    time.Since(start),
		TotalReads:  totalReads,
		TotalWrites: totalWrites,
	}, nil
}
