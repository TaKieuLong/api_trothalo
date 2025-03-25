package dto

// HolidayResponse là DTO cho response của holiday
type HolidayResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	FromDate  string `json:"fromDate"`
	ToDate    string `json:"toDate"`
	Price     int    `json:"price"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// CreateHolidayRequest là DTO cho yêu cầu tạo mới holiday
type CreateHolidayRequest struct {
	ID       uint   `json:"id"`
	Name     string `json:"name" binding:"required"`
	FromDate string `json:"fromDate" binding:"required"`
	ToDate   string `json:"toDate" binding:"required"`
	Price    int    `json:"price" binding:"required"`
}
