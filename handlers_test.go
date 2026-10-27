package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB initializes a clean in-memory SQLite database for each test
func setupTestDB(t *testing.T) {
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// 1. Create the orders table for testing
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER DEFAULT 0,
		customer_name TEXT NOT NULL,
		customer_phone TEXT DEFAULT '',
		delivery_address TEXT DEFAULT '',
		service_name TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		total_cost INTEGER NOT NULL,
		status TEXT DEFAULT 'Pending',
		payment_status TEXT DEFAULT 'Unpaid'
	)`)
	if err != nil {
		t.Fatalf("Failed to create orders table: %v", err)
	}

	// 2. Create the users table for testing
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL,
		phone TEXT UNIQUE NOT NULL,
		address TEXT NOT NULL,
		password_hash TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// 3. Create the services table for testing
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		price REAL NOT NULL
	)`)
	if err != nil {
		t.Fatalf("Failed to create services table: %v", err)
	}

	// Seed a default test service
	_, err = db.Exec(`INSERT INTO services (name, price) VALUES ('Shirt (Wash and Iron)', 1500)`)
	if err != nil {
		t.Fatalf("Failed to seed test service: %v", err)
	}
}

// Test 1: Ensure unauthenticated users cannot update order status
func TestUnauthorizedUpdateStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/update-status", nil)
	rec := httptest.NewRecorder()

	UpdateStatusHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %d", rec.Code)
	}
}

// Test 2: Verify Staff Login rejects wrong password and accepts valid password
func TestLoginHandler(t *testing.T) {
	// Case A: Wrong password
	badForm := url.Values{}
	badForm.Add("password", "wrongpass")
	reqBad := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(badForm.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()

	LoginHandler(recBad, reqBad)

	if recBad.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for wrong password, got %d", recBad.Code)
	}

	// Case B: Correct default password ("apexadmin")
	goodForm := url.Values{}
	goodForm.Add("password", "apexadmin")
	reqGood := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(goodForm.Encode()))
	reqGood.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recGood := httptest.NewRecorder()

	LoginHandler(recGood, reqGood)

	if recGood.Code != http.StatusSeeOther {
		t.Errorf("Expected 303 Redirect for valid login, got %d", recGood.Code)
	}

	// Check that the signed staff_session cookie was set
	cookies := recGood.Result().Cookies()
	foundCookie := false
	for _, c := range cookies {
		if c.Name == "staff_session" && c.Value == signSessionValue("authenticated") {
			foundCookie = true
		}
	}
	if !foundCookie {
		t.Errorf("Expected signed staff_session cookie to be set on successful login")
	}
}

// Test 3: Book an order and verify it appears in the JSON API with accurate pricing
func TestBookAndAPIOrdersHandler(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	// Pick the first service from database dynamically
	services := getServicesFromDB()
	testService := services[0]

	form := url.Values{}
	form.Add("customer_name", "Wealth Test")
	form.Add("service_name", testService.Name)
	form.Add("quantity", "3")

	reqBook := httptest.NewRequest(http.MethodPost, "/book", strings.NewReader(form.Encode()))
	reqBook.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBook.AddCookie(&http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated")})
	recBook := httptest.NewRecorder()

	BookHandler(recBook, reqBook)

	if recBook.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 Redirect after booking, got %d", recBook.Code)
	}

	// Now call GET /api/orders with Staff cookie to verify the order was saved and priced properly
	reqAPI := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	reqAPI.AddCookie(&http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated")})
	recAPI := httptest.NewRecorder()

	APIorderHandler(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/orders, got %d", recAPI.Code)
	}

	var orders []Order
	if err := json.NewDecoder(recAPI.Body).Decode(&orders); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if len(orders) != 1 {
		t.Fatalf("Expected 1 order in database, got %d", len(orders))
	}

	expectedCost := int(testService.Price) * 3
	if orders[0].CustomerName != "Wealth Test" || orders[0].TotalCost != expectedCost {
		t.Errorf("Order mismatch: got customer=%s cost=%d, want customer=Wealth Test cost=%d",
			orders[0].CustomerName, orders[0].TotalCost, expectedCost)
	}
}

// Test 4: Verify CSV Export requires Staff Auth and returns text/csv
func TestExportCSVHandler(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	// Case A: Unauthenticated request should fail with 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/export-csv", nil)
	recUnauth := httptest.NewRecorder()
	ExportCSVHandler(recUnauth, reqUnauth)

	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for CSV export without cookie, got %d", recUnauth.Code)
	}

	// Case B: Authenticated staff request should return 200 OK and text/csv
	reqAuth := httptest.NewRequest(http.MethodGet, "/export-csv", nil)
	reqAuth.AddCookie(&http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated")})
	recAuth := httptest.NewRecorder()
	ExportCSVHandler(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for authenticated CSV export, got %d", recAuth.Code)
	}

	contentType := recAuth.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("Expected Content-Type text/csv, got %s", contentType)
	}
}

