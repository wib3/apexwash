# 🧺 ApexWash — The Sntino Executive Care

**Engineered by IB**

**Engineered by IB**  
🌐 **Live Demo:** [https://apexwash.onrender.com](https://apexwash.onrender.com) | 📡 **Live JSON API:** [https://apexwash.onrender.com/api/orders](https://apexwash.onrender.com/api/orders)

ApexWash is a full-stack laundry order management and executive dashboard web application built in **Go (Golang)** using the standard library and backed by **SQLite**. It enables laundry operations to dynamically price services, book customer pickups, filter and search orders in real time, protect staff actions via HTTP cookie authentication, and expose order data through a JSON REST API.

---

## ✨ Key Features

- **Live Executive Metrics:** Real-time calculation of active shop orders and total pipeline value (`₦`).
- **Dynamic Service Pricing:** Automatic total cost calculation based on garment category and quantity.
- **Full CRUD Order Management:**
  - **Create:** Book customer orders with instant SQLite persistence (`INSERT`).
  - **Read:** View all active orders and live status stages (`SELECT`).
  - **Update:** Advance laundry orders step-by-step from *Pending* → *Washing & Ironing* → *Ready for Pickup* (`UPDATE`).
  - **Delete:** Clear completed orders from the active board upon customer pickup (`DELETE`).
- **Dynamic Search & Status Filtering:** Server-side filtering by customer name (`LIKE` queries) and workflow stage via URL query parameters (`?search=` and `?status=`).
- **Role-Based Staff Authentication:** Stateless session management using `http.Cookie` (`HttpOnly`) that hides admin controls in Customer View and guards mutation routes (`401 Unauthorized`).
- **JSON REST API:** Dedicated `GET /api/orders` endpoint powered by `encoding/json` for external or mobile integrations.
- **Security:** Uses parameterized SQL queries (`?` placeholders) to prevent SQL injection.
- **Clean Modular Architecture:** Separated concerns across routing, HTTP handlers, data models, and database initialization.

---

## 🛠️ Tech Stack

- **Backend:** Go Standard Library (`net/http`, `html/template`, `database/sql`, `encoding/json`, `strconv`)
- **Database:** SQLite (`modernc.org/sqlite` — pure Go, CGo-free driver)
- **Frontend:** Semantic HTML5 & Custom Flexbox CSS3

---

## 📡 Routes & API Reference

| Method | Endpoint | Access | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/` | Public | Renders dashboard (supports `?search=` and `?status=` filters) |
| `POST` | `/book` | Public | Creates a new laundry order in SQLite |
| `POST` | `/login` | Public | Validates staff password and sets `HttpOnly` session cookie |
| `POST` | `/logout` | Staff | Clears the staff session cookie (`MaxAge: -1`) |
| `POST` | `/update-status` | Staff Only | Advances an order to the next workflow stage |
| `POST` | `/delete-order` | Staff Only | Marks an order as picked up and removes it from active board |
| `GET` | `/api/orders` | Public | Returns all active orders as a JSON array (`application/json`) |

### Sample JSON Response (`GET /api/orders`)
```json
[
  {
    "id": 8,
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
├── models.go            # Data structures (LaundryItem, Order, PageData) & JSON tags
├── handlers.go          # HTTP handlers for CRUD, Auth, Filtering, and JSON API
├── database.go          # SQLite connection and automatic schema initialization
├── static/
│   └── style.css        # Executive dashboard styling
├── templates/
│   └── index.html       # Dynamic Go HTML template with role-based UI
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
   - **Web Dashboard:** `http://localhost:8081` *(Default Staff Password: `apexadmin`)*
   - **JSON API:** `http://localhost:8081/api/orders`