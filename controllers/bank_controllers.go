package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"new/config"
	"new/dto"
	"new/models"
	"strings"

	"github.com/gin-gonic/gin"
)

func CreateBank(c *gin.Context) {
	var bankRequest dto.BankRequest

	if err := c.ShouldBindJSON(&bankRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Lỗi khi ràng buộc dữ liệu", "error": err.Error()})
		return
	}

	bankRequest.BankShortName = strings.ToUpper(bankRequest.BankShortName)

	var existingBankByName models.BankFake
	if err := config.DB.Where("bank_name = ?", bankRequest.BankName).First(&existingBankByName).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Ngân hàng đã tồn tại"})
		return
	}

	var existingBankByShortName models.BankFake
	if err := config.DB.Where("bank_short_name = ?", bankRequest.BankShortName).First(&existingBankByShortName).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Tên viết tắt ngân hàng đã tồn tại"})
		return
	}

	if err := bankRequest.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": err.Error()})
		return
	}

	var accountNumbers []string
	if err := json.Unmarshal(bankRequest.AccountNumbers, &accountNumbers); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Lỗi khi giải mã danh sách số tài khoản", "error": err.Error()})
		return
	}

	accountSet := make(map[string]struct{})
	for _, accountNumber := range accountNumbers {
		if _, exists := accountSet[accountNumber]; exists {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Danh sách số tài khoản chứa số tài khoản trùng lặp"})
			return
		}
		accountSet[accountNumber] = struct{}{}
	}

	bankRequest.AccountNumbers, _ = json.Marshal(accountNumbers)

	bank := models.BankFake{
		BankName:       bankRequest.BankName,
		BankShortName:  bankRequest.BankShortName,
		AccountNumbers: bankRequest.AccountNumbers,
		Icon:           bankRequest.Icon,
	}

	if err := config.DB.Create(&bank).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi tạo ngân hàng", "error": err.Error()})
		return
	}

	bankResponse := dto.BankResponse{
		ID:             bank.ID,
		BankName:       bank.BankName,
		BankShortName:  bank.BankShortName,
		AccountNumbers: bank.AccountNumbers,
		Icon:           bank.Icon,
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Tạo ngân hàng thành công",
		"data": bankResponse,
	})
}

func AddAccountNumbers(c *gin.Context) {
	var request dto.AddAccountNumbersRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Lỗi khi ràng buộc dữ liệu: " + err.Error()})
		return
	}

	var bank models.BankFake
	if err := config.DB.First(&bank, request.BankID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Ngân hàng không tồn tại với ID: " + fmt.Sprint(request.BankID)})
		return
	}

	if request.AccountNumbers != nil {
		var accounts []string
		if err := json.Unmarshal(request.AccountNumbers, &accounts); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Định dạng số tài khoản không hợp lệ: " + err.Error()})
			return
		}

		accountMap := make(map[string]int)
		duplicates := []string{}

		for _, account := range accounts {
			accountMap[account]++
			if accountMap[account] == 2 {
				duplicates = append(duplicates, account)
			}
		}

		if len(duplicates) > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Có số tài khoản trùng lặp"})
			return
		}

		existingAccounts := make([]string, 0)
		for _, account := range accounts {
			var count int64
			err := config.DB.Model(&models.BankFake{}).Where("id = ? AND account_numbers::jsonb @> ?::jsonb", bank.ID, fmt.Sprintf(`["%s"]`, account)).Count(&count).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi kiểm tra số tài khoản: " + err.Error()})
				return
			}
			if count > 0 {
				existingAccounts = append(existingAccounts, account)
			}
		}

		if len(existingAccounts) > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Có số tài khoản trùng lặp trong cơ sở dữ liệu"})
			return
		}

		var existingAccountNumbers []string
		if err := json.Unmarshal(bank.AccountNumbers, &existingAccountNumbers); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi giải mã số tài khoản hiện có: " + err.Error()})
			return
		}
		existingAccountNumbers = append(existingAccountNumbers, accounts...)
		bank.AccountNumbers, _ = json.Marshal(existingAccountNumbers)

		if err := bank.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Số tài khoản không hợp lệ: " + err.Error()})
			return
		}
	}

	if err := config.DB.Save(&bank).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể cập nhật ngân hàng: " + err.Error()})
		return
	}

	bankResponse := dto.BankResponse{
		ID:             bank.ID,
		BankName:       bank.BankName,
		BankShortName:  bank.BankShortName,
		AccountNumbers: bank.AccountNumbers,
		Icon:           bank.Icon,
	}

	c.JSON(http.StatusOK, gin.H{"code": 1, "mess": "Cập nhật ngân hàng thành công", "data": bankResponse})
}

func GetAllBanks(c *gin.Context) {
	var banks []models.BankFake

	if err := config.DB.Find(&banks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy danh sách ngân hàng", "error": err.Error()})
		return
	}

	var bankResponses []dto.BankResponse
	for _, bank := range banks {
		bankResponses = append(bankResponses, dto.BankResponse{
			ID:             bank.ID,
			BankName:       bank.BankName,
			BankShortName:  bank.BankShortName,
			AccountNumbers: bank.AccountNumbers,
			Icon:           bank.Icon,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy danh sách ngân hàng thành công",
		"data": bankResponses,
	})
}

func DeleteAllBanks(c *gin.Context) {
	if err := config.DB.Exec("DELETE FROM bank_fakes").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi xóa tất cả ngân hàng", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Xóa tất cả ngân hàng thành công",
	})
}
