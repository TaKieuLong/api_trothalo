package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"new/config"
	"new/dto"
	"new/models"
	"new/services"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type RevenueResponse struct {
	TotalRevenue         float64            `json:"totalRevenue"`
	CurrentMonthRevenue  float64            `json:"currentMonthRevenue"`
	LastMonthRevenue     float64            `json:"lastMonthRevenue"`
	CurrentWeekRevenue   float64            `json:"currentWeekRevenue"`
	MonthlyRevenue       []dto.MonthRevenue `json:"monthlyRevenue"`
	VAT                  float64            `json:"vat"`
	ActualMonthlyRevenue float64            `json:"actualMonthlyRevenue"`
}

type UserRevenueResponse struct {
	ID         uint    `json:"id"`
	Date       string  `json:"date"`
	OrderCount int     `json:"order_count"`
	Revenue    float64 `json:"revenue"`
	User       struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		PhoneNumber string `json:"phone_number"`
	} `json:"user"`
}

func GetTotalRevenue(c *gin.Context) {
	var totalRevenue, currentMonthRevenue, currentWeekRevenue float64
	var lastMonthRevenue sql.NullFloat64
	var monthlyRevenue []dto.MonthRevenue
	var vat, actualMonthlyRevenue float64 // Thêm VAT và doanh thu thực tháng này

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

	// Chỉ xử lý cho role 1 và role 2
	if currentUserRole != 1 && currentUserRole != 2 {
		c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Không có quyền truy cập"})
		return
	}

	tx := config.DB.Model(&models.Invoice{})
	if currentUserRole == 2 {
		tx = tx.Where("order_id IN (?)", config.DB.Table("orders").
			Select("orders.id").
			Joins("JOIN accommodations ON accommodations.id = orders.accommodation_id").
			Where("accommodations.user_id = ?", currentUserID))
	}

	var invoices []models.Invoice
	if err := tx.Find(&invoices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lấy danh sách hóa đơn"})
		return
	}

	for _, invoice := range invoices {
		totalRevenue += invoice.TotalAmount

		currentMonth := time.Now().Format("2006-01")
		if invoice.CreatedAt.Format("2006-01") == currentMonth {
			currentMonthRevenue += invoice.TotalAmount
		}

		lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
		if invoice.CreatedAt.Format("2006-01") == lastMonth {
			lastMonthRevenue.Float64 += invoice.TotalAmount
		}

		currentWeekStart := time.Now().AddDate(0, 0, -int(time.Now().Weekday()))
		currentWeekEnd := currentWeekStart.AddDate(0, 0, 6)
		if invoice.CreatedAt.After(currentWeekStart) && invoice.CreatedAt.Before(currentWeekEnd) {
			currentWeekRevenue += invoice.TotalAmount
		}
	}

	currentYear := time.Now().Year()
	for i := 1; i <= 12; i++ {
		month := fmt.Sprintf("%d-%02d", currentYear, i)
		var revenue, orderCount float64

		for _, invoice := range invoices {
			if invoice.CreatedAt.Format("2006-01") == month {
				revenue += invoice.TotalAmount
				orderCount++
			}
		}

		monthlyRevenue = append(monthlyRevenue, dto.MonthRevenue{
			Month:      fmt.Sprintf("Tháng %d", i),
			Revenue:    revenue,
			OrderCount: int(orderCount),
		})
	}

	if currentUserRole == 1 {
		// Tính doanh thu cho role 1
		totalRevenue *= 0.30
		currentMonthRevenue *= 0.30
		lastMonthRevenue.Float64 *= 0.30
		currentWeekRevenue *= 0.30
		for i := range monthlyRevenue {
			monthlyRevenue[i].Revenue *= 0.30
		}
	} else if currentUserRole == 2 {
		// Tính doanh thu cho role 2
		vat = currentMonthRevenue * 30 / 100
		actualMonthlyRevenue = currentMonthRevenue - vat
		totalRevenue -= (totalRevenue * 30 / 100)
	}

	response := RevenueResponse{
		TotalRevenue:         totalRevenue,
		CurrentMonthRevenue:  currentMonthRevenue,
		LastMonthRevenue:     lastMonthRevenue.Float64,
		CurrentWeekRevenue:   currentWeekRevenue,
		MonthlyRevenue:       monthlyRevenue,
		VAT:                  vat,
		ActualMonthlyRevenue: actualMonthlyRevenue,
	}

	c.JSON(http.StatusOK, response)
}

