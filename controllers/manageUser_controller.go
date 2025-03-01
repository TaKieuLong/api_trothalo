package controllers

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"new/config"
	"new/services"
	"strings"
	"time"

	"new/models"

	"github.com/gin-gonic/gin"
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
			"userId": user.ID,
			"amount": user.Amount,
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