// Test 5: Verify ReceiptHandler returns 200 OK for valid order and 404 for missing order
func TestReceiptHandler(t *testing.T) {
	setupTestDB(t)

	// Seed a sample order in the test database
	_, err := db.Exec(
		"INSERT INTO orders (customer_name, service_name, quantity, total_cost, status) VALUES (?, ?, ?, ?, ?)",
		"Wealth Ibe", "Suit (2-Piece)", 2, 5000, "Ready for Pickup",
	)
	if err != nil {
		t.Fatalf("Failed to insert test order: %v", err)
	}

	// 1. Test unauthenticated guest access -> should be blocked with 403 Forbidden
	unauthReq := httptest.NewRequest(http.MethodGet, "/receipt?id=1", nil)
	unauthRR := httptest.NewRecorder()
	ReceiptHandler(unauthRR, unauthReq)

	if unauthRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for unauthenticated receipt access, got %d", unauthRR.Code)
	}

	// 2. Test authorized access (with signed staff_session cookie) -> should return 200 OK
	authReq := httptest.NewRequest(http.MethodGet, "/receipt?id=1", nil)
	authReq.AddCookie(&http.Cookie{
		Name:  "staff_session",
		Value: signSessionValue("authenticated"),
	})
	authRR := httptest.NewRecorder()
	ReceiptHandler(authRR, authReq)

	if authRR.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for valid authorized receipt, got %d", authRR.Code)
	}
	if !strings.Contains(authRR.Body.String(), "Wealth Ibe") {
		t.Errorf("Expected receipt body to contain customer name 'Wealth Ibe'")
	}

	// 3. Test non-existent order ID -> should return 404 Not Found
	notFoundReq := httptest.NewRequest(http.MethodGet, "/receipt?id=999", nil)
	notFoundReq.AddCookie(&http.Cookie{
		Name:  "staff_session",
		Value: signSessionValue("authenticated"),
	})
	notFoundRR := httptest.NewRecorder()
	ReceiptHandler(notFoundRR, notFoundReq)

	if notFoundRR.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for missing order receipt, got %d", notFoundRR.Code)
	}
}

