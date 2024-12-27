// Package db provides the SQLite integration for NSQLite.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"github.com/nsqlite/nsqlite/internal/log"
	"github.com/orsinium-labs/enum"
)

// Config represents the configuration for a DB instance.
type Config struct {
	// Logger is the shared NSQLite logger.
	Logger log.Logger
	// Directory is the directory where the database files are stored.
	Directory string
	// DisableOptimizations disables the startup performance optimizations
	// for the underlying SQLite database.
	DisableOptimizations bool
	// TransactionIdleTimeout if a transaction is not active for this duration, it
	// will be rolled back.
	TransactionIdleTimeout time.Duration
}

// DB represents the SQLite integration for NSQLite.
type DB struct {
	Config
	isInitialized           bool
	readWriteConn           *sql.DB
	readOnlyConn            *sql.DB
	transactions            map[string]transactionData
	transactionsMutex       sync.Mutex
	transactionsMonitorStop chan any
	writeChan               chan writeTask
	writeQueueCount         int64
	stats                   DBStats
	statsFilePath           string
	statsStop               chan any
	wg                      sync.WaitGroup
}

// DBStats holds counters and status info about DB usage.
type DBStats struct {
	TotalReadQueries     int64 `json:"totalReadQueries"`
	TotalWriteQueries    int64 `json:"totalWriteQueries"`
	TotalBeginQueries    int64 `json:"totalBeginQueries"`
	TotalCommitQueries   int64 `json:"totalCommitQueries"`
	TotalRollbackQueries int64 `json:"totalRollbackQueries"`
	TransactionsInFlight int64 `json:"-"` // Derived in real time, not persistent.
	WriteQueueLength     int64 `json:"-"` // Derived in real time, not persistent.
}

// transactionData holds a *sql.Tx and the last time it was accessed.
type transactionData struct {
	tx       *sql.Tx
	lastUsed time.Time
}

// Query represents a query to be executed.
type Query struct {
	TxId   string
	Query  string
	Params []any
}

// writeTask holds the info needed to process a single write request.
type writeTask struct {
	ctx        context.Context
	query      Query
	resultChan chan QueryResult
	errorChan  chan error
}

// WriteResult represents the result of a write query.
type WriteResult struct {
	LastInsertID int64
	RowsAffected int64
}

// ReadResult represents the result of a read query.
type ReadResult struct {
	Columns []string
	Types   []string
	Values  *[][]any
}

// QueryResult represents the result of a query.
type QueryResult struct {
	Type        queryType
	TxId        string
	WriteResult WriteResult
	ReadResult  ReadResult
}

func createDSN(
	dbPath string, isReadOnly bool, disableOptimizations bool,
) string {
	qp := url.Values{}
	qp.Add("_foreign_keys", "true")
	qp.Add("_busy_timeout", "5000")

	if isReadOnly {
		qp.Add("_query_only", "true")
	}

	if !disableOptimizations {
		qp.Add("_journal_mode", "WAL")
		qp.Add("_synchronous", "NORMAL")
		qp.Add("_cache_size", "10000")

		// TODO: Implement a way to set these optimizations.
		// - PRAGMA temp_store = MEMORY;
		// - PRAGMA mmap_size = 536870912; // 512MB
	}

	return fmt.Sprintf("file:%s?%s", dbPath, qp.Encode())
}

