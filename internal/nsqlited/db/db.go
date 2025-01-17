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
	"github.com/nsqlite/nsqlite/internal/nsqlited/log"
	"github.com/nsqlite/nsqlite/internal/nsqlited/sqlitec"
	"github.com/nsqlite/nsqlite/internal/nsqlited/sqlitedrv"
	"github.com/nsqlite/nsqlite/internal/nsqlited/stats"
	"github.com/nsqlite/nsqlite/internal/util/syncutil"
	"github.com/orsinium-labs/enum"
)

var (
	ErrTxNotFound = errors.New("transaction not found or timed out, check your settings")
	ErrTxWithinTx = errors.New("cannot start a transaction within a transaction")
	ErrTxOnlyOne  = errors.New("only only one transaction is allowed at a time")
	ErrTxNotMatch = errors.New("transaction ID does not match the currently active transaction")
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
	isInitialized     bool
	readWriteConn     *sql.DB
	readOnlyConn      *sql.DB
	txId              syncutil.AtomicString
	txIdLastUsed      syncutil.AtomicTime
	txIdleMonitorStop chan any
	writeMu           sync.Mutex
	closeWg           sync.WaitGroup
}

// Query represents a query to be executed.
type Query struct {
	TxId   string
	Query  string
	Params []sqlitec.QueryParam
}

// QueryResult represents the result of a query.
type QueryResult struct {
	Type queryType
	TxId string

	LastInsertID int64
	RowsAffected int64

	Columns []string
	Types   []string
	Rows    [][]any
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
	readWriteConnector := newConnector(databasePath, false)
	readOnlyConnector := newConnector(databasePath, true)

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
	readOnlyConn.SetMaxIdleConns(100)

	db := &DB{
		Config:            config,
		isInitialized:     true,
		readWriteConn:     readWriteConn,
		readOnlyConn:      readOnlyConn,
		txId:              *syncutil.NewAtomicString(""),
		txIdLastUsed:      *syncutil.NewAtomicTime(time.Now()),
		txIdleMonitorStop: make(chan any),
		writeMu:           sync.Mutex{},
		closeWg:           sync.WaitGroup{},
	}

	db.closeWg.Add(1)
	go db.txIdleMonitor(config.TxIdleTimeout)

	config.Logger.InfoNs(log.NsDatabase, "database started")
	return db, nil
}

// getRawConn returns a raw connection from *sql.DB and a function to return
// it to the pool.
func (db *DB) getRawConn(ctx context.Context, dbPool *sql.DB) (*sqlitec.Conn, func() error, error) {
	dConn, err := dbPool.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get connection from pool: %w", err)
	}

	var sqlitecConn *sqlitec.Conn
	err = dConn.Raw(func(driverConn any) error {
		dc, ok := driverConn.(*sqlitedrv.Conn)
		if !ok {
			return fmt.Errorf("failed to cast driver connection")
		}
		sqlitecConn = dc.RawConn()
		return nil
	})
	if err != nil {
		dConn.Close()
		return nil, nil, fmt.Errorf("failed to get raw connection: %w", err)
	}

	return sqlitecConn, dConn.Close, nil
}

// getReadWriteRawConn returns the read-write connection and a function to
// return it to the pool.
func (db *DB) getReadWriteRawConn(ctx context.Context) (*sqlitec.Conn, func() error, error) {
	return db.getRawConn(ctx, db.readWriteConn)
}

// getReadOnlyRawConn returns the read-only connection and a function to return it
// to the pool.
func (db *DB) getReadOnlyRawConn(ctx context.Context) (*sqlitec.Conn, func() error, error) {
	return db.getRawConn(ctx, db.readOnlyConn)
}

// IsInitialized returns whether the DB instance is initialized.
func (db *DB) IsInitialized() bool {
	return db.isInitialized
}

// txIdleMonitor rolls back the current transaction if not used within the timeout.
func (db *DB) txIdleMonitor(timeout time.Duration) {
	defer db.closeWg.Done()
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		select {
		case <-db.txIdleMonitorStop:
			return
		case <-ticker.C:
			if db.txId.Load() == "" {
				continue
			}
			if time.Since(db.txIdLastUsed.Load()) > timeout {
				_, _ = db.executeRollbackQuery(context.Background(), db.txId.Load())
			}
		}
	}
}

// Close attempts a graceful shutdown of everything this DB manages.
func (db *DB) Close() error {
	close(db.txIdleMonitorStop)
	db.closeWg.Wait()

	if db.txId.Load() != "" {
		_, _ = db.executeRollbackQuery(context.Background(), db.txId.Load())
	}

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

	conn, returnConn, err := db.getReadOnlyRawConn(ctx)
	if err != nil {
		return QueryTypeUnknown, fmt.Errorf("failed to get connection: %w", err)
	}
	defer func() { _ = returnConn() }()

	stmt, err := conn.Prepare(query)
	if err != nil {
		return QueryTypeUnknown, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Finalize() }()

	if stmt.ReadOnly() {
		return QueryTypeRead, nil
	}
	return QueryTypeWrite, nil
}

// Query executes an SQLite query.
func (db *DB) Query(ctx context.Context, query Query) (QueryResult, error) {
	res, err := db.query(ctx, query)
	if err != nil {
		db.DBStats.IncErrors()
	}

	return res, err
}

