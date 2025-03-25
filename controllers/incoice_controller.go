package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"new/config"
	"new/models"
	"new/services"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
)

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

type InvoiceUserResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
}

type ToTalResponse struct {
	User                 InvoiceUserResponse `json:"user"`
	TotalAmount          float64             `json:"totalAmount"`
	CurrentMonthRevenue  float64             `json:"currentMonthRevenue"`
	LastMonthRevenue     float64             `json:"lastMonthRevenue"`
	CurrentWeekRevenue   float64             `json:"currentWeekRevenue"`
	VAT                  float64             `json:"vat"`
	ActualMonthlyRevenue float64             `json:"actualMonthlyRevenue"`
	VatLastMonth         float64             `json:"vatLastMonth"`
}

func GetInvoices(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Authorization header is missing"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := GetUserIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Invalid token"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Unable to connect to Redis"})
		return
	}

	var allInvoices []InvoiceResponse

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
				c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Không có quyền truy cập"})
				return
			}
			tx = tx.Where("order_id IN (?)", config.DB.Table("orders").
				Select("orders.id").
				Joins("JOIN accommodations ON accommodations.id = orders.accommodation_id").
				Where("accommodations.user_id = ?", adminID))
		}

		var invoices []models.Invoice
		if err := tx.Find(&invoices).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Unable to fetch invoices"})
			return
		}

		for _, invoice := range invoices {
			allInvoices = append(allInvoices, InvoiceResponse{
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
			})
		}

		// Lưu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, allInvoices, 60*time.Minute); err != nil {
			log.Printf("Error caching invoices: %v", err)
		}
	}

	filteredInvoices := make([]InvoiceResponse, 0)
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
		filteredInvoices = []InvoiceResponse{}
	} else if end > total {
		filteredInvoices = filteredInvoices[start:]
	} else {
		filteredInvoices = filteredInvoices[start:end]
	}

	// Trả kết quả
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy data hóa đơn thành công",
		"data": filteredInvoices,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func GetDetailInvoice(c *gin.Context) {
	var invoice models.Invoice
	if err := config.DB.Where("id = ?", c.Param("id")).First(&invoice).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "message": "Không tìm thấy hóa đơn!"})
		return
	}
	var order models.Order
	if err := config.DB.Where("id = ?", invoice.OrderID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "message": "Không tìm thấy đơn hàng liên quan!"})
		return
	}
	var user models.User
	if err := config.DB.Where("id = ?", order.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "message": "Không tìm thấy người dùng liên quan!"})
		return
	}
	invoiceResponse := InvoiceResponse{
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
		User: InvoiceUserResponse{
			ID:          user.ID,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
		},
	}

	c.JSON(http.StatusOK, gin.H{"code": 1, "message": "Lấy chi tiết hóa đơn thành công", "data": invoiceResponse})
}

type MonthRevenue struct {
	Month      string  `json:"month"`
	Revenue    float64 `json:"revenue"`
	OrderCount int     `json:"orderCount"`
}

func UpdatePaymentStatus(c *gin.Context) {
	var request struct {
		ID          uint `json:"id"`
		PaymentType int  `json:"paymentType"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu yêu cầu không hợp lệ"})
		return
	}

	var invoice models.Invoice
	if err := config.DB.Where("id = ?", request.ID).First(&invoice).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Hóa đơn không tìm thấy"})
		return
	}

	invoice.PaymentType = &request.PaymentType
	currentTime := time.Now()
	invoice.PaymentDate = &currentTime
	invoice.Status = 1
	invoice.RemainingAmount = 0
	invoice.PaidAmount = invoice.TotalAmount

	if err := config.DB.Save(&invoice).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể cập nhật trạng thái thanh toán"})
		return
	}

	redisClient, err := config.ConnectRedis()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể kết nối Redis"})
		return
	}

	cacheKeyPattern := "invoices:*"
	keys, err := redisClient.Keys(config.Ctx, cacheKeyPattern).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể tìm kiếm cache Redis"})
		return
	}

	if len(keys) > 0 {
		err := redisClient.Del(config.Ctx, keys...).Err()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể xóa cache Redis"})
			return
		}
	}

	page, limit := 0, 10
	cacheKey := fmt.Sprintf("invoices:all:page=%d:limit=%d", page, limit)

	var invoices []models.Invoice
	var invoiceResponses []InvoiceResponse
	var totalInvoices int64

	tx := config.DB.Model(&models.Invoice{})
	if err := tx.Count(&totalInvoices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể đếm số lượng hóa đơn"})
		return
	}

	if err := tx.Offset(page * limit).Limit(limit).Find(&invoices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lấy danh sách hóa đơn"})
		return
	}

	for _, invoice := range invoices {
		invoiceResponses = append(invoiceResponses, InvoiceResponse{
			ID:              invoice.ID,
			InvoiceCode:     invoice.InvoiceCode,
			OrderID:         invoice.OrderID,
			TotalAmount:     invoice.TotalAmount,
			PaidAmount:      invoice.PaidAmount,
			RemainingAmount: invoice.RemainingAmount,
			Status:          invoice.Status,
			PaymentDate:     nil,
			AdminID:         invoice.AdminID,
			CreatedAt:       invoice.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:       invoice.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	responseData := gin.H{
		"code": 1,
		"mess": "Cập nhật trạng thái thanh toán thành công",
		"data": invoiceResponses,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": totalInvoices,
		},
	}

	jsonData, err := json.Marshal(responseData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi xử lý dữ liệu"})
		return
	}

	ttl := 10 * time.Minute
	err = redisClient.Set(config.Ctx, cacheKey, jsonData, ttl).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lưu cache Redis"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Cập nhật trạng thái thanh toán thành công",
	})
}

func SendPay(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Thiếu header Authorization"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := GetUserIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Token không hợp lệ"})
		return
	}

	if currentUserRole != 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Bạn không có quyền truy cập", "id": currentUserID})
		return
	}

	var request struct {
		Email        string `json:"email" binding:"required"`
		Vat          int    `json:"vat" binding:"required"`
		VatLastMonth int    `json:"vatLastMonth"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ", "error": err.Error()})
		return
	}

	email := request.Email
	vat := request.Vat
	vatLastMonth := request.VatLastMonth
	totalVat := vat + vatLastMonth

	qrCodeURL := fmt.Sprintf(
		"https://img.vietqr.io/image/SACOMBANK-060915374450-compact.jpg?amount=%d&addInfo=Chuyen%%20khoan%%20phi%%20",
		totalVat,
	)

	if err := services.SendPayEmail(email, vat, vatLastMonth, totalVat, qrCodeURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể gửi email", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Email đã được gửi thành công",
	})
}

func DeleteKeysByPattern(ctx context.Context, rdb *redis.Client, pattern string) error {
	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := rdb.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("lỗi khi xóa key %s: %v", iter.Val(), err)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("lỗi khi duyệt các key với pattern %s: %v", pattern, err)
	}
	return nil
}
