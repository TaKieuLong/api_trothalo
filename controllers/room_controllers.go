package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"new/config"
	"new/models"
	"new/services"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

type Request struct {
	RoomId       uint            `json:"id"`
	RoomName     string          `json:"roomName"`
	Type         uint            `json:"type"`
	NumBed       int             `json:"numBed"`
	NumTolet     int             `json:"numTolet"`
	Acreage      int             `json:"acreage"`
	Price        int             `json:"price"`
	DaysPrice    json.RawMessage `json:"daysPrice"`
	HolidayPrice json.RawMessage `json:"holidayPrice"`
	Description  string          `json:"description"`
	Status       int             `json:"status"`
	Avatar       string          `json:"avatar"`
	Img          json.RawMessage `json:"img"`
	Num          int             `json:"num"`
	Furniture    json.RawMessage `json:"furniture" gorm:"type:json"`
	People       int             `json:"people"`
}

type DayPrice struct {
	Day   string `json:"day"`
	Price int    `json:"price"`
}

type RoomResponse struct {
	RoomId    uint      `json:"id"`
	RoomName  string    `json:"roomName"`
	Type      uint      `json:"type"`
	NumBed    int       `json:"numBed"`
	NumTolet  int       `json:"numTolet"`
	Acreage   int       `json:"acreage"`
	Price     int       `json:"price"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Status    int       `json:"status"`
	Avatar    string    `json:"avatar"`
	People    int       `json:"people"`
	Parents   Parents   `json:"parents"`
}

type Parents struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type RoomDetail struct {
	RoomId      uint            `json:"id" gorm:"primaryKey"`
	RoomName    string          `json:"roomName"`
	Type        uint            `json:"type"`
	NumBed      int             `json:"numBed"`
	NumTolet    int             `json:"numTolet"`
	Acreage     int             `json:"acreage"`
	Price       int             `json:"price"`
	Description string          `json:"description"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Status      int             `json:"status"`
	Avatar      string          `json:"avatar"`
	Img         json.RawMessage `json:"img" gorm:"type:json"`
	Num         int             `json:"num"`
	Furniture   json.RawMessage `json:"furniture" gorm:"type:json"`
	People      int             `json:"people"`
	Parent      Parents         `json:"parent"`
}

var CacheKey2 = "accommodations:all"

func getRoomStatuses(roomID uint, fromDate, toDate time.Time) ([]models.RoomStatus, error) {
	var statuses []models.RoomStatus
	err := config.DB.Where("room_id = ? AND status != 0 AND from_date <= ? AND to_date >= ?", roomID, toDate, fromDate).
		Find(&statuses).Error
	if err != nil {
		return nil, err
	}
	return statuses, nil
}

func GetRoomBookingDates(c *gin.Context) {
	roomID := c.DefaultQuery("id", "")
	date := c.DefaultQuery("date", "")

	if roomID == "" || date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id và date là bắt buộc"})
		return
	}

	layout := "01/2006"
	parsedDate, err := time.Parse(layout, date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ngày không hợp lệ, vui lòng sử dụng định dạng mm/yyyy"})
		return
	}

	// Tính toán ngày đầu tháng và ngày cuối tháng
	firstDay := time.Date(parsedDate.Year(), parsedDate.Month(), 1, 0, 0, 0, 0, time.Local)
	lastDay := firstDay.AddDate(0, 1, -1)

	// Tạo danh sách tất cả các ngày trong tháng
	var allDates []time.Time
	for day := firstDay; day.Before(lastDay.AddDate(0, 0, 1)); day = day.AddDate(0, 0, 1) {
		allDates = append(allDates, day)
	}

	// Lấy trạng thái phòng trong tháng yêu cầu
	var statuses []models.RoomStatus
	db := config.DB

	err = db.Where("room_id = ?", roomID).Find(&statuses).Error
	if err != nil {
		log.Printf("Error retrieving room statuses: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi khi lấy thông tin trạng thái phòng"})
		return
	}

	// Tạo mảng để chứa thông tin trạng thái từng ngày
	var roomResponses []map[string]interface{}

	// Duyệt qua tất cả các ngày trong tháng
	for _, currentDate := range allDates {
		var statusFound bool
		for _, status := range statuses {
			// Kiểm tra nếu ngày hiện tại nằm trong khoảng FromDate và ToDate
			if status.FromDate.Year() == currentDate.Year() && status.FromDate.Month() == currentDate.Month() &&
				currentDate.After(status.FromDate.AddDate(0, 0, -1)) && currentDate.Before(status.ToDate.AddDate(0, 0, 1)) {

				roomResponses = append(roomResponses, map[string]interface{}{
					"date":   currentDate.Format("02/01/2006"),
					"status": status.Status, // Trạng thái từ DB
				})
				statusFound = true
				break
			}
		}

		// Nếu không có trạng thái cho ngày này, thêm trạng thái là 0
		if !statusFound {
			roomResponses = append(roomResponses, map[string]interface{}{
				"date":   currentDate.Format("02/01/2006"),
				"status": 0, // Trạng thái là 0 khi không có booking
			})
		}
	}

	// Trả về kết quả
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy danh sách phòng thành công",
		"data": roomResponses,
	})
}

