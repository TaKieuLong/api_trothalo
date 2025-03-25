package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"new/config"
	"new/dto"
	"new/models"
	"new/response"
	"new/services"
	"new/types"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func convertToOrderAccommodationResponse(accommodation models.Accommodation) dto.OrderAccommodationResponse {
	return dto.OrderAccommodationResponse{
		ID:       accommodation.ID,
		Type:     accommodation.Type,
		Name:     accommodation.Name,
		Address:  accommodation.Address,
		Ward:     accommodation.Ward,
		District: accommodation.District,
		Province: accommodation.Province,
		Price:    accommodation.Price,
		Avatar:   accommodation.Avatar,
	}
}

func convertToOrderRoomResponse(room models.Room) dto.OrderRoomResponse {
	return dto.OrderRoomResponse{
		ID:              room.RoomId,
		AccommodationID: room.AccommodationID,
		RoomName:        room.RoomName,
		Price:           room.Price,
	}
}

// Chuyển chuỗi ngày string thành dạng timestamp
func ConvertDateToISOFormat(dateStr string) (time.Time, error) {
	parsedDate, err := time.Parse("02/01/2006", dateStr)
	if err != nil {
		return time.Time{}, err
	}
	return parsedDate, nil
}

func GetOrders(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	var orders []models.Order
	query := config.DB.Preload("User").Preload("Accommodation")

	if currentUserRole == 1 {
		query = query.Where("user_id = ?", currentUserID)
	}

	if err := query.Find(&orders).Error; err != nil {
		response.ServerError(c)
		return
	}

	var orderResponses []dto.OrderResponse
	for _, order := range orders {
		orderResponses = append(orderResponses, dto.OrderResponse{
			ID:              order.ID,
			UserID:          *order.UserID,
			AccommodationID: order.AccommodationID,
			TotalPrice:      order.TotalPrice,
			Status:          order.Status,
			CreatedAt:       order.CreatedAt.Format(time.RFC3339),
			UpdatedAt:       order.UpdatedAt.Format(time.RFC3339),
			User: types.InvoiceUserResponse{
				ID:          order.User.ID,
				Name:        order.User.Name,
				Email:       order.User.Email,
				PhoneNumber: order.User.PhoneNumber,
			},
			Accommodation: dto.AccommodationResponse{
				ID:      order.Accommodation.ID,
				Name:    order.Accommodation.Name,
				Address: order.Accommodation.Address,
				Price:   order.Accommodation.Price,
			},
		})
	}

	response.Success(c, orderResponses)
}

func CreateOrder(c *gin.Context) {
	var request dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 1 {
		response.Forbidden(c)
		return
	}

	var accommodation models.Accommodation
	if err := config.DB.First(&accommodation, request.AccommodationID).Error; err != nil {
		response.NotFound(c)
		return
	}

	order := models.Order{
		UserID:          &currentUserID,
		AccommodationID: request.AccommodationID,
		Status:          0,
		TotalPrice:      float64(accommodation.Price),
		Price:           accommodation.Price,
	}

	if err := config.DB.Create(&order).Error; err != nil {
		response.ServerError(c)
		return
	}

	var user models.User
	if err := config.DB.First(&user, currentUserID).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, dto.OrderResponse{
		ID:              order.ID,
		UserID:          *order.UserID,
		AccommodationID: order.AccommodationID,
		TotalPrice:      order.TotalPrice,
		Status:          order.Status,
		CreatedAt:       order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       order.UpdatedAt.Format(time.RFC3339),
		User: types.InvoiceUserResponse{
			ID:          user.ID,
			Name:        user.Name,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
		},
		Accommodation: dto.AccommodationResponse{
			ID:      order.Accommodation.ID,
			Name:    order.Accommodation.Name,
			Address: order.Accommodation.Address,
			Price:   order.Accommodation.Price,
		},
	})
}