// NewDB creates a new DB instance.
func NewDB(config Config) (*DB, error) {
	if !config.Logger.IsInitialized() {
		return nil, errors.New("logger is required")
	}
	if config.Directory == "" {
		return nil, errors.New("database directory is required")
	}
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	if config.TransactionIdleTimeout <= 0 {
		return nil, errors.New("transaction idle timeout must be provided")
	}

	statsFilePath := path.Join(config.Directory, "stats.json")
	databasePath := path.Join(config.Directory, "database.sqlite")
	readWriteDSN := createDSN(databasePath, false, config.DisableOptimizations)
	readOnlyDSN := createDSN(databasePath, true, config.DisableOptimizations)

	readWriteConn, err := sql.Open("sqlite3", readWriteDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open write connection: %w", err)
	}
	if err := readWriteConn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping write connection: %w", err)
	}
	readWriteConn.SetConnMaxIdleTime(0)
	readWriteConn.SetConnMaxLifetime(0)
	readWriteConn.SetMaxIdleConns(1)
	readWriteConn.SetMaxOpenConns(1)

	readOnlyConn, err := sql.Open("sqlite3", readOnlyDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open read connection: %w", err)
	}
	if err := readOnlyConn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping read connection: %w", err)
	}
	readOnlyConn.SetConnMaxIdleTime(0)
	readOnlyConn.SetConnMaxLifetime(0)
	readOnlyConn.SetMaxIdleConns(5)

	db := &DB{
		Config:                  config,
		isInitialized:           true,
		readWriteConn:           readWriteConn,
		readOnlyConn:            readOnlyConn,
		transactions:            make(map[string]transactionData),
		transactionsMutex:       sync.Mutex{},
		transactionsMonitorStop: make(chan any),
		writeChan:               make(chan writeTask),
		writeQueueCount:         0,
		stats:                   DBStats{},
		statsFilePath:           statsFilePath,
		statsStop:               make(chan any),
		wg:                      sync.WaitGroup{},
	}

	db.loadStatsFromFile()

	db.wg.Add(1)
	go db.processWriteChan()

	db.wg.Add(1)
	go db.monitorIdleTransactions(config.TransactionIdleTimeout)

	db.wg.Add(1)
	go db.runStatsSync()

	config.Logger.InfoNs(log.NsDatabase, "database started")
	return db, nil
}

// IsInitialized returns whether the DB instance is initialized.
func (db *DB) IsInitialized() bool {
	return db.isInitialized
}

// loadStatsFromFile loads counters from the JSON file if present.
func (db *DB) loadStatsFromFile() {
	b, err := os.ReadFile(db.statsFilePath)
	if err != nil {
		return
	}

	stats := DBStats{}
	if err := json.Unmarshal(b, &stats); err != nil {
		return
	}

	atomic.StoreInt64(&db.stats.TotalReadQueries, stats.TotalReadQueries)
	atomic.StoreInt64(&db.stats.TotalWriteQueries, stats.TotalWriteQueries)
	atomic.StoreInt64(&db.stats.TotalBeginQueries, stats.TotalBeginQueries)
	atomic.StoreInt64(&db.stats.TotalCommitQueries, stats.TotalCommitQueries)
	atomic.StoreInt64(&db.stats.TotalRollbackQueries, stats.TotalRollbackQueries)
}

// saveStatsToFile stores counters (except in-flight data) into the JSON file.
func (db *DB) saveStatsToFile() {
	stats := DBStats{
		TotalReadQueries:     atomic.LoadInt64(&db.stats.TotalReadQueries),
		TotalWriteQueries:    atomic.LoadInt64(&db.stats.TotalWriteQueries),
		TotalBeginQueries:    atomic.LoadInt64(&db.stats.TotalBeginQueries),
		TotalCommitQueries:   atomic.LoadInt64(&db.stats.TotalCommitQueries),
		TotalRollbackQueries: atomic.LoadInt64(&db.stats.TotalRollbackQueries),
	}

	b, err := json.Marshal(stats)
	if err != nil {
		return
	}

	if err := os.WriteFile(db.statsFilePath, b, 0644); err != nil {
		return
	}
}

// runStatsSync periodically flushes stats to the JSON file.
func (db *DB) runStatsSync() {
	defer db.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-db.statsStop:
			db.saveStatsToFile()
			return
		case <-ticker.C:
			db.saveStatsToFile()
		}
	}
}

