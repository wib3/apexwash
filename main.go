package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	InitDB()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/book", BookHandler)
	http.HandleFunc("/update-status", UpdateStatusHandler)
	http.HandleFunc("/delete-order", DeleteOrderHandler)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/api/orders", APIorderHandler)
	http.HandleFunc("/export-csv", ExportCSVHandler)
	http.HandleFunc("/receipt", ReceiptHandler)
	http.HandleFunc("/login-customer", CustomerLoginHandler)
	http.HandleFunc("/logout-customer", CustomerLogoutHandler)
	http.HandleFunc("/signup", SignupHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Println("ApexWash Server is live on http://localhost:" + port)
	http.ListenAndServe(":"+port, nil)
}
