package main

import (
	"fmt"
	"net/url"
	"strings"
)

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

// WhatsAppURL generates a pre-filled wa.me link if the order has a valid customer phone number.
func (o Order) WhatsAppURL() string {
	if o.CustomerPhone == "" {
		return ""
	}

	// Convert 08031234567 -> 2348031234567 for wa.me compatibility
	intlPhone := o.CustomerPhone
	if strings.HasPrefix(intlPhone, "0") && len(intlPhone) == 11 {
		intlPhone = "234" + intlPhone[1:]
	} else if strings.HasPrefix(intlPhone, "+") {
		intlPhone = strings.TrimPrefix(intlPhone, "+")
	}

	var msg string
	if strings.EqualFold(o.Status, "Ready") || strings.EqualFold(o.Status, "Ready for Pickup") {
		msg = fmt.Sprintf(
			"Hello %s! Your ApexWash order #%d (%dx %s) is READY for pickup/delivery! Total: ₦%d. Thank you for choosing ApexWash!",
			o.CustomerName, o.ID, o.Quantity, o.ServiceName, o.TotalCost,
		)
	} else {
		msg = fmt.Sprintf(
			"Hello %s! Quick update on your ApexWash order #%d (%dx %s) — current status: %s. Total: ₦%d.",
			o.CustomerName, o.ID, o.Quantity, o.ServiceName, o.Status, o.TotalCost,
		)
	}

	return "https://wa.me/" + intlPhone + "?text=" + url.QueryEscape(msg)
}