// monitorIdleTransactions rolls back transactions not used within the timeout.
func (db *DB) monitorIdleTransactions(timeout time.Duration) {
	defer db.wg.Done()
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		select {
		case <-db.transactionsMonitorStop:
			return
		case <-ticker.C:
			func() {
				db.transactionsMutex.Lock()
				defer db.transactionsMutex.Unlock()
				now := time.Now()
				for txID, data := range db.transactions {
					if now.Sub(data.lastUsed) > timeout {
						_ = data.tx.Rollback()
						delete(db.transactions, txID)
					}
				}
			}()
		}
	}
}

// Close attempts a graceful shutdown of everything this DB manages.
func (db *DB) Close() error {
	close(db.writeChan)
	close(db.transactionsMonitorStop)
	close(db.statsStop)

	db.wg.Wait()
	db.saveStatsToFile()

	db.transactionsMutex.Lock()
	for txId, data := range db.transactions {
		_ = data.tx.Rollback()
		delete(db.transactions, txId)
	}
	db.transactionsMutex.Unlock()

	if db.readWriteConn != nil {
		if err := db.readWriteConn.Close(); err != nil {
			return fmt.Errorf("failed to close write connection: %w", err)
		}
	}

	if db.readOnlyConn != nil {
		if err := db.readOnlyConn.Close(); err != nil {
			return fmt.Errorf("failed to close read connections: %w", err)
		}
	}

	return nil
}

// processWriteChan processes all incoming write tasks one at a time.
func (db *DB) processWriteChan() {
	defer db.wg.Done()
	for task := range db.writeChan {
		tx, found := db.getTransactionById(task.query.TxId)
		var result sql.Result
		var err error

		if found {
			result, err = tx.Exec(task.query.Query, task.query.Params...)
		} else {
			result, err = db.readWriteConn.Exec(task.query.Query, task.query.Params...)
		}
		if err != nil {
			task.errorChan <- fmt.Errorf("failed to execute write query: %w", err)
			continue
		}

		lastInsertId, err := result.LastInsertId()
		if err != nil {
			task.errorChan <- fmt.Errorf("failed to get last insert ID: %w", err)
			continue
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			task.errorChan <- fmt.Errorf("failed to get rows affected: %w", err)
			continue
		}

		atomic.AddInt64(&db.stats.TotalWriteQueries, 1)
		task.resultChan <- QueryResult{
			TxId: task.query.TxId,
			Type: QueryTypeWrite,
			WriteResult: WriteResult{
				LastInsertID: lastInsertId,
				RowsAffected: rowsAffected,
			},
		}
	}
}

// queryType represents the type of a given SQLite query.
type queryType enum.Member[string]

var (
	QueryTypeUnknown  = queryType{Value: "unknown"}
	QueryTypeRead     = queryType{Value: "read"}
	QueryTypeWrite    = queryType{Value: "write"}
	QueryTypeBegin    = queryType{Value: "begin"}
	QueryTypeCommit   = queryType{Value: "commit"}
	QueryTypeRollback = queryType{Value: "rollback"}
)

// detectQueryType detects the type of query between read, write, begin, commit,
// and rollback.
func (db *DB) detectQueryType(
	ctx context.Context, query string,
) (queryType, error) {
	trimmed := strings.ToLower(strings.TrimSpace(query))

	switch {
	case strings.HasPrefix(trimmed, "begin"):
		return QueryTypeBegin, nil
	case strings.HasPrefix(trimmed, "commit"):
		return QueryTypeCommit, nil
	case strings.HasPrefix(trimmed, "rollback"), strings.HasPrefix(trimmed, "end transaction"):
		return QueryTypeRollback, nil
	}

	conn, err := db.readOnlyConn.Conn(ctx)
	if err != nil {
		return QueryTypeUnknown, fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	isReadOnly := false
	err = conn.Raw(func(driverConn any) error {
		sqliteConn := driverConn.(*sqlite3.SQLiteConn)
		drvStmt, err := sqliteConn.Prepare(query)
		if err != nil {
			return err
		}
		defer drvStmt.Close()
		sqliteStmt := drvStmt.(*sqlite3.SQLiteStmt)
		isReadOnly = sqliteStmt.Readonly()
		return nil
	})
	if err != nil {
		return QueryTypeUnknown, fmt.Errorf("failed to prepare statement: %w", err)
	}

	if isReadOnly {
		return QueryTypeRead, nil
	}
	return QueryTypeWrite, nil
}

// Query executes an SQLite query.
func (db *DB) Query(ctx context.Context, query Query) (QueryResult, error) {
	typeOfQuery, err := db.detectQueryType(ctx, query.Query)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to detect query type: %w", err)
	}

	switch typeOfQuery {
	case QueryTypeRead:
		return db.executeReadQuery(ctx, query)
	case QueryTypeBegin:
		return db.executeBeginQuery()
	case QueryTypeCommit:
		return db.executeCommitQuery(query)
	case QueryTypeRollback:
		return db.executeRollbackQuery(query)
	case QueryTypeWrite:
		return db.executeWriteQuery(ctx, query)
	}

	return QueryResult{}, fmt.Errorf("unknown query type: %s", typeOfQuery.Value)
}

