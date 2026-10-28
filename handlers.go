package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
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

func isSuperAdmin(r *http.Request) bool {
	cookie, err := r.Cookie("super_admin_session")
	if err == nil {
		if val, ok := verifySessionValue(cookie.Value); ok && val == "super_admin" {
			return true
		}
	}
	return false
}

func BookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not Allowed", http.StatusMethodNotAllowed)
		return
	}

	staff := isStaff(r)
	currentUser := getLoggedInCustomer(r)

	// Gatekeeper: If not Staff and not a logged-in Customer, save cart in a cookie and redirect to Signup
	if !staff && currentUser == nil {
		service := strings.TrimSpace(r.FormValue("service_name"))
		qtyStr := strings.TrimSpace(r.FormValue("quantity"))

		if service != "" && qtyStr != "" {
			qty, _ := strconv.Atoi(qtyStr)
			if qty > 0 {
				pending := PendingOrder{ServiceName: service, Quantity: qty}
				pendingJSON, _ := json.Marshal(pending)

				http.SetCookie(w, &http.Cookie{
					Name:     "pending_order",
					Value:    base64.StdEncoding.EncodeToString(pendingJSON),
					Path:     "/",
					MaxAge:   3600,
					HttpOnly: true,
				})
			}
		}
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	customer := strings.TrimSpace(r.FormValue("customer_name"))
	phone := strings.TrimSpace(r.FormValue("customer_phone"))
	address := strings.TrimSpace(r.FormValue("delivery_address"))
	userID := 0

	if currentUser != nil {
		userID = currentUser.ID
		customer = currentUser.FullName
		phone = currentUser.Phone
		if address == "" {
			address = currentUser.Address
		}
	} else if staff {
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

	var unitPrice float64
	err = db.QueryRow("SELECT price FROM services WHERE name = ?", service).Scan(&unitPrice)
	if err != nil {
		http.Error(w, "Selected service not found", http.StatusBadRequest)
		return
	}

	totalCost := int(float64(qty) * unitPrice)

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
			if searchQuery != "" {
				query += " AND o.customer_name LIKE ?"
				args = append(args, "%"+searchQuery+"%")
			}
		} else if currentUser != nil {
			query += " AND (o.user_id = ? OR (o.user_id = 0 AND o.customer_name = ?))"
			args = append(args, currentUser.ID, currentUser.FullName)
		}

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
		Services:     getServicesFromDB(),
		Orders:       orders,
		TotalOrders:  len(orders),
		TotalRevenue: totalRevenue,
		SearchQuery:  searchQuery,
		StatusFilter: statusFilter,
		IsStaff:      staff,
		IsSuperAdmin: isSuperAdmin(r),
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
	http.SetCookie(w, &http.Cookie{
		Name:     "super_admin_session",
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

func SignupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl, err := template.ParseFiles("templates/signup.html")
		if err != nil {
			http.Error(w, "Failed to load signup page", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
		return
	}

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

	var newUser User
	db.QueryRow("SELECT id, full_name, phone, address FROM users WHERE id = ?", userID).Scan(&newUser.ID, &newUser.FullName, &newUser.Phone, &newUser.Address)
	processPendingOrder(w, r, &newUser)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func CustomerLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identifier := strings.TrimSpace(r.FormValue("identifier"))
	password := r.FormValue("password")

	// 1. Unified Master Admin Check
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
		http.SetCookie(w, &http.Cookie{Name: "super_admin_session", Value: signSessionValue("super_admin"), Path: "/", HttpOnly: true})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if normalizedPhone, ok := normalizeAndValidatePhone(identifier); ok {
		identifier = normalizedPhone
	}

	// 2. Database Account Check
	var userID int
	var passwordHash string
	var role string
	var accountStatus string

	err := db.QueryRow(
		"SELECT id, password_hash, COALESCE(role, 'customer'), COALESCE(account_status, 'active') FROM users WHERE email = ? OR phone = ?",
		identifier, identifier,
	).Scan(&userID, &passwordHash, &role, &accountStatus)

	if err == sql.ErrNoRows {
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
		redirectURL := "/?login_error=Incorrect password. Please try again.&identifier=" + url.QueryEscape(identifier)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// 4. Role-Based Session Assignment
	switch role {
	case "super_admin":
		http.SetCookie(w, &http.Cookie{Name: "customer_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated"), Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "super_admin_session", Value: signSessionValue("super_admin"), Path: "/", HttpOnly: true})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	case "staff":
		http.SetCookie(w, &http.Cookie{Name: "customer_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "super_admin_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated"), Path: "/", HttpOnly: true})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Regular Customer Success
	http.SetCookie(w, &http.Cookie{
		Name:     "customer_session",
		Value:    signSessionValue(strconv.Itoa(userID)),
		Path:     "/",
		HttpOnly: true,
	})

	var loggedInUser User
	db.QueryRow("SELECT id, full_name, phone, address FROM users WHERE id = ?", userID).Scan(&loggedInUser.ID, &loggedInUser.FullName, &loggedInUser.Phone, &loggedInUser.Address)
	processPendingOrder(w, r, &loggedInUser)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

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

func processPendingOrder(w http.ResponseWriter, r *http.Request, user *User) {
	cookie, err := r.Cookie("pending_order")
	if err != nil || cookie.Value == "" {
		return
	}

	decodedJSON, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return
	}

	var pending PendingOrder
	if err := json.Unmarshal(decodedJSON, &pending); err != nil {
		return
	}

	var unitPrice float64
	err = db.QueryRow("SELECT price FROM services WHERE name = ?", pending.ServiceName).Scan(&unitPrice)
	if err != nil {
		return
	}

	totalCost := float64(pending.Quantity) * unitPrice

	insertSQL := `INSERT INTO orders (user_id, customer_name, customer_phone, delivery_address, service_name, quantity, total_cost, status, payment_status) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(insertSQL, user.ID, user.FullName, user.Phone, user.Address, pending.ServiceName, pending.Quantity, totalCost, "Pending", "Unpaid")

	if err == nil {
		http.SetCookie(w, &http.Cookie{Name: "pending_order", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	}
}

func getServicesFromDB() []Service {
	rows, err := db.Query("SELECT id, name, price FROM services ORDER BY id ASC")
	if err != nil {
		return []Service{}
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.Name, &s.Price); err == nil {
			services = append(services, s)
		}
	}
	if err := rows.Err(); err != nil {
		return []Service{}
	}

	return services
}

// AdminDashboardHandler renders the Super Admin control center
func AdminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	if !isSuperAdmin(r) {
		http.Error(w, "Forbidden: Super Admin access required", http.StatusForbidden)
		return
	}

	rows, err := db.Query("SELECT id, full_name, email, phone, address, COALESCE(role, 'customer'), COALESCE(account_status, 'active') FROM users ORDER BY id DESC")
	if err != nil {
		http.Error(w, "Failed to load users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &u.Address, &u.Role, &u.AccountStatus); err == nil {
			users = append(users, u)
		}
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error reading users list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := AdminDashboardData{
		ShopName: "ApexWash",
		Services: getServicesFromDB(),
		Users:    users,
		Message:  r.URL.Query().Get("msg"),
	}

	tmpl, err := template.ParseFiles("templates/admin.html")
	if err != nil {
		http.Error(w, "Failed to load admin template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Failed to render admin template: "+err.Error(), http.StatusInternalServerError)
	}
}

// SaveServiceHandler adds a new service or updates an existing service's price
func SaveServiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isSuperAdmin(r) {
		http.Error(w, "Forbidden: Super Admin access required", http.StatusForbidden)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	priceStr := strings.TrimSpace(r.FormValue("price"))
	price, err := strconv.ParseFloat(priceStr, 64)
	if name == "" || err != nil || price <= 0 {
		http.Error(w, "Valid service name and positive price required", http.StatusBadRequest)
		return
	}

	res, err := db.Exec("UPDATE services SET price = ? WHERE name = ?", price, name)
	if err != nil {
		http.Error(w, "Failed to update service", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		_, err = db.Exec("INSERT INTO services (name, price) VALUES (?, ?)", name, price)
		if err != nil {
			http.Error(w, "Failed to insert service", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/admin?msg=Service+pricing+updated", http.StatusSeeOther)
}

// DeleteServiceHandler removes a service from the catalog
func DeleteServiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isSuperAdmin(r) {
		http.Error(w, "Forbidden: Super Admin access required", http.StatusForbidden)
		return
	}

	serviceID := r.FormValue("service_id")
	_, _ = db.Exec("DELETE FROM services WHERE id = ?", serviceID)
	http.Redirect(w, r, "/admin?msg=Service+removed", http.StatusSeeOther)
}

// UpdateUserAccessHandler lets Super Admin change a user's role or blacklist/activate them
func UpdateUserAccessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isSuperAdmin(r) {
		http.Error(w, "Forbidden: Super Admin access required", http.StatusForbidden)
		return
	}

	userID := r.FormValue("user_id")
	role := r.FormValue("role")
	status := r.FormValue("account_status")

	if role != "customer" && role != "staff" && role != "super_admin" {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}
	if status != "active" && status != "blacklisted" {
		http.Error(w, "Invalid account status", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("UPDATE users SET role = ?, account_status = ? WHERE id = ?", role, status, userID)
	if err != nil {
		http.Error(w, "Failed to update user access", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin?msg=User+permissions+updated", http.StatusSeeOther)
}
