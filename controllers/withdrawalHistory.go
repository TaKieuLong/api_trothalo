package controllers

import (
	"net/http"
	"strings"
	"time"

	"new/config"
	"new/models"

	"github.com/gin-gonic/gin"
)

type WithdrawalHistoryInput struct {
	Amount int64 `json:"amount" binding:"required"`
}

type WithdrawalHistoryResponse struct {
	ID        uint      `json:"id"`
	Amount    int64     `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	User      Actor     `json:"user"`
}

// CreateWithdrawalHistory tạo một lịch sử rút tiền mới
func CreateWithdrawalHistory(c *gin.Context) {

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

	var input WithdrawalHistoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ", "err": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.First(&user, currentUserID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lấy thông tin người dùng", "err": err.Error()})
		return
	}

	// Tính số tiền cho phép rút: nhỏ hơn 80% số dư hiện có của user
	allowedWithdrawal := user.Amount * 80 / 100
	if input.Amount >= allowedWithdrawal {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Số tiền rút phải nhỏ hơn 20% số dư của bạn"})
		return
	}

	withdrawal := models.WithdrawalHistory{
		UserID: currentUserID,
		Amount: input.Amount,
	}

	if err := config.DB.Create(&withdrawal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể tạo lịch sử rút tiền", "err": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Tạo lịch sử rút tiền thành công",
	})
}

func GetWithdrawalHistory(c *gin.Context) {

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

	if currentUserRole != 1 && currentUserRole != 2 {
		c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Bạn không có quyền truy cập", "id": currentUserID})
		return
	}

	var withdrawals []models.WithdrawalHistory
	dbQuery := config.DB.Preload("User").Preload("User.Banks")

	if currentUserRole == 2 {
		dbQuery = dbQuery.Where("user_id = ?", currentUserID)
	}

	if err := dbQuery.Find(&withdrawals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 0,
			"mess": "Không thể lấy lịch sử rút tiền",
			"err":  err.Error(),
		})
		return
	}

	var responses []WithdrawalHistoryResponse
	for _, w := range withdrawals {

		resp := WithdrawalHistoryResponse{
			ID:        w.ID,
			Amount:    w.Amount,
			Status:    w.Status,
			CreatedAt: w.CreatedAt,
			UpdatedAt: w.UpdatedAt,
			User: Actor{
				Name:          w.User.Name,
				Email:         w.User.Email,
				PhoneNumber:   w.User.PhoneNumber,
				BankShortName: w.User.Banks[0].BankShortName,
				AccountNumber: w.User.Banks[0].AccountNumber,
				BankName:      w.User.Banks[0].BankName,
			},
		}
		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy lịch sử rút tiền thành công",
		"data": responses,
	})
}

func ConfirmWithdrawalHistory(c *gin.Context) {
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

	var input struct {
		ID     uint   `json:"id" binding:"required"`
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ", "err": err.Error()})
		return
	}

	var withdrawal models.WithdrawalHistory
	if err := config.DB.First(&withdrawal, input.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Không tìm thấy đơn rút tiền", "err": err.Error()})
		return
	}

	if input.Status == "1" {
		var user models.User
		if err := config.DB.First(&user, withdrawal.UserID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Không tìm thấy user", "err": err.Error()})
			return
		}

		user.Amount = user.Amount - withdrawal.Amount

		if err := config.DB.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể cập nhật số dư của user", "err": err.Error()})
			return
		}
	}

	withdrawal.Status = input.Status
	if err := config.DB.Save(&withdrawal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể cập nhật trạng thái", "err": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Cập nhật trạng thái đơn rút tiền thành công",
	})
}
