package nsqlitebench

import (
	"database/sql"
	"fmt"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/nsqlite/nsqlitego"
)

func createMattnDriver(dir string) (*sql.DB, error) {
	dbPath := path.Join(dir, "mattn", "bench.db")

	if err := os.MkdirAll(path.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	fmt.Println("mattn/go-sqlite3 db path:", dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func createNsqliteDriver(_ string) (*sql.DB, error) {
	db, err := sql.Open("nsqlite", "http://localhost:9876")
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
