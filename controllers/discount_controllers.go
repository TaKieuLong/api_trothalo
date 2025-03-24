package controllers

import (
	"net/url"
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

var layout = "02/01/2006"

func ConvertDateToComparableFormat(dateStr string) (string, error) {
	parsedDate, err := time.Parse(layout, dateStr)
	if err != nil {
		return "", err
	}
	return parsedDate.Format("20060102"), nil
}

func GetAllDiscounts(c *gin.Context) {
	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	nameFilter := c.Query("name")
	fromDateStr := c.Query("fromDate")
	toDateStr := c.Query("toDate")

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

	tx := config.DB.Model(&models.Discount{})

	if nameFilter != "" {
		decodedNameFilter, _ := url.QueryUnescape(nameFilter)
		tx = tx.Where("name LIKE ?", "%"+decodedNameFilter+"%")
	}

	if fromDateStr != "" {
		fromDateComparable, err := ConvertDateToComparableFormat(fromDateStr)
		if err != nil {
			response.BadRequest(c, "Sai định dạng fromDate")
			return
		}

		if toDateStr != "" {
			toDateComparable, err := ConvertDateToComparableFormat(toDateStr)
			if err != nil {
				response.BadRequest(c, "Sai định dạng toDate")
				return
			}
			tx = tx.Where("SUBSTRING(from_date, 7, 4) || SUBSTRING(from_date, 4, 2) || SUBSTRING(from_date, 1, 2) >= ? AND SUBSTRING(to_date, 7, 4) || SUBSTRING(to_date, 4, 2) || SUBSTRING(to_date, 1, 2) <= ?", fromDateComparable, toDateComparable)
		} else {
			tx = tx.Where("SUBSTRING(from_date, 7, 4) || SUBSTRING(from_date, 4, 2) || SUBSTRING(from_date, 1, 2) >= ?", fromDateComparable)
		}
	}

	var totalDiscounts int64
	if err := tx.Count(&totalDiscounts).Error; err != nil {
		response.ServerError(c)
		return
	}
	tx = tx.Order("updated_at desc")

	var discounts []models.Discount
	if err := tx.Offset(page * limit).Limit(limit).Find(&discounts).Error; err != nil {
		response.ServerError(c)
		return
	}

	var discountResponses []dto.DiscountResponse
	for _, discount := range discounts {
		discountResponses = append(discountResponses, dto.DiscountResponse{
			ID:        discount.ID,
			Name:      discount.Name,
			Quantity:  discount.Quantity,
			FromDate:  discount.FromDate,
			ToDate:    discount.ToDate,
			Discount:  discount.Discount,
			Status:    discount.Status,
			CreatedAt: discount.CreatedAt,
			UpdatedAt: discount.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"code": 1,
		"mess": "Lấy danh sách chương trình giảm giá thành công",
		"data": discountResponses,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": totalDiscounts,
		}})
}

func GetDiscountDetail(c *gin.Context) {
	var discount models.Discount
	discountId := c.Param("id")
	if err := config.DB.Where("id = ?", discountId).First(&discount).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Success(c, discount)
}

func CreateDiscount(c *gin.Context) {
	var request dto.CreateDiscountRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

	discount := models.Discount{
		Name:        request.Name,
		Description: request.Description,
		Quantity:    request.Quantity,
		FromDate:    request.FromDate,
		ToDate:      request.ToDate,
		Discount:    request.Discount,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := validator.ValidateDiscount(&discount); err != nil {
		response.Error(c, 0, err.Error())
		return
	}

	if err := config.DB.Create(&discount).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, discount)
}

func UpdateDiscount(c *gin.Context) {
	id := c.Param("id")

	var discount models.Discount
	if err := config.DB.First(&discount, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	var request dto.UpdateDiscountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

	discount.Name = request.Name
	discount.Description = request.Description
	discount.Quantity = request.Quantity
	discount.FromDate = request.FromDate
	discount.ToDate = request.ToDate
	discount.Discount = request.Discount
	discount.Status = request.Status
	discount.UpdatedAt = time.Now()

	if err := validator.ValidateDiscount(&discount); err != nil {
		response.Error(c, 0, err.Error())
		return
	}

	if err := config.DB.Save(&discount).Error; err != nil {
		response.ServerError(c)
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "benefits:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
	}

	response.Success(c, discount)
}

func DeleteDiscount(c *gin.Context) {
	id := c.Param("id")

	var discount models.Discount
	if err := config.DB.First(&discount, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	if err := config.DB.Delete(&discount).Error; err != nil {
		response.ServerError(c)
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "benefits:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
	}

	response.Success(c, gin.H{"message": "Xóa mã giảm giá thành công"})
}

func ChangeDiscountStatus(c *gin.Context) {
	var request dto.ChangeDiscountStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Dữ liệu không hợp lệ: "+err.Error())
		return
	}

	var discount models.Discount
	if err := config.DB.First(&discount, request.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	discount.Status = request.Status

	if err := discount.ValidateStatusDiscount(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := config.DB.Model(&discount).Update("status", request.Status).Error; err != nil {
		response.ServerError(c)
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		cacheKey := "benefits:all"
		_ = services.DeleteFromRedis(config.Ctx, rdb, cacheKey)
	}

	response.Success(c, discount)
}
