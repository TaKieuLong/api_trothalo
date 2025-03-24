package dto

import (
	"new/types"
)

// InvoiceResponse là DTO cho response của invoice
type InvoiceResponse struct {
	ID              uint                      `json:"id"`
	InvoiceCode     string                    `json:"invoiceCode"`
	OrderID         uint                      `json:"orderId"`
	TotalAmount     float64                   `json:"totalAmount"`
	PaidAmount      float64                   `json:"paidAmount"`
	RemainingAmount float64                   `json:"remainingAmount"`
	Status          int                       `json:"status"`
	PaymentDate     *string                   `json:"paymentDate,omitempty"`
	CreatedAt       string                    `json:"createdAt"`
	UpdatedAt       string                    `json:"updatedAt"`
	User            types.InvoiceUserResponse `json:"user"`
	AdminID         uint                      `json:"adminId"`
}

// CreateInvoiceRequest là DTO cho request tạo invoice
type CreateInvoiceRequest struct {
	OrderID     uint    `json:"orderId" binding:"required"`
	TotalAmount float64 `json:"totalAmount" binding:"required"`
	PaidAmount  float64 `json:"paidAmount" binding:"required"`
}

// UpdateInvoiceRequest là DTO cho request cập nhật invoice
type UpdateInvoiceRequest struct {
	PaidAmount float64 `json:"paidAmount" binding:"required"`
}

// ToTalResponse là DTO cho response tổng doanh thu
type ToTalResponse struct {
	TotalRevenue       float64        `json:"totalRevenue"`
	LastMonthRevenue   float64        `json:"lastMonthRevenue"`
	CurrentWeekRevenue float64        `json:"currentWeekRevenue"`
	MonthlyRevenue     []MonthRevenue `json:"monthlyRevenue"`
}

// MonthRevenue là DTO cho doanh thu theo tháng
type MonthRevenue struct {
	Month   string  `json:"month"`
	Revenue float64 `json:"revenue"`
}

// UserRevenueResponse là DTO cho response doanh thu theo user
type UserRevenueResponse struct {
	ID         uint                      `json:"id"`
	Date       string                    `json:"date"`
	OrderCount int                       `json:"orderCount"`
	Revenue    float64                   `json:"revenue"`
	User       types.InvoiceUserResponse `json:"user"`
}