func GetAllRooms(c *gin.Context) {
	// Xác thực token
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

	// Lấy các tham số filter
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

	typeFilter := c.Query("type")
	statusFilter := c.Query("status")
	nameFilter := c.Query("name")
	accommodationFilter := c.Query("accommodation")
	numBedFilter := c.Query("numBed")
	numToletFilter := c.Query("numTolet")
	peopleFilter := c.Query("people")

	// Tạo cache key động
	var cacheKey string
	if currentUserRole == 2 {
		cacheKey = fmt.Sprintf("rooms:admin:%d", currentUserID)
	} else if currentUserRole == 3 {
		cacheKey = fmt.Sprintf("rooms:receptionist:%d", currentUserID)
	} else {
		cacheKey = "rooms:all"
	}

	// Kết nối Redis
	rdb, err := config.ConnectRedis()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể kết nối Redis"})
		return
	}

	var allRooms []models.Room

	// Lấy dữ liệu từ Redis
	if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &allRooms); err != nil || len(allRooms) == 0 {
		tx := config.DB.Model(&models.Room{}).Preload("Parent")

		if currentUserRole == 2 {
			// Lấy phòng theo admin
			tx = tx.Joins("JOIN accommodations ON accommodations.id = rooms.accommodation_id").Where("accommodations.user_id = ?", currentUserID)
		} else if currentUserRole == 3 {
			// Lấy phòng theo admin (vị trí receptionist)
			var adminID int
			if err := config.DB.Model(&models.User{}).Select("admin_id").Where("id = ?", currentUserID).Scan(&adminID).Error; err != nil || adminID == 0 {
				c.JSON(http.StatusForbidden, gin.H{"code": 0, "mess": "Không có quyền truy cập"})
				return
			}
			tx = tx.Joins("JOIN accommodations ON accommodations.id = rooms.accommodation_id").Where("accommodations.user_id = ?", adminID)
		}

		if err := tx.Find(&allRooms).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lấy danh sách phòng"})
			return
		}

		var roomDetails []RoomDetail
		for _, room := range allRooms {
			roomDetails = append(roomDetails, RoomDetail{
				RoomId:      room.RoomId,
				RoomName:    room.RoomName,
				Type:        room.Type,
				NumBed:      room.NumBed,
				NumTolet:    room.NumTolet,
				Acreage:     room.Acreage,
				Price:       room.Price,
				Description: room.Description,

				CreatedAt: room.CreatedAt,
				UpdatedAt: room.UpdatedAt,
				Status:    room.Status,
				Avatar:    room.Avatar,
				Img:       room.Img,
				Num:       room.Num,
				Furniture: room.Furniture,
				People:    room.People,
				Parent: Parents{
					Id:   room.Parent.ID,
					Name: room.Parent.Name,
				},
			})
		}

		// Lưu dữ liệu ép kiểu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, roomDetails, 10*time.Minute); err != nil {
			log.Printf("Lỗi khi lưu danh sách phòng vào Redis: %v", err)
		}
	}

	// Áp dụng filter trên dữ liệu từ Redis
	filteredRooms := make([]models.Room, 0)
	for _, room := range allRooms {
		if typeFilter != "" {
			parsedTypeFilter, err := strconv.Atoi(typeFilter)
			if err == nil && room.Type != uint(parsedTypeFilter) {
				continue
			}
		}
		if statusFilter != "" {
			parsedStatus, _ := strconv.Atoi(statusFilter)
			if room.Status != parsedStatus {
				continue
			}
		}
		if nameFilter != "" {
			decodedNameFilter, _ := url.QueryUnescape(nameFilter)
			if !strings.Contains(strings.ToLower(room.RoomName), strings.ToLower(decodedNameFilter)) {
				continue
			}
		}
		if accommodationFilter != "" {
			decodedAccommodationFilter, _ := url.QueryUnescape(accommodationFilter)
			if !strings.Contains(strings.ToLower(room.Parent.Name), strings.ToLower(decodedAccommodationFilter)) {
				continue
			}
		}
		if numBedFilter != "" {
			numBed, _ := strconv.Atoi(numBedFilter)
			if room.NumBed != numBed {
				continue
			}
		}
		if numToletFilter != "" {
			numTolet, _ := strconv.Atoi(numToletFilter)
			if room.NumTolet != numTolet {
				continue
			}
		}
		if peopleFilter != "" {
			people, _ := strconv.Atoi(peopleFilter)
			if room.People != people {
				continue
			}
		}
		filteredRooms = append(filteredRooms, room)
	}

	// Tính toán tổng số phòng sau khi lọc
	total := len(filteredRooms)

	// Pagination
	start := page * limit
	end := start + limit
	if start >= total {
		filteredRooms = []models.Room{}
	} else if end > total {
		filteredRooms = filteredRooms[start:]
	} else {
		filteredRooms = filteredRooms[start:end]
	}

	// Chuẩn bị response
	roomResponses := make([]RoomResponse, 0)
	for _, room := range filteredRooms {
		roomResponses = append(roomResponses, RoomResponse{
			RoomId:    room.RoomId,
			RoomName:  room.RoomName,
			Type:      room.Type,
			NumBed:    room.NumBed,
			NumTolet:  room.NumTolet,
			Acreage:   room.Acreage,
			Price:     room.Price,
			CreatedAt: room.CreatedAt,
			UpdatedAt: room.UpdatedAt,
			Status:    room.Status,
			Avatar:    room.Avatar,
			People:    room.People,
			Parents: Parents{
				Id:   room.Parent.ID,
				Name: room.Parent.Name,
			},
		})
	}

	// Response với pagination và total count
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy danh sách phòng thành công",
		"data": roomResponses,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func GetAllRoomsUser(c *gin.Context) {
	var totalRooms int64

	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	typeFilter := c.Query("type")
	statusFilter := c.Query("status")
	nameFilter := c.Query("name")
	accommodationFilter := c.Query("accommodation")
	accommodationIdFilter := c.Query("accommodationId")
	numBedFilter := c.Query("numBed")
	numToletFilter := c.Query("numTolet")
	peopleFilter := c.Query("people")

	fromDateStr := c.Query("fromDate")
	toDateStr := c.Query("toDate")

	limit := 10
	page := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	var fromDate, toDate time.Time
	var err error

	if fromDateStr != "" {
		fromDate, err = time.Parse("02/01/2006", fromDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu fromDate không hợp lệ"})
			return
		}
	}

	if toDateStr != "" {
		toDate, err = time.Parse("02/01/2006", toDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu toDate không hợp lệ"})
			return
		}
	}

	offset := page * limit

	// Kết nối Redis
	rdb, err := config.ConnectRedis()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể kết nối Redis"})
		return
	}

	cacheKey := "rooms:all"
	var allRooms []models.Room

	// Lấy dữ liệu từ Redis
	if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &allRooms); err != nil || len(allRooms) == 0 {
		// Nếu Redis không có dữ liệu, lấy từ DB
		if err := config.DB.Model(&models.Room{}).Preload("Parent").Find(&allRooms).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lấy danh sách phòng từ database"})
			return
		}

		var allRoomsDetails []RoomDetail
		for _, room := range allRooms {
			roomDetail := RoomDetail{
				RoomId:   room.RoomId,
				RoomName: room.RoomName,
				Type:     room.Type,
				NumBed:   room.NumBed,
				NumTolet: room.NumTolet,
				Acreage:  room.Acreage,
				Price:    room.Price,

				CreatedAt: room.CreatedAt,
				UpdatedAt: room.UpdatedAt,
				Status:    room.Status,
				Avatar:    room.Avatar,
				People:    room.People,
				Parent: Parents{
					Id:   room.Parent.ID,
					Name: room.Parent.Name,
				},
				Img:       room.Img,
				Furniture: room.Furniture,
			}
			allRoomsDetails = append(allRoomsDetails, roomDetail)
		}

		// Lưu dữ liệu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, allRoomsDetails, 10*time.Minute); err != nil {
			log.Printf("Lỗi khi lưu danh sách phòng vào Redis: %v", err)
		}
	}

	// Áp dụng filter trên dữ liệu từ Redis
	filteredRooms := make([]models.Room, 0)
	for _, room := range allRooms {
		if typeFilter != "" {
			parsedTypeFilter, err := strconv.Atoi(typeFilter)
			if err == nil && room.Type != uint(parsedTypeFilter) {
				continue
			}
		}

		if statusFilter != "" {
			parsedStatus, _ := strconv.Atoi(statusFilter)
			if room.Status != parsedStatus {
				continue
			}
		}
		if nameFilter != "" {
			decodedNameFilter, _ := url.QueryUnescape(nameFilter)
			if !strings.Contains(strings.ToLower(room.RoomName), strings.ToLower(decodedNameFilter)) {
				continue
			}
		}
		if accommodationFilter != "" {
			decodedAccommodationFilter, _ := url.QueryUnescape(accommodationFilter)
			if !strings.Contains(strings.ToLower(room.Parent.Name), strings.ToLower(decodedAccommodationFilter)) {
				continue
			}
		}
		if accommodationIdFilter != "" {
			parsedAccommodationId, err := strconv.Atoi(accommodationIdFilter)
			if err == nil {
				if room.AccommodationID != uint(parsedAccommodationId) && room.Parent.ID != uint(parsedAccommodationId) {
					continue
				}
			}
		}
		if numBedFilter != "" {
			numBed, _ := strconv.Atoi(numBedFilter)
			if room.NumBed != numBed {
				continue
			}
		}
		if numToletFilter != "" {
			numTolet, _ := strconv.Atoi(numToletFilter)
			if room.NumTolet != numTolet {
				continue
			}
		}
		if peopleFilter != "" {
			people, _ := strconv.Atoi(peopleFilter)
			if room.People != people {
				continue
			}
		}

		statuses, err := getRoomStatuses(room.RoomId, fromDate, toDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể lấy trạng thái của accommodation"})
			return
		}

		if len(statuses) > 0 {
			continue
		}

		filteredRooms = append(filteredRooms, room)
	}

	// Đếm tổng số phòng sau khi lọc
	totalRooms = int64(len(filteredRooms))

	// Phân trang
	startIndex := offset
	endIndex := offset + limit
	if startIndex > len(filteredRooms) {
		startIndex = len(filteredRooms)
	}
	if endIndex > len(filteredRooms) {
		endIndex = len(filteredRooms)
	}
	paginatedRooms := filteredRooms[startIndex:endIndex]

	roomResponses := make([]RoomResponse, 0)
	for _, room := range paginatedRooms {
		roomResponses = append(roomResponses, RoomResponse{
			RoomId:    room.RoomId,
			RoomName:  room.RoomName,
			Type:      room.Type,
			NumBed:    room.NumBed,
			NumTolet:  room.NumTolet,
			Acreage:   room.Acreage,
			Price:     room.Price,
			CreatedAt: room.CreatedAt,
			UpdatedAt: room.UpdatedAt,
			Status:    room.Status,
			Avatar:    room.Avatar,
			People:    room.People,
			Parents: Parents{
				Id:   room.Parent.ID,
				Name: room.Parent.Name,
			},
		})
	}

	// Trả về kết quả
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy danh sách phòng thành công",
		"data": roomResponses,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": totalRooms,
		},
	})
}