// executeBeginQuery executes a begin query.
func (db *DB) executeBeginQuery() (QueryResult, error) {
	tx, err := db.readWriteConn.Begin()
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	txId, err := uuid.NewRandom()
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to generate transaction ID: %w", err)
	}

	data := transactionData{
		tx:       tx,
		lastUsed: time.Now(),
	}

	db.transactionsMutex.Lock()
	db.transactions[txId.String()] = data
	db.transactionsMutex.Unlock()

	atomic.AddInt64(&db.stats.TotalBeginQueries, 1)

	return QueryResult{
		Type: QueryTypeBegin,
		TxId: txId.String(),
	}, nil
}

// executeCommitQuery commits an existing transaction.
func (db *DB) executeCommitQuery(query Query) (QueryResult, error) {
	tx, found := db.getTransactionById(query.TxId)
	if !found {
		return QueryResult{}, fmt.Errorf("no transaction found for commit")
	}
	if err := tx.Commit(); err != nil {
		return QueryResult{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	db.transactionsMutex.Lock()
	delete(db.transactions, query.TxId)
	db.transactionsMutex.Unlock()

	atomic.AddInt64(&db.stats.TotalCommitQueries, 1)

	return QueryResult{
		Type: QueryTypeCommit,
		TxId: query.TxId,
	}, nil
}

// executeRollbackQuery rolls back an existing transaction.
func (db *DB) executeRollbackQuery(query Query) (QueryResult, error) {
	tx, found := db.getTransactionById(query.TxId)
	if !found {
		return QueryResult{}, fmt.Errorf("no transaction found for rollback")
	}
	if err := tx.Rollback(); err != nil {
		return QueryResult{}, fmt.Errorf("failed to rollback transaction: %w", err)
	}

	db.transactionsMutex.Lock()
	delete(db.transactions, query.TxId)
	db.transactionsMutex.Unlock()

	atomic.AddInt64(&db.stats.TotalRollbackQueries, 1)

	return QueryResult{
		Type: QueryTypeRollback,
		TxId: query.TxId,
	}, nil
}

// getTransactionById returns a transaction by its ID and updates its
// lastUsed time.
func (db *DB) getTransactionById(txId string) (*sql.Tx, bool) {
	if txId == "" {
		return nil, false
	}

	db.transactionsMutex.Lock()
	defer db.transactionsMutex.Unlock()

	data, found := db.transactions[txId]
	if !found {
		return nil, false
	}

	data.lastUsed = time.Now()
	db.transactions[txId] = data

	return data.tx, true
}

// executeWriteQuery increments the write queue count, sends the task,
// waits for a response, and then decrements the counter.
func (db *DB) executeWriteQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	atomic.AddInt64(&db.writeQueueCount, 1)

	resultChan := make(chan QueryResult)
	errorChan := make(chan error)

	task := writeTask{
		ctx:        ctx,
		query:      query,
		resultChan: resultChan,
		errorChan:  errorChan,
	}

	db.writeChan <- task

	select {
	case res := <-resultChan:
		atomic.AddInt64(&db.writeQueueCount, -1)
		return res, nil
	case err := <-errorChan:
		atomic.AddInt64(&db.writeQueueCount, -1)
		return QueryResult{}, err
	case <-ctx.Done():
		atomic.AddInt64(&db.writeQueueCount, -1)
		return QueryResult{}, ctx.Err()
	}
}

// executeReadQuery executes a read query.
func (db *DB) executeReadQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	tx, found := db.getTransactionById(query.TxId)
	var (
		result *sql.Rows
		err    error
	)

	if found {
		result, err = tx.QueryContext(ctx, query.Query, query.Params...)
	} else {
		result, err = db.readOnlyConn.QueryContext(ctx, query.Query, query.Params...)
	}
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to execute read query: %w", err)
	}
	defer result.Close()

	columns, err := result.Columns()
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get columns: %w", err)
	}

	types, typesOk := []string{}, false
	values := [][]any{}
	for result.Next() {
		row := make([]any, len(columns))
		scans := make([]any, len(columns))
		for i := range scans {
			scans[i] = &row[i]
		}

		if err = result.Scan(scans...); err != nil {
			return QueryResult{}, fmt.Errorf("failed to scan row: %w", err)
		}

		if !typesOk {
			enhancedTypes, err := db.getColumnTypes(result, row)
			if err != nil {
				return QueryResult{}, fmt.Errorf("failed to get column types: %w", err)
			}
			types, typesOk = enhancedTypes, true
		}

		values = append(values, row)
	}

	atomic.AddInt64(&db.stats.TotalReadQueries, 1)
	return QueryResult{
		TxId: query.TxId,
		Type: QueryTypeRead,
		ReadResult: ReadResult{
			Columns: columns,
			Types:   types,
			Values:  &values,
		},
	}, nil
}

