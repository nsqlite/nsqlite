package sqlitec

/*
#cgo LDFLAGS: -Wl,--allow-multiple-definition
#include "sqlite3.c"
*/
import "C"
import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"
)

// Conn represents a high-level connection to a SQLite database.
//
// https://www.sqlite.org/c3ref/sqlite3.html
type Conn struct {
	cDB *C.sqlite3
}

// Stmt represents a prepared statement in SQLite.
//
// https://www.sqlite.org/c3ref/stmt.html
type Stmt struct {
	conn  *Conn
	cStmt *C.sqlite3_stmt
}

// getLastError returns the last error message from the SQLite database.
func (conn *Conn) getLastError() error {
	if conn.cDB == nil {
		return errors.New("failed to get last error: database connection is nil")
	}
	return errors.New(C.GoString(C.sqlite3_errmsg(conn.cDB)))
}

// Open opens a new SQLite database connection using the given path.
//
// https://www.sqlite.org/c3ref/open.html
func Open(filePath string) (*Conn, error) {
	cFilePath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cFilePath))

	var db *C.sqlite3
	resCode := C.sqlite3_open(cFilePath, &db)
	if resCode != SQLITE_OK {
		errMsg := (&Conn{cDB: db}).getLastError()
		_ = C.sqlite3_close(db)
		return nil, fmt.Errorf("failed to open database: %s: %s", getResCodeStr(resCode), errMsg)
	}

	return &Conn{cDB: db}, nil
}

// Close finalizes the connection to the SQLite database.
//
// https://www.sqlite.org/c3ref/close.html
func (conn *Conn) Close() error {
	if conn.cDB == nil {
		return nil
	}

	// The sqlite3_close_v2() interface is intended for use with host
	// languages that are garbage collected, and where the order in which
	// destructors are called is arbitrary.
	resCode := C.sqlite3_close_v2(conn.cDB)
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to close database: %s: %s", getResCodeStr(resCode), conn.getLastError())
	}
	conn.cDB = nil

	return nil
}

// LastInsertRowID returns the row ID of the most recent successful INSERT
// into the database from the current connection.
//
// https://www.sqlite.org/c3ref/last_insert_rowid.html
func (conn *Conn) LastInsertRowID() int64 {
	return int64(C.sqlite3_last_insert_rowid(conn.cDB))
}

// RowsAffected returns the number of rows modified, inserted, or deleted by
// the most recent successful INSERT, UPDATE, or DELETE statement from the
// current connection.
//
// https://www.sqlite.org/c3ref/changes.html
func (conn *Conn) RowsAffected() int64 {
	return int64(C.sqlite3_changes(conn.cDB))
}

// QueryOrExecResult represents the result for QueryOrExec.
type QueryOrExecResult struct {
	Time         time.Duration
	LastInsertID int64
	RowsAffected int64
	Columns      []string
	Types        []string
	Rows         [][]any
}

// QueryOrExec executes the given SQL query on the SQLite database connection
// from start to finish, returning the result of the query for both write and
// read operations.
func (conn *Conn) QueryOrExec(query string) (*QueryOrExecResult, error) {
	start := time.Now()

	stmt, err := conn.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer func() {
		_ = stmt.Finalize()
	}()

	var lastInsertID, rowsAffected int64
	var columns []string
	var types []string
	var rows [][]any
	columnCount := stmt.ColumnCount()

	// Bind parameters
	// Detect if is read or write depending on the number of columns in the result
	// Scan the result values

	if columnCount == 0 {
		hasNext := true
		err = nil
		for hasNext {
			hasNext, err = stmt.Step()
			if err != nil {
				return nil, fmt.Errorf("failed to step statement: %w", err)
			}
		}

		lastInsertID = conn.LastInsertRowID()
		rowsAffected = conn.RowsAffected()
	}

	if columnCount > 0 {
		columns = make([]string, columnCount)
		types = make([]string, columnCount)
		rows = make([][]any, 0)

		for i := 0; i < columnCount; i++ {
			columns[i] = stmt.ColumnName(i)
			types[i] = stmt.ColumnType(i)
		}

		hasNext := true
		err = nil
		for hasNext {
			hasNext, err = stmt.Step()
			if err != nil {
				return nil, fmt.Errorf("failed to step statement: %w", err)
			}

			row := make([]any, columnCount)
			for i := 0; i < columnCount; i++ {
				switch strings.ToLower(types[i]) {
				case "integer":
					row[i] = stmt.ColumnInt(i)
				case "real":
					row[i] = stmt.ColumnFloat64(i)
				case "text":
					row[i] = stmt.ColumnText(i)
				case "blob":
					row[i] = stmt.ColumnBlob(i)
				default:
					row[i] = nil
				}
			}
			rows = append(rows, row)
		}
	}

	return &QueryOrExecResult{
		Time:         time.Since(start),
		LastInsertID: lastInsertID,
		RowsAffected: rowsAffected,
		Columns:      columns,
		Types:        types,
		Rows:         rows,
	}, nil
}

