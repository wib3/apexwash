package main

type ServiceItem struct {
	Name  string
	Price int
}

type PageData struct {
	ShopName     string
	Tagline      string
	Services     []ServiceItem
	Orders       []Order
	TotalOrders  int
	TotalRevenue int
	SearchQuery  string
	StatusFilter string
	IsStaff      bool
	CurrentUser  *User
}

type Order struct {
	ID            int    `json:"id"`
	UserID        int    `json:"user_id"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone,omitempty"`
	ServiceName   string `json:"service_name"`
	Quantity      int    `json:"quantity"`
	TotalCost     int    `json:"total_cost"`
	Status        string `json:"status"`
}

type User struct {
	ID           int    `json:"id"`
	FullName     string `json:"full_name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Address      string `json:"address"`
	PasswordHash string `json:"-"`
}

var serviceList = []ServiceItem{
	{Name: "Shirt(Wash and Iron)", Price: 500},
	{Name: "Trousers", Price: 600},
	{Name: "Native Wear(2-piece)", Price: 1200},
	{Name: "Bed Sheet/Duvet", Price: 1500},
}
