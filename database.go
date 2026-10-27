package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB() {
	var err error
	// FIXED: "sqlite" instead of "sqlite3" to match the modernc.org driver
	db, err = sql.Open("sqlite", "./apexwash.db")
	if err != nil {
		log.Fatal(err)
	}

	// 1. Updated Users table with address, role, and account_status
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		phone TEXT,
		address TEXT,
		role TEXT DEFAULT 'customer',
		account_status TEXT DEFAULT 'active'
	);`
	_, err = db.Exec(createUsersTable)
	if err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// 2. New Services table for dynamic pricing
	createServicesTable := `
	CREATE TABLE IF NOT EXISTS services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		price REAL NOT NULL
	);`
	_, err = db.Exec(createServicesTable)
	if err != nil {
		log.Fatalf("Failed to create services table: %v", err)
	}

	// 3. Restored Orders table
	createOrdersTable := `
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		service_name TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		total_cost REAL NOT NULL,
		status TEXT DEFAULT 'Pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	_, err = db.Exec(createOrdersTable)
	if err != nil {
		log.Fatalf("Failed to create orders table: %v", err)
	}

	// Auto-fill prices if the table is new
	seedDefaultServices(db)
}

// seedDefaultServices fills the DB with your base prices on the first run
func seedDefaultServices(db *sql.DB) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM services").Scan(&count)

	if count == 0 {
		defaultServices := []Service{
			{Name: "Shirt (Wash and Iron)", Price: 1500},
			{Name: "Suit (2-Piece)", Price: 3000},
			{Name: "Trousers", Price: 1200},
			{Name: "Native Wear (Agbada)", Price: 2500},
			{Name: "Bed Sheet", Price: 2000},
		}

		for _, s := range defaultServices {
			_, err := db.Exec("INSERT INTO services (name, price) VALUES (?, ?)", s.Name, s.Price)
			if err != nil {
				log.Printf("Failed to seed service %s: %v", s.Name, err)
			}
		}
		log.Println("Seeded default laundry services into the database.")
	}
}