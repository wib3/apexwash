package main

import (
	"fmt"
	"log"
	"net/http"
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

	fmt.Println("Server is active on http://localhost:8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("server error: ", err)
	}
}