func GetTotal(c *gin.Context) {
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

	nameFilter := c.Query("name")

	var users []models.User

	query := config.DB.Where("role = ?", 2)

	if nameFilter != "" {
		query = query.Where("name ILIKE ? OR email ILIKE ? OR phone_number ILIKE ?", "%"+nameFilter+"%", "%"+nameFilter+"%", "%"+nameFilter+"%")
	}

	if err := query.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lấy danh sách người dùng"})
		return
	}

	calculateRevenue := func(userID uint) (float64, float64, float64, float64, float64, float64, float64, error) {
		var totalAmount, currentMonthRevenue, lastMonthRevenue, currentWeekRevenue float64

		// Tổng doanh thu
		if err := config.DB.Model(&models.Invoice{}).
			Where("admin_id = ?", userID).
			Select("COALESCE(SUM(total_amount), 0)").
			Scan(&totalAmount).Error; err != nil {
			return 0, 0, 0, 0, 0, 0, 0, nil
		}

		// Doanh thu tháng hiện tại
		if err := config.DB.Model(&models.Invoice{}).
			Where("admin_id = ? AND EXTRACT(MONTH FROM created_at) = EXTRACT(MONTH FROM CURRENT_DATE) AND EXTRACT(YEAR FROM created_at) = EXTRACT(YEAR FROM CURRENT_DATE)", userID).
			Select("COALESCE(SUM(total_amount), 0)").
			Scan(&currentMonthRevenue).Error; err != nil {
			return 0, 0, 0, 0, 0, 0, 0, nil
		}

		// Doanh thu tháng trước
		if err := config.DB.Model(&models.Invoice{}).
			Where("admin_id = ? AND EXTRACT(MONTH FROM created_at) = EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 MONTH') AND EXTRACT(YEAR FROM created_at) = EXTRACT(YEAR FROM CURRENT_DATE)", userID).
			Select("COALESCE(SUM(total_amount), 0)").
			Scan(&lastMonthRevenue).Error; err != nil {
			return 0, 0, 0, 0, 0, 0, 0, nil
		}

		// Doanh thu tuần hiện tại
		if err := config.DB.Model(&models.Invoice{}).
			Where("admin_id = ? AND EXTRACT(WEEK FROM created_at) = EXTRACT(WEEK FROM CURRENT_DATE) AND EXTRACT(YEAR FROM created_at) = EXTRACT(YEAR FROM CURRENT_DATE)", userID).
			Select("COALESCE(SUM(total_amount), 0)").
			Scan(&currentWeekRevenue).Error; err != nil {
			return 0, 0, 0, 0, 0, 0, 0, nil
		}

		// Tính VAT (30% của doanh thu tháng hiện tại và tháng trước)
		vat := currentMonthRevenue * 0.3
		vatLastMonth := lastMonthRevenue * 0.3

		// Tính doanh thu thực tế hàng tháng (doanh thu tháng hiện tại trừ VAT)
		actualMonthlyRevenue := currentMonthRevenue - vat

		return totalAmount, currentMonthRevenue, lastMonthRevenue, currentWeekRevenue, vat, vatLastMonth, actualMonthlyRevenue, nil
	}

	var totalResponses []dto.TotalResponse
	for _, user := range users {
		totalAmount, currentMonthRevenue, lastMonthRevenue, currentWeekRevenue, vat, vatLastMonth, actualMonthlyRevenue, err := calculateRevenue(user.ID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": fmt.Sprintf("Không thể tính doanh thu cho người dùng %d", user.ID)})
			return
		}

		totalResponses = append(totalResponses, dto.TotalResponse{
			User: dto.InvoiceUserResponse{
				ID:          user.ID,
				Email:       user.Email,
				PhoneNumber: user.PhoneNumber,
			},
			TotalAmount:          totalAmount,
			CurrentMonthRevenue:  currentMonthRevenue,
			LastMonthRevenue:     lastMonthRevenue,
			CurrentWeekRevenue:   currentWeekRevenue,
			VAT:                  vat,
			VatLastMonth:         vatLastMonth,
			ActualMonthlyRevenue: actualMonthlyRevenue,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy doanh thu của người dùng thành công",
		"data": totalResponses,
	})
}