// getColumnTypes returns the column types for a read query.
//
// It tryes to get the column types from the result, but if it has empty
// types, it tries infering them from the first row following the SQLite
// datatypes documentation https://www.sqlite.org/datatype3.html.
func (db *DB) getColumnTypes(
	result *sql.Rows, singleRow []any,
) ([]string, error) {
	types, err := result.ColumnTypes()
	if err != nil {
		return []string{}, fmt.Errorf("failed to get column types: %w", err)
	}

	typeNames := make([]string, len(types))
	hasEmptyTypes := false
	for i, t := range types {
		typeNames[i] = strings.ToLower(t.DatabaseTypeName())
		if typeNames[i] == "" {
			hasEmptyTypes = true
		}
	}

	if !hasEmptyTypes {
		return typeNames, nil
	}

	for i := range typeNames {
		if typeNames[i] != "" {
			continue
		}

		switch singleRow[i].(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			typeNames[i] = "integer"
		case float32, float64:
			typeNames[i] = "real"
		case bool:
			typeNames[i] = "boolean"
		case []byte:
			typeNames[i] = "blob"
		case string:
			typeNames[i] = "text"
		default:
			typeNames[i] = ""
		}
	}

	return typeNames, nil
}

// GetStats returns the current DBStats. The persistent counters are loaded
// atomically, while TransactionsInFlight and WriteQueueLength are derived in
// real time.
func (db *DB) GetStats() DBStats {
	var stats DBStats
	stats.TotalReadQueries = atomic.LoadInt64(&db.stats.TotalReadQueries)
	stats.TotalWriteQueries = atomic.LoadInt64(&db.stats.TotalWriteQueries)
	stats.TotalBeginQueries = atomic.LoadInt64(&db.stats.TotalBeginQueries)
	stats.TotalCommitQueries = atomic.LoadInt64(&db.stats.TotalCommitQueries)
	stats.TotalRollbackQueries = atomic.LoadInt64(&db.stats.TotalRollbackQueries)

	db.transactionsMutex.Lock()
	stats.TransactionsInFlight = int64(len(db.transactions))
	db.transactionsMutex.Unlock()

	stats.WriteQueueLength = atomic.LoadInt64(&db.writeQueueCount)

	return stats
}
