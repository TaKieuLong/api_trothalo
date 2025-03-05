package controllers

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"new/config"
	"new/services"
	"sort"
	"strconv"
	"strings"
	"time"

	"new/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UpdateBalanceRequest struct {
	UserID uint  `json:"userId" binding:"required"`
	Amount int64 `json:"amount" binding:"required"`
}

func GetUsersByAdminID(adminID uint) ([]models.User, error) {
	var users []models.User
	if err := config.DB.Where("admin_id = ?", adminID).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func GetCheckedInUsers(startDate string, endDate string, users []models.User) ([]models.CheckInRecord, error) {
	var CheckIn []models.CheckInRecord
	userIDs := getUserIDs(users)

	if err := config.DB.
		Table("check_in_records").
		Where("DATE(check_in_records.date) BETWEEN ? AND ?", startDate, endDate).
		Where("check_in_records.user_id IN (?)", userIDs).
		Find(&CheckIn).Error; err != nil {
		return nil, err
	}

	return CheckIn, nil
}

func GetUserSalaries(startDate string, endDate string, users []models.User) ([]models.UserSalary, error) {
	var salaries []models.UserSalary
	userIDs := getUserIDs(users)

	if err := config.DB.
		Table("user_salaries").
		Where("DATE(salary_date) BETWEEN ? AND ?", startDate, endDate).
		Where("user_id IN (?)", userIDs).
		Find(&salaries).Error; err != nil {
		return nil, err
	}

	return salaries, nil
}

func getUserIDs(users []models.User) []uint {
	ids := make([]uint, len(users))
	for i, user := range users {
		ids[i] = user.ID
	}
	return ids
}

func GetUserAcc(c *gin.Context) {
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

	if currentUserRole != 2 {
		c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Không có quyền truy cập"})
		return
	}

	cacheKey := fmt.Sprintf("accommodations:admin:%d", currentUserID)

	rdb, err := config.ConnectRedis()
	if err != nil {
		log.Printf("Không thể kết nối Redis: %v", err)
	}

	var allAccommodations []models.Accommodation

	tx := config.DB.Model(&models.Accommodation{}).
		Preload("Rooms").
		Preload("Rates").
		Preload("Benefits").
		Preload("User").
		Preload("User.Banks").
		Where("user_id = ?", currentUserID)

	if err := tx.Find(&allAccommodations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lấy danh sách chỗ ở"})
		return
	}

	accUser := make([]AccommodationDetailResponse, 0)
	for _, acc := range allAccommodations {
		user := acc.User
		// Lấy thông tin ngân hàng nếu có
		bankShortName := ""
		accountNumber := ""
		bankName := ""
		if len(user.Banks) > 0 {
			bankShortName = user.Banks[0].BankShortName
			accountNumber = user.Banks[0].AccountNumber
			bankName = user.Banks[0].BankName
		}

		accUser = append(accUser, AccommodationDetailResponse{
			ID:               acc.ID,
			Type:             acc.Type,
			Name:             acc.Name,
			Address:          acc.Address,
			CreateAt:         acc.CreateAt,
			UpdateAt:         acc.UpdateAt,
			Avatar:           acc.Avatar,
			ShortDescription: acc.ShortDescription,
			Status:           acc.Status,
			Num:              acc.Num,
			Furniture:        acc.Furniture,
			People:           acc.People,
			Price:            acc.Price,
			NumBed:           acc.NumBed,
			NumTolet:         acc.NumTolet,
			Benefits:         acc.Benefits,
			TimeCheckIn:      acc.TimeCheckIn,
			TimeCheckOut:     acc.TimeCheckOut,
			Province:         acc.Province,
			District:         acc.District,
			Ward:             acc.Ward,
			Longitude:        acc.Longitude,
			Latitude:         acc.Latitude,
			User: Actor{
				Name:          user.Name,
				Email:         user.Email,
				PhoneNumber:   user.PhoneNumber,
				BankShortName: bankShortName,
				AccountNumber: accountNumber,
				BankName:      bankName,
			},
		})
	}

	if rdb != nil {
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, accUser, 60*time.Minute); err != nil {
			log.Printf("Lỗi khi lưu danh sách chỗ ở vào Redis: %v", err)
		}
	}

	if rdb != nil {
		if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &allAccommodations); err == nil && len(allAccommodations) > 0 {
			goto RESPONSE
		}
	}

