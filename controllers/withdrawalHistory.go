package controllers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"new/config"
	"new/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/unicode/norm"
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

// Bỏ dấu viết thường
func removeDiacritics(s string) string {
	// Chuẩn hóa chuỗi về dạng NFD (Normalization Form Decomposition)
	t := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range t {
		// Loại bỏ các ký tự dấu (non-spacing marks)
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
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
	// Kiểm tra header Authorization
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

	// Lấy dữ liệu từ DB
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

	// Chuyển đổi dữ liệu thành responses
	var responses []WithdrawalHistoryResponse
	for _, w := range withdrawals {
		resp := WithdrawalHistoryResponse{
			ID:        w.ID,
			Amount:    w.Amount,
			Status:    w.Status,
			CreatedAt: w.CreatedAt,
			UpdatedAt: w.UpdatedAt,
			User: Actor{
				Name:        w.User.Name,
				Email:       w.User.Email,
				PhoneNumber: w.User.PhoneNumber,
			},
		}

		if len(w.User.Banks) > 0 {
			resp.User.BankShortName = w.User.Banks[0].BankShortName
			resp.User.AccountNumber = w.User.Banks[0].AccountNumber
			resp.User.BankName = w.User.Banks[0].BankName
		}
		responses = append(responses, resp)
	}

	statusFilter := c.Query("status")
	if statusFilter != "" {
		var filtered []WithdrawalHistoryResponse
		for _, resp := range responses {
			if resp.Status == statusFilter {
				filtered = append(filtered, resp)
			}
		}
		responses = filtered
	}

	nameFilter := c.Query("name")
	if nameFilter != "" {
		var filtered []WithdrawalHistoryResponse
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

	sort.Slice(responses, func(i, j int) bool {
		return responses[i].CreatedAt.After(responses[j].CreatedAt)
	})

	total := len(responses)
	start := page * limit
	end := start + limit

	if start >= total {
		responses = []WithdrawalHistoryResponse{}
	} else if end > total {
		responses = responses[start:]
	} else {
		responses = responses[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy lịch sử rút tiền thành công",
		"data": responses,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
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
