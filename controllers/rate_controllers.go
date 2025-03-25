package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"new/config"
	"new/dto"
	"new/models"
	"new/response"
	"new/services"
	"new/validator"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GetAllRates(c *gin.Context) {
	accommodationIdFilter := c.DefaultQuery("accommodationId", "")

	cacheKey := "rates:all"
	if accommodationIdFilter != "" {
		cacheKey = fmt.Sprintf("rates:accommodation:%s", accommodationIdFilter)
	}

	// Kết nối Redis
	rdb, err := config.ConnectRedis()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể kết nối Redis", "error": err.Error()})
		return
	}

	var rates []models.Rate

	// Lấy dữ liệu từ Redis
	err = services.GetFromRedis(config.Ctx, rdb, cacheKey, &rates)
	if err == nil && len(rates) > 0 {
		var rateResponses []dto.RateResponse
		for _, rate := range rates {
			rateResponse := dto.RateResponse{
				ID:              rate.ID,
				AccommodationID: rate.AccommodationID,
				Comment:         rate.Comment,
				Star:            rate.Star,
				CreatedAt:       rate.CreatedAt,
				UpdatedAt:       rate.UpdatedAt,
				User: dto.UserInfo{
					ID:     rate.User.ID,
					Name:   rate.User.Name,
					Avatar: rate.User.Avatar,
				},
			}
			rateResponses = append(rateResponses, rateResponse)
		}
		response.Success(c, rateResponses)
		return
	}

	// Lấy dữ liệu từ database
	tx := config.DB.Preload("User")
	if accommodationIdFilter != "" {
		if parsedAccommodationId, err := strconv.Atoi(accommodationIdFilter); err == nil {
			tx = tx.Where("accommodation_id = ?", parsedAccommodationId)
		}
	}

	if err := tx.Limit(20).Find(&rates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy danh sách đánh giá", "error": err.Error()})
		return
	}

	var rateResponses []dto.RateResponse
	for _, rate := range rates {
		rateResponse := dto.RateResponse{
			ID:              rate.ID,
			AccommodationID: rate.AccommodationID,
			Comment:         rate.Comment,
			Star:            rate.Star,
			CreatedAt:       rate.CreatedAt,
			UpdatedAt:       rate.UpdatedAt,
			User: dto.UserInfo{
				ID:     rate.User.ID,
				Name:   rate.User.Name,
				Avatar: rate.User.Avatar,
			},
		}
		rateResponses = append(rateResponses, rateResponse)
	}

	rateResponsesJSON, err := json.Marshal(rateResponses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi serialize dữ liệu", "error": err.Error()})
		return
	}

	if err := services.SetToRedis(config.Ctx, rdb, cacheKey, rateResponsesJSON, 10*time.Minute); err != nil {
		log.Printf("Lỗi khi lưu danh sách đánh giá vào Redis: %v", err)
	}

	response.Success(c, rateResponses)
}

func CreateRate(c *gin.Context) {
	var request dto.CreateRateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

	rate := models.Rate{
		AccommodationID: request.AccommodationID,
		Comment:         request.Comment,
		Star:            request.Star,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := validator.ValidateRate(&rate); err != nil {
		response.Error(c, 0, err.Error())
		return
	}

	if err := config.DB.Create(&rate).Error; err != nil {
		response.ServerError(c)
		return
	}

	if err := services.UpdateAccommodationRating(rate.AccommodationID); err != nil {
		response.Error(c, 0, "Failed to update accommodation rating")
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "rates:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
		cacheKey2 := "accommodations:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey2)
	}

	response.Success(c, rate)
}

func GetRateDetail(c *gin.Context) {
	id := c.Param("id")
	var rate models.Rate
	if err := config.DB.Preload("User").First(&rate, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	rateResponse := dto.RateResponse{
		ID:              rate.ID,
		AccommodationID: rate.AccommodationID,
		Comment:         rate.Comment,
		Star:            rate.Star,
		CreatedAt:       rate.CreatedAt,
		UpdatedAt:       rate.UpdatedAt,
		User: dto.UserInfo{
			ID:     rate.User.ID,
			Name:   rate.User.Name,
			Avatar: rate.User.Avatar,
		},
	}

	response.Success(c, rateResponse)
}

func DeleteRate(c *gin.Context) {
	id := c.Param("id")

	var rate models.Rate
	if err := config.DB.First(&rate, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	if err := config.DB.Delete(&rate).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{"message": "Xóa đánh giá thành công"})
}

func UpdateRate(c *gin.Context) {
	var request dto.UpdateRateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

	var existingRate models.Rate
	if err := config.DB.First(&existingRate, request.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	existingRate.Comment = request.Comment
	existingRate.Star = request.Star
	existingRate.UpdatedAt = time.Now()

	if err := validator.ValidateRate(&existingRate); err != nil {
		response.Error(c, 0, err.Error())
		return
	}

	if err := config.DB.Save(&existingRate).Error; err != nil {
		response.ServerError(c)
		return
	}

	if err := services.UpdateAccommodationRating(request.AccommodationID); err != nil {
		response.Error(c, 0, "Failed to update accommodation rating")
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "rates:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
		cacheKey2 := "accommodations:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey2)
	}

	response.Success(c, existingRate)
}
