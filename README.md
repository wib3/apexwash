# 🧺 ApexWash — The Sntino Executive Care

**Engineered by IB**

ApexWash is a full-stack laundry order management and executive dashboard web application built in **Go (Golang)** and backed by **SQLite**. It enables laundry operations to dynamically price services, book customer pickups, track order lifecycles across stages, and monitor real-time active pipeline revenue.

---

## ✨ Key Features

- **Live Executive Metrics:** Real-time calculation of active shop orders and total pipeline value (`₦`).
- **Dynamic Service Pricing:** Automatic total cost calculation based on garment category and quantity.
- **Full CRUD Order Management:**
  - **Create:** Book customer orders with instant SQLite persistence (`INSERT`).
  - **Read:** View all active orders and live status stages (`SELECT`).
  - **Update:** Advance laundry orders step-by-step from *Pending* → *Washing & Ironing* → *Ready for Pickup* (`UPDATE`).
  - **Delete:** Clear completed orders from the active board upon customer pickup (`DELETE`).
- **Security:** Uses parameterized SQL queries (`?` placeholders) to prevent SQL injection.
- **Clean Modular Architecture:** Separated concerns across routing, HTTP handlers, data models, and database initialization.

---

## 🛠️ Tech Stack

- **Backend:** Go (`net/http`, `html/template`, `database/sql`, `strconv`)
- **Database:** SQLite (`modernc.org/sqlite` — pure Go, CGo-free driver)
- **Frontend:** Semantic HTML5 & Custom Flexbox CSS3

---

## 📂 Project Structure

```text
apexwash/
├── main.go              # Entry point, static file server, and route registration
├── models.go            # Data structures (LaundryItem, Order, PageData) & price catalog
├── handlers.go          # HTTP handlers for CRUD operations and business logic
├── database.go          # SQLite connection and automatic schema initialization
├── static/
│   └── style.css        # Executive dashboard styling
├── templates/
│   └── index.html       # Dynamic Go HTML template
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
   Navigate to `http://localhost:8081`