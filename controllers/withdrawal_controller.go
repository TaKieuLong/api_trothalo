package controllers

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"new/config"
	"new/dto"
	"new/models"
	"new/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/unicode/norm"
)

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
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 2 {
		response.Forbidden(c)
		return
	}

	var input dto.CreateWithdrawalRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Dữ liệu không hợp lệ")
		return
	}

	var user models.User
	if err := config.DB.First(&user, currentUserID).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Tính số tiền cho phép rút: nhỏ hơn 80% số dư hiện có của user
	allowedWithdrawal := user.Amount * 80 / 100
	if input.Amount >= allowedWithdrawal {
		response.BadRequest(c, "Số tiền rút phải nhỏ hơn 20% số dư của bạn")
		return
	}

	withdrawal := models.WithdrawalHistory{
		UserID:          currentUserID,
		Amount:          input.Amount,
		BankID:          input.BankID,
		Note:            input.Note,
		TransactionCode: input.TransactionCode,
	}

	if err := config.DB.Create(&withdrawal).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{
		"mess": "Tạo lịch sử rút tiền thành công",
	})
}

func GetWithdrawalHistory(c *gin.Context) {
	// Kiểm tra header Authorization
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 1 && currentUserRole != 2 {
		response.Forbidden(c)
		return
	}

	// Lấy dữ liệu từ DB
	var withdrawals []models.WithdrawalHistory
	dbQuery := config.DB.Preload("User").Preload("User.Banks")
	if currentUserRole == 2 {
		dbQuery = dbQuery.Where("user_id = ?", currentUserID)
	}

	if err := dbQuery.Find(&withdrawals).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Chuyển đổi dữ liệu thành responses
	var responses []dto.WithdrawalHistoryResponse
	for _, w := range withdrawals {
		resp := dto.WithdrawalHistoryResponse{
			ID:              w.ID,
			UserID:          w.UserID,
			Amount:          w.Amount,
			Status:          w.Status,
			CreatedAt:       w.CreatedAt.Format(time.RFC3339),
			UpdatedAt:       w.UpdatedAt.Format(time.RFC3339),
			Note:            w.Note,
			TransactionCode: w.TransactionCode,
			User: &dto.UserResponse{
				ID:          w.User.ID,
				Name:        w.User.Name,
				Email:       w.User.Email,
				PhoneNumber: w.User.PhoneNumber,
				Role:        w.User.Role,
			},
		}

		if len(w.User.Banks) > 0 {
			resp.User.Banks = []dto.Bank{
				{
					BankName:      w.User.Banks[0].BankName,
					AccountNumber: w.User.Banks[0].AccountNumber,
					BankShortName: w.User.Banks[0].BankShortName,
				},
			}
		}
		responses = append(responses, resp)
	}

	statusFilter := c.Query("status")
	if statusFilter != "" {
		var filtered []dto.WithdrawalHistoryResponse
		statusInt, _ := strconv.Atoi(statusFilter)
		for _, resp := range responses {
			if resp.Status == statusInt {
				filtered = append(filtered, resp)
			}
		}
		responses = filtered
	}

	nameFilter := c.Query("name")
	if nameFilter != "" {
		var filtered []dto.WithdrawalHistoryResponse
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
		timeI, _ := time.Parse(time.RFC3339, responses[i].CreatedAt)
		timeJ, _ := time.Parse(time.RFC3339, responses[j].CreatedAt)
		return timeI.After(timeJ)
	})

	total := len(responses)
	start := page * limit
	end := start + limit

	if start >= total {
		responses = []dto.WithdrawalHistoryResponse{}
	} else if end > total {
		responses = responses[start:]
	} else {
		responses = responses[start:end]
	}

	response.Success(c, gin.H{
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
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	_, currentUserRole, err := GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 1 {
		response.Forbidden(c)
		return
	}

	var input dto.UpdateWithdrawalStatusRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Dữ liệu không hợp lệ")
		return
	}

	var withdrawal models.WithdrawalHistory
	if err := config.DB.First(&withdrawal, input.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	withdrawal.Status = input.Status
	withdrawal.Note = input.Note
	withdrawal.TransactionCode = input.TransactionCode

	if err := config.DB.Save(&withdrawal).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{
		"mess": "Cập nhật trạng thái thành công",
	})
}