func CreateRoom(c *gin.Context) {
	var newRoom models.Room
	// Xác thực token
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
	if err := c.ShouldBindJSON(&newRoom); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu đầu vào không hợp lệ", "details": err.Error()})
		return
	}

	if err := newRoom.ValidateStatus(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": err.Error()})
		return
	}

	furnitureJSON, err := json.Marshal(newRoom.Furniture)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể mã hóa holidayPrice", "details": err.Error()})
		return
	}

	imgJSON, err := json.Marshal(newRoom.Img)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể mã hóa img", "details": err.Error()})
		return
	}
	var accommodation models.Accommodation
	if err := config.DB.First(&accommodation, newRoom.AccommodationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 0, "message": "Không tìm thấy cơ sở lưu trú!"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "message": "Lỗi server", "details": err.Error()})
		return
	}
	newRoom.Parent = accommodation
	newRoom.Img = imgJSON
	newRoom.Furniture = furnitureJSON

	if err := config.DB.Create(&newRoom).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể tạo phòng", "details": err.Error()})
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		switch currentUserRole {
		case 1: // Super Admin
			_ = services.DeleteFromRedis(config.Ctx, rdb, "rooms:all")
			_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
		case 2: // Admin
			adminCacheKey := fmt.Sprintf("rooms:admin:%d", currentUserID)
			_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
			_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
			var receptionistIDs []int
			if err := config.DB.Model(&models.User{}).Where("admin_id = ?", currentUserID).Pluck("id", &receptionistIDs).Error; err == nil {
				for _, receptionistID := range receptionistIDs {
					receptionistCacheKey := fmt.Sprintf("rooms:receptionist:%d", receptionistID)
					_ = services.DeleteFromRedis(config.Ctx, rdb, receptionistCacheKey)
					_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
				}
			}
		case 3: // Receptionist
			var adminID int
			if err := config.DB.Model(&models.User{}).Select("admin_id").Where("id = ?", currentUserID).Scan(&adminID).Error; err == nil {
				adminCacheKey := fmt.Sprintf("rooms:admin:%d", adminID)
				receptionistCacheKey := fmt.Sprintf("rooms:receptionist:%d", currentUserID)
				_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
				_ = services.DeleteFromRedis(config.Ctx, rdb, receptionistCacheKey)
				_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 1, "mess": "Tạo phòng thành công", "data": newRoom})
}

