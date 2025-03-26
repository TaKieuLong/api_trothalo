package controllers

import (
	"log"
	"net/url"
	"new/config"
	"new/dto"
	"new/models"
	"new/response"
	"new/services"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Lọc benefit theo status
func filterBenefitsByStatus(benefits []models.Benefit, status int) []dto.BenefitResponse {
	var filtered []dto.BenefitResponse
	for _, b := range benefits {
		if b.Status == status {
			filtered = append(filtered, dto.BenefitResponse{
				Id:   b.Id,
				Name: b.Name,
			})
		}
	}
	return filtered
}

// Lọc Benefit cho cms
func filterBenefits(benefits []models.Benefit, statusFilter, nameFilter string) []dto.BenefitResponse {
	var filtered []dto.BenefitResponse
	for _, b := range benefits {
		// Filter theo status
		if statusFilter != "" {
			parsedStatus, err := strconv.Atoi(statusFilter)
			if err == nil && b.Status != parsedStatus {
				continue
			}
		}

		// Filter theo name
		if nameFilter != "" {
			decodedNameFilter, _ := url.QueryUnescape(nameFilter)
			if !strings.Contains(strings.ToLower(b.Name), strings.ToLower(decodedNameFilter)) {
				continue
			}
		}

		filtered = append(filtered, dto.BenefitResponse{
			Id:   b.Id,
			Name: b.Name,
		})
	}
	return filtered
}

func GetAllBenefit(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	currentUserRole := 0
	if authHeader != "" {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		_, role, err := GetUserIDFromToken(tokenString)
		if err != nil {
			response.Unauthorized(c)
			return
		}
		currentUserRole = role
	}

	statusFilter := c.Query("status")
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

	// Redis cache key
	cacheKey := "benefits:all"
	rdb, err := config.ConnectRedis()
	if err != nil {
		response.ServerError(c)
		return
	}

	var allBenefits []models.Benefit

	err = services.GetFromRedis(config.Ctx, rdb, cacheKey, &allBenefits)
	if err != nil || len(allBenefits) == 0 {
		if err := config.DB.Find(&allBenefits).Error; err != nil {
			response.ServerError(c)
			return
		}

		// Lưu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, allBenefits, 24*60*60*time.Minute); err != nil {
			log.Printf("Lỗi khi lưu danh sách lợi ích vào Redis: %v", err)
		}
	}

	var filteredBenefits []dto.BenefitResponse

	//filter role = 1,2,3 cho sidebar cms, còn lại filter cho web user
	if currentUserRole != 0 {
		filteredBenefits = filterBenefits(allBenefits, statusFilter, nameFilter)
	} else {
		filteredBenefits = filterBenefitsByStatus(allBenefits, 0)
	}

	// Pagination
	total := len(filteredBenefits)
	if currentUserRole == 0 {
		// Nếu userRole là 0, không áp dụng phân trang, trả về tất cả dữ liệu
		response.Success(c, filteredBenefits)
		return
	}

	//Các user khác phân trang
	start := page * limit
	end := start + limit

	if start >= total {
		filteredBenefits = []dto.BenefitResponse{}
	} else if end > total {
		filteredBenefits = filteredBenefits[start:]
	} else {
		filteredBenefits = filteredBenefits[start:end]
	}

	// Trả về kết quả với phân trang
	response.SuccessWithPagination(c, filteredBenefits, page, limit, total)
}

func CreateBenefit(c *gin.Context) {
	var benefitRequests []dto.CreateBenefitRequest

	if err := c.ShouldBindJSON(&benefitRequests); err != nil {
		response.BadRequest(c, "Dữ liệu không hợp lệ")
		return
	}

	var benefit []models.Benefit
	for _, benefitRequest := range benefitRequests {
		benefit = append(benefit, models.Benefit{Name: benefitRequest.Name})
	}
	if err := config.DB.Create(&benefit).Error; err != nil {
		response.ServerError(c)
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "benefits:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
	}

	response.Success(c, benefit)
}

func GetBenefitDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "ID không hợp lệ")
		return
	}

	var benefit models.Benefit
	if err := config.DB.First(&benefit, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	response.Success(c, benefit)
}

func UpdateBenefit(c *gin.Context) {
	var request dto.UpdateBenefitRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Dữ liệu không hợp lệ")
		return
	}

	var benefit models.Benefit
	if err := config.DB.First(&benefit, request.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	benefit.Name = request.Name

	if err := config.DB.Save(&benefit).Error; err != nil {
		response.ServerError(c)
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "benefits:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
	}

	response.Success(c, benefit)
}

func ChangeBenefitStatus(c *gin.Context) {
	var request dto.ChangeBenefitStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Dữ liệu không hợp lệ")
		return
	}

	var benefit models.Benefit
	if err := config.DB.First(&benefit, request.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	benefit.Status = request.Status

	if err := benefit.ValidateStatus(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := config.DB.Model(&benefit).Update("status", request.Status).Error; err != nil {
		response.ServerError(c)
		return
	}

	benefit.Status = request.Status

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "benefits:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
	}

	response.Success(c, benefit)
}
