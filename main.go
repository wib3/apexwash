package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

type ServiceItem struct {
	Name  string
	Price int
}

type PageData struct {
	ShopName string
	Tagline  string
	Services []ServiceItem
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Could not load page: "+err.Error(), http.StatusInternalServerError)
		return
	}
	data := PageData{
		ShopName: "ApexWash",
		Tagline:  "Engineered by IB",
		Services: []ServiceItem{
			{Name: "Shirt(Wash and Iron)", Price: 500},
			{Name: "Trousers", Price: 600},
			{Name: "Native Wear(2-piece)", Price: 1200},
			{Name: "Bed Sheet/Duvet", Price: 1500},
		},
	}
	tmpl.Execute(w, data)
}

func main() {
	http.HandleFunc("/", HomeHandler)
	fmt.Println("Server is active on http://localhost:8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("server error: ", err)
	}
}
