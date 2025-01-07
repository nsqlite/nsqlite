// Package db provides the SQLite integration for NSQLite.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"github.com/nsqlite/nsqlite/internal/nsqlited/log"
	"github.com/nsqlite/nsqlite/internal/nsqlited/stats"
	"github.com/orsinium-labs/enum"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found or timed out, check your settings")
)

// Config represents the configuration for a DB instance.
type Config struct {
	// Logger is the shared NSQLite logger.
	Logger log.Logger
	// DBStats is an instance of dbstats.DBStats.
	DBStats *stats.DBStats
	// DataDirectory is the directory where the database files are stored.
	DataDirectory string
	// TxIdleTimeout if a transaction is not active for this duration, it
	// will be rolled back.
	TxIdleTimeout time.Duration
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
	writeMu                 sync.Mutex
	wg                      sync.WaitGroup
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

// NewDB creates a new DB instance.
func NewDB(config Config) (*DB, error) {
	if !config.Logger.IsInitialized() {
		return nil, errors.New("logger is required")
	}
	if config.DataDirectory == "" {
		return nil, errors.New("database directory is required")
	}
	if err := os.MkdirAll(config.DataDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	if config.TxIdleTimeout <= 0 {
		return nil, errors.New("transaction idle timeout must be provided")
	}

	databasePath := path.Join(config.DataDirectory, "database.sqlite")

	readWriteConnector, err := newConnector(databasePath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create read-write connector: %w", err)
	}

	readOnlyConnector, err := newConnector(databasePath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create read-only connector: %w", err)
	}

	readWriteConn := sql.OpenDB(readWriteConnector)
	if err := readWriteConn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping write connection: %w", err)
	}
	readWriteConn.SetConnMaxIdleTime(0)
	readWriteConn.SetConnMaxLifetime(0)
	readWriteConn.SetMaxIdleConns(1)
	readWriteConn.SetMaxOpenConns(1)

	readOnlyConn := sql.OpenDB(readOnlyConnector)
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
		wg:                      sync.WaitGroup{},
	}

	db.wg.Add(1)
	go db.monitorIdleTransactions(config.TxIdleTimeout)

	config.Logger.InfoNs(log.NsDatabase, "database started")
	return db, nil
}

// IsInitialized returns whether the DB instance is initialized.
func (db *DB) IsInitialized() bool {
	return db.isInitialized
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
	close(db.transactionsMonitorStop)

	db.wg.Wait()
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
func (db *DB) detectQueryType(ctx context.Context, query string) (queryType, error) {
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

	if typeOfQuery == QueryTypeBegin && query.TxId != "" {
		return QueryResult{}, errors.New("stepping, cannot start a transaction within a transaction")
	}

	switch typeOfQuery {
	case QueryTypeBegin:
		return db.executeBeginQuery()
	case QueryTypeCommit:
		return db.executeCommitQuery(query)
	case QueryTypeRollback:
		return db.executeRollbackQuery(query)
	case QueryTypeRead:
		return db.executeReadQuery(ctx, query)
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

	db.DBStats.IncBegins()
	return QueryResult{
		Type: QueryTypeBegin,
		TxId: txId.String(),
	}, nil
}

// executeCommitQuery commits an existing transaction.
func (db *DB) executeCommitQuery(query Query) (QueryResult, error) {
	tx, found, _ := db.getTransactionById(query.TxId)
	if !found {
		return QueryResult{}, ErrTransactionNotFound
	}
	if err := tx.Commit(); err != nil {
		return QueryResult{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	db.transactionsMutex.Lock()
	delete(db.transactions, query.TxId)
	db.transactionsMutex.Unlock()

	db.DBStats.IncCommits()
	return QueryResult{
		Type: QueryTypeCommit,
		TxId: query.TxId,
	}, nil
}

// executeRollbackQuery rolls back an existing transaction.
func (db *DB) executeRollbackQuery(query Query) (QueryResult, error) {
	tx, found, _ := db.getTransactionById(query.TxId)
	if !found {
		return QueryResult{}, ErrTransactionNotFound
	}
	if err := tx.Rollback(); err != nil {
		return QueryResult{}, fmt.Errorf("failed to rollback transaction: %w", err)
	}

	db.transactionsMutex.Lock()
	delete(db.transactions, query.TxId)
	db.transactionsMutex.Unlock()

	db.DBStats.IncRollbacks()
	return QueryResult{
		Type: QueryTypeRollback,
		TxId: query.TxId,
	}, nil
}

// getTransactionById returns a transaction by its ID and updates its
// lastUsed time.
//
//   - If txId is empty, it returns false without an error.
//   - If txId is not empty, it returns false with an error if the transaction
//     is not found.
//   - If txId is not empty and the transaction is found, it returns true
//     without an error.
func (db *DB) getTransactionById(txId string) (*sql.Tx, bool, error) {
	if txId == "" {
		return nil, false, nil
	}

	db.transactionsMutex.Lock()
	defer db.transactionsMutex.Unlock()

	data, found := db.transactions[txId]
	if !found {
		return nil, false, ErrTransactionNotFound
	}

	data.lastUsed = time.Now()
	db.transactions[txId] = data

	return data.tx, true, nil
}

// executeWriteQuery increments the write queue count, sends the task,
// waits for a response, and then decrements the counter.
func (db *DB) executeWriteQuery(ctx context.Context, query Query) (QueryResult, error) {
	db.DBStats.IncQueuedWrites()
	defer db.DBStats.DecQueuedWrites()

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	tx, found, err := db.getTransactionById(query.TxId)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get transaction: %w", err)
	}

	var result sql.Result
	if found {
		result, err = tx.ExecContext(ctx, query.Query, query.Params...)
	} else {
		result, err = db.readWriteConn.ExecContext(ctx, query.Query, query.Params...)
	}
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to execute write query: %w", err)
	}

	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get rows affected: %w", err)
	}

	db.DBStats.IncWrites()
	return QueryResult{
		TxId: query.TxId,
		Type: QueryTypeWrite,
		WriteResult: WriteResult{
			LastInsertID: lastInsertId,
			RowsAffected: rowsAffected,
		},
	}, nil
}

// executeReadQuery executes a read query.
func (db *DB) executeReadQuery(ctx context.Context, query Query) (QueryResult, error) {
	tx, found, err := db.getTransactionById(query.TxId)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get transaction: %w", err)
	}

	var result *sql.Rows
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

	db.DBStats.IncReads()
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
func (db *DB) getColumnTypes(result *sql.Rows, singleRow []any) ([]string, error) {
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
