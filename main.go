package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
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

var serviceList = []ServiceItem{
	{Name: "Shirt(Wash and Iron)", Price: 500},
	{Name: "Trousers", Price: 600},
	{Name: "Native Wear(2-piece)", Price: 1200},
	{Name: "Bed Sheet/Duvet", Price: 1500},
}

func BookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not Allowed", http.StatusMethodNotAllowed)
		return
	}
	customer := r.FormValue("customer_name")
	service := r.FormValue("service_name")
	quantity := r.FormValue("quantity")

	qty, err := strconv.Atoi(quantity)
	if err != nil {
		http.Error(w, "Invalid Quantity Entered", http.StatusBadRequest)
		return
	}

	var unitprice int
	for _, item := range serviceList {
		if item.Name == service {
			unitprice = item.Price
			break
		}
	}

	totalCost := qty * unitprice

	fmt.Fprintf(w, "Order Recieved! Thank you, %s. You booked %s (Qty: %s). Total Bill: ₦%d", customer, service, quantity, totalCost)
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
		Services: serviceList,
	}
	tmpl.Execute(w, data)
}

func main() {
	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/book", BookHandler)
	fmt.Println("Server is active on http://localhost:8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("server error: ", err)
	}
}
