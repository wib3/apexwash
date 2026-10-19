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
		log.Fatal("Failed to connect to database", err)
	}
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_name TEXT NOT NULL,
		service_name TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		total_cost INTEGER NOT NULL,
		status TEXT NOT NULL
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal("Failed to create table: ", err)
	}
}