RESPONSE:
	accommodationsResponse := make([]gin.H, 0)
	for _, acc := range allAccommodations {
		accommodationsResponse = append(accommodationsResponse, gin.H{
			"id":   acc.ID,
			"name": acc.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy danh sách chỗ ở thành công",
		"data": accommodationsResponse,
	})
}

func (u UserController) UpdateUserBalance(c *gin.Context) {
	var req UpdateBalanceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ"})
		return
	}

	if req.Amount > 2000000 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Không được vượt quá 2.000.000"})
		return
	} else if req.Amount < -1000000 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Không được nhỏ hơn -1.000.000"})
		return
	}

	var user models.User

	if err := config.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
		return
	}

	now := time.Now()

	if user.DateCheck.Year() == now.Year() && user.DateCheck.Month() == now.Month() {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Bạn đã cập nhật lương trong tháng này rồi, không thể cập nhật nữa!"})
		return
	}

	user.Amount += req.Amount
	user.DateCheck = now

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi cập nhật số dư"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Cập nhật số dư thành công",
		"data": gin.H{
			"userId":    user.ID,
			"amount":    user.Amount,
			"dateCheck": user.DateCheck,
		},
	})
}

func (u UserController) UpdateUserAccommodation(c *gin.Context) {

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Authorization header is missing"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, err := GetIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Invalid token"})
		return
	}

	var req struct {
		UserID          uint `json:"userId"`
		AccommodationID uint `json:"accommodationId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ"})
		return
	}

	var user models.User
	if err := config.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
		return
	}

	if user.Role != 3 || user.AdminId == nil || *user.AdminId != currentUserID {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không thuộc phạm quyền của bạn"})
		return
	}

	var count int64
	if err := config.DB.Model(&models.Accommodation{}).
		Where("id = ? AND user_id = ?", req.AccommodationID, currentUserID).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi kiểm tra quyền sở hữu"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Bạn không sở hữu lưu trú này"})
		return
	}

	user.AccommodationID = &req.AccommodationID
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi cập nhật địa điểm điểm danh"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Phân quyền thành công",
		"data": gin.H{
			"userId":          user.ID,
			"accommodationId": user.AccommodationID,
		},
	})
}

func (u UserController) CheckInUser(c *gin.Context) {
	var req struct {
		UserID    uint    `json:"userId"`
		Longitude float64 `json:"longitude"`
		Latitude  float64 `json:"latitude"`
	}

	var currentTime = time.Now()

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ"})
		return
	}

	var user models.User
	if err := config.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
		return
	}

	if user.AccommodationID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Người dùng chưa có thông tin lưu trú"})
		return
	}

	var accommodation models.Accommodation
	if err := config.DB.First(&accommodation, user.AccommodationID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không tìm thấy thông tin lưu trú"})
		return
	}

	const earthRadiusKm = 6371.0

	distance := func(lat1, lon1, lat2, lon2 float64) float64 {
		lat1Rad, lon1Rad := lat1*(math.Pi/180), lon1*(math.Pi/180)
		lat2Rad, lon2Rad := lat2*(math.Pi/180), lon2*(math.Pi/180)
		dLat, dLon := lat2Rad-lat1Rad, lon2Rad-lon1Rad

		a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
		c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

		return earthRadiusKm * c
	}

	d := distance(accommodation.Latitude, accommodation.Longitude, req.Latitude, req.Longitude)

	if d > 1.0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Vị trí không hợp lệ"})
		return
	}

	var existingRecord models.CheckInRecord
	today := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())

	if err := config.DB.Where("user_id = ? AND DATE(date) = ?", req.UserID, today).First(&existingRecord).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng đã điểm danh hôm nay"})
		return
	}

	user.Status = 1

	checkInRecord := models.CheckInRecord{
		UserID: req.UserID,
		Date:   currentTime,
	}

	if err := config.DB.Create(&checkInRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lưu thông tin điểm danh", "err": err})
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		adminCacheKey := fmt.Sprintf("user_checkin:%d", *user.AdminId)
		_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Điểm danh thành công",
		"data": gin.H{
			"userId":      user.ID,
			"status":      user.Status,
			"timeCheckIn": currentTime,
		},
	})
}

func GetUserCalendar(c *gin.Context) {
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

	if currentUserRole != 2 {
		c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Không có quyền truy cập"})
		return
	}

	date := c.Query("date")

	if date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Thiếu tham số date"})
		return
	}

	parsedDate, err := time.Parse("01/2006", date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Định dạng date không hợp lệ"})
		return
	}

	year, month, _ := parsedDate.Date()
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	days := make([]gin.H, 0)

	users, err := GetUsersByAdminID(currentUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy danh sách user"})
		return
	}

	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	endDate := fmt.Sprintf("%04d-%02d-%02d", year, month, daysInMonth)

	checkedInUsers, err := GetCheckedInUsers(startDate, endDate, users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy dữ liệu điểm danh", "err": err})
		return
	}

	for day := 1; day <= daysInMonth; day++ {
		dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		userList := make([]gin.H, 0)

		for _, record := range checkedInUsers {
			if record.Date.Format("2006-01-02") == dateStr {
				for _, user := range users {
					if user.ID == record.UserID {
						userList = append(userList, gin.H{"id": user.ID, "name": user.Name})
					}
				}
			}
		}

		days = append(days, gin.H{
			"date":  dateStr,
			"users": userList,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  1,
		"mess":  "Lấy danh sách ngày thành công",
		"total": daysInMonth,
		"data":  days,
	})
}

func CalculateUserSalaryInit(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Authorization header is missing"})
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, err := GetIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Invalid token"})
		return
	}

	var req struct {
		UserID uint `json:"userId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ"})
		return
	}

	var user models.User
	if err := config.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
		return
	}
	if user.Role != 3 || user.AdminId == nil || *user.AdminId != currentUserID {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không thuộc phạm quyền của bạn"})
		return
	}

	// Xác định thời gian
	currentMonth := time.Now().Format("2006-01")
	startOfMonth, _ := time.Parse("2006-01-02", currentMonth+"-01")
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	// Lấy dữ liệu điểm danh
	startDate := currentMonth + "-01"
	endDate := startOfNextMonth.Add(-time.Hour * 24).Format("2006-01-02")

	checkIns, err := GetCheckedInUsers(startDate, endDate, []models.User{user})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy dữ liệu điểm danh"})
		return
	}

	totalDays := startOfNextMonth.Add(-time.Hour * 24).Day()
	attendanceCount := len(checkIns)
	absenceCount := totalDays - attendanceCount
	amount := int(math.Round(float64(user.Amount)/float64(totalDays)*float64(attendanceCount)/1000) * 1000)

	// Kiểm tra hoặc cập nhật lương
	var userSalary models.UserSalary
	if err := config.DB.Where("user_id = ? AND salary_date >= ? AND salary_date < ?", user.ID, startOfMonth, startOfNextMonth).First(&userSalary).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			userSalary = models.UserSalary{
				UserID:     user.ID,
				Amount:     amount,
				Attendance: attendanceCount,
				Absence:    absenceCount,
				SalaryDate: time.Now(),
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi kiểm tra lương người dùng"})
			return
		}
	} else {
		userSalary.Amount = amount
		userSalary.Attendance = attendanceCount
		userSalary.Absence = absenceCount
		userSalary.SalaryDate = time.Now()
	}

	if err := config.DB.Save(&userSalary).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi cập nhật lương"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Tính lương thành công",
		"data": gin.H{
			"id":         userSalary.ID,
			"userId":     user.ID,
			"amount":     amount,
			"attendance": attendanceCount,
			"absence":    absenceCount,
			"date":       currentMonth,
			"totalDays":  totalDays,
		},
	})
}