// query is the underlying logic for Query.
func (db *DB) query(ctx context.Context, query Query) (QueryResult, error) {
	typeOfQuery, err := db.detectQueryType(ctx, query.Query)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to detect query type: %w", err)
	}

	switch typeOfQuery {
	case QueryTypeBegin:
		return db.executeBeginQuery(ctx, query.TxId)
	case QueryTypeCommit:
		return db.executeCommitQuery(ctx, query.TxId)
	case QueryTypeRollback:
		return db.executeRollbackQuery(ctx, query.TxId)
	case QueryTypeRead:
		return db.executeReadQuery(ctx, query)
	case QueryTypeWrite:
		return db.executeWriteQuery(ctx, query)
	}

	return QueryResult{}, fmt.Errorf("unknown query type: %s", typeOfQuery.Value)
}

// executeBeginQuery executes a begin query using the read-write connection.
func (db *DB) executeBeginQuery(ctx context.Context, queryTxId string) (QueryResult, error) {
	// TODO: Add support for queuing transactions when one is already active.
	if db.txId.Load() != "" {
		return QueryResult{}, ErrTxWithinTx
	}

	if db.isCurrentTx(queryTxId) {
		return QueryResult{}, ErrTxWithinTx
	}

	conn, returnConn, err := db.getReadWriteRawConn(ctx)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get read-write connection from pool: %w", err)
	}
	defer func() { _ = returnConn() }()

	if _, err = conn.Query("BEGIN TRANSACTION", nil); err != nil {
		return QueryResult{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	txId := uuid.NewString()
	db.txId.Store(txId)
	db.txIdLastUsed.Store(time.Now())
	db.DBStats.IncBegins()

	return QueryResult{
		Type: QueryTypeBegin,
		TxId: txId,
	}, nil
}

// executeCommitQuery commits the existing transaction with the given ID.
func (db *DB) executeCommitQuery(ctx context.Context, queryTxId string) (QueryResult, error) {
	if !db.isCurrentTx(queryTxId) {
		return QueryResult{}, ErrTxNotFound
	}

	conn, returnConn, err := db.getReadWriteRawConn(ctx)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get read-write connection from pool: %w", err)
	}
	defer func() { _ = returnConn() }()

	if _, err = conn.Query("COMMIT", nil); err != nil {
		return QueryResult{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	db.txId.Store("")
	db.txIdLastUsed.Store(time.Now())
	db.DBStats.IncCommits()

	return QueryResult{
		Type: QueryTypeCommit,
		TxId: queryTxId,
	}, nil
}

// executeRollbackQuery rolls back an existing transaction.
func (db *DB) executeRollbackQuery(ctx context.Context, queryTxId string) (QueryResult, error) {
	if !db.isCurrentTx(queryTxId) {
		return QueryResult{}, ErrTxNotFound
	}

	conn, returnConn, err := db.getReadWriteRawConn(ctx)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get read-write connection from pool: %w", err)
	}
	defer func() { _ = returnConn() }()

	if _, err = conn.Query("ROLLBACK", nil); err != nil {
		return QueryResult{}, fmt.Errorf("failed to rollback transaction: %w", err)
	}

	db.txId.Store("")
	db.txIdLastUsed.Store(time.Now())
	db.DBStats.IncRollbacks()

	return QueryResult{
		Type: QueryTypeRollback,
		TxId: queryTxId,
	}, nil
}

// isCurrentTx returns true if the provided transaction ID is the current one.
// it also updates the lastUsed time.
func (db *DB) isCurrentTx(txId string) bool {
	current := db.txId.Load()
	if txId == "" || current == "" || txId != current {
		return false
	}

	db.txIdLastUsed.Store(time.Now())
	return true
}

// matchCurrentTx returns true if the provided transaction ID is empty or matches
// the current transaction ID.
func (db *DB) matchCurrentTx(txId string) bool {
	if txId == "" {
		return true
	}

	return db.isCurrentTx(txId)
}

// executeWriteQuery increments the write queue count, sends the task,
// waits for a response, and then decrements the counter.
func (db *DB) executeWriteQuery(ctx context.Context, query Query) (QueryResult, error) {
	db.DBStats.IncQueuedWrites()
	defer db.DBStats.DecQueuedWrites()

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	if !db.matchCurrentTx(query.TxId) {
		return QueryResult{}, ErrTxNotMatch
	}

	conn, returnConn, err := db.getReadWriteRawConn(ctx)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get read-write connection from pool: %w", err)
	}
	defer func() { _ = returnConn() }()

	res, err := conn.Query(query.Query, query.Params)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to execute write query: %w", err)
	}

	db.DBStats.IncWrites()
	return QueryResult{
		TxId:         query.TxId,
		Type:         QueryTypeWrite,
		LastInsertID: res.LastInsertID,
		RowsAffected: res.RowsAffected,
		Columns:      res.Columns,
		Types:        res.Types,
		Rows:         res.Rows,
	}, nil
}

// executeReadQuery executes a read query.
func (db *DB) executeReadQuery(ctx context.Context, query Query) (QueryResult, error) {
	if !db.matchCurrentTx(query.TxId) {
		return QueryResult{}, ErrTxNotMatch
	}

	conn, returnConn, err := db.getReadOnlyRawConn(ctx)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to get connection: %w", err)
	}
	defer func() { _ = returnConn() }()

	res, err := conn.Query(query.Query, query.Params)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to execute read query: %w", err)
	}

	db.DBStats.IncReads()
	return QueryResult{
		TxId:         query.TxId,
		Type:         QueryTypeRead,
		LastInsertID: res.LastInsertID,
		RowsAffected: res.RowsAffected,
		Columns:      res.Columns,
		Types:        res.Types,
		Rows:         res.Rows,
	}, nil
}
