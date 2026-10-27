package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// sessionSecret returns the HMAC signing key from SESSION_SECRET env var (with a fallback for local dev/testing).
func sessionSecret() []byte {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "apexwash-default-hmac-secret-key-2026"
	}
	return []byte(secret)
}

// signSessionValue attaches an HMAC-SHA256 signature to a raw session value: "value.signature"
func signSessionValue(val string) string {
	mac := hmac.New(sha256.New, sessionSecret())
	mac.Write([]byte(val))
	sig := hex.EncodeToString(mac.Sum(nil))
	return val + "." + sig
}

// verifySessionValue validates the HMAC-SHA256 signature and returns the original payload if untampered.
func verifySessionValue(cookieVal string) (string, bool) {
	parts := strings.SplitN(cookieVal, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	val, providedSig := parts[0], parts[1]

	mac := hmac.New(sha256.New, sessionSecret())
	mac.Write([]byte(val))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return "", false
	}
	return val, true
}

func isStaff(r *http.Request) bool {
	cookie, err := r.Cookie("staff_session")
	if err != nil || cookie.Value == "" {
		return false
	}
	val, ok := verifySessionValue(cookie.Value)
	return ok && val == "authenticated"
}

func BookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not Allowed", http.StatusMethodNotAllowed)
		return
	}

	staff := isStaff(r)
	currentUser := getLoggedInCustomer(r)

	// Gatekeeper: If not Staff and not a logged-in Customer, redirect to Signup
	if !staff && currentUser == nil {
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	customer := strings.TrimSpace(r.FormValue("customer_name"))
	phone := strings.TrimSpace(r.FormValue("customer_phone"))
	address := strings.TrimSpace(r.FormValue("delivery_address"))
	userID := 0

	if currentUser != nil {
		// Logged-in customer: auto-fill from their account profile
		userID = currentUser.ID
		customer = currentUser.FullName
		phone = currentUser.Phone
		if address == "" {
			address = currentUser.Address
		}
	} else if staff {
		// Staff walk-in order: validate phone if provided so WhatsApp button works
		if phone != "" {
			validPhone, ok := normalizeAndValidatePhone(phone)
			if !ok {
				http.Error(w, "Invalid Nigerian phone number for walk-in customer", http.StatusBadRequest)
				return
			}
			phone = validPhone
		}
		if address == "" {
			address = "Shop Pickup"
		}
	}

	if customer == "" {
		http.Error(w, "Customer name is required", http.StatusBadRequest)
		return
	}

	service := r.FormValue("service_name")
	quantity := r.FormValue("quantity")

	qty, err := strconv.Atoi(quantity)
	if err != nil || qty <= 0 {
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

	insertSQL := `INSERT INTO orders (user_id, customer_name, customer_phone, delivery_address, service_name, quantity, total_cost, status, payment_status) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(insertSQL, userID, customer, phone, address, service, qty, totalCost, "Pending", "Unpaid")
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
	loginError := r.URL.Query().Get("login_error")
	identifier := r.URL.Query().Get("identifier")

	staff := isStaff(r)
	currentUser := getLoggedInCustomer(r)

	var orders []Order
	totalRevenue := 0

	// Only query orders if the visitor is Staff OR a logged-in Customer
	if staff || currentUser != nil {
		query := `
			SELECT o.id, o.user_id, o.customer_name,
			       COALESCE(NULLIF(o.customer_phone, ''), u.phone, ''),
			       COALESCE(NULLIF(o.delivery_address, ''), u.address, 'Shop Pickup'),
			       o.service_name, o.quantity, o.total_cost, o.status,
			       COALESCE(NULLIF(o.payment_status, ''), 'Unpaid')
			FROM orders o
			LEFT JOIN users u ON o.user_id = u.id
			WHERE 1=1`
		var args []interface{}

		if staff {
			// Staff can search across all customers
			if searchQuery != "" {
				query += " AND o.customer_name LIKE ?"
				args = append(args, "%"+searchQuery+"%")
			}
		} else if currentUser != nil {
			// Logged-in customer sees orders linked to their user_id (or legacy v2.0 orders with user_id=0 matching their name)
			query += " AND (o.user_id = ? OR (o.user_id = 0 AND o.customer_name = ?))"
			args = append(args, currentUser.ID, currentUser.FullName)
		}

		// Both Staff and Customers can filter by workflow status
		if statusFilter != "" {
			query += " AND o.status = ?"
			args = append(args, statusFilter)
		}

		query += " ORDER BY o.id DESC"

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "Failed to load orders", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var o Order
			if err := rows.Scan(&o.ID, &o.UserID, &o.CustomerName, &o.CustomerPhone, &o.DeliveryAddress, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status, &o.PaymentStatus); err == nil {
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
		IsStaff:      staff,
		CurrentUser:  currentUser,
		LoginError:   loginError,
		Identifier:   identifier,
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

func TogglePaymentHandler(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || id <= 0 {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	var currentPayment string
	err = db.QueryRow("SELECT COALESCE(NULLIF(payment_status, ''), 'Unpaid') FROM orders WHERE id = ?", id).Scan(&currentPayment)
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	newPayment := "Paid"
	if strings.EqualFold(currentPayment, "Paid") {
		newPayment = "Unpaid"
	}

	_, err = db.Exec("UPDATE orders SET payment_status = ? WHERE id = ?", newPayment, id)
	if err != nil {
		http.Error(w, "Failed to update payment status", http.StatusInternalServerError)
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
		Value:    signSessionValue("authenticated"),
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
	staff := isStaff(r)
	currentUser := getLoggedInCustomer(r)

	if !staff && currentUser == nil {
		http.Error(w, "Unauthorized: Login required to access API", http.StatusUnauthorized)
		return
	}

	query := `
		SELECT o.id, o.user_id, o.customer_name,
		       COALESCE(NULLIF(o.customer_phone, ''), u.phone, ''),
		       COALESCE(NULLIF(o.delivery_address, ''), u.address, 'Shop Pickup'),
		       o.service_name, o.quantity, o.total_cost, o.status,
		       COALESCE(NULLIF(o.payment_status, ''), 'Unpaid')
		FROM orders o
		LEFT JOIN users u ON o.user_id = u.id
		WHERE 1=1`
	var args []interface{}

	if !staff && currentUser != nil {
		query += " AND (o.user_id = ? OR (o.user_id = 0 AND o.customer_name = ?))"
		args = append(args, currentUser.ID, currentUser.FullName)
	}

	query += " ORDER BY o.id DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, "Could not load orders: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.CustomerName, &o.CustomerPhone, &o.DeliveryAddress, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status, &o.PaymentStatus); err == nil {
			orders = append(orders, o)
		}
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error reading orders: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func ExportCSVHandler(w http.ResponseWriter, r *http.Request) {
	if !isStaff(r) {
		http.Error(w, "Unauthorized: Staff login required to export data", http.StatusUnauthorized)
		return
	}
	rows, err := db.Query("SELECT id, customer_name, service_name, quantity, total_cost, status, COALESCE(NULLIF(payment_status, ''), 'Unpaid') FROM orders ORDER BY id DESC")
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

	writer.Write([]string{"Order ID", "Customer Name", "Service", "Quantity", "Total Cost (NGN)", "Status", "Payment Status"})

	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.CustomerName, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status, &o.PaymentStatus); err != nil {
			continue
		}
		writer.Write([]string{
			strconv.Itoa(o.ID),
			o.CustomerName,
			o.ServiceName,
			strconv.Itoa(o.Quantity),
			strconv.Itoa(o.TotalCost),
			o.Status,
			o.PaymentStatus,
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
	err = db.QueryRow(`
		SELECT o.id, o.user_id, o.customer_name,
		       COALESCE(NULLIF(o.customer_phone, ''), u.phone, ''),
		       COALESCE(NULLIF(o.delivery_address, ''), u.address, 'Shop Pickup'),
		       o.service_name, o.quantity, o.total_cost, o.status,
		       COALESCE(NULLIF(o.payment_status, ''), 'Unpaid')
		FROM orders o
		LEFT JOIN users u ON o.user_id = u.id
		WHERE o.id = ?`,
		orderID,
	).Scan(&o.ID, &o.UserID, &o.CustomerName, &o.CustomerPhone, &o.DeliveryAddress, &o.ServiceName, &o.Quantity, &o.TotalCost, &o.Status, &o.PaymentStatus)
	if err != nil {
		http.Error(w, "Order receipt not found", http.StatusNotFound)
		return
	}

	// Security Check: Only Staff or the Customer who owns the order can view the receipt
	staff := isStaff(r)
	currentUser := getLoggedInCustomer(r)
	ownsOrder := currentUser != nil && (o.UserID == currentUser.ID || (o.UserID == 0 && o.CustomerName == currentUser.FullName))

	if !staff && !ownsOrder {
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
	rawID, ok := verifySessionValue(cookie.Value)
	if !ok {
		return nil
	}
	userID, err := strconv.Atoi(rawID)
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
	// 1. If GET request, render the dedicated Signup page
	if r.Method == http.MethodGet {
		tmpl, err := template.ParseFiles("templates/signup.html")
		if err != nil {
			http.Error(w, "Failed to load signup page", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
		return
	}

	// 2. Only allow POST for form submission
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	address := strings.TrimSpace(r.FormValue("address"))
	password := r.FormValue("password")

	if fullName == "" || email == "" || phone == "" || address == "" || password == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}
	cleanPhone, valid := normalizeAndValidatePhone(phone)
	if !valid {
		http.Error(w, "Invalid Nigerian phone number (e.g. 08031234567)", http.StatusBadRequest)
		return
	}
	phone = cleanPhone

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Server error encrypting password", http.StatusInternalServerError)
		return
	}

	res, err := db.Exec(
		"INSERT INTO users (full_name, email, phone, address, password_hash) VALUES (?, ?, ?, ?, ?)",
		fullName, email, phone, address, string(hashedBytes),
	)
	if err != nil {
		http.Error(w, "Email or phone number is already registered", http.StatusConflict)
		return
	}

	userID, _ := res.LastInsertId()

	http.SetCookie(w, &http.Cookie{
		Name:     "customer_session",
		Value:    signSessionValue(strconv.FormatInt(userID, 10)),
		Path:     "/",
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// CustomerLoginHandler authenticates a customer by phone or email + password
func CustomerLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identifier := strings.TrimSpace(r.FormValue("identifier"))
	password := r.FormValue("password")

	// 1. Unified Admin Check
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@apexwash.com"
	}
	staffPassword := os.Getenv("STAFF_PASSWORD")
	if staffPassword == "" {
		staffPassword = "admin123"
	}

	if strings.EqualFold(identifier, adminEmail) && password == staffPassword {
		http.SetCookie(w, &http.Cookie{Name: "customer_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated"), Path: "/", HttpOnly: true})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if normalizedPhone, ok := normalizeAndValidatePhone(identifier); ok {
		identifier = normalizedPhone
	}

	// 2. Regular Customer Check (with Blacklist and Specific Errors)
	var userID int
	var passwordHash string
	var accountStatus string

	err := db.QueryRow(
		"SELECT id, password_hash, account_status FROM users WHERE email = ? OR phone = ?",
		identifier, identifier,
	).Scan(&userID, &passwordHash, &accountStatus)

	if err == sql.ErrNoRows {
		// INLINE ERROR: Account not found
		redirectURL := "/?login_error=Account not found. Please create an account.&identifier=" + url.QueryEscape(identifier)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	} else if err != nil {
		redirectURL := "/?login_error=Server error. Please try again.&identifier=" + url.QueryEscape(identifier)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// BLACKLIST GATEKEEPER
	if accountStatus == "blacklisted" {
		redirectURL := "/?login_error=This account is suspended. Please contact support.&identifier=" + url.QueryEscape(identifier)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// 3. Password Verification
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		// INLINE ERROR: Wrong password
		redirectURL := "/?login_error=Incorrect password. Please try again.&identifier=" + url.QueryEscape(identifier)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Success
	http.SetCookie(w, &http.Cookie{
		Name:     "customer_session",
		Value:    signSessionValue(strconv.Itoa(userID)),
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

// normalizeAndValidatePhone cleans a Nigerian phone number to a standard 11-digit
// format (e.g. "08031234567") and validates its length, digits, and network prefix.
func normalizeAndValidatePhone(raw string) (string, bool) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")

	if strings.HasPrefix(cleaned, "+234") {
		cleaned = "0" + strings.TrimPrefix(cleaned, "+234")
	} else if strings.HasPrefix(cleaned, "234") && len(cleaned) == 13 {
		cleaned = "0" + strings.TrimPrefix(cleaned, "234")
	}

	if len(cleaned) != 11 {
		return "", false
	}

	for _, ch := range cleaned {
		if ch < '0' || ch > '9' {
			return "", false
		}
	}

	validPrefixes := []string{"070", "071", "080", "081", "090", "091"}
	prefix := cleaned[:3]
	for _, vp := range validPrefixes {
		if prefix == vp {
			return cleaned, true
		}
	}

	return "", false
}