func CalculateUserSalary(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Authorization header is missing"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, err := GetIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Invalid token"})
		return
	}

	var req struct {
		SalaryID uint `json:"salaryId"`
		UserID   uint `json:"userId"`
		Bonus    int  `json:"bonus"`
		Penalty  int  `json:"penalty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ"})
		return
	}

	var user models.User
	if err := config.DB.Preload("Banks").First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
		return
	}

	var banks []Bank
	for _, bank := range user.Banks {
		banks = append(banks, Bank{
			BankName:      bank.BankName,
			AccountNumber: bank.AccountNumber,
			BankShortName: bank.BankShortName,
		})
	}

	if user.Role != 3 || user.AdminId == nil || *user.AdminId != currentUserID {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không thuộc phạm quyền của bạn"})
		return
	}

	baseSalary := user.Amount
	totalSalary := int(baseSalary) + req.Bonus - req.Penalty

	// Tìm hoặc tạo bản ghi usersalary
	var userSalary models.UserSalary
	if err := config.DB.Where("id = ?", req.SalaryID).First(&userSalary).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Bản ghi lương không tồn tại"})
		return
	}

	// Cập nhật thông tin lương
	userSalary.TotalSalary = int(totalSalary)
	userSalary.Bonus = req.Bonus
	userSalary.Penalty = req.Penalty

	if err := config.DB.Save(&userSalary).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lưu thông tin lương"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Tính lương thành công",
		"data": gin.H{
			"userId":      user.ID,
			"baseSalary":  user.Amount,
			"bonus":       req.Bonus,
			"penalty":     req.Penalty,
			"totalSalary": totalSalary,
			"bank":        banks,
		},
	})
}

func UpdateSalaryStatus(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Authorization header is missing"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, err := GetIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Invalid token"})
		return
	}

	var req struct {
		SalaryID uint `json:"salaryId"`
		Status   bool `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu không hợp lệ"})
		return
	}

	var userSalary models.UserSalary
	if err := config.DB.First(&userSalary, req.SalaryID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Bản ghi lương không tồn tại"})
		return
	}

	var user models.User
	if err := config.DB.First(&user, userSalary.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
		return
	}

	if user.AdminId == nil || *user.AdminId != currentUserID {
		c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Bạn không có quyền cập nhật trạng thái lương"})
		return
	}

	// Cập nhật trạng thái
	userSalary.Status = req.Status
	if err := config.DB.Save(&userSalary).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi cập nhật trạng thái"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Cập nhật trạng thái thành công",
		"data": gin.H{
			"salaryId": req.SalaryID,
			"status":   req.Status,
		},
	})
}

