package main

import (
	"html/template"
	"net/http"
	"strconv"
)

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
