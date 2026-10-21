package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB initializes a clean in-memory SQLite database for each test
func setupTestDB(t *testing.T) {
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	createTableSQL := `CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_name TEXT,
		service_name TEXT,
		quantity INTEGER,
		total_cost INTEGER,
		status TEXT
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
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
	defer db.Close()

	// Insert a dummy order into the in-memory test DB
	_, err := db.Exec(
		"INSERT INTO orders (customer_name, service_name, quantity, total_cost, status) VALUES (?, ?, ?, ?, ?)",
		"Wealth Ibe", "Suit (Dry Clean)", 2, 5000, "Pending",
	)
	if err != nil {
		t.Fatalf("Failed to insert test order: %v", err)
	}

	// Case A: Valid order ID (id=1) should return 200 OK
	reqValid := httptest.NewRequest(http.MethodGet, "/receipt?id=1", nil)
	recValid := httptest.NewRecorder()
	ReceiptHandler(recValid, reqValid)

	if recValid.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for valid receipt, got %d", recValid.Code)
	}

	// Case B: Non-existent order ID (id=999) should return 404 Not Found
	reqMissing := httptest.NewRequest(http.MethodGet, "/receipt?id=999", nil)
	recMissing := httptest.NewRecorder()
	ReceiptHandler(recMissing, reqMissing)

	if recMissing.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for missing receipt, got %d", recMissing.Code)
	}
}
