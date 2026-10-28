package main

import (
	"fmt"
	"net/url"
	"strings"
)

// PendingOrder holds a cart securely in a cookie during login/signup
type PendingOrder struct {
	ServiceName string `json:"service_name"`
	Quantity    int    `json:"quantity"`
}

type PageData struct {
	ShopName      string
	Tagline       string
	Services      []Service
	Orders        []Order
	TotalOrders   int
	TotalRevenue  int
	SearchQuery   string
	StatusFilter  string
	IsStaff       bool
	IsSuperAdmin  bool
	CurrentUser   *User
	CustomerError string
	LoginError    string
	Identifier    string
}

// AdminDashboardData holds all data for the /admin control center
type AdminDashboardData struct {
	ShopName string
	Services []Service
	Users    []User
	Message  string
}

// Service represents a dynamically priced laundry item
type Service struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type Order struct {
	ID              int    `json:"id"`
	UserID          int    `json:"user_id"`
	CustomerName    string `json:"customer_name"`
	CustomerPhone   string `json:"customer_phone,omitempty"`
	DeliveryAddress string `json:"delivery_address,omitempty"`
	ServiceName     string `json:"service_name"`
	Quantity        int    `json:"quantity"`
	TotalCost       int    `json:"total_cost"`
	Status          string `json:"status"`
	PaymentStatus   string `json:"payment_status"`
}

type User struct {
	ID           int    `json:"id"`
	FullName     string `json:"full_name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Address      string `json:"address"`
	PasswordHash string `json:"-"`
	// Enterprise roles and statuses
	Role          string `json:"role"`           // 'customer', 'staff', 'super_admin'
	AccountStatus string `json:"account_status"` // 'active', 'blacklisted'
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

	payInfo := o.PaymentStatus
	if payInfo == "" {
		payInfo = "Unpaid"
	}

	var msg string
	if strings.EqualFold(o.Status, "Ready") || strings.EqualFold(o.Status, "Ready for Pickup") {
		msg = fmt.Sprintf(
			"Hello %s! Your ApexWash order #%d (%dx %s) is READY for pickup/delivery! Total: ₦%d (%s). Thank you for choosing ApexWash!",
			o.CustomerName, o.ID, o.Quantity, o.ServiceName, o.TotalCost, payInfo,
		)
	} else {
		msg = fmt.Sprintf(
			"Hello %s! Quick update on your ApexWash order #%d (%dx %s) — current status: %s. Total: ₦%d (%s).",
			o.CustomerName, o.ID, o.Quantity, o.ServiceName, o.Status, o.TotalCost, payInfo,
		)
	}

	return "https://wa.me/" + intlPhone + "?text=" + url.QueryEscape(msg)
}
