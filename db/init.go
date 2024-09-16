package db

import (
	"database/sql"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)

	return db, nil
}

func CreateTables() {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip TEXT,
			expression TEXT,
			result TEXT
		);
		CREATE TABLE IF NOT EXISTS leaderboard (
			ip TEXT PRIMARY KEY,
			total_value REAL,
			count INTEGER,
			min_value REAL,
			min_expression TEXT,
			max_value REAL,
			max_expression TEXT
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func IsLockedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}