func GetOrderByID(c *gin.Context) {
	id := c.Param("id")

	var order models.Order
	if err := config.DB.First(&order, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	var user models.User
	if order.UserID != nil {
		if err := config.DB.First(&user, order.UserID).Error; err != nil {
			response.NotFound(c)
			return
		}
	}

	var accommodation models.Accommodation
	if err := config.DB.First(&accommodation, order.AccommodationID).Error; err != nil {
		response.NotFound(c)
		return
	}

	var rooms []models.Room
	if err := config.DB.Where("room_id IN ?", order.RoomID).Find(&rooms).Error; err != nil {
		response.NotFound(c)
		return
	}

	var roomResponses []dto.OrderRoomResponse
	for _, room := range rooms {
		roomResponses = append(roomResponses, convertToOrderRoomResponse(room))
	}

	orderResponse := dto.OrderUserResponse{
		ID:               order.ID,
		User:             dto.ActorResponse{Name: user.Name, Email: user.Email, PhoneNumber: user.PhoneNumber},
		Accommodation:    convertToOrderAccommodationResponse(accommodation),
		Room:             roomResponses,
		CheckInDate:      order.CheckInDate,
		CheckOutDate:     order.CheckOutDate,
		Status:           order.Status,
		CreatedAt:        order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        order.UpdatedAt.Format(time.RFC3339),
		Price:            order.Price,
		HolidayPrice:     order.HolidayPrice,
		CheckInRushPrice: order.CheckInRushPrice,
		SoldOutPrice:     order.SoldOutPrice,
		DiscountPrice:    order.DiscountPrice,
		TotalPrice:       order.TotalPrice,
	}

	response.Success(c, orderResponse)
}

func UpdateOrder(c *gin.Context) {
	id := c.Param("id")

	var order models.Order
	if err := config.DB.First(&order, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	var request dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, "Lỗi khi ràng buộc dữ liệu: "+err.Error())
		return
	}

	// Kiểm tra phòng có tồn tại và còn trống không
	var rooms []models.Room
	if err := config.DB.Where("room_id IN ?", request.RoomID).Find(&rooms).Error; err != nil {
		response.ServerError(c)
		return
	}

	if len(rooms) != len(request.RoomID) {
		response.Error(c, http.StatusBadRequest, "Một số phòng không tồn tại")
		return
	}

	// Kiểm tra phòng có trống trong khoảng thời gian đặt không
	checkInDate, err := time.Parse("02/01/2006", request.CheckInDate)
	if err != nil {
		response.ValidationError(c, "Ngày nhận phòng không hợp lệ")
		return
	}

	checkOutDate, err := time.Parse("02/01/2006", request.CheckOutDate)
	if err != nil {
		response.ValidationError(c, "Ngày trả phòng không hợp lệ")
		return
	}

	for _, room := range rooms {
		var existingOrders []models.Order
		if err := config.DB.Where("room_id @> ? AND id != ?", []uint{room.RoomId}, order.ID).
			Where("(check_in_date <= ? AND check_out_date >= ?) OR (check_in_date <= ? AND check_out_date >= ?)",
				request.CheckOutDate, request.CheckInDate,
				request.CheckInDate, request.CheckInDate).
			Find(&existingOrders).Error; err != nil {
			response.ServerError(c)
			return
		}

		if len(existingOrders) > 0 {
			response.Error(c, http.StatusBadRequest, fmt.Sprintf("Phòng %s đã được đặt trong khoảng thời gian này", room.RoomName))
			return
		}
	}

	// Tính toán giá
	var accommodation models.Accommodation
	if err := config.DB.First(&accommodation, request.AccommodationID).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Tính số ngày ở
	days := int(checkOutDate.Sub(checkInDate).Hours() / 24)

	// Tính giá cơ bản
	basePrice := 0
	for _, room := range rooms {
		basePrice += room.Price * days
	}

	// Tính giá lễ
	holidayPrice := 0.0
	var holidays []models.Holiday
	if err := config.DB.Where("date BETWEEN ? AND ?", checkInDate, checkOutDate).Find(&holidays).Error; err != nil {
		response.ServerError(c)
		return
	}

	for range holidays {
		holidayPrice += float64(basePrice) * 0.1 // Tăng 10% vào ngày lễ
	}

	// Tính giá check-in gấp
	checkInRushPrice := 0.0
	if time.Now().Add(24 * time.Hour).After(checkInDate) {
		checkInRushPrice = float64(basePrice) * 0.05 // Tăng 5% nếu check-in gấp
	}

	// Tính giá sold out
	soldOutPrice := 0.0
	if len(rooms) == 1 {
		soldOutPrice = float64(basePrice) * 0.05 // Tăng 5% nếu chỉ còn 1 phòng
	}

	// Tính giá discount
	discountPrice := 0.0
	var discount models.Discount
	if err := config.DB.Where("status = ? AND start_date <= ? AND end_date >= ?", 1, checkInDate, checkOutDate).First(&discount).Error; err == nil {
		discountPrice = float64(basePrice) * float64(discount.Discount) / 100
	}

	// Tính tổng giá
	totalPrice := float64(basePrice) + holidayPrice + checkInRushPrice + soldOutPrice - discountPrice

	// Cập nhật order
	userID := request.UserID
	order.UserID = &userID
	order.AccommodationID = request.AccommodationID
	order.RoomID = request.RoomID
	order.CheckInDate = request.CheckInDate
	order.CheckOutDate = request.CheckOutDate
	order.GuestName = request.GuestName
	order.GuestEmail = request.GuestEmail
	order.GuestPhone = request.GuestPhone
	order.Price = basePrice
	order.HolidayPrice = holidayPrice
	order.CheckInRushPrice = checkInRushPrice
	order.SoldOutPrice = soldOutPrice
	order.DiscountPrice = discountPrice
	order.TotalPrice = totalPrice
	order.UpdatedAt = time.Now()

	if err := config.DB.Save(&order).Error; err != nil {
		response.ServerError(c)
		return
	}

	var user models.User
	if err := config.DB.First(&user, *order.UserID).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, dto.OrderResponse{
		ID:              order.ID,
		UserID:          *order.UserID,
		AccommodationID: order.AccommodationID,
		TotalPrice:      order.TotalPrice,
		Status:          order.Status,
		CreatedAt:       order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       order.UpdatedAt.Format(time.RFC3339),
		User: types.InvoiceUserResponse{
			ID:          user.ID,
			Name:        user.Name,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
		},
		Accommodation: dto.AccommodationResponse{
			ID:      order.Accommodation.ID,
			Name:    order.Accommodation.Name,
			Address: order.Accommodation.Address,
			Price:   order.Accommodation.Price,
		},
	})
}

