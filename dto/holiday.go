package dto

import (
	"new/response"
	"time"
)

// HolidayResponse là DTO cho response của holiday
type HolidayResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	FromDate  string    `json:"fromDate"`
	ToDate    string    `json:"toDate"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreateHolidayRequest là DTO cho yêu cầu tạo mới holiday
type CreateHolidayRequest struct {
	Name     string  `json:"name" binding:"required"`
	FromDate string  `json:"fromDate" binding:"required"`
	ToDate   string  `json:"toDate" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
}

type UpdateHolidayRequest struct {
	Name     string  `json:"name" binding:"required"`
	FromDate string  `json:"fromDate" binding:"required"`
	ToDate   string  `json:"toDate" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
}

type HolidayListResponse struct {
	Holidays   []HolidayResponse   `json:"holidays"`
	Pagination response.Pagination `json:"pagination"`
}