func GetRoomDetail(c *gin.Context) {
	roomId := c.Param("id")

	// Kết nối Redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể kết nối Redis"})
		return
	}

	// Key cache cho tất cả rooms
	cacheKey := "rooms:all"

	// Lấy danh sách rooms từ cache
	var cachedRooms []models.Room
	if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &cachedRooms); err == nil {
		// Tìm room theo ID trong cache
		for _, room := range cachedRooms {
			if fmt.Sprintf("%d", room.RoomId) == roomId {
				// Tạo response từ cache
				c.JSON(http.StatusOK, gin.H{
					"code": 1,
					"mess": "Lấy thông tin phòng thành công (từ cache)",
					"data": buildRoomDetailResponse(room),
				})
				return
			}
		}
	}

	// Nếu không tìm thấy trong cache, truy vấn từ database
	var room models.Room
	if err := config.DB.Preload("Parent").First(&room, roomId).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Phòng không tồn tại"})
		return
	}

	// Trả về kết quả từ database
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"mess": "Lấy thông tin phòng thành công",
		"data": buildRoomDetailResponse(room),
	})
}

func UpdateRoom(c *gin.Context) {
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

	var request Request

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu đầu vào không hợp lệ", "details": err.Error()})
		return
	}

	var room models.Room

	if err := config.DB.First(&room, request.RoomId).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Phòng không tồn tại"})
		return
	}

	if err := room.ValidateStatus(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": err.Error()})
		return
	}

	imgJSON, err := json.Marshal(request.Img)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể mã hóa img", "details": err.Error()})
		return
	}

	furnitureJSON, err := json.Marshal(request.Furniture)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể mã hóa holidayPrice", "details": err.Error()})
		return
	}

	if request.RoomName != "" {
		room.RoomName = request.RoomName
	}

	if request.Type > 0 {
		room.Type = request.Type
	}

	if request.NumBed != 0 {
		room.NumBed = request.NumBed
	}

	if request.NumTolet != 0 {
		room.NumTolet = request.NumTolet
	}

	if request.Acreage != 0 {
		room.Acreage = request.Acreage
	}

	if request.Price != 0 {
		room.Price = request.Price
	}

	if request.Description != "" {
		room.Description = request.Description
	}

	if request.Status != 0 {
		room.Status = request.Status
	}

	if request.Avatar != "" {
		room.Avatar = request.Avatar
	}

	if len(request.Img) > 0 {
		room.Img = imgJSON
	}

	if len(request.Furniture) > 0 {
		room.Furniture = furnitureJSON
	}

	if err := config.DB.Save(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể cập nhật phòng", "details": err.Error()})
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		switch currentUserRole {
		case 1: // Super Admin
			_ = services.DeleteFromRedis(config.Ctx, rdb, "rooms:all")
			_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
		case 2: // Admin
			adminCacheKey := fmt.Sprintf("rooms:admin:%d", currentUserID)
			_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
			_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
			_ = services.DeleteFromRedis(config.Ctx, rdb, "rooms:all")
			var receptionistIDs []int
			if err := config.DB.Model(&models.User{}).Where("admin_id = ?", currentUserID).Pluck("id", &receptionistIDs).Error; err == nil {
				for _, receptionistID := range receptionistIDs {
					receptionistCacheKey := fmt.Sprintf("rooms:receptionist:%d", receptionistID)
					_ = services.DeleteFromRedis(config.Ctx, rdb, receptionistCacheKey)
				}
			}
		case 3: // Receptionist
			var adminID int
			if err := config.DB.Model(&models.User{}).Select("admin_id").Where("id = ?", currentUserID).Scan(&adminID).Error; err == nil {
				adminCacheKey := fmt.Sprintf("rooms:admin:%d", adminID)
				_ = services.DeleteFromRedis(config.Ctx, rdb, "rooms:all")
				receptionistCacheKey := fmt.Sprintf("rooms:receptionist:%d", currentUserID)
				_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
				_ = services.DeleteFromRedis(config.Ctx, rdb, receptionistCacheKey)
				_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 1, "mess": "Cập nhật phòng thành công", "data": room})
}

