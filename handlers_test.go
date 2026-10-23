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
		customer_name TEXT NOT NULL,
		service_name TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		total_cost INTEGER NOT NULL,
		status TEXT DEFAULT 'Pending'
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

	// Check that the staff_session cookie was set
	cookies := recGood.Result().Cookies()
	foundCookie := false
	for _, c := range cookies {
		if c.Name == "staff_session" && c.Value == "authenticated" {
			foundCookie = true
		}
	}
	if !foundCookie {
		t.Errorf("Expected staff_session cookie to be set on successful login")
	}
}

// Test 3: Book an order and verify it appears in the JSON API with accurate pricing
func TestBookAndAPIOrdersHandler(t *testing.T) {
	setupTestDB(t)
	defer db.Close()

	// Pick the first service from Services catalog dynamically so the test never breaks if prices change
	testService := serviceList[0]

	form := url.Values{}
	form.Add("customer_name", "Wealth Test")
	form.Add("service_name", testService.Name)
	form.Add("quantity", "3")

	reqBook := httptest.NewRequest(http.MethodPost, "/book", strings.NewReader(form.Encode()))
	reqBook.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBook := httptest.NewRecorder()

	BookHandler(recBook, reqBook)

	if recBook.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 Redirect after booking, got %d", recBook.Code)
	}

	// Now call GET /api/orders to verify the order was saved and priced properly
	reqAPI := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
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

	expectedCost := testService.Price * 3
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
	reqAuth.AddCookie(&http.Cookie{Name: "staff_session", Value: "authenticated"})
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

	// 2. Test authorized access (with staff_session cookie) -> should return 200 OK
	authReq := httptest.NewRequest(http.MethodGet, "/receipt?id=1", nil)
	authReq.AddCookie(&http.Cookie{
		Name:  "staff_session",
		Value: "authenticated",
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
		Value: "authenticated",
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

	// Verify customer_session cookie was issued
	foundCookie := false
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "customer_session" && c.Value != "" {
			foundCookie = true
		}
	}
	if !foundCookie {
		t.Errorf("Expected customer_session cookie to be set upon login")
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
		Value: strconv.FormatInt(wealthID, 10),
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
		Value: "1",
	})

	rr := httptest.NewRecorder()
	CustomerLoginHandler(rr, req)

	// 2. Expect a 303 redirect back to "/"
	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 SeeOther, got %d", rr.Code)
	}

	// 3. Verify staff_session is issued and customer_session is cleared
	var hasStaffCookie bool
	var clearedCustomerCookie bool

	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "staff_session" && cookie.Value == "authenticated" {
			hasStaffCookie = true
		}
		if cookie.Name == "customer_session" && cookie.MaxAge < 0 {
			clearedCustomerCookie = true
		}
	}

	if !hasStaffCookie {
		t.Error("Expected staff_session cookie to be set to 'authenticated'")
	}
	if !clearedCustomerCookie {
		t.Error("Expected existing customer_session cookie to be cleared (MaxAge < 0)")
	}
}