// Exec executes the given SQL query on the SQLite database connection
// from start to finish, without returning any data.
//
// https://www.sqlite.org/c3ref/exec.html
func (conn *Conn) Exec(query string) error {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var errMsg *C.char
	resCode := C.sqlite3_exec(conn.cDB, cQuery, nil, nil, &errMsg)
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to execute query: %s: %s", getResCodeStr(resCode), C.GoString(errMsg))
	}

	return nil
}

// Prepare compiles the given SQL query into a prepared statement.
//
// https://www.sqlite.org/c3ref/prepare.html
func (conn *Conn) Prepare(query string) (*Stmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var cStmt *C.sqlite3_stmt
	resCode := C.sqlite3_prepare_v2(conn.cDB, cQuery, C.int(-1), &cStmt, nil)
	if resCode != SQLITE_OK {
		return nil, fmt.Errorf("failed to prepare statement: %s: %s", getResCodeStr(resCode), conn.getLastError())
	}
	return &Stmt{conn: conn, cStmt: cStmt}, nil
}

// ReadOnly returns true if the given SQL query is read-only.
//
// https://www.sqlite.org/c3ref/stmt_readonly.html
func (stmt *Stmt) ReadOnly() bool {
	return C.sqlite3_stmt_readonly(stmt.cStmt) != 0
}

// BindInt binds an int parameter at the given index.
//
// https://www.sqlite.org/c3ref/bind_blob.html
func (stmt *Stmt) BindInt(index int, value int) error {
	if stmt.cStmt == nil {
		return fmt.Errorf("cannot bind to a nil statement")
	}

	resCode := C.sqlite3_bind_int(stmt.cStmt, C.int(index), C.int(value))
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to bind int: %s", getResCodeStr(resCode))
	}
	return nil
}

// BindInt64 binds an int64 parameter at the given index.
//
// https://www.sqlite.org/c3ref/bind_blob.html
func (stmt *Stmt) BindInt64(index int, value int64) error {
	if stmt.cStmt == nil {
		return fmt.Errorf("cannot bind to a nil statement")
	}

	resCode := C.sqlite3_bind_int64(stmt.cStmt, C.int(index), C.sqlite3_int64(value))
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to bind int64: %s", getResCodeStr(resCode))
	}
	return nil
}

// BindFloat64 binds a float64 parameter at the given index.
//
// https://www.sqlite.org/c3ref/bind_blob.html
func (stmt *Stmt) BindFloat64(index int, value float64) error {
	if stmt.cStmt == nil {
		return fmt.Errorf("cannot bind to a nil statement")
	}

	resCode := C.sqlite3_bind_double(stmt.cStmt, C.int(index), C.double(value))
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to bind float64: %s", getResCodeStr(resCode))
	}
	return nil
}

// BindText binds a string parameter at the given index.
//
// https://www.sqlite.org/c3ref/bind_blob.html
func (stmt *Stmt) BindText(index int, value string) error {
	if stmt.cStmt == nil {
		return fmt.Errorf("cannot bind to a nil statement")
	}
	cStr := C.CString(value)
	defer C.free(unsafe.Pointer(cStr))

	resCode := C.cust_sqlite3_bind_text(stmt.cStmt, C.int(index), cStr, C.int(-1))
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to bind text: %s", getResCodeStr(resCode))
	}
	return nil
}

// BindBlob binds a byte slice parameter at the given index.
//
// https://www.sqlite.org/c3ref/bind_blob.html
func (stmt *Stmt) BindBlob(index int, data []byte) error {
	if stmt.cStmt == nil {
		return fmt.Errorf("cannot bind to a nil statement")
	}
	if len(data) == 0 {
		return stmt.BindNull(index)
	}

	resCode := C.cust_sqlite3_bind_blob(stmt.cStmt, C.int(index), unsafe.Pointer(&data[0]), C.int(len(data)))
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to bind blob: %s", getResCodeStr(resCode))
	}
	return nil
}