func DeleteOrder(c *gin.Context) {
	id := c.Param("id")

	var order models.Order
	if err := config.DB.First(&order, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	if err := config.DB.Delete(&order).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{"message": "Xóa đơn hàng thành công"})
}

func ChangeOrderStatus(c *gin.Context) {
	var req dto.StatusUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	if _, _, err := services.GetUserIDFromToken(tokenString); err != nil {
		response.Unauthorized(c)
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("Accommodation.User").
		First(&order, req.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	if req.Status == 2 {
		if order.Status == 1 {
			var invoice models.Invoice
			if err := config.DB.Where("order_id = ?", order.ID).First(&invoice).Error; err == nil {
				// Xóa invoice
				if err := config.DB.Delete(&invoice).Error; err != nil {
					response.ServerError(c)
					return
				}

				today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)

				var userRevenue models.UserRevenue
				if err := config.DB.Where("user_id = ? AND date = ?", invoice.AdminID, today).First(&userRevenue).Error; err == nil {
					newRevenue := userRevenue.Revenue - invoice.TotalAmount
					newOrderCount := userRevenue.OrderCount - 1
					if newOrderCount < 0 {
						newOrderCount = 0
					}

					if err := config.DB.Model(&userRevenue).Updates(map[string]interface{}{
						"revenue":     newRevenue,
						"order_count": newOrderCount,
					}).Error; err != nil {
						response.ServerError(c)
						return
					}
				} else {
					response.NotFound(c)
					return
				}
			} else {
				response.NotFound(c)
				return
			}
		}

		if len(order.RoomID) > 0 {
			for _, room := range order.Room {
				var roomStatus models.RoomStatus
				if err := config.DB.Where("room_id = ? AND status = ?", room.RoomId, 1).First(&roomStatus).Error; err == nil {
					roomStatus.Status = 0
					if err := config.DB.Save(&roomStatus).Error; err != nil {
						response.ServerError(c)
						return
					}
				}
			}
		} else {
			var accommodationStatus models.AccommodationStatus
			if err := config.DB.Where("accommodation_id = ? AND status = ?", order.AccommodationID, 1).First(&accommodationStatus).Error; err == nil {
				accommodationStatus.Status = 0
				if err := config.DB.Save(&accommodationStatus).Error; err != nil {
					response.ServerError(c)
					return
				}
			}
		}
	}

	if req.Status == 1 {
		var existingInvoice models.Invoice
		if err := config.DB.Where("order_id = ?", order.ID).First(&existingInvoice).Error; err == nil {
			response.Error(c, http.StatusConflict, "Hóa đơn đã tồn tại cho đơn hàng này")
			return
		}

		var Remaining = order.TotalPrice - req.PaidAmount

		invoice := models.Invoice{
			OrderID:         order.ID,
			TotalAmount:     order.TotalPrice,
			PaidAmount:      req.PaidAmount,
			RemainingAmount: Remaining,
			AdminID:         order.Accommodation.UserID,
		}

		if err := config.DB.Create(&invoice).Error; err != nil {
			response.ServerError(c)
			return
		}

		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)

		var userRevenue models.UserRevenue
		if err := config.DB.Where("user_id = ? AND date = ?", invoice.AdminID, today).First(&userRevenue).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newUserRevenue := models.UserRevenue{
					UserID:     invoice.AdminID,
					Date:       today,
					Revenue:    invoice.TotalAmount,
					OrderCount: 1,
				}
				if err := config.DB.Create(&newUserRevenue).Error; err != nil {
					response.ServerError(c)
					return
				}
			} else {
				response.ServerError(c)
				return
			}
		} else {
			if err := config.DB.Model(&userRevenue).Updates(map[string]interface{}{
				"revenue":     userRevenue.Revenue + invoice.TotalAmount,
				"order_count": userRevenue.OrderCount + 1,
			}).Error; err != nil {
				response.ServerError(c)
				return
			}
		}
	}

	order.Status = req.Status
	order.UpdatedAt = time.Now()

	if err := config.DB.Save(&order).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Xóa cache
	rdb, err := config.ConnectRedis()
	if err != nil {
		response.ServerError(c)
		return
	}
	iter := rdb.Scan(context.Background(), 0, "orders:*", 0).Iterator()
	for iter.Next(context.Background()) {
		rdb.Del(context.Background(), iter.Val())
	}
	if err := iter.Err(); err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{"message": "Trạng thái đơn hàng đã được cập nhật"})
}