func GetToday(c *gin.Context) {
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

	if currentUserRole != 2 {
		c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Bạn không có quyền truy cập", "id": currentUserID})
		return
	}

	now := time.Now()
	year, month, _ := now.Date()
	loc := now.Location()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc)

	var revenues []models.UserRevenue
	if err := config.DB.
		Where("user_id = ? ", currentUserID).
		Find(&revenues).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"mess": "Không thể lấy doanh thu của người dùng",
			"err":  err.Error(),
		})
		return
	}

	revenueMap := make(map[string]models.UserRevenue)
	for _, rev := range revenues {
		dateStr := rev.Date.Format("2006-01-02")
		revenueMap[dateStr] = rev
	}

	var result []gin.H
	for d := firstDay; !d.After(lastDay); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		if rev, ok := revenueMap[dateStr]; ok {
			result = append(result, gin.H{
				"date":        dateStr,
				"order_count": rev.OrderCount,
				"revenue":     rev.Revenue,
				"user_id":     rev.UserID,
			})
		} else {
			result = append(result, gin.H{
				"date":        dateStr,
				"order_count": 0,
				"revenue":     0,
				"user_id":     currentUserID,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy doanh thu của người dùng thành công",
		"data": result,
	})
}

func GetTodayUser(c *gin.Context) {
	revenues, err := services.GetTodayUserRevenue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"mess": "Lỗi khi lấy doanh thu của người dùng",
			"err":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy doanh thu của người dùng thành công",
		"data": revenues,
	})
}

func GetUserRevene(c *gin.Context) {

	fromDateStr := c.Query("fromDate")
	toDateStr := c.Query("toDate")
	nameFilter := c.Query("name")
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

	dbQuery := config.DB.Preload("User")

	if fromDateStr != "" {
		fromDate, err := time.Parse("02/01/2006", fromDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "fromDate không hợp lệ, định dạng: dd/mm/yyyy"})
			return
		}
		dbQuery = dbQuery.Where("date >= ?", fromDate)
	}

	if toDateStr != "" {
		toDate, err := time.Parse("02/01/2006", toDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "toDate không hợp lệ, định dạng: dd/mm/yyyy"})
			return
		}
		dbQuery = dbQuery.Where("date <= ?", toDate)
	}

	var revenues []models.UserRevenue
	if err := dbQuery.Find(&revenues).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"mess": "Không thể lấy doanh thu của người dùng",
			"err":  err.Error(),
		})
		return
	}

	var responses []UserRevenueResponse
	for _, rev := range revenues {
		var resp UserRevenueResponse
		resp.ID = rev.ID
		resp.Date = rev.Date.Format("2006-01-02")
		resp.OrderCount = rev.OrderCount
		resp.Revenue = rev.Revenue

		resp.User.ID = rev.User.ID
		resp.User.Name = rev.User.Name
		resp.User.Email = rev.User.Email
		resp.User.PhoneNumber = rev.User.PhoneNumber

		responses = append(responses, resp)
	}

	if nameFilter != "" {
		var filtered []UserRevenueResponse
		normalizedFilter := removeDiacritics(strings.ToLower(strings.ReplaceAll(nameFilter, " ", "")))
		for _, resp := range responses {
			normalizedName := removeDiacritics(strings.ToLower(strings.ReplaceAll(resp.User.Name, " ", "")))
			normalizedPhone := removeDiacritics(strings.ToLower(strings.ReplaceAll(resp.User.PhoneNumber, " ", "")))
			if strings.Contains(normalizedName, normalizedFilter) || strings.Contains(normalizedPhone, normalizedFilter) {
				filtered = append(filtered, resp)
			}
		}
		responses = filtered
	}

	sort.Slice(responses, func(i, j int) bool {
		t1, _ := time.Parse("2006-01-02", responses[i].Date)
		t2, _ := time.Parse("2006-01-02", responses[j].Date)
		return t1.After(t2)
	})

	total := len(responses)
	start := page * limit
	end := start + limit

	if start >= total {
		responses = []UserRevenueResponse{}
	} else if end > total {
		responses = responses[start:]
	} else {
		responses = responses[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy doanh thu của người dùng thành công",
		"data": responses,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}
