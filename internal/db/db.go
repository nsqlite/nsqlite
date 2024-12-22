// Package db provides the SQLite integration for NSQLite.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"github.com/orsinium-labs/enum"
)

// DB represents the SQLite integration for NSQLite.
type DB struct {
	readWriteConn     *sql.DB
	readOnlyConn      *sql.DB
	transactions      map[string]*sql.Tx
	transactionsMutex sync.Mutex
	writeChan         chan writeTask
	wg                sync.WaitGroup
}

// writeTask holds the info needed to process a single write request.
type writeTask struct {
	ctx        context.Context
	query      Query
	resultChan chan QueryResult
	errorChan  chan error
}

// Query represents a query to be executed.
type Query struct {
	TxId   string
	Query  string
	Params []any
}

// QueryResult represents the result of a query.
type QueryResult struct {
	Type        queryType
	TxId        string
	WriteResult sql.Result
	ReadResult  *sql.Rows
}

// Config represents the configuration for the NewDB function.
type Config struct {
	// Directory is the directory where the database files are stored.
	Directory string
	// DisableOptimizations disables the startup performance optimizations
	// for the underlying SQLite database.
	DisableOptimizations bool
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
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	var (
		databasePath = path.Join(config.Directory, "database.sqlite")
		readWriteDSN = createDSN(databasePath, false, config.DisableOptimizations)
		readOnlyDSN  = createDSN(databasePath, true, config.DisableOptimizations)
	)

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
		readWriteConn:     readWriteConn,
		readOnlyConn:      readOnlyConn,
		transactions:      make(map[string]*sql.Tx),
		transactionsMutex: sync.Mutex{},
		writeChan:         make(chan writeTask),
	}

	db.wg.Add(1)
	go db.processWriteChan()

	return db, nil
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

		task.resultChan <- QueryResult{
			Type:        QueryTypeWrite,
			WriteResult: result,
			TxId:        task.query.TxId,
		}
	}
}

// Close attempts a graceful shutdown of everything this DB manages.
func (db *DB) Close() error {
	close(db.writeChan)
	db.wg.Wait()

	db.transactionsMutex.Lock()
	for txId, tx := range db.transactions {
		_ = tx.Rollback()
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

// queryType represents the type of a given SQLite query.
type queryType = enum.Member[string]

var (
	QueryTypeUnknown  = queryType{Value: "unknown"}
	QueryTypeRead     = queryType{Value: "read"}
	QueryTypeWrite    = queryType{Value: "write"}
	QueryTypeBegin    = queryType{Value: "begin"}
	QueryTypeCommit   = queryType{Value: "commit"}
	QueryTypeRollback = queryType{Value: "rollback"}
)

// detectQueryType detects the type of query between read, write, begin, commit, and rollback.
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
func (db *DB) Query(
	ctx context.Context, query Query,
) (QueryResult, error) {
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

	db.transactionsMutex.Lock()
	db.transactions[txId.String()] = tx
	db.transactionsMutex.Unlock()

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

	return QueryResult{
		Type: QueryTypeRollback,
		TxId: query.TxId,
	}, nil
}

// getTransactionById returns a transaction by its ID.
func (db *DB) getTransactionById(txId string) (*sql.Tx, bool) {
	if txId == "" {
		return nil, false
	}

	db.transactionsMutex.Lock()
	defer db.transactionsMutex.Unlock()

	tx, found := db.transactions[txId]
	return tx, found
}

// executeWriteQuery executes a write query using the single writer channel.
func (db *DB) executeWriteQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
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
		return res, nil
	case err := <-errorChan:
		return QueryResult{}, err
	case <-ctx.Done():
		return QueryResult{}, ctx.Err()
	}
}

// executeReadQuery executes a read query.
func (db *DB) executeReadQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	tx, found := db.getTransactionById(query.TxId)
	var result *sql.Rows
	var err error

	if found {
		result, err = tx.QueryContext(ctx, query.Query, query.Params...)
	} else {
		result, err = db.readOnlyConn.QueryContext(ctx, query.Query, query.Params...)
	}

	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to execute read query: %w", err)
	}

	return QueryResult{
		Type:       QueryTypeRead,
		ReadResult: result,
		TxId:       query.TxId,
	}, nil
}
