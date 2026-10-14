package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

type PageData struct{
	ShopName string
	Tagline	string
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil{
		http.Error(w, "Could not load page: "+err.Error(), http.StatusInternalServerError)
		return
	}
	data := PageData{
		ShopName: "ApexWash",
		Tagline: "Engineered by IB",
	}
	tmpl.Execute(w, data)
}

func main() {
	http.HandleFunc("/", HomeHandler)
	fmt.Println("Server is active on http://localhost:8080")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("server error: ", err)
	}
}
