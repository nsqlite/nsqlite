package db

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/url"

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
	// DSN optimizations supported in mattn/go-sqlite3s
	qp := url.Values{}
	qp.Add("_journal_mode", "WAL")
	qp.Add("_busy_timeout", "5000")
	qp.Add("_synchronous", "NORMAL")
	qp.Add("_cache_size", "10000")
	qp.Add("_foreign_keys", "true")
	if c.isReadOnly {
		qp.Add("_query_only", "true")
	}

	// Other optimizations that are not supported in mattn/go-sqlite3
	optimizations := []string{
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA mmap_size = 536870912;", // 512MB
	}

	conn, err := c.driver.Open(fmt.Sprintf("file:%s?%s", c.dbPath, qp.Encode()))
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
