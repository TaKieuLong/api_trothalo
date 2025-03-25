package controllers

import (
	"log"
	"new/config"
	"new/dto"
	"new/models"
	"new/response"
	"new/services"
	"strconv"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
)

// GetHolidays lấy tất cả kỳ nghỉ
func GetHolidays(c *gin.Context) {
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

	// Tạo cache key dựa trên role
	var cacheKey string
	if currentUserRole == 2 {
		cacheKey = "holidays:admin"
	} else if currentUserRole == 3 {
		cacheKey = "holidays:receptionist"
	} else {
		cacheKey = "holidays:all"
	}

	// Kết nối Redis
	rdb, err := config.ConnectRedis()
	if err != nil {
		response.ServerError(c)
		return
	}

	var allHolidays []dto.HolidayResponse

	// Lấy dữ liệu từ Redis
	if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &allHolidays); err != nil || len(allHolidays) == 0 {
		tx := config.DB.Model(&models.Holiday{})

		var holidays []models.Holiday
		if err := tx.Find(&holidays).Error; err != nil {
			response.ServerError(c)
			return
		}

		for _, holiday := range holidays {
			allHolidays = append(allHolidays, dto.HolidayResponse{
				ID:        holiday.ID,
				Name:      holiday.Name,
				FromDate:  holiday.FromDate,
				ToDate:    holiday.ToDate,
				Price:     holiday.Price,
				CreatedAt: holiday.CreatedAt,
				UpdatedAt: holiday.UpdatedAt,
			})
		}

		// Lưu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, allHolidays, 60*time.Minute); err != nil {
			log.Printf("Error caching holidays: %v", err)
		}
	}

	total := len(allHolidays)
	start := page * limit
	end := start + limit
	if start >= total {
		allHolidays = []dto.HolidayResponse{}
	} else if end > total {
		allHolidays = allHolidays[start:]
	} else {
		allHolidays = allHolidays[start:end]
	}

	response.Success(c, dto.HolidayListResponse{
		Holidays: allHolidays,
		Pagination: response.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// CreateHoliday tạo một kỳ nghỉ mới
func CreateHoliday(c *gin.Context) {
	var request dto.CreateHolidayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

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

	if currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	fromDate, err := time.Parse("2006-01-02 15:04:05", request.FromDate)
	if err != nil {
		response.ValidationError(c, "Định dạng ngày bắt đầu không hợp lệ")
		return
	}

	toDate, err := time.Parse("2006-01-02 15:04:05", request.ToDate)
	if err != nil {
		response.ValidationError(c, "Định dạng ngày kết thúc không hợp lệ")
		return
	}

	if toDate.Before(fromDate) {
		response.ValidationError(c, "Ngày kết thúc phải sau ngày bắt đầu")
		return
	}

	holiday := models.Holiday{
		Name:     request.Name,
		FromDate: request.FromDate,
		ToDate:   request.ToDate,
		Price:    request.Price,
	}

	if err := config.DB.Create(&holiday).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Xóa cache
	rdb, err := config.ConnectRedis()
	if err == nil {
		iter := rdb.Scan(config.Ctx, 0, "holidays:*", 0).Iterator()
		for iter.Next(config.Ctx) {
			rdb.Del(config.Ctx, iter.Val())
		}
	}

	response.Success(c, dto.HolidayResponse{
		ID:        holiday.ID,
		Name:      holiday.Name,
		FromDate:  holiday.FromDate,
		ToDate:    holiday.ToDate,
		Price:     holiday.Price,
		CreatedAt: holiday.CreatedAt,
		UpdatedAt: holiday.UpdatedAt,
	})
}

func GetDetailHoliday(c *gin.Context) {
	var holiday models.Holiday
	if err := config.DB.Where("id = ?", c.Param("id")).First(&holiday).Error; err != nil {
		response.NotFound(c)
		return
	}

	response.Success(c, dto.HolidayResponse{
		ID:        holiday.ID,
		Name:      holiday.Name,
		FromDate:  holiday.FromDate,
		ToDate:    holiday.ToDate,
		Price:     holiday.Price,
		CreatedAt: holiday.CreatedAt,
		UpdatedAt: holiday.UpdatedAt,
	})
}

// UpdateHoliday cập nhật một kỳ nghỉ
func UpdateHoliday(c *gin.Context) {
	var request dto.CreateHolidayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

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

	if currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	var holiday models.Holiday
	if err := config.DB.Where("id = ?", c.Param("id")).First(&holiday).Error; err != nil {
		response.NotFound(c)
		return
	}

	fromDate, err := time.Parse("2006-01-02 15:04:05", request.FromDate)
	if err != nil {
		response.ValidationError(c, "Định dạng ngày bắt đầu không hợp lệ")
		return
	}

	toDate, err := time.Parse("2006-01-02 15:04:05", request.ToDate)
	if err != nil {
		response.ValidationError(c, "Định dạng ngày kết thúc không hợp lệ")
		return
	}

	if toDate.Before(fromDate) {
		response.ValidationError(c, "Ngày kết thúc phải sau ngày bắt đầu")
		return
	}

	holiday.Name = request.Name
	holiday.FromDate = request.FromDate
	holiday.ToDate = request.ToDate
	holiday.Price = request.Price

	if err := config.DB.Save(&holiday).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Xóa cache
	rdb, err := config.ConnectRedis()
	if err == nil {
		iter := rdb.Scan(config.Ctx, 0, "holidays:*", 0).Iterator()
		for iter.Next(config.Ctx) {
			rdb.Del(config.Ctx, iter.Val())
		}
	}

	response.Success(c, dto.HolidayResponse{
		ID:        holiday.ID,
		Name:      holiday.Name,
		FromDate:  holiday.FromDate,
		ToDate:    holiday.ToDate,
		Price:     holiday.Price,
		CreatedAt: holiday.CreatedAt,
		UpdatedAt: holiday.UpdatedAt,
	})
}

func DeleteHoliday(c *gin.Context) {
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

	if currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	var holiday models.Holiday
	if err := config.DB.Where("id = ?", c.Param("id")).First(&holiday).Error; err != nil {
		response.NotFound(c)
		return
	}

	if err := config.DB.Delete(&holiday).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Xóa cache
	rdb, err := config.ConnectRedis()
	if err == nil {
		iter := rdb.Scan(config.Ctx, 0, "holidays:*", 0).Iterator()
		for iter.Next(config.Ctx) {
			rdb.Del(config.Ctx, iter.Val())
		}
	}

	response.Success(c, nil)
}

func GetHolidaysByDateRange(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		response.ValidationError(c, "Ngày bắt đầu không hợp lệ")
		return
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		response.ValidationError(c, "Ngày kết thúc không hợp lệ")
		return
	}

	if end.Before(start) {
		response.ValidationError(c, "Ngày kết thúc phải sau ngày bắt đầu")
		return
	}

	var holidays []models.Holiday
	if err := config.DB.Where("from_date BETWEEN ? AND ?", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05")).Find(&holidays).Error; err != nil {
		response.ServerError(c)
		return
	}

	var holidayResponses []dto.HolidayResponse
	for _, holiday := range holidays {
		holidayResponses = append(holidayResponses, dto.HolidayResponse{
			ID:        holiday.ID,
			Name:      holiday.Name,
			FromDate:  holiday.FromDate,
			ToDate:    holiday.ToDate,
			Price:     holiday.Price,
			CreatedAt: holiday.CreatedAt,
			UpdatedAt: holiday.UpdatedAt,
		})
	}

	response.Success(c, holidayResponses)
}
