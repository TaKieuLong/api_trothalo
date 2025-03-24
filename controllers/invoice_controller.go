package controllers

import (
	"fmt"
	"log"
	"net/url"
	"new/config"
	"new/dto"
	"new/models"
	"new/response"
	"new/services"
	"new/types"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetInvoices lấy tất cả hóa đơn
func GetInvoices(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	invoiceCodeFilter := c.Query("invoiceCode")
	statusFilter := c.Query("status")

	pageStr := c.Query("page")
	limitStr := c.Query("limit")

	page := 0
	limit := 10

	if pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage >= 0 {
			page = parsedPage
		}
	}

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Tạo cache key dựa trên role và userID
	var cacheKey string
	if currentUserRole == 2 {
		cacheKey = fmt.Sprintf("invoices:admin:%d", currentUserID)
	} else if currentUserRole == 3 {
		cacheKey = fmt.Sprintf("invoices:receptionist:%d", currentUserID)
	} else {
		cacheKey = "invoices:all"
	}

	// Kết nối Redis
	rdb, err := config.ConnectRedis()
	if err != nil {
		response.ServerError(c)
		return
	}

	var allInvoices []dto.InvoiceResponse

	// Lấy dữ liệu từ Redis
	if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &allInvoices); err != nil || len(allInvoices) == 0 {
		tx := config.DB.Model(&models.Invoice{})

		if currentUserRole == 2 {
			tx = tx.Where("order_id IN (?)", config.DB.Table("orders").
				Select("orders.id").
				Joins("JOIN accommodations ON accommodations.id = orders.accommodation_id").
				Where("accommodations.user_id = ?", currentUserID))
		} else if currentUserRole == 3 {
			var adminID int
			if err := config.DB.Model(&models.User{}).Select("admin_id").Where("id = ?", currentUserID).Scan(&adminID).Error; err != nil {
				response.Forbidden(c)
				return
			}
			tx = tx.Where("order_id IN (?)", config.DB.Table("orders").
				Select("orders.id").
				Joins("JOIN accommodations ON accommodations.id = orders.accommodation_id").
				Where("accommodations.user_id = ?", adminID))
		}

		var invoices []models.Invoice
		if err := tx.Find(&invoices).Error; err != nil {
			response.ServerError(c)
			return
		}

		for _, invoice := range invoices {
			var order models.Order
			if err := config.DB.First(&order, invoice.OrderID).Error; err != nil {
				continue
			}
			var user models.User
			if err := config.DB.First(&user, order.UserID).Error; err != nil {
				continue
			}

			var paymentDate *string
			if invoice.PaymentDate != nil {
				dateStr := invoice.PaymentDate.Format("2006-01-02 15:04:05")
				paymentDate = &dateStr
			}

			allInvoices = append(allInvoices, dto.InvoiceResponse{
				ID:              invoice.ID,
				InvoiceCode:     invoice.InvoiceCode,
				OrderID:         invoice.OrderID,
				TotalAmount:     invoice.TotalAmount,
				PaidAmount:      invoice.PaidAmount,
				RemainingAmount: invoice.RemainingAmount,
				Status:          invoice.Status,
				PaymentDate:     paymentDate,
				CreatedAt:       invoice.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:       invoice.UpdatedAt.Format("2006-01-02 15:04:05"),
				AdminID:         invoice.AdminID,
				User: types.InvoiceUserResponse{
					ID:          user.ID,
					Name:        user.Name,
					Email:       user.Email,
					PhoneNumber: user.PhoneNumber,
				},
			})
		}

		// Lưu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, allInvoices, 60*time.Minute); err != nil {
			log.Printf("Error caching invoices: %v", err)
		}
	}

	filteredInvoices := make([]dto.InvoiceResponse, 0)
	for _, invoice := range allInvoices {
		if invoiceCodeFilter != "" {
			decodedNameFilter, _ := url.QueryUnescape(invoiceCodeFilter)
			if !strings.Contains(strings.ToLower(invoice.InvoiceCode), strings.ToLower(decodedNameFilter)) {
				continue
			}
		}
		if statusFilter != "" {
			status, _ := strconv.Atoi(statusFilter)
			if invoice.Status != status {
				continue
			}
		}
		filteredInvoices = append(filteredInvoices, invoice)
	}

	sort.Slice(filteredInvoices, func(i, j int) bool {
		return filteredInvoices[i].CreatedAt > filteredInvoices[j].CreatedAt
	})
	total := len(filteredInvoices)

	start := page * limit
	end := start + limit
	if start >= total {
		filteredInvoices = []dto.InvoiceResponse{}
	} else if end > total {
		filteredInvoices = filteredInvoices[start:]
	} else {
		filteredInvoices = filteredInvoices[start:end]
	}

	response.Success(c, gin.H{
		"data": filteredInvoices,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// GetDetailInvoice lấy chi tiết hóa đơn
func GetDetailInvoice(c *gin.Context) {
	var invoice models.Invoice
	if err := config.DB.Where("id = ?", c.Param("id")).First(&invoice).Error; err != nil {
		response.NotFound(c)
		return
	}
	var order models.Order
	if err := config.DB.Where("id = ?", invoice.OrderID).First(&order).Error; err != nil {
		response.NotFound(c)
		return
	}
	var user models.User
	if err := config.DB.Where("id = ?", order.UserID).First(&user).Error; err != nil {
		response.NotFound(c)
		return
	}
	invoiceResponse := dto.InvoiceResponse{
		ID:              invoice.ID,
		InvoiceCode:     invoice.InvoiceCode,
		OrderID:         invoice.OrderID,
		TotalAmount:     invoice.TotalAmount,
		PaidAmount:      invoice.PaidAmount,
		RemainingAmount: invoice.RemainingAmount,
		Status:          invoice.Status,
		PaymentDate:     nil,
		CreatedAt:       invoice.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:       invoice.UpdatedAt.Format("2006-01-02 15:04:05"),
		AdminID:         invoice.AdminID,
		User: types.InvoiceUserResponse{
			ID:          user.ID,
			Name:        user.Name,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
		},
	}

	if invoice.PaymentDate != nil {
		dateStr := invoice.PaymentDate.Format("2006-01-02 15:04:05")
		invoiceResponse.PaymentDate = &dateStr
	}

	response.Success(c, invoiceResponse)
}

// UpdatePaymentStatus cập nhật trạng thái thanh toán
func UpdatePaymentStatus(c *gin.Context) {
	var request struct {
		ID          uint   `json:"id"`
		Status      int    `json:"status"`
		PaymentDate string `json:"paymentDate"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

	var invoice models.Invoice
	if err := config.DB.First(&invoice, request.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	invoice.Status = request.Status
	if request.PaymentDate != "" {
		paymentDate, err := time.Parse("2006-01-02 15:04:05", request.PaymentDate)
		if err != nil {
			response.ValidationError(c, "Định dạng ngày thanh toán không hợp lệ")
			return
		}
		invoice.PaymentDate = &paymentDate
	}
	invoice.UpdatedAt = time.Now()

	if err := config.DB.Save(&invoice).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Xóa cache
	rdb, err := config.ConnectRedis()
	if err == nil {
		iter := rdb.Scan(config.Ctx, 0, "invoices:*", 0).Iterator()
		for iter.Next(config.Ctx) {
			rdb.Del(config.Ctx, iter.Val())
		}
	}

	var responseData dto.InvoiceResponse
	var order models.Order
	if err := config.DB.First(&order, invoice.OrderID).Error; err == nil {
		var user models.User
		if err := config.DB.First(&user, order.UserID).Error; err == nil {
			responseData = dto.InvoiceResponse{
				ID:              invoice.ID,
				InvoiceCode:     invoice.InvoiceCode,
				OrderID:         invoice.OrderID,
				TotalAmount:     invoice.TotalAmount,
				PaidAmount:      invoice.PaidAmount,
				RemainingAmount: invoice.RemainingAmount,
				Status:          invoice.Status,
				CreatedAt:       invoice.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:       invoice.UpdatedAt.Format("2006-01-02 15:04:05"),
				AdminID:         invoice.AdminID,
				User: types.InvoiceUserResponse{
					ID:          user.ID,
					Name:        user.Name,
					Email:       user.Email,
					PhoneNumber: user.PhoneNumber,
				},
			}

			if invoice.PaymentDate != nil {
				dateStr := invoice.PaymentDate.Format("2006-01-02 15:04:05")
				responseData.PaymentDate = &dateStr
			}
		}
	}

	response.Success(c, responseData)
}

// SendPay gửi thanh toán
func SendPay(c *gin.Context) {
	var request struct {
		ID     uint    `json:"id"`
		Amount float64 `json:"amount"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

	var invoice models.Invoice
	if err := config.DB.First(&invoice, request.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	if request.Amount > invoice.RemainingAmount {
		response.Error(c, 0, "Số tiền thanh toán không được lớn hơn số tiền còn lại")
		return
	}

	invoice.PaidAmount += request.Amount
	invoice.RemainingAmount -= request.Amount
	invoice.UpdatedAt = time.Now()

	if invoice.RemainingAmount == 0 {
		invoice.Status = 1
		now := time.Now()
		invoice.PaymentDate = &now
	}

	if err := config.DB.Save(&invoice).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Xóa cache
	rdb, err := config.ConnectRedis()
	if err == nil {
		iter := rdb.Scan(config.Ctx, 0, "invoices:*", 0).Iterator()
		for iter.Next(config.Ctx) {
			rdb.Del(config.Ctx, iter.Val())
		}
	}

	var responseData dto.InvoiceResponse
	var order models.Order
	if err := config.DB.First(&order, invoice.OrderID).Error; err == nil {
		var user models.User
		if err := config.DB.First(&user, order.UserID).Error; err == nil {
			responseData = dto.InvoiceResponse{
				ID:              invoice.ID,
				InvoiceCode:     invoice.InvoiceCode,
				OrderID:         invoice.OrderID,
				TotalAmount:     invoice.TotalAmount,
				PaidAmount:      invoice.PaidAmount,
				RemainingAmount: invoice.RemainingAmount,
				Status:          invoice.Status,
				CreatedAt:       invoice.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:       invoice.UpdatedAt.Format("2006-01-02 15:04:05"),
				AdminID:         invoice.AdminID,
				User: types.InvoiceUserResponse{
					ID:          user.ID,
					Name:        user.Name,
					Email:       user.Email,
					PhoneNumber: user.PhoneNumber,
				},
			}

			if invoice.PaymentDate != nil {
				dateStr := invoice.PaymentDate.Format("2006-01-02 15:04:05")
				responseData.PaymentDate = &dateStr
			}
		}
	}

	response.Success(c, responseData)
}