func TestCustomerSignupAndLogin(t *testing.T) {
	setupTestDB(t)

	// 1. Test Customer Signup
	signupData := url.Values{}
	signupData.Set("full_name", "Bilyamin Umar")
	signupData.Set("email", "bilyamin@example.com")
	signupData.Set("phone", "08031234567")
	signupData.Set("address", "14 Ahmadu Bello Way")
	signupData.Set("password", "superSecretWash123")

	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(signupData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	SignupHandler(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected signup redirect 303, got %d", rr.Code)
	}

	// Verify password was hashed with bcrypt in SQLite (never stored as plain text)
	var storedHash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE phone = ?", "08031234567").Scan(&storedHash)
	if err != nil {
		t.Fatalf("Failed to query newly created user: %v", err)
	}
	if storedHash == "superSecretWash123" || !strings.HasPrefix(storedHash, "$2a$") {
		t.Errorf("Expected bcrypt hash starting with $2a$, got %s", storedHash)
	}

	// 2. Test Customer Login with Phone Number + Password
	loginData := url.Values{}
	loginData.Set("identifier", "08031234567")
	loginData.Set("password", "superSecretWash123")

	loginReq := httptest.NewRequest(http.MethodPost, "/login-customer", strings.NewReader(loginData.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()

	CustomerLoginHandler(loginRR, loginReq)

	if loginRR.Code != http.StatusSeeOther {
		t.Fatalf("Expected login redirect 303, got %d", loginRR.Code)
	}

	// Verify signed customer_session cookie was issued
	foundCookie := false
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "customer_session" {
			if val, ok := verifySessionValue(c.Value); ok && val != "" {
				foundCookie = true
			}
		}
	}
	if !foundCookie {
		t.Errorf("Expected valid HMAC-signed customer_session cookie to be set upon login")
	}
}

func TestCustomerOrderPrivacy(t *testing.T) {
	setupTestDB(t)

	// Create Customer 1 (Wealth Ibe)
	res, err := db.Exec(
		"INSERT INTO users (full_name, email, phone, address, password_hash) VALUES (?, ?, ?, ?, ?)",
		"Wealth Ibe", "wealth@example.com", "08011111111", "Otukpo", "hash1",
	)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
	wealthID, _ := res.LastInsertId()

	// Seed two orders: one for Wealth Ibe, one for another customer (Chinedu Okafor)
	_, _ = db.Exec("INSERT INTO orders (customer_name, service_name, quantity, total_cost, status) VALUES (?, ?, ?, ?, ?)",
		"Wealth Ibe", "Suit (2-Piece)", 1, 2500, "Pending")
	_, _ = db.Exec("INSERT INTO orders (customer_name, service_name, quantity, total_cost, status) VALUES (?, ?, ?, ?, ?)",
		"Chinedu Okafor", "Agbada / Senator", 2, 4000, "Pending")

	// Request HomeHandler logged in as Wealth Ibe
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "customer_session",
		Value: signSessionValue(strconv.FormatInt(wealthID, 10)),
	})
	rr := httptest.NewRecorder()

	HomeHandler(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Wealth Ibe") {
		t.Errorf("Expected logged-in customer to see their own order")
	}
	if strings.Contains(body, "Chinedu Okafor") {
		t.Errorf("Privacy violation: Customer 'Wealth Ibe' should NOT see 'Chinedu Okafor's order")
	}
}

func TestUnifiedAdminLogin(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	// 1. Send Admin credentials (with uppercase ADMIN to test case-insensitivity)
	form := url.Values{}
	form.Add("identifier", "ADMIN@apexwash.com")
	form.Add("password", "admin123")

	req := httptest.NewRequest(http.MethodPost, "/login-customer", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Attach an active customer cookie to verify Admin login clears it
	req.AddCookie(&http.Cookie{
		Name:  "customer_session",
		Value: signSessionValue("1"),
	})

	rr := httptest.NewRecorder()
	CustomerLoginHandler(rr, req)

	// 2. Expect a 303 redirect back to "/"
	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 SeeOther, got %d", rr.Code)
	}

	// 3. Verify signed staff_session is issued and customer_session is cleared
	var hasStaffCookie bool
	var clearedCustomerCookie bool

	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "staff_session" && cookie.Value == signSessionValue("authenticated") {
			hasStaffCookie = true
		}
		if cookie.Name == "customer_session" && cookie.MaxAge < 0 {
			clearedCustomerCookie = true
		}
	}

	if !hasStaffCookie {
		t.Error("Expected signed staff_session cookie to be set")
	}
	if !clearedCustomerCookie {
		t.Error("Expected existing customer_session cookie to be cleared (MaxAge < 0)")
	}
}

func TestPhoneValidation(t *testing.T) {
	// 1. Test valid international format with spaces and dashes
	clean, ok := normalizeAndValidatePhone("+234 803-123-4567")
	if !ok || clean != "08031234567" {
		t.Errorf("Expected '+234 803-123-4567' to normalize to '08031234567', got '%s' (ok=%v)", clean, ok)
	}

	// 2. Test invalid network prefix (060 does not exist in Nigeria)
	_, ok = normalizeAndValidatePhone("06012345678")
	if ok {
		t.Error("Expected invalid prefix '06012345678' to be rejected")
	}

	// 3. Test SignupHandler rejects bad phone number with 400 Bad Request
	setupTestDB(t)
	defer db.Close()

	form := url.Values{}
	form.Add("full_name", "Bad Phone User")
	form.Add("email", "badphone@example.com")
	form.Add("phone", "12345")
	form.Add("address", "10 Test Street")
	form.Add("password", "secret123")

	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	SignupHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid phone number, got %d", rr.Code)
	}
}

