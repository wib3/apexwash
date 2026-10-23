# 🧺 ApexWash — The Sntino Executive Care (v3.0)

**Engineered by IB**  
🌐 **Live Demo:** [https://apexwash.onrender.com](https://apexwash.onrender.com) | 📡 **Live JSON API:** [https://apexwash.onrender.com/api/orders](https://apexwash.onrender.com/api/orders)

ApexWash is a full-stack, multi-tenant laundry order management and executive dashboard web application built in **Go (Golang)** using the standard library and backed by **SQLite**. It enables laundry operations to dynamically price services, authenticate customers and staff through a unified portal, book customer pickups with relational privacy isolation, filter and search orders in real time, generate printable digital receipts, send 1-click WhatsApp status alerts, export accounting data to CSV, and expose order data through a JSON REST API.

---

## ✨ Key Features

### 🔐 v3.0 Multi-Tenant Auth, Privacy & Operations
- **Unified Authentication Portal (`/customer-login` & `/signup`):** Dedicated customer registration with `bcrypt` password hashing (`golang.org/x/crypto/bcrypt`), paired with a smart login gateway that authenticates both Customers (via email or phone) and Admin Staff (`admin` / `STAFF_PASSWORD`) from a single form.
- **Nigerian Phone Number Normalization:** Built-in validator (`normalizeAndValidatePhone`) that accepts local (`0803...`) and international (`+234...`, `234...`) formats, strips formatting characters, verifies network prefixes (`070`, `080`, `081`, `090`, `091`), and standardizes numbers to 11 digits before database storage.
- **Relational Order Isolation (`user_id` + `LEFT JOIN`):** Links orders directly to `users.id` (`user_id`) so customers only see their own orders—even if two customers share the exact same full name—while joining live customer phone numbers for staff operations.
- **IDOR-Protected Digital Receipts (`GET /receipt?id=X`):** Strict ownership verification (`403 Forbidden`) ensuring only Staff or the exact customer who owns the order can view its `#APX-ID` printable invoice.
- **One-Click Staff WhatsApp Alerts (`wa.me` Integration):** Staff dashboard dynamically converts local Nigerian numbers (`0803...` → `234803...`) and generates pre-filled WhatsApp notification links (**💬 Notify**) tailored to the order's current workflow stage.

### 📊 Core v1.0 & v2.0 Executive Dashboard Features
- **Live Executive Metrics:** Real-time calculation of active shop orders and total pipeline value (`₦`).
- **Dynamic Service Pricing:** Automatic total cost calculation based on garment category and quantity.
- **Full CRUD Order Management:**
  - **Create:** Book customer orders with instant SQLite persistence (`INSERT`).
  - **Read:** View active orders and live status stages (`SELECT` with `LEFT JOIN`).
  - **Update:** Advance laundry orders step-by-step from *Pending* → *Washing & Ironing* → *Ready for Pickup* (`UPDATE`).
  - **Delete:** Clear completed orders from the active board upon customer pickup (`DELETE`).
- **Dynamic Search & Status Filtering:** Server-side filtering by customer name (`LIKE` queries) and workflow stage via URL query parameters (`?search=` and `?status=`).
- **Printable Digital Receipts:** Official `#APX-ID` customer invoices featuring `@media print` PDF styling and customer WhatsApp sharing.
- **Staff CSV Accounting Export (`GET /export-csv`):** Streams live SQLite order data directly into a downloadable `.csv` spreadsheet for Excel or Google Sheets reconciliation.
- **Role-Based Staff Security:** Stateless session management using `http.Cookie` (`HttpOnly`) that hides admin controls in Customer View and guards mutation routes (`401 Unauthorized`).
- **JSON REST API:** Dedicated `GET /api/orders` endpoint powered by `encoding/json` for external or mobile integrations.
- **SQL Injection Protection:** Uses parameterized SQL queries (`?` placeholders) across all database operations.

---

## 🛠️ Tech Stack

- **Backend:** Go Standard Library (`net/http`, `html/template`, `database/sql`, `encoding/json`, `encoding/csv`, `net/url`, `strconv`)
- **Security:** `golang.org/x/crypto/bcrypt` for password hashing & `HttpOnly` session cookies
- **Database:** SQLite (`modernc.org/sqlite` — pure Go, CGo-free driver) with automatic schema migration
- **Frontend:** Semantic HTML5 & Custom Flexbox CSS3
- **Testing:** 11 automated unit & HTTP integration tests using `testing` and `net/http/httptest` against an isolated in-memory SQLite database (`:memory:`)
- **Deployment:** Render (12-Factor App configuration via `PORT` and `STAFF_PASSWORD` environment variables)

---

## 📡 Routes & API Reference

| Method | Endpoint | Access | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/` | Public / Role-Aware | Renders dashboard (Staff sees all orders + metrics; Customers see only their own orders) |
| `GET/POST` | `/signup` | Public | Dedicated customer registration with Nigerian phone validation & `bcrypt` hashing |
| `POST` | `/customer-login` | Public | Unified login portal for both Customers (email/phone) and Admin Staff (`admin`) |
| `POST` | `/customer-logout` | Customer | Clears the active `customer_session` cookie |
| `POST` | `/login` | Public | Direct staff password login endpoint (`HttpOnly` cookie) |
| `POST` | `/logout` | Staff | Clears the `staff_session` cookie (`MaxAge: -1`) |
| `POST` | `/book` | Public / Customer | Creates a new laundry order in SQLite (automatically linked to `user_id` if logged in) |
| `GET` | `/receipt?id={id}` | Owner / Staff Only | IDOR-protected printable digital receipt (`403 Forbidden` for unauthorized users) |
| `GET` | `/api/orders` | Public | Returns active orders as a JSON array (`application/json`) |
| `POST` | `/update-status` | Staff Only | Advances an order to the next workflow stage |
| `POST` | `/delete-order` | Staff Only | Marks an order as picked up and removes it from the active board |
| `GET` | `/export-csv` | Staff Only | Downloads full order accounting report as `apexwash_orders.csv` |

### Sample JSON Response (`GET /api/orders`)
```json
[
  {
    "id": 8,
    "user_id": 1,
    "customer_name": "Wealth Ibe",
    "service_name": "Shirt(Wash and Iron)",
    "quantity": 1,
    "total_cost": 500,
    "status": "Washing & Ironing"
  }
]
```

---

## 📂 Project Structure

```text
apexwash/
├── main.go              # Entry point, static file server, and route registration
├── models.go            # Data structures (LaundryItem, Order, User, PageData) & WhatsAppURL helper
├── handlers.go          # HTTP handlers for CRUD, Unified Auth, Phone Validation, CSV, and Receipts
├── handlers_test.go     # 11 automated unit and HTTP integration tests
├── database.go          # SQLite connection and automatic schema initialization/migration
├── static/
│   └── style.css        # Executive dashboard styling
├── templates/
│   ├── index.html       # Dynamic Go HTML template with role-based Staff & Customer views
│   ├── signup.html      # Dedicated customer registration page
│   └── receipt.html     # Printable customer invoice template
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
   git clone [https://github.com/wib3/apexwash.git](https://github.com/wib3/apexwash.git)
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
   - **Web Dashboard:** `http://localhost:8081` *(Default Staff Login: Identifier `admin` / Password `apexadmin`)*
   - **Customer Signup:** `http://localhost:8081/signup`
   - **JSON API:** `http://localhost:8081/api/orders`

---

## 🧪 Running Automated Tests

ApexWash includes **11 unit and HTTP integration tests** using Go's standard `testing` and `net/http/httptest` packages, backed by an isolated in-memory SQLite database (`:memory:`):

- `TestUnauthorizedUpdateStatus` — Guards staff mutation endpoints (`401 Unauthorized`)
- `TestLoginHandler` — Verifies staff login cookie creation
- `TestBookAndAPIOrdersHandler` — Tests order creation and JSON API serialization
- `TestExportCSVHandler` — Verifies staff-only CSV accounting export
- `TestReceiptHandler` — Tests printable receipt rendering and IDOR security checks
- `TestCustomerSignupAndLogin` — Verifies `bcrypt` signup and login via email or phone
- `TestCustomerOrderPrivacy` — Ensures customers only see their own orders
- `TestUnifiedAdminLogin` — Verifies Admin login via the unified `/customer-login` portal
- `TestPhoneValidation` — Tests Nigerian mobile number normalization (`+234...` → `0803...`) and prefix validation
- `TestSameNameOrderIsolation` — Verifies relational `user_id` isolation when two users share the exact same name
- `TestWhatsAppURLGeneration` — Verifies international `wa.me/234...` formatting and status message generation

```bash
go test -v
```