// Package db provides the SQLite integration for NSQLite.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"runtime"
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
	// ReadOnlyPoolSize is the number of connections for the read-only pool.
	ReadOnlyPoolSize int
}

// NewDB creates a new DB instance.
func NewDB(config Config) (*DB, error) {
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	databasePath := path.Join(config.Directory, "database.sqlite")
	readWriteConn, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open write connection: %w", err)
	}

	readWriteConn.SetConnMaxIdleTime(0)
	readWriteConn.SetConnMaxLifetime(0)
	readWriteConn.SetMaxIdleConns(1)
	readWriteConn.SetMaxOpenConns(1)
	if err := readWriteConn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping write connection: %w", err)
	}

	readOnlyConn, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open read connection: %w", err)
	}

	maxIdleConns := 1
	maxOpenConns := 1
	if config.ReadOnlyPoolSize < 1 {
		maxIdleConns = runtime.NumCPU()
		maxOpenConns = runtime.NumCPU() * 2
	}
	if config.ReadOnlyPoolSize > 1 {
		maxIdleConns = config.ReadOnlyPoolSize / 2
		maxOpenConns = config.ReadOnlyPoolSize
	}

	readOnlyConn.SetConnMaxIdleTime(0)
	readOnlyConn.SetConnMaxLifetime(0)
	readOnlyConn.SetMaxIdleConns(maxIdleConns)
	readOnlyConn.SetMaxOpenConns(maxOpenConns)
	if err := readOnlyConn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping read connection: %w", err)
	}

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
		tx, found := db.GetTransactionById(task.query.TxId)
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
			Type:        queryTypeWrite,
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
	queryTypeUnknown  = queryType{Value: "unknown"}
	queryTypeRead     = queryType{Value: "read"}
	queryTypeWrite    = queryType{Value: "write"}
	queryTypeBegin    = queryType{Value: "begin"}
	queryTypeCommit   = queryType{Value: "commit"}
	queryTypeRollback = queryType{Value: "rollback"}
)

// DetectQueryType detects the type of query between read, write, begin, commit, and rollback.
func (db *DB) DetectQueryType(
	ctx context.Context, query string,
) (queryType, error) {
	trimmed := strings.ToLower(strings.TrimSpace(query))

	switch {
	case strings.HasPrefix(trimmed, "begin"):
		return queryTypeBegin, nil
	case strings.HasPrefix(trimmed, "commit"):
		return queryTypeCommit, nil
	case strings.HasPrefix(trimmed, "rollback"):
		return queryTypeRollback, nil
	}

	conn, err := db.readOnlyConn.Conn(ctx)
	if err != nil {
		return queryTypeUnknown, fmt.Errorf("failed to get connection: %w", err)
	}

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
		return queryTypeUnknown, fmt.Errorf("failed to prepare statement: %w", err)
	}

	if isReadOnly {
		return queryTypeRead, nil
	}
	return queryTypeWrite, nil
}

// HandleQuery handles a query based on its type.
func (db *DB) HandleQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	typeOfQuery, err := db.DetectQueryType(ctx, query.Query)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to detect query type: %w", err)
	}

	switch typeOfQuery {
	case queryTypeRead:
		return db.ExecuteReadQuery(ctx, query)
	case queryTypeBegin:
		return db.ExecuteBeginQuery(ctx, query)
	case queryTypeCommit:
		return db.ExecuteCommitQuery(ctx, query)
	case queryTypeRollback:
		return db.ExecuteRollbackQuery(ctx, query)
	case queryTypeWrite:
		return db.ExecuteWriteQuery(ctx, query)
	}

	return QueryResult{}, fmt.Errorf("unknown query type: %s", typeOfQuery.Value)
}

// ExecuteBeginQuery executes a begin query.
func (db *DB) ExecuteBeginQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
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
		Type: queryTypeBegin,
		TxId: txId.String(),
	}, nil
}

// ExecuteCommitQuery commits an existing transaction.
func (db *DB) ExecuteCommitQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	tx, found := db.GetTransactionById(query.TxId)
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
		Type: queryTypeCommit,
		TxId: query.TxId,
	}, nil
}

// ExecuteRollbackQuery rolls back an existing transaction.
func (db *DB) ExecuteRollbackQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	tx, found := db.GetTransactionById(query.TxId)
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
		Type: queryTypeRollback,
		TxId: query.TxId,
	}, nil
}

// GetTransactionById returns a transaction by its ID.
func (db *DB) GetTransactionById(txId string) (*sql.Tx, bool) {
	if txId == "" {
		return nil, false
	}

	db.transactionsMutex.Lock()
	defer db.transactionsMutex.Unlock()

	tx, found := db.transactions[txId]
	return tx, found
}

// ExecuteWriteQuery executes a write query using the single writer channel.
func (db *DB) ExecuteWriteQuery(
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

// ExecuteReadQuery executes a read query.
func (db *DB) ExecuteReadQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	tx, found := db.GetTransactionById(query.TxId)
	var result *sql.Rows
	var err error

	if found {
		result, err = tx.Query(query.Query, query.Params...)
	} else {
		result, err = db.readOnlyConn.Query(query.Query, query.Params...)
	}

	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to execute read query: %w", err)
	}

	return QueryResult{
		Type:       queryTypeRead,
		ReadResult: result,
		TxId:       query.TxId,
	}, nil
}
