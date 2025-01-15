// Package sqlite provides a lightweight wrapper for the SQLite C library to
// be used by NSQLite internally.
//
//   - https://www.sqlite.org/cintro.html
//   - https://www.sqlite.org/c3ref/intro.html
package sqlitec

/*
#cgo LDFLAGS: -Wl,--allow-multiple-definition
#include "sqlite3.c"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
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

// TODO: Add bind() methods to Stmt

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

// TODO: Add column() methods to Stmt

// Finalize frees the resources associated with this statement.
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