func GetOrderDetail(c *gin.Context) {
	orderId := c.Param("id")

	var order models.Order
	if err := config.DB.Preload("User").
		Preload("Accommodation").
		Preload("Room").
		Where("id = ?", orderId).
		First(&order).Error; err != nil {
		response.NotFound(c)
		return
	}

	var user dto.ActorResponse
	if order.UserID != nil {
		user = dto.ActorResponse{Name: order.User.Name, Email: order.User.Email, PhoneNumber: order.User.PhoneNumber}
	} else {
		user = dto.ActorResponse{Name: order.GuestName, Email: order.GuestEmail, PhoneNumber: order.GuestPhone}
	}

	accommodationResponse := convertToOrderAccommodationResponse(order.Accommodation)

	var roomResponses []dto.OrderRoomResponse
	for _, room := range order.Room {
		roomResponse := convertToOrderRoomResponse(room)
		roomResponses = append(roomResponses, roomResponse)
	}

	orderResponse := dto.OrderUserResponse{
		ID:               order.ID,
		User:             user,
		Accommodation:    accommodationResponse,
		Room:             roomResponses,
		CheckInDate:      order.CheckInDate,
		CheckOutDate:     order.CheckOutDate,
		Status:           order.Status,
		CreatedAt:        order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        order.UpdatedAt.Format(time.RFC3339),
		Price:            order.Price,
		HolidayPrice:     order.HolidayPrice,
		CheckInRushPrice: order.CheckInRushPrice,
		SoldOutPrice:     order.SoldOutPrice,
		DiscountPrice:    order.DiscountPrice,
		TotalPrice:       order.TotalPrice,
	}

	response.Success(c, orderResponse)
}