func TestSameNameOrderIsolation(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	// 1. Insert two distinct accounts with the exact same Full Name ("John Doe")
	_, err := db.Exec(`
		INSERT INTO users (id, full_name, email, phone, address, password_hash)
		VALUES 
			(1, 'John Doe', 'john1@example.com', '08031111111', '1 First St', 'hash1'),
			(2, 'John Doe', 'john2@example.com', '08032222222', '2 Second St', 'hash2')
	`)
	if err != nil {
		t.Fatalf("Failed to seed duplicate-name users: %v", err)
	}

	// 2. Customer #1 ("John Doe", ID 1) books an order
	form := url.Values{}
	form.Add("service_name", "Wash & Fold")
	form.Add("quantity", "3")

	bookReq := httptest.NewRequest(http.MethodPost, "/book", strings.NewReader(form.Encode()))
	bookReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	bookReq.AddCookie(&http.Cookie{Name: "customer_session", Value: signSessionValue("1")})

	bookRR := httptest.NewRecorder()
	BookHandler(bookRR, bookReq)

	if bookRR.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 SeeOther on booking, got %d", bookRR.Code)
	}

	// 3. Customer #2 (also named "John Doe", ID 2) tries to view Order #1's receipt
	receiptReq := httptest.NewRequest(http.MethodGet, "/receipt?id=1", nil)
	receiptReq.AddCookie(&http.Cookie{Name: "customer_session", Value: signSessionValue("2")})

	receiptRR := httptest.NewRecorder()
	ReceiptHandler(receiptRR, receiptReq)

	if receiptRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden when a different 'John Doe' (ID 2) accesses Order #1, got %d", receiptRR.Code)
	}
}

func TestWhatsAppURLGeneration(t *testing.T) {
	// 1. Order with registered Nigerian phone number & "Ready for Pickup" status
	readyOrder := Order{
		ID:            7,
		CustomerName:  "Wealth",
		CustomerPhone: "08031234567",
		ServiceName:   "Wash & Fold",
		Quantity:      3,
		TotalCost:     4500,
		Status:        "Ready for Pickup",
		PaymentStatus: "Paid",
	}

	waURL := readyOrder.WhatsAppURL()
	if !strings.HasPrefix(waURL, "https://wa.me/2348031234567?text=") {
		t.Errorf("Expected WhatsApp URL to start with https://wa.me/2348031234567?text=, got %s", waURL)
	}
	if !strings.Contains(waURL, "READY+for+pickup") {
		t.Errorf("Expected Ready message in URL, got %s", waURL)
	}

	// 2. Guest walk-in order with no phone number should return empty string
	guestOrder := Order{
		ID:            8,
		CustomerName:  "Walk-in Guest",
		CustomerPhone: "",
		Status:        "Pending",
	}
	if guestOrder.WhatsAppURL() != "" {
		t.Errorf("Expected empty WhatsAppURL for guest without phone, got %s", guestOrder.WhatsAppURL())
	}
}

