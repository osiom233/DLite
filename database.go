package main

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"time"
)

func InitDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "DLite.db")
	if err != nil {
		return nil, err
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS DLite (
		token TEXT PRIMARY KEY,
		path TEXT,
		expire DATETIME
	);`

	if _, err := db.Exec(createTable); err != nil {
		return nil, err
	}

	return db, nil
}

func InsertToken(db *sql.DB, token, path string, expire time.Time) error {
	_, err := db.Exec("INSERT INTO DLite (token, path, expire) VALUES (?, ?, ?)", token, path, expire)
	return err
}

func GetTokenPath(db *sql.DB, token string) (string, error) {
	var path string
	err := db.QueryRow("SELECT path FROM DLite WHERE token = ?", token).Scan(&path)
	return path, err
}

func DeleteToken(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM DLite WHERE token = ?", token)
	return err
}

func DeleteExpiredTokens(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM DLite WHERE expire < ?", time.Now())
	return err
}
