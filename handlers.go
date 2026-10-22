package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strconv"

	"golang.org/x/crypto/bcrypt"
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
	if currentUser := getLoggedInCustomer(r); currentUser != nil {
		customer = currentUser.FullName
	}
	if customer == "" {
		http.Error(w, "Customer name is required", 400)
		return
	}
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
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	searchQuery := r.URL.Query().Get("search")
	statusFilter := r.URL.Query().Get("status")

	staff := isStaff(r)
	currentUser := getLoggedInCustomer(r)

	var orders []Order
	totalRevenue := 0

	// Only query orders if the visitor is Staff OR a logged-in Customer
	if staff || currentUser != nil {
		query := "SELECT id, customer_name, service_name, quantity, total_cost, status FROM orders WHERE 1=1"
		var args []interface{}

		if staff {
			// Staff can search across all customers
			if searchQuery != "" {
				query += " AND customer_name LIKE ?"
				args = append(args, "%"+searchQuery+"%")
			}
		} else if currentUser != nil {
			// Logged-in customer ONLY sees orders matching their exact account name
			query += " AND customer_name = ?"
			args = append(args, currentUser.FullName)
		}

		// Both Staff and Customers can filter by workflow status
		if statusFilter != "" {
			query += " AND status = ?"
			args = append(args, statusFilter)
		}

		query += " ORDER BY id DESC"

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "Failed to load orders", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var o Order
			if err := rows.Scan(&o.ID, &o.CustomerName, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status); err == nil {
				orders = append(orders, o)
				totalRevenue += o.TotalCost
			}
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "Error reading orders: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Could not load template", http.StatusInternalServerError)
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
		CurrentUser:  currentUser,
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
	err = db.QueryRow(
		"SELECT id, customer_name, service_name, quantity, total_cost, status FROM orders WHERE id = ?",
		orderID,
	).Scan(&o.ID, &o.CustomerName, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status)
	if err != nil {
		http.Error(w, "Order receipt not found", http.StatusNotFound)
		return
	}

	// Security Check: Only Staff or the Customer who owns the order can view the receipt
	staff := isStaff(r)
	currentUser := getLoggedInCustomer(r)
	if !staff && (currentUser == nil || currentUser.FullName != o.CustomerName) {
		http.Error(w, "Unauthorized: You can only view your own receipts", http.StatusForbidden)
		return
	}

	tmpl, err := template.ParseFiles("templates/receipt.html")
	if err != nil {
		http.Error(w, "Failed to load receipt template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, o)
}

func getLoggedInCustomer(r *http.Request) *User {
	cookie, err := r.Cookie("customer_session")
	if err != nil || cookie.Value == "" {
		return nil
	}
	userID, err := strconv.Atoi(cookie.Value)
	if err != nil {
		return nil
	}
	var u User
	err = db.QueryRow("SELECT id, full_name, email, phone, address FROM users WHERE id = ?", userID).Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &u.Address)
	if err != nil {
		return nil
	}
	return &u
}

// SignupHandler registers a new customer account with a bcrypt-hashed password
func SignupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	fullName := r.FormValue("full_name")
	email := r.FormValue("email")
	phone := r.FormValue("phone")
	address := r.FormValue("address")
	password := r.FormValue("password")

	if fullName == "" || email == "" || phone == "" || address == "" || password == "" {
		http.Error(w, "All fields are required to create an account", http.StatusBadRequest)
		return
	}
	// Hash the password using bcrypt
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Invalid phone/email or password", http.StatusUnauthorized)
		return
	}

	// Insert new customer into SQLite
	res, err := db.Exec("INSERT INTO users (full_name, email, phone, address, password_hash) VALUES (?, ?, ?, ?, ?)", fullName, email, phone, address, string(hashedBytes))
	if err != nil {
		http.Error(w, "Email or phone number is already registered", http.StatusConflict)
		return
	}
	newUserID, _ := res.LastInsertId()

	http.SetCookie(w, &http.Cookie{
		Name:     "customer_session",
		Value:    strconv.FormatInt(newUserID, 10),
		Path:     "/",
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// CustomerLoginHandler authenticates a customer by phone or email + password
func CustomerLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	identifier := r.FormValue("identifier")
	password := r.FormValue("password")

	var userID int
	var storedHash string

	err := db.QueryRow("SELECT id, password_hash FROM users WHERE phone = ? OR email = ?", identifier, identifier).Scan(&userID, &storedHash)
	if err == sql.ErrNoRows || err != nil {
		http.Error(w, "Invalid phone/email or password", http.StatusUnauthorized)
		return
	}
	// Compare the provided password against the stored bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
		http.Error(w, "Invalid phone/email or password", http.StatusUnauthorized)
		return
	}
	// Valid credentials: set the customer_session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "customer_session",
		Value:    strconv.Itoa(userID),
		Path:     "/",
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// CustomerLogoutHandler clears the customer_session cookie
func CustomerLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "customer_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
