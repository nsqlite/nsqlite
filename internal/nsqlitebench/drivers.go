package nsqlitebench

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/nsqlite/nsqlitego"
)

func createMattnDriver(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func createNsqliteDriver(dsn string) (*sql.DB, error) {
	db, err := sql.Open("nsqlite", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
