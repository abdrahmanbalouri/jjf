package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// DB is the database connection instance.
var DB *sql.DB

// InitDB initializes the database connection.
func InitDB(dataSourceName string) error {
	var err error
	DB, err = sql.Open("sqlite", dataSourceName)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	_, err = DB.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		DB.Close()
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	_, err = DB.Exec("PRAGMA busy_timeout = 10000;")
	if err != nil {
		DB.Close()
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	_, err = DB.Exec("PRAGMA cache_size = -20000;")
	if err != nil {
		DB.Close()
		return fmt.Errorf("failed to set cache size: %w", err)
	}

	if err = DB.Ping(); err != nil {
		DB.Close()
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Database connection established successfully with WAL and busy timeout.")
	return nil
}

func CloseDB() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		} else {
			log.Println("Database connection closed.")
		}
	}
}

// CreateTables creates all necessary tables in the database.
func CreateTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			nickname TEXT UNIQUE NOT NULL,
			age INTEGER,
			gender TEXT,
			first_name TEXT,
			last_name TEXT,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_online BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE IF NOT EXISTS posts (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			category TEXT,
			title TEXT,
			content TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			post_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			content TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(post_id) REFERENCES posts(id),
			FOREIGN KEY(user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS private_messages (
			id TEXT PRIMARY KEY,
			sender_id TEXT NOT NULL,
			receiver_id TEXT NOT NULL,
			content TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_read BOOLEAN DEFAULT FALSE,
			FOREIGN KEY(sender_id) REFERENCES users(id),
			FOREIGN KEY(receiver_id) REFERENCES users(id)
		)`,
	}

	for _, table := range tables {
		_, err := DB.Exec(table)
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}
	log.Println("Database tables checked/created successfully.")
	return nil
}

// IsDuplicateKeyError checks if the error is a duplicate key error.
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "UNIQUE constraint failed: users.nickname" ||
		err.Error() == "UNIQUE constraint failed: users.email"
}