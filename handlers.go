package main

import (
	"encoding/csv"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strconv"
)

func isStaff(r *http.Request) bool {
	cookie, err := r.Cookie("staff_session")
	return err == nil && cookie.Value == "authenticated"
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

	insertSQL := `INSERT INTO orders (customer_name, service_name, quantity, total_cost, status) VALUES(?, ?, ?, ?, ?)`
	_, err = db.Exec(insertSQL, customer, service, qty, totalCost, "Pending")
	if err != nil {
		http.Error(w, "Failed to save order", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Could not load page: "+err.Error(), http.StatusInternalServerError)
		return
	}
	searchQuery := r.URL.Query().Get("search")
	statusFilter := r.URL.Query().Get("status")

	query := "SELECT id, customer_name, service_name, quantity, total_cost, status FROM orders WHERE 1=1"
	var args []any

	if searchQuery != "" {
		query += " AND customer_name LIKE ?"
		args = append(args, "%"+searchQuery+"%")
	}
	if statusFilter != "" {
		query += " AND status = ?"
		args = append(args, statusFilter)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, "Could not load orders", http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	var orders []Order
	var totalRevenue int

	for rows.Next() {
		var o Order
		rows.Scan(&o.ID, &o.CustomerName, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status)
		orders = append(orders, o)
		totalRevenue += o.TotalCost
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error reading orders: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := PageData{
		ShopName:     "ApexWash",
		Tagline:      "Engineered by IB",
		Services:     serviceList,
		Orders:       orders,
		TotalOrders:  len(orders),
		TotalRevenue: totalRevenue,
		SearchQuery:  searchQuery,
		StatusFilter: statusFilter,
		IsStaff:      isStaff(r),
	}
	tmpl.Execute(w, data)
}

func UpdateStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isStaff(r) {
		http.Error(w, "Unauthorized: Staff login required", http.StatusUnauthorized)
		return
	}
	idstr := r.FormValue("order_id")
	id, err := strconv.Atoi(idstr)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}
	var currentStatus string
	err = db.QueryRow("SELECT status FROM orders WHERE id = ?", id).Scan(&currentStatus)
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	var newStatus string
	switch currentStatus {
	case "Pending", "pending":
		newStatus = "Washing & Ironing"
	case "Washing & Ironing":
		newStatus = "Ready for Pickup"
	default:
		newStatus = currentStatus
	}

	_, err = db.Exec("UPDATE orders SET status = ? WHERE id = ?", newStatus, id)
	if err != nil {
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func DeleteOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isStaff(r) {
		http.Error(w, "Unauthorized: Staff login required", http.StatusUnauthorized)
		return
	}
	idStr := r.FormValue("order_id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("DELETE FROM orders WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Could not delete order", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	expectedpassword := os.Getenv("STAFF_PASSWORD")
	if expectedpassword == "" {
		expectedpassword = "apexadmin"
	}

	password := r.FormValue("password")
	if password != expectedpassword {
		http.Error(w, "Invalid staff password", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "staff_session",
		Value:    "authenticated",
		Path:     "/",
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "staff_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func APIorderHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, customer_name, service_name, quantity, total_cost, status FROM orders")
	if err != nil {
		http.Error(w, "Could not load orders: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error reading orders: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []Order{}

	for rows.Next() {
		var o Order
		rows.Scan(&o.ID, &o.CustomerName, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status)
		orders = append(orders, o)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func ExportCSVHandler(w http.ResponseWriter, r *http.Request) {
	if !isStaff(r) {
		http.Error(w, "Unauthorized: Staff login required to export data", http.StatusUnauthorized)
		return
	}
	rows, err := db.Query("SELECT id, customer_name, service_name, quantity, total_cost, status FROM orders ORDER BY id DESC")
	if err != nil {
		http.Error(w, "Failed to query orders for export", http.StatusInternalServerError)
		return
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error exporting orders: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachement; filename="apexwash_orders.csv"`)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"Order ID", "Customer Name", "Service", "Quantity", "Total Cost (NGN)", "Status"})

	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.CustomerName, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status); err != nil {
			continue
		}
		writer.Write([]string{
			strconv.Itoa(o.ID),
			o.CustomerName,
			o.ServiceName,
			strconv.Itoa(o.Quantity),
			strconv.Itoa(o.TotalCost),
			o.Status,
		})
	}
}

func ReceiptHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil || orderID <= 0 {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}
	var o Order
	err = db.QueryRow("SELECT id, customer_name, service_name, quantity, total_cost, status FROM orders WHERE id = ?", orderID).Scan(&o.ID, &o.CustomerName, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status)
	if err != nil {
		http.Error(w, "Order receipt not found", http.StatusNotFound)
		return
	}
	tmpl, err := template.ParseFiles("templates/receipt.html")
	if err != nil {
		http.Error(w, "Failed to load receipt template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, o)
}