func GetUserCheckin(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Authorization header is missing"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, err := GetIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Invalid token"})
		return
	}

	// Kết nối Redis và tạo cacheKey
	rdb, err := config.ConnectRedis()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể kết nối Redis"})
	}
	cacheKey := fmt.Sprintf("user_checkin:%d", currentUserID)

	var response []gin.H
	if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &response); err == nil && len(response) > 0 {
		// Nếu không có dữ liệu cache, truy vấn DB
		var user models.User
		if err := config.DB.First(&user, currentUserID).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
			return
		}
	}

	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("01/2006")
	}

	parsedDate, err := time.Parse("01/2006", date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Định dạng date không hợp lệ"})
		return
	}
	year, month, _ := parsedDate.Date()
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()

	users, err := GetUsersByAdminID(currentUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy danh sách user"})
		return
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].UpdatedAt.After(users[j].UpdatedAt)
	})

	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	endDate := fmt.Sprintf("%04d-%02d-%02d", year, month, daysInMonth)

	checkedInUsers, err := GetCheckedInUsers(startDate, endDate, users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy dữ liệu điểm danh", "err": err})
		return
	}

	for _, u := range users {
		checkinCount := 0
		var checkinDates []time.Time
		for _, ci := range checkedInUsers {
			if ci.UserID == u.ID {
				checkinCount++
				checkinDates = append(checkinDates, ci.Date)
			}
		}
		notCheckedInDays := daysInMonth - checkinCount

		response = append(response, gin.H{
			"id":               u.ID,
			"name":             u.Name,
			"phoneNumber":      u.PhoneNumber,
			"amount":           u.Amount,
			"checkinCount":     checkinCount,
			"notCheckedInDays": notCheckedInDays,
			"checkinDates":     checkinDates,
		})
	}

	// Lưu cache vào Redis
	if err := services.SetToRedis(config.Ctx, rdb, cacheKey, response, 30*time.Minute); err != nil {
		log.Printf("Lỗi khi lưu dữ liệu điểm danh vào Redis: %v", err)
	}

	nameFilter := c.Query("name")
	phoneFilter := c.Query("phone")
	var filteredResponse []gin.H
	for _, r := range response {
		nameVal, _ := r["name"].(string)
		phoneVal, _ := r["phoneNumber"].(string)
		if (nameFilter == "" || strings.Contains(strings.ToLower(nameVal), strings.ToLower(nameFilter))) &&
			(phoneFilter == "" || strings.Contains(phoneVal, phoneFilter)) {
			filteredResponse = append(filteredResponse, r)
		}
	}

	total := len(filteredResponse)

	page := 0
	limit := 10
	if pageStr := c.Query("page"); pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage >= 0 {
			page = parsedPage
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	startIdx := page * limit
	endIdx := startIdx + limit
	if startIdx >= len(filteredResponse) {
		filteredResponse = []gin.H{}
	} else if endIdx > len(filteredResponse) {
		filteredResponse = filteredResponse[startIdx:]
	} else {
		filteredResponse = filteredResponse[startIdx:endIdx]
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Tính lương thành công",
		"data": filteredResponse,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func GetUserSalary(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Authorization header is missing"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, err := GetIDFromToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "mess": "Invalid token"})
		return
	}

	var user models.User
	if err := config.DB.First(&user, currentUserID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "mess": "Người dùng không tồn tại"})
		return
	}

	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("01/2006")
	}

	parsedDate, err := time.Parse("01/2006", date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Định dạng date không hợp lệ"})
		return
	}
	year, month, _ := parsedDate.Date()
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()

	users, err := GetUsersByAdminID(currentUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy danh sách user"})
		return
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].UpdatedAt.After(users[j].UpdatedAt)
	})

	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	endDate := fmt.Sprintf("%04d-%02d-%02d", year, month, daysInMonth)

	userSalaries, err := GetUserSalaries(startDate, endDate, users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Lỗi khi lấy dữ liệu điểm danh", "err": err})
		return
	}

	userMap := make(map[uint]models.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	var response []gin.H
	for _, s := range userSalaries {

		userInfo, ok := userMap[s.UserID]
		if !ok {
			continue
		}

		response = append(response, gin.H{
			"id":          s.UserID,
			"amount":      userInfo.Amount,
			"name":        userInfo.Name,
			"phoneNumber": userInfo.PhoneNumber,
			"totalSalary": s.TotalSalary,
			"bonus":       s.Bonus,
			"penalty":     s.Penalty,
			"status":      s.Status,
			"code":        s.ID,
		})
	}

	nameFilter := c.Query("name")
	phoneFilter := c.Query("phone")
	var filteredResponse []gin.H
	for _, r := range response {
		nameVal, _ := r["name"].(string)
		phoneVal, _ := r["phoneNumber"].(string)
		if (nameFilter == "" || strings.Contains(strings.ToLower(nameVal), strings.ToLower(nameFilter))) &&
			(phoneFilter == "" || strings.Contains(phoneVal, phoneFilter)) {
			filteredResponse = append(filteredResponse, r)
		}
	}

	total := len(filteredResponse)

	page := 0
	limit := 10
	if pageStr := c.Query("page"); pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage >= 0 {
			page = parsedPage
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	startIdx := page * limit
	endIdx := startIdx + limit
	if startIdx >= len(filteredResponse) {
		filteredResponse = []gin.H{}
	} else if endIdx > len(filteredResponse) {
		filteredResponse = filteredResponse[startIdx:]
	} else {
		filteredResponse = filteredResponse[startIdx:endIdx]
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Tính lương thành công",
		"data": filteredResponse,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}
