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
	writeQueue        chan func() sql.Result
	writeQueueMutex   sync.Mutex
	transactions      map[string]*sql.Tx
	transactionsMutex sync.Mutex
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

	return &DB{
		readWriteConn:     readWriteConn,
		readOnlyConn:      readOnlyConn,
		writeQueue:        make(chan func() sql.Result),
		writeQueueMutex:   sync.Mutex{},
		transactions:      make(map[string]*sql.Tx),
		transactionsMutex: sync.Mutex{},
	}, nil
}

// Close closes all the database connections.
func (db *DB) Close() error {
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
	queryTypeUnknown = queryType{Value: "unknown"}
	queryTypeRead    = queryType{Value: "read"}
	queryTypeWrite   = queryType{Value: "write"}
	queryTypeBegin   = queryType{Value: "begin"}
)

// DetectQueryType detects the type of query between read, write, and begin.
//
// It uses sqlite3_stmt_readonly to determine if the query is read-only.
func (db *DB) DetectQueryType(
	ctx context.Context, query string,
) (queryType, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(query)), "begin") {
		return queryTypeBegin, nil
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

// Query represents a query to be executed
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

// ExecuteWriteQuery executes a write query.
func (db *DB) ExecuteWriteQuery(
	ctx context.Context, query Query,
) (QueryResult, error) {
	tx, found := db.GetTransactionById(query.TxId)
	var result sql.Result
	var err error

	if found {
		result, err = tx.Exec(query.Query, query.Params...)
	} else {
		result, err = db.readWriteConn.Exec(query.Query, query.Params...)
	}

	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to execute write query: %w", err)
	}

	return QueryResult{
		Type:        queryTypeWrite,
		WriteResult: result,
		TxId:        query.TxId,
	}, nil
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