func ChangeRoomStatus(c *gin.Context) {
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

	var input struct {
		RoomId uint `json:"id"`
		Status int  `json:"status"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 0, "mess": "Dữ liệu đầu vào không hợp lệ"})
		return
	}

	var room models.Room

	if err := config.DB.First(&room, input.RoomId).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 0, "mess": "Phòng không tồn tại"})
		return
	}

	room.Status = input.Status
	if err := config.DB.Save(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "mess": "Không thể thay đổi trạng thái phòng"})
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		switch currentUserRole {
		case 1: // Super Admin
			_ = services.DeleteFromRedis(config.Ctx, rdb, "rooms:all")
		case 2: // Admin
			adminCacheKey := fmt.Sprintf("rooms:admin:%d", currentUserID)
			_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
			_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
			_ = services.DeleteFromRedis(config.Ctx, rdb, "rooms:all")
			var receptionistIDs []int
			if err := config.DB.Model(&models.User{}).Where("admin_id = ?", currentUserID).Pluck("id", &receptionistIDs).Error; err == nil {
				for _, receptionistID := range receptionistIDs {
					receptionistCacheKey := fmt.Sprintf("rooms:receptionist:%d", receptionistID)
					_ = services.DeleteFromRedis(config.Ctx, rdb, receptionistCacheKey)
				}
			}
		case 3: // Receptionist
			var adminID int
			if err := config.DB.Model(&models.User{}).Select("admin_id").Where("id = ?", currentUserID).Scan(&adminID).Error; err == nil {
				adminCacheKey := fmt.Sprintf("rooms:admin:%d", adminID)
				receptionistCacheKey := fmt.Sprintf("rooms:receptionist:%d", currentUserID)
				_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
				_ = services.DeleteFromRedis(config.Ctx, rdb, receptionistCacheKey)
				_ = services.DeleteFromRedis(config.Ctx, rdb, CacheKey2)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 1, "mess": "Thay đổi trạng thái phòng thành công", "data": room})
}

// Hàm set response cho details
func buildRoomDetailResponse(room models.Room) RoomDetail {
	return RoomDetail{
		RoomId:      room.RoomId,
		RoomName:    room.RoomName,
		Type:        room.Type,
		NumBed:      room.NumBed,
		NumTolet:    room.NumTolet,
		Acreage:     room.Acreage,
		Price:       room.Price,
		Description: room.Description,
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
		Status:      room.Status,
		Avatar:      room.Avatar,
		Img:         room.Img,
		Num:         room.Num,
		Furniture:   room.Furniture,
		People:      room.People,
		Parent: Parents{
			Id:   room.Parent.ID,
			Name: room.Parent.Name,
		},
	}
}