// BindNull binds a NULL value at the given index.
//
// https://www.sqlite.org/c3ref/bind_blob.html
func (stmt *Stmt) BindNull(index int) error {
	if stmt.cStmt == nil {
		return fmt.Errorf("cannot bind to a nil statement")
	}

	resCode := C.sqlite3_bind_null(stmt.cStmt, C.int(index))
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to bind null: %s", getResCodeStr(resCode))
	}
	return nil
}

// Step advances the statement to the next row of data, returning true if a new row
// is available, or false if there are no more rows. If an error occurs, it is returned.
//
// https://www.sqlite.org/c3ref/step.html
func (stmt *Stmt) Step() (bool, error) {
	resCode := C.sqlite3_step(stmt.cStmt)

	if resCode == SQLITE_DONE {
		return false, nil
	}

	if resCode == SQLITE_ROW {
		return true, nil
	}

	return false, fmt.Errorf("failed to step statement: %s", getResCodeStr(resCode))
}

// ColumnCount returns the number of columns in the current result row.
//
// https://www.sqlite.org/c3ref/column_count.html
func (stmt *Stmt) ColumnCount() int {
	return int(C.sqlite3_column_count(stmt.cStmt))
}

// ColumnName returns the name of the column at the given index.
//
// https://www.sqlite.org/c3ref/column_name.html
func (stmt *Stmt) ColumnName(colIndex int) string {
	return C.GoString(C.sqlite3_column_name(stmt.cStmt, C.int(colIndex)))
}

// ColumnType returns the type of the column at the given index.
//
// https://www.sqlite.org/c3ref/column_decltype.html
func (stmt *Stmt) ColumnType(colIndex int) string {
	return C.GoString(C.sqlite3_column_decltype(stmt.cStmt, C.int(colIndex)))
}

// ColumnInt returns the column value at the given index as int.
//
// https://www.sqlite.org/c3ref/column_blob.html
func (stmt *Stmt) ColumnInt(colIndex int) int {
	return int(C.sqlite3_column_int(stmt.cStmt, C.int(colIndex)))
}

// ColumnInt64 returns the column value at the given index as int64.
//
// https://www.sqlite.org/c3ref/column_blob.html
func (stmt *Stmt) ColumnInt64(colIndex int) int64 {
	return int64(C.sqlite3_column_int64(stmt.cStmt, C.int(colIndex)))
}

// ColumnFloat64 returns the column value at the given index as float64.
//
// https://www.sqlite.org/c3ref/column_blob.html
func (stmt *Stmt) ColumnFloat64(colIndex int) float64 {
	return float64(C.sqlite3_column_double(stmt.cStmt, C.int(colIndex)))
}

// ColumnText returns the column value at the given index as a string.
//
// https://www.sqlite.org/c3ref/column_blob.html
func (stmt *Stmt) ColumnText(colIndex int) string {
	text := (*C.char)(unsafe.Pointer(C.sqlite3_column_text(stmt.cStmt, C.int(colIndex))))
	if text == nil {
		return ""
	}
	length := C.sqlite3_column_bytes(stmt.cStmt, C.int(colIndex))
	return C.GoStringN(text, length)
}

// ColumnBlob returns the column value at the given index as a byte slice.
//
// https://www.sqlite.org/c3ref/column_blob.html
func (stmt *Stmt) ColumnBlob(colIndex int) []byte {
	size := C.sqlite3_column_bytes(stmt.cStmt, C.int(colIndex))
	if size <= 0 {
		return nil
	}
	dataPtr := C.sqlite3_column_blob(stmt.cStmt, C.int(colIndex))
	if dataPtr == nil {
		return nil
	}
	return C.GoBytes(dataPtr, size)
}

// Finalize frees the resources associated with this statement.
//
// https://www.sqlite.org/c3ref/finalize.html
func (stmt *Stmt) Finalize() error {
	if stmt.cStmt == nil {
		return nil
	}

	resCode := C.sqlite3_finalize(stmt.cStmt)
	if resCode != SQLITE_OK {
		return fmt.Errorf("failed to finalize statement: %s: %s", getResCodeStr(resCode), C.GoString(C.sqlite3_errmsg(stmt.conn.cDB)))
	}
	stmt.cStmt = nil

	return nil
}
