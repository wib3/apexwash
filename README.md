# 🧺 ApexWash — The Sntino Executive Care (v4.0)

**Engineered by IB**  
🌐 **Live Demo:** [https://apexwash.onrender.com](https://apexwash.onrender.com) | 📡 **Live JSON API:** [https://apexwash.onrender.com/api/orders](https://apexwash.onrender.com/api/orders)

ApexWash is a full-stack, multi-tenant laundry order management and executive dashboard web application built in **Go (Golang)** using the standard library and backed by **SQLite**. It enables laundry operations to dynamically price services, authenticate customers and staff through a unified portal with HMAC-SHA256 cryptographic session security, enforce authenticated online bookings while supporting staff walk-in delivery logistics, track payment statuses (`Paid` / `Unpaid`), filter and search orders in real time, generate IDOR-protected printable digital receipts, send 1-click WhatsApp status alerts, export accounting data to CSV, and expose order data through a protected JSON REST API.

---

## ✨ Key Features

### 🛡️ v4.0 Security, Walk-In Logistics & Payment Tracking
- **Cryptographic Session Security (`HMAC-SHA256`):** Signs all `customer_session` and `staff_session` cookies using Go's `crypto/hmac` and `crypto/sha256` packages (`value.signature`). Prevents client-side cookie tampering and account impersonation via browser DevTools.
- **Authenticated Booking Gatekeeper (`POST /book`):** Blocks unauthenticated guests from submitting orders (redirecting them to `/signup`) and renders a clean login/signup gatekeeper prompt on the dashboard UI.
- **Walk-In Delivery & Contact Logistics:** Stores `customer_phone` and `delivery_address` directly on orders—automatically populated from the customer's profile for online bookings, or entered manually by Staff for counter walk-ins requiring WhatsApp notifications and home delivery.
- **Payment Status Tracking (`POST /toggle-payment`):** Tracks whether each order is **`Unpaid`** or **`Paid`**, displaying live status badges on the dashboard and printable receipts alongside a 1-click Staff toggle button.
- **Protected JSON REST API (`GET /api/orders`):** Enforces session authentication (`401 Unauthorized` for guests), returning all shop orders for Staff and strictly isolated personal orders for logged-in Customers.

### 🔐 v3.0 Multi-Tenant Auth, Privacy & Operations
- **Unified Authentication Portal (`/login-customer` & `/signup`):** Dedicated customer registration with `bcrypt` password hashing (`golang.org/x/crypto/bcrypt`), paired with a smart login gateway that authenticates both Customers (via email or phone) and Admin Staff (`admin@apexwash.com` / `STAFF_PASSWORD`) from a single form.
- **Nigerian Phone Number Normalization:** Built-in validator (`normalizeAndValidatePhone`) that accepts local (`0803...`) and international (`+234...`, `234...`) formats, strips formatting characters, verifies network prefixes (`070`, `071`, `080`, `081`, `090`, `091`), and standardizes numbers to 11 digits before database storage.
- **Relational Order Isolation (`user_id` + `LEFT JOIN`):** Links orders directly to `users.id` (`user_id`) so customers only see their own orders—even if two customers share the exact same full name.
- **IDOR-Protected Digital Receipts (`GET /receipt?id=X`):** Strict ownership verification (`403 Forbidden`) ensuring only Staff or the exact customer who owns the order can view its `#APX-ID` printable invoice.
- **One-Click Staff WhatsApp Alerts (`wa.me` Integration):** Staff dashboard dynamically converts local Nigerian numbers (`0803...` → `234803...`) and generates pre-filled WhatsApp notification links (**💬 Notify**) tailored to the order's current workflow stage and payment status.

### 📊 Core v1.0 & v2.0 Executive Dashboard Features
- **Live Executive Metrics:** Real-time calculation of active shop orders and total pipeline value (`₦`).
- **Dynamic Service Pricing:** Automatic total cost calculation based on garment category and quantity.
- **Full CRUD Order Management:**
  - **Create:** Book customer orders with instant SQLite persistence (`INSERT`).
  - **Read:** View active orders and live status stages (`SELECT` with `LEFT JOIN`).
  - **Update:** Advance laundry orders step-by-step from *Pending* → *Washing & Ironing* → *Ready for Pickup* (`UPDATE`).
  - **Delete:** Clear completed orders from the active board upon customer pickup (`DELETE`).
- **Dynamic Search & Status Filtering:** Server-side filtering by customer name (`LIKE` queries) and workflow stage via URL query parameters (`?search=` and `?status=`).
- **Printable Digital Receipts:** Official `#APX-ID` customer invoices featuring `@media print` PDF styling, delivery address details, payment status badges, and 1-click WhatsApp sharing.
- **Staff CSV Accounting Export (`GET /export-csv`):** Streams live SQLite order data (including payment statuses) directly into a downloadable `.csv` spreadsheet for Excel or Google Sheets reconciliation.
- **SQL Injection Protection:** Uses parameterized SQL queries (`?` placeholders) across all database operations.

---

## 🛠️ Tech Stack

- **Backend:** Go Standard Library (`net/http`, `html/template`, `database/sql`, `crypto/hmac`, `crypto/sha256`, `encoding/json`, `encoding/csv`, `net/url`, `strconv`)
- **Security:** `golang.org/x/crypto/bcrypt` for password hashing + `HMAC-SHA256` signed `HttpOnly` session cookies
- **Database:** SQLite (`modernc.org/sqlite` — pure Go, CGo-free driver) with automatic schema migration
- **Frontend:** Semantic HTML5 & Custom Flexbox CSS3
- **Testing:** 14 automated unit & HTTP integration tests using `testing` and `net/http/httptest` against an isolated in-memory SQLite database (`:memory:`)
- **Deployment:** Render (12-Factor App configuration via `PORT`, `ADMIN_EMAIL`, `STAFF_PASSWORD`, and `SESSION_SECRET` environment variables)

---

## 📡 Routes & API Reference

| Method | Endpoint | Access | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/` | Public / Role-Aware | Renders dashboard (Staff sees all orders + metrics; Customers see only their own orders) |
| `GET/POST` | `/signup` | Public | Dedicated customer registration with Nigerian phone validation & `bcrypt` hashing |
| `POST` | `/login-customer` | Public | Unified login portal for both Customers (email/phone) and Admin Staff (`admin@apexwash.com`) |
| `POST` | `/logout-customer` | Customer | Clears the active `customer_session` cookie |
| `POST` | `/login` | Public | Direct staff password login endpoint (`HttpOnly` HMAC-signed cookie) |
| `POST` | `/logout` | Staff | Clears the `staff_session` cookie (`MaxAge: -1`) |
| `POST` | `/book` | Customer / Staff Only | Creates a new laundry order (redirects unauthenticated guests to `/signup`) |
| `GET` | `/receipt?id={id}` | Owner / Staff Only | IDOR-protected printable digital receipt (`403 Forbidden` for unauthorized users) |
| `GET` | `/api/orders` | Customer / Staff Only | Returns authorized orders as a JSON array (`401 Unauthorized` for guests) |
| `POST` | `/update-status` | Staff Only | Advances an order to the next workflow stage |
| `POST` | `/toggle-payment` | Staff Only | Toggles an order's payment status between `Unpaid` and `Paid` |
| `POST` | `/delete-order` | Staff Only | Marks an order as picked up and removes it from the active board |
| `GET` | `/export-csv` | Staff Only | Downloads full order accounting report as `apexwash_orders.csv` |

### Sample JSON Response (`GET /api/orders`)
```json
[
  {
    "id": 8,
    "user_id": 1,
    "customer_name": "Wealth Ibe",
    "customer_phone": "08031234567",
    "delivery_address": "14 GRA Road, Otukpo",
    "service_name": "Shirt(Wash and Iron)",
    "quantity": 1,
    "total_cost": 500,
    "status": "Washing & Ironing",
    "payment_status": "Paid"
  }
]
```

---

## 📂 Project Structure

```text
apexwash/
├── main.go              # Entry point, static file server, and route registration
├── models.go            # Data structures (LaundryItem, Order, User, PageData) & WhatsAppURL helper
├── handlers.go          # HTTP handlers for CRUD, HMAC Auth, Phone Validation, Payments, CSV, and Receipts
├── handlers_test.go     # 14 automated unit and HTTP integration tests
├── database.go          # SQLite connection and automatic schema initialization/migration
├── static/
│   └── style.css        # Executive dashboard styling
├── templates/
│   ├── index.html       # Dynamic Go HTML template with role-based Staff & Customer views
│   ├── signup.html      # Dedicated customer registration page
│   └── receipt.html     # Printable customer invoice template with payment & delivery details
├── .gitignore           # Excludes local SQLite database files (*.db)
└── go.mod               # Go module definition and dependencies
```

---

## 🚀 Getting Started Locally

### Prerequisites
- [Go](https://go.dev/dl/) installed on your machine (v1.20+ recommended).

### Installation & Run

1. **Clone the repository:**
   ```bash
   git clone https://github.com/wib3/apexwash.git
   cd apexwash
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Start the server:**
   ```bash
   go run .
   ```

4. **Open in your browser:**
   - **Web Dashboard:** `http://localhost:8081` *(Default Unified Admin Login: `admin@apexwash.com` / `admin123`)*
   - **Customer Signup:** `http://localhost:8081/signup`
   - **JSON API:** `http://localhost:8081/api/orders`

---

## 🧪 Running Automated Tests

ApexWash includes **14 unit and HTTP integration tests** using Go's standard `testing` and `net/http/httptest` packages, backed by an isolated in-memory SQLite database (`:memory:`):

1. `TestUnauthorizedUpdateStatus` — Guards staff mutation endpoints (`401 Unauthorized`)
2. `TestLoginHandler` — Verifies staff login and HMAC-signed cookie creation
3. `TestBookAndAPIOrdersHandler` — Tests authenticated order creation and JSON API serialization
4. `TestExportCSVHandler` — Verifies staff-only CSV accounting export
5. `TestReceiptHandler` — Tests printable receipt rendering and IDOR security checks
6. `TestCustomerSignupAndLogin` — Verifies `bcrypt` signup and login via email or phone
7. `TestCustomerOrderPrivacy` — Ensures customers only see their own orders
8. `TestUnifiedAdminLogin` — Verifies Admin login via the unified `/login-customer` portal
9. `TestPhoneValidation` — Tests Nigerian mobile number normalization (`+234...` → `0803...`) and prefix validation
10. `TestSameNameOrderIsolation` — Verifies relational `user_id` isolation when two users share the exact same name
11. `TestWhatsAppURLGeneration` — Verifies international `wa.me/234...` formatting, status, and payment message generation
12. `TestBookingGatekeeperAndWalkIn` — Verifies unauthenticated `/book` redirects to `/signup` and walk-in phone/address persistence
13. `TestSessionTamperingAndAPISecurity` — Verifies HMAC signature rejection on tampered cookies and `401 Unauthorized` on unauthenticated `/api/orders` requests
14. `TestTogglePaymentHandler` — Verifies staff-only payment status toggling (`Unpaid` ↔ `Paid`)

```bash
go test -v
```