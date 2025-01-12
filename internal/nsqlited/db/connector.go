package db

import (
	"context"
	"database/sql/driver"

	"github.com/mattn/go-sqlite3"
)

type connector struct {
	driver     driver.Driver
	dbPath     string
	isReadOnly bool
}

func newConnector(dbPath string, readOnly bool) (driver.Connector, error) {
	return &connector{
		driver:     &sqlite3.SQLiteDriver{},
		dbPath:     dbPath,
		isReadOnly: readOnly,
	}, nil
}

// Connect creates a new database connection with the NSQLite optimizations.
func (c *connector) Connect(context.Context) (driver.Conn, error) {
	optimizations := []string{
		"PRAGMA JOURNAL_MODE = WAL;",
		"PRAGMA BUSY_TIMEOUT = 5000;",
		"PRAGMA SYNCHRONOUS = NORMAL;",
		"PRAGMA CACHE_SIZE = 10000;",
		"PRAGMA FOREIGN_KEYS = true;",
		"PRAGMA TEMP_STORE = MEMORY;",
		"PRAGMA MMAP_SIZE = 536870912;", // 512MB
	}

	if c.isReadOnly {
		optimizations = append(optimizations, "PRAGMA QUERY_ONLY = true;")
	}

	conn, err := c.driver.Open("file:" + c.dbPath)
	if err != nil {
		return nil, err
	}

	for _, optimization := range optimizations {
		if err := exec(conn, optimization); err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

func (c *connector) Driver() driver.Driver {
	return c.driver
}

func exec(conn driver.Conn, query string) error {
	stmt, err := conn.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(nil)
	return err
}
