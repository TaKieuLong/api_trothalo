package controllers

import (
	"new/config"
	"new/dto"
	"new/models"
	"new/response"
	"new/services"
	"new/types"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// getRevenueData lấy dữ liệu doanh thu
func getRevenueData() (float64, float64, float64, []dto.MonthRevenue, error) {
	var totalRevenue float64
	var lastMonthRevenue float64
	var currentWeekRevenue float64
	var monthlyRevenue []dto.MonthRevenue

	now := time.Now().UTC()
	lastMonth := now.AddDate(0, -1, 0)
	startOfWeek := now.AddDate(0, 0, -int(now.Weekday()))

	// Lấy tổng doanh thu
	if err := config.DB.Model(&models.Invoice{}).Where("status = ?", 1).Select("COALESCE(SUM(total_amount), 0)").Scan(&totalRevenue).Error; err != nil {
		return 0, 0, 0, nil, err
	}

	// Lấy doanh thu tháng trước
	if err := config.DB.Model(&models.Invoice{}).Where("status = ? AND created_at >= ? AND created_at < ?", 1, lastMonth, now).Select("COALESCE(SUM(total_amount), 0)").Scan(&lastMonthRevenue).Error; err != nil {
		return 0, 0, 0, nil, err
	}

	// Lấy doanh thu tuần hiện tại
	if err := config.DB.Model(&models.Invoice{}).Where("status = ? AND created_at >= ?", 1, startOfWeek).Select("COALESCE(SUM(total_amount), 0)").Scan(&currentWeekRevenue).Error; err != nil {
		return 0, 0, 0, nil, err
	}

	// Lấy doanh thu theo tháng
	var results []struct {
		Month   string
		Revenue float64
	}
	if err := config.DB.Model(&models.Invoice{}).Where("status = ?", 1).Select("TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM') as month, SUM(total_amount) as revenue").Group("month").Order("month DESC").Limit(12).Scan(&results).Error; err != nil {
		return 0, 0, 0, nil, err
	}

	for _, result := range results {
		monthlyRevenue = append(monthlyRevenue, dto.MonthRevenue{
			Month:   result.Month,
			Revenue: result.Revenue,
		})
	}

	return totalRevenue, lastMonthRevenue, currentWeekRevenue, monthlyRevenue, nil
}

// GetTotalRevenue lấy tổng doanh thu
func GetTotalRevenue(c *gin.Context) {
	totalRevenue, lastMonthRevenue, currentWeekRevenue, monthlyRevenue, err := getRevenueData()
	if err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, dto.ToTalResponse{
		TotalRevenue:       totalRevenue,
		LastMonthRevenue:   lastMonthRevenue,
		CurrentWeekRevenue: currentWeekRevenue,
		MonthlyRevenue:     monthlyRevenue,
	})
}

// GetTotal lấy tổng doanh thu theo user
func GetTotal(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	_, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 1 && currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	totalRevenue, lastMonthRevenue, currentWeekRevenue, monthlyRevenue, err := getRevenueData()
	if err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, dto.ToTalResponse{
		TotalRevenue:       totalRevenue,
		LastMonthRevenue:   lastMonthRevenue,
		CurrentWeekRevenue: currentWeekRevenue,
		MonthlyRevenue:     monthlyRevenue,
	})
}

// getTodayRevenue lấy doanh thu theo ngày
func getTodayRevenue(adminID *uint) (float64, error) {
	var todayRevenue float64
	today := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 0, 0, 0, 0, time.UTC)

	query := config.DB.Model(&models.Invoice{}).Where("status = ? AND created_at AT TIME ZONE 'UTC' >= ?", 1, today)
	if adminID != nil {
		query = query.Where("admin_id = ?", *adminID)
	}

	if err := query.Select("COALESCE(SUM(total_amount), 0)").Scan(&todayRevenue).Error; err != nil {
		return 0, err
	}

	return todayRevenue, nil
}

// GetToday lấy doanh thu hôm nay
func GetToday(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	_, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 1 && currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	todayRevenue, err := getTodayRevenue(nil)
	if err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{
		"revenue": todayRevenue,
	})
}

// GetTodayUser lấy doanh thu hôm nay theo user
func GetTodayUser(c *gin.Context) {
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

	if currentUserRole != 1 && currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	todayRevenue, err := getTodayRevenue(&currentUserID)
	if err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{
		"revenue": todayRevenue,
	})
}

// GetUserRevene lấy doanh thu theo user
func GetUserRevene(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	_, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 1 && currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	var userRevenues []dto.UserRevenueResponse
	today := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 0, 0, 0, 0, time.UTC)

	var results []struct {
		ID         uint
		Date       string
		OrderCount int
		Revenue    float64
		UserID     uint
	}

	if err := config.DB.Model(&models.UserRevenue{}).Where("date = ?", today.Format("2006-01-02")).Find(&results).Error; err != nil {
		response.ServerError(c)
		return
	}

	for _, result := range results {
		var user models.User
		if err := config.DB.First(&user, result.UserID).Error; err != nil {
			continue
		}

		userRevenues = append(userRevenues, dto.UserRevenueResponse{
			ID:         result.ID,
			Date:       result.Date,
			OrderCount: result.OrderCount,
			Revenue:    result.Revenue,
			User: types.InvoiceUserResponse{
				ID:          user.ID,
				Name:        user.Name,
				Email:       user.Email,
				PhoneNumber: user.PhoneNumber,
			},
		})
	}

	response.Success(c, userRevenues)
}