func GetOrdersByUserId(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, _, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	var user models.User
	if err := config.DB.First(&user, currentUserID).Error; err != nil {
		response.ServerError(c)
		return
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

	if user.PhoneNumber == "" {
		response.BadRequest(c, "Số điện thoại người dùng không được để trống")
		return
	}

	var ordersToUpdate []models.Order
	if err := config.DB.Where("guest_phone = ? AND user_id IS NULL", user.PhoneNumber).Find(&ordersToUpdate).Error; err != nil {
		response.ServerError(c)
		return
	}

	for _, order := range ordersToUpdate {
		if order.GuestPhone == user.PhoneNumber {
			order.UserID = &currentUserID
			if err := config.DB.Save(&order).Error; err != nil {
				response.ServerError(c)
				return
			}
		}
	}

	var totalOrders int64
	if err := config.DB.Model(&models.Order{}).Where("user_id = ?", currentUserID).Count(&totalOrders).Error; err != nil {
		response.ServerError(c)
		return
	}

	var orders []models.Order
	result := config.DB.Preload("User").
		Preload("Accommodation").
		Preload("Room").
		Where("user_id = ?", currentUserID).
		Order("created_at DESC").
		Offset(page * limit).
		Limit(limit).
		Find(&orders)

	if result.Error != nil {
		response.ServerError(c)
		return
	}

	if len(orders) == 0 {
		response.Success(c, gin.H{
			"data":  []dto.OrderUserResponse{},
			"page":  page,
			"limit": limit,
			"total": totalOrders,
		})
		return
	}

	orderResponses := make([]dto.OrderUserResponse, 0)
	for _, order := range orders {
		var user dto.ActorResponse
		if order.UserID != nil {
			user = dto.ActorResponse{Name: order.User.Name, Email: order.User.Email, PhoneNumber: order.User.PhoneNumber}
		} else {
			user = dto.ActorResponse{Name: order.GuestName, Email: order.GuestEmail, PhoneNumber: order.GuestPhone}
		}

		accommodationResponse := convertToOrderAccommodationResponse(order.Accommodation)
		var roomResponses []dto.OrderRoomResponse
		for _, room := range order.Room {
			roomResponse := convertToOrderRoomResponse(room)
			roomResponses = append(roomResponses, roomResponse)
		}

		var invoiceCode string
		if order.Status == 1 {
			var invoice models.Invoice
			if err := config.DB.Where("order_id = ?", order.ID).First(&invoice).Error; err == nil {
				invoiceCode = invoice.InvoiceCode
			}
		}

		orderResponse := dto.OrderUserResponse{
			ID:               order.ID,
			User:             user,
			Accommodation:    accommodationResponse,
			Room:             roomResponses,
			CheckInDate:      order.CheckInDate,
			CheckOutDate:     order.CheckOutDate,
			Status:           order.Status,
			CreatedAt:        order.CreatedAt.Format(time.RFC3339),
			UpdatedAt:        order.UpdatedAt.Format(time.RFC3339),
			Price:            order.Price,
			HolidayPrice:     order.HolidayPrice,
			CheckInRushPrice: order.CheckInRushPrice,
			SoldOutPrice:     order.SoldOutPrice,
			DiscountPrice:    order.DiscountPrice,
			TotalPrice:       order.TotalPrice,
			InvoiceCode:      invoiceCode,
		}
		orderResponses = append(orderResponses, orderResponse)
	}

	response.Success(c, gin.H{
		"data":  orderResponses,
		"page":  page,
		"limit": limit,
		"total": totalOrders,
	})
}

func GetOrder(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	currentUserID, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	orderID := c.Param("id")
	var order models.Order
	query := config.DB.Preload("User").Preload("Accommodation").First(&order, orderID)

	if currentUserRole == 1 {
		query = query.Where("user_id = ?", currentUserID)
	}

	if err := query.Error; err != nil {
		response.NotFound(c)
		return
	}

	response.Success(c, dto.OrderResponse{
		ID:              order.ID,
		UserID:          *order.UserID,
		AccommodationID: order.AccommodationID,
		TotalPrice:      order.TotalPrice,
		Status:          order.Status,
		CreatedAt:       order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       order.UpdatedAt.Format(time.RFC3339),
		User: types.InvoiceUserResponse{
			ID:          order.User.ID,
			Name:        order.User.Name,
			Email:       order.User.Email,
			PhoneNumber: order.User.PhoneNumber,
		},
		Accommodation: dto.AccommodationResponse{
			ID:      order.Accommodation.ID,
			Name:    order.Accommodation.Name,
			Address: order.Accommodation.Address,
			Price:   order.Accommodation.Price,
		},
	})
}

func UpdateOrderStatus(c *gin.Context) {
	var request dto.UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	_, currentUserRole, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if currentUserRole != 2 && currentUserRole != 3 {
		response.Forbidden(c)
		return
	}

	orderID := c.Param("id")
	var order models.Order
	if err := config.DB.First(&order, orderID).Error; err != nil {
		response.NotFound(c)
		return
	}

	order.Status = request.Status
	if err := config.DB.Save(&order).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, gin.H{
		"message": "Cập nhật trạng thái order thành công",
	})
}
