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
}

type Order struct {
	ID           int
	CustomerName string
	ServiceName  string
	Quantity     int
	TotalCost    int
	Status       string
}

var serviceList = []ServiceItem{
	{Name: "Shirt(Wash and Iron)", Price: 500},
	{Name: "Trousers", Price: 600},
	{Name: "Native Wear(2-piece)", Price: 1200},
	{Name: "Bed Sheet/Duvet", Price: 1500},
}
