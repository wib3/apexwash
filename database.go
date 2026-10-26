package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite", "./apexwash.db")
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	createOrdersTable := `
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER DEFAULT 0,
		customer_name TEXT NOT NULL,
		customer_phone TEXT DEFAULT '',
		delivery_address TEXT DEFAULT '',
		service_name TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		total_cost INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'Pending'
	);`

	_, err = db.Exec(createOrdersTable)
	if err != nil {
		log.Fatal("Failed to create orders table: ", err)
	}

	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL, 
		email TEXT NOT NULL UNIQUE,
		phone TEXT NOT NULL UNIQUE,
		address TEXT NOT NULL,
		password_hash TEXT NOT NULL
	);`

	_, err = db.Exec(createUsersTable)
	if err != nil {
		log.Fatal("Failed to create users table: ", err)
	}

	// Safely upgrade existing local databases created in earlier versions
	_, _ = db.Exec("ALTER TABLE orders ADD COLUMN user_id INTEGER DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE orders ADD COLUMN customer_phone TEXT DEFAULT '';")
	_, _ = db.Exec("ALTER TABLE orders ADD COLUMN delivery_address TEXT DEFAULT '';")
}