func TestBookingGatekeeperAndWalkIn(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	services := getServicesFromDB()

	// 1. Unauthenticated guest tries to book -> must redirect to /signup
	guestForm := url.Values{}
	guestForm.Add("customer_name", "Anonymous Guest")
	guestForm.Add("service_name", services[0].Name)
	guestForm.Add("quantity", "2")

	guestReq := httptest.NewRequest(http.MethodPost, "/book", strings.NewReader(guestForm.Encode()))
	guestReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	guestRR := httptest.NewRecorder()

	BookHandler(guestRR, guestReq)

	if guestRR.Code != http.StatusSeeOther || guestRR.Header().Get("Location") != "/signup" {
		t.Fatalf("Expected unauthenticated booking to redirect to /signup, got status %d (Location: %s)",
			guestRR.Code, guestRR.Header().Get("Location"))
	}

	// 2. Staff logs a walk-in order requiring home delivery & WhatsApp notifications
	staffForm := url.Values{}
	staffForm.Add("customer_name", "Chief Walk-In")
	staffForm.Add("customer_phone", "+2348039998888")
	staffForm.Add("delivery_address", "14 GRA Road, Otukpo")
	staffForm.Add("service_name", services[0].Name)
	staffForm.Add("quantity", "4")

	staffReq := httptest.NewRequest(http.MethodPost, "/book", strings.NewReader(staffForm.Encode()))
	staffReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	staffReq.AddCookie(&http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated")})
	staffRR := httptest.NewRecorder()

	BookHandler(staffRR, staffReq)

	if staffRR.Code != http.StatusSeeOther || staffRR.Header().Get("Location") != "/" {
		t.Fatalf("Expected staff walk-in booking to redirect to /, got %d", staffRR.Code)
	}

	// Verify phone normalization and delivery address persistence in SQLite
	var savedPhone, savedAddr string
	err := db.QueryRow("SELECT customer_phone, delivery_address FROM orders WHERE customer_name = 'Chief Walk-In'").Scan(&savedPhone, &savedAddr)
	if err != nil {
		t.Fatalf("Failed to query saved walk-in order: %v", err)
	}
	if savedPhone != "08039998888" {
		t.Errorf("Expected normalized walk-in phone 08039998888, got %s", savedPhone)
	}
	if savedAddr != "14 GRA Road, Otukpo" {
		t.Errorf("Expected delivery address '14 GRA Road, Otukpo', got %s", savedAddr)
	}
}

// Test 13: Verify HMAC Cookie Tampering Rejection & /api/orders Authentication
func TestSessionTamperingAndAPISecurity(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	// Seed User #1 and User #2, plus an order belonging to User #2
	_, _ = db.Exec(`INSERT INTO users (id, full_name, email, phone, address, password_hash) VALUES 
		(1, 'User One', 'u1@example.com', '08030000001', 'Addr 1', 'hash1'),
		(2, 'User Two', 'u2@example.com', '08030000002', 'Addr 2', 'hash2')`)
	_, _ = db.Exec(`INSERT INTO orders (id, user_id, customer_name, service_name, quantity, total_cost, status) VALUES 
		(1, 2, 'User Two', 'Wash & Fold', 2, 2000, 'Pending')`)

	// 1. User #1 takes their valid signed cookie ("1.<sig_for_1>") and tampers with the ID in DevTools to "2.<sig_for_1>"
	validUser1Cookie := signSessionValue("1")
	parts := strings.SplitN(validUser1Cookie, ".", 2)
	tamperedCookie := "2." + parts[1]

	tamperedReq := httptest.NewRequest(http.MethodGet, "/receipt?id=1", nil)
	tamperedReq.AddCookie(&http.Cookie{Name: "customer_session", Value: tamperedCookie})
	tamperedRR := httptest.NewRecorder()

	ReceiptHandler(tamperedRR, tamperedReq)

	if tamperedRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for tampered session cookie, got %d", tamperedRR.Code)
	}

	// 2. Unauthenticated request to GET /api/orders must be rejected with 401 Unauthorized
	unauthAPIReq := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	unauthAPIRR := httptest.NewRecorder()
	APIorderHandler(unauthAPIRR, unauthAPIReq)

	if unauthAPIRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for unauthenticated /api/orders request, got %d", unauthAPIRR.Code)
	}
}

// Test 14: Verify Staff Payment Status Toggle (Unpaid <-> Paid)
func TestTogglePaymentHandler(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`INSERT INTO orders (id, customer_name, service_name, quantity, total_cost, status, payment_status)
		VALUES (1, 'Chief Obi', 'Agbada / Senator', 1, 2000, 'Pending', 'Unpaid')`)
	if err != nil {
		t.Fatalf("Failed to seed order: %v", err)
	}

	// 1. Unauthenticated toggle attempt -> 401 Unauthorized
	unauthForm := url.Values{"order_id": {"1"}}
	unauthReq := httptest.NewRequest(http.MethodPost, "/toggle-payment", strings.NewReader(unauthForm.Encode()))
	unauthReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unauthRR := httptest.NewRecorder()

	TogglePaymentHandler(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for unauthenticated payment toggle, got %d", unauthRR.Code)
	}

	// 2. Staff toggles payment -> should become "Paid"
	staffForm := url.Values{"order_id": {"1"}}
	staffReq := httptest.NewRequest(http.MethodPost, "/toggle-payment", strings.NewReader(staffForm.Encode()))
	staffReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	staffReq.AddCookie(&http.Cookie{Name: "staff_session", Value: signSessionValue("authenticated")})
	staffRR := httptest.NewRecorder()

	TogglePaymentHandler(staffRR, staffReq)
	if staffRR.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 SeeOther on staff payment toggle, got %d", staffRR.Code)
	}

	var payStatus string
	_ = db.QueryRow("SELECT payment_status FROM orders WHERE id = 1").Scan(&payStatus)
	if payStatus != "Paid" {
		t.Errorf("Expected payment_status to be 'Paid', got '%s'", payStatus)
	}
}
