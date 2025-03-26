package dto

import "new/types"

// InvoiceResponse là DTO cho response của invoice
type InvoiceResponse struct {
	ID              uint                `json:"id"`
	InvoiceCode     string              `json:"invoiceCode"`
	OrderID         uint                `json:"orderId"`
	TotalAmount     float64             `json:"totalAmount"`
	PaidAmount      float64             `json:"paidAmount"`
	RemainingAmount float64             `json:"remainingAmount"`
	Status          int                 `json:"status"`
	PaymentDate     *string             `json:"paymentDate,omitempty"`
	CreatedAt       string              `json:"createdAt"`
	UpdatedAt       string              `json:"updatedAt"`
	User            InvoiceUserResponse `json:"user"`
	AdminID         uint                `json:"adminId"`
}

// InvoiceUserResponse là DTO cho thông tin user trong invoice
type InvoiceUserResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
}

// TotalResponse là DTO cho response tổng doanh thu
type TotalResponse struct {
	User                 InvoiceUserResponse `json:"user"`
	TotalAmount          float64             `json:"totalAmount"`
	CurrentMonthRevenue  float64             `json:"currentMonthRevenue"`
	LastMonthRevenue     float64             `json:"lastMonthRevenue"`
	CurrentWeekRevenue   float64             `json:"currentWeekRevenue"`
	VAT                  float64             `json:"vat"`
	ActualMonthlyRevenue float64             `json:"actualMonthlyRevenue"`
	VatLastMonth         float64             `json:"vatLastMonth"`
}

// MonthRevenue là DTO cho doanh thu theo tháng
type MonthRevenue struct {
	Month      string  `json:"month"`
	Revenue    float64 `json:"revenue"`
	OrderCount int     `json:"orderCount"`
}

// UpdatePaymentRequest là DTO cho request cập nhật thanh toán
type UpdatePaymentRequest struct {
	ID          uint `json:"id"`
	PaymentType int  `json:"paymentType"`
}

// SendPayRequest là DTO cho request gửi thanh toán
type SendPayRequest struct {
	Email        string `json:"email" binding:"required"`
	Vat          int    `json:"vat" binding:"required"`
	VatLastMonth int    `json:"vatLastMonth"`
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

// UserRevenueResponse là DTO cho response doanh thu theo user
type UserRevenueResponse struct {
	ID         uint                      `json:"id"`
	Date       string                    `json:"date"`
	OrderCount int                       `json:"orderCount"`
	Revenue    float64                   `json:"revenue"`
	User       types.InvoiceUserResponse `json:"user"`
}

// RevenueResponse là DTO cho response doanh thu
type RevenueResponse struct {
	TotalRevenue         float64        `json:"totalRevenue"`
	CurrentMonthRevenue  float64        `json:"currentMonthRevenue"`
	LastMonthRevenue     float64        `json:"lastMonthRevenue"`
	CurrentWeekRevenue   float64        `json:"currentWeekRevenue"`
	MonthlyRevenue       []MonthRevenue `json:"monthlyRevenue"`
	VAT                  float64        `json:"vat"`
	ActualMonthlyRevenue float64        `json:"actualMonthlyRevenue"`
}
