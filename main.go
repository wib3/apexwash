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
	Orders   []Order
}

type Order struct {
	ID           int
	CustomerName string
	ServiceName  string
	Quantity     int
	TotalCost    int
	Status       string
}

var serviceList = []ServiceItem{
	{Name: "Shirt(Wash and Iron)", Price: 500},
	{Name: "Trousers", Price: 600},
	{Name: "Native Wear(2-piece)", Price: 1200},
	{Name: "Bed Sheet/Duvet", Price: 1500},
}

var orderList []Order

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

	newOrder := Order{
		ID:           len(orderList) + 1,
		CustomerName: customer,
		ServiceName:  service,
		Quantity:     qty,
		TotalCost:    totalCost,
		Status:       "Pending",
	}

	orderList = append(orderList, newOrder)

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
		Orders:   orderList,
	}
	tmpl.Execute(w, data)
}

func main() {
	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/book", BookHandler)
	http.HandleFunc("/update-status", UpdateStatusHandler)
	fmt.Println("Server is active on http://localhost:8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("server error: ", err)
	}
}

func UpdateStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idstr := r.FormValue("order_id")
	id, err := strconv.Atoi(idstr)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}
	for i := range orderList {
		if orderList[i].ID == id {
			if orderList[i].Status == "Pending" {
				orderList[i].Status = "Washing and ironing"
			} else if orderList[i].Status == "Washing and ironing" {
				orderList[i].Status = "Ready for pickup"
			}
			break
		}

	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
