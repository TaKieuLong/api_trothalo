package controllers

import (
	"fmt"
	"log"
	"net/http"
	"new/config"
	"new/dto"
	"new/models"
	"new/response"
	"new/services"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserController struct {
	DB    *gorm.DB
	Redis *redis.Client
}

func NewUserController(mySQL *gorm.DB, redisCli *redis.Client) UserController {
	return UserController{
		DB:    mySQL,
		Redis: redisCli,
	}
}

func (u UserController) GetUsers(c *gin.Context) {
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

	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	statusStr := c.Query("status")
	name := c.Query("name")
	roleStr := c.Query("role")

	page := 0
	limit := 10

	if pageStr != "" {
		page, _ = strconv.Atoi(pageStr)
	}
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	// Tạo cache key dựa trên vai trò và bộ lọc
	var cacheKey string
	if currentUserRole == 1 {
		cacheKey = "users:all"
	} else if currentUserRole == 2 {
		cacheKey = fmt.Sprintf("users:role_3:admin_%d", currentUserID)
	} else {
		response.Forbidden(c)
		return
	}

	// Kết nối Redis
	rdb, err := config.ConnectRedis()
	if err != nil {
		log.Printf("Không thể kết nối Redis: %v", err)
	}

	var allUsers []models.User

	// Kiểm tra cache
	if err := services.GetFromRedis(config.Ctx, rdb, cacheKey, &allUsers); err != nil || len(allUsers) == 0 {
		// Nếu không có dữ liệu trong cache, truy vấn từ DB
		query := u.DB.Preload("Banks").Preload("Children")

		if currentUserRole == 3 {
			var adminID int
			if err := u.DB.Model(&models.User{}).Select("admin_id").Where("id = ?", currentUserID).Scan(&adminID).Error; err != nil {
				response.ServerError(c)
				return
			}

			query = query.Where("role = 3 AND admin_id = ?", adminID)
		} else if currentUserRole == 2 {
			query = query.Where("role = 3 AND admin_id = ?", currentUserID)
		}

		if err := query.Find(&allUsers).Error; err != nil {
			response.ServerError(c)
			return
		}

		// Lưu dữ liệu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, allUsers, 10*time.Minute); err != nil {
			log.Printf("Lỗi khi lưu danh sách người dùng vào Redis: %v", err)
		}
	}

	var filteredUsers []models.User
	for _, user := range allUsers {
		// Lọc theo status
		if statusStr != "" {
			status, _ := strconv.Atoi(statusStr)
			if user.Status != status {
				continue
			}
		}

		// Lọc theo name
		if name != "" && !strings.Contains(strings.ToLower(user.Name), strings.ToLower(name)) &&
			!strings.Contains(strings.ToLower(user.PhoneNumber), strings.ToLower(name)) &&
			!strings.Contains(strings.ToLower(user.Email), strings.ToLower(name)) {
			continue
		}

		// Lọc theo role
		if roleStr != "" {
			role, _ := strconv.Atoi(roleStr)
			if user.Role != role {
				continue
			}
		}

		filteredUsers = append(filteredUsers, user)
	}

	// Lọc và chuẩn bị response
	var userResponses []dto.UserResponse
	for _, user := range filteredUsers {
		if currentUserRole == 1 && user.Role == 3 {
			continue
		}

		if user.ID == uint(currentUserID) {
			continue
		}

		if currentUserRole == 2 {
			if user.Role != 3 || user.AdminId == nil || *user.AdminId != uint(currentUserID) {
				continue
			}
		}

		var banks []dto.Bank
		for _, bank := range user.Banks {
			banks = append(banks, dto.Bank{
				BankName:      bank.BankName,
				AccountNumber: bank.AccountNumber,
				BankShortName: bank.BankShortName,
			})
		}

		var childrenResponses []dto.UserResponse
		for _, child := range user.Children {
			var childBanks []dto.Bank
			for _, bank := range child.Banks {
				childBanks = append(childBanks, dto.Bank{
					BankName:      bank.BankName,
					AccountNumber: bank.AccountNumber,
					BankShortName: bank.BankShortName,
				})
			}

			childrenResponses = append(childrenResponses, dto.UserResponse{
				ID:          child.ID,
				Name:        child.Name,
				Email:       child.Email,
				PhoneNumber: child.PhoneNumber,
				Role:        child.Role,
				CreatedAt:   child.CreatedAt.Format(time.RFC3339),
				UpdatedAt:   child.UpdatedAt.Format(time.RFC3339),
				Banks:       childBanks,
				Status:      child.Status,
				IsVerified:  child.IsVerified,
				Avatar:      child.Avatar,
				Amount:      child.Amount,
			})
		}

		userResponses = append(userResponses, dto.UserResponse{
			ID:          user.ID,
			Name:        user.Name,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
			Role:        user.Role,
			CreatedAt:   user.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   user.UpdatedAt.Format(time.RFC3339),
			Banks:       banks,
			Children:    childrenResponses,
			Status:      user.Status,
			IsVerified:  user.IsVerified,
			Avatar:      user.Avatar,
			Amount:      user.Amount,
			AdminId:     user.AdminId,
		})
	}

	// Sắp xếp và phân trang
	sort.Slice(userResponses, func(i, j int) bool {
		return userResponses[i].ID < userResponses[j].ID
	})

	start := page * limit
	end := start + limit
	if end > len(userResponses) {
		end = len(userResponses)
	}

	paginatedUsers := userResponses[start:end]

	response.Success(c, dto.UserListResponse{
		Data:  paginatedUsers,
		Page:  page,
		Limit: limit,
		Total: int64(len(userResponses)),
	})
}

func (u *UserController) CreateUser(c *gin.Context) {
	var input dto.CreateUserRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Kiểm tra email đã tồn tại chưa
	var existingUser models.User
	if err := config.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email đã tồn tại"})
		return
	}

	// Tạo user mới
	user := models.User{
		Name:        input.Username,
		Email:       input.Email,
		PhoneNumber: input.Phone,
		Role:        input.Role,
		Amount:      input.Amount,
	}

	// Mã hóa mật khẩu
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.Password = string(hashedPassword)

	// Tạo user trong database
	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Nếu có thông tin ngân hàng, tạo bank account
	if input.BankID > 0 {
		bank := models.Bank{
			UserId:        user.ID,
			BankId:        uint(input.BankID),
			AccountNumber: input.AccountNumber,
		}
		if err := config.DB.Create(&bank).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bank account"})
			return
		}
	}

	// Tạo response
	userResponse := dto.UserResponse{
		ID:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		PhoneNumber: user.PhoneNumber,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
		Status:      user.Status,
		IsVerified:  user.IsVerified,
		Avatar:      user.Avatar,
		DateOfBirth: user.DateOfBirth,
		Amount:      user.Amount,
	}

	c.JSON(http.StatusCreated, userResponse)
}

func (u UserController) GetUserByID(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := config.DB.Preload("Banks").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Kiểm tra quyền truy cập
	userIDFromToken := c.GetUint("userID")
	userRoleFromToken := c.GetInt("userRole")
	if userIDFromToken != uint(userID) && userRoleFromToken != 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Không có quyền truy cập thông tin người dùng khác"})
		return
	}

	// Chuyển đổi Banks sang định dạng DTO
	var banks []dto.Bank
	for _, bank := range user.Banks {
		banks = append(banks, dto.Bank{
			BankName:      bank.BankName,
			AccountNumber: bank.AccountNumber,
			BankShortName: bank.BankShortName,
		})
	}

	// Chuyển đổi Children sang định dạng DTO
	var children []dto.UserResponse
	for _, child := range user.Children {
		children = append(children, dto.UserResponse{
			ID:          child.ID,
			Name:        child.Name,
			Email:       child.Email,
			PhoneNumber: child.PhoneNumber,
			Role:        child.Role,
			CreatedAt:   child.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   child.UpdatedAt.Format("2006-01-02 15:04:05"),
			Status:      child.Status,
			IsVerified:  child.IsVerified,
			Avatar:      child.Avatar,
			DateOfBirth: child.DateOfBirth,
			Amount:      child.Amount,
		})
	}

	resp := dto.UserResponse{
		ID:               user.ID,
		Name:             user.Name,
		Email:            user.Email,
		PhoneNumber:      user.PhoneNumber,
		Role:             user.Role,
		CreatedAt:        user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        user.UpdatedAt.Format("2006-01-02 15:04:05"),
		Banks:            banks,
		Children:         children,
		Status:           user.Status,
		IsVerified:       user.IsVerified,
		Avatar:           user.Avatar,
		DateOfBirth:      user.DateOfBirth,
		Amount:           user.Amount,
		AccommodationIDs: user.AccommodationIDs,
		AdminId:          user.AdminId,
	}

	c.JSON(http.StatusOK, resp)
}

func (u UserController) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var input dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Cập nhật thông tin user
	user.Name = input.Username
	user.PhoneNumber = input.Phone
	user.Avatar = input.Avatar
	user.DateOfBirth = input.DateOfBirth

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Tạo response
	userResponse := dto.UserResponse{
		ID:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		PhoneNumber: user.PhoneNumber,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
		Status:      user.Status,
		IsVerified:  user.IsVerified,
		Avatar:      user.Avatar,
		DateOfBirth: user.DateOfBirth,
		Amount:      user.Amount,
	}

	c.JSON(http.StatusOK, userResponse)
}

func (u UserController) ChangeUserStatus(c *gin.Context) {
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

	var statusRequest dto.UserStatusRequest
	if err := c.ShouldBindJSON(&statusRequest); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	var user models.User

	if currentUserRole == 2 {
		if err := u.DB.Where("id = ? AND admin_id = ?", statusRequest.ID, currentUserID).First(&user).Error; err != nil {
			response.NotFound(c)
			return
		}
	} else if currentUserRole == 1 {
		if err := u.DB.First(&user, statusRequest.ID).Error; err != nil {
			response.NotFound(c)
			return
		}

		if user.Role == 2 {
			var childUsers []models.User
			if err := u.DB.Where("admin_id = ?", user.ID).Find(&childUsers).Error; err != nil {
				response.ServerError(c)
				return
			}

			for _, child := range childUsers {
				child.Status = statusRequest.Status
				if err := u.DB.Save(&child).Error; err != nil {
					response.ServerError(c)
					return
				}
			}
		}
	} else {
		response.Forbidden(c)
		return
	}

	user.Status = statusRequest.Status
	if err := u.DB.Save(&user).Error; err != nil {
		response.ServerError(c)
		return
	}

	//Xóa redis
	rdb, redisErr := config.ConnectRedis()
	if redisErr == nil {
		switch currentUserRole {
		case 1:
			_ = services.DeleteFromRedis(config.Ctx, rdb, "user:all")
		case 3:
			adminCacheKey := fmt.Sprintf("users:role_3:admin_%d", currentUserID)
			_ = services.DeleteFromRedis(config.Ctx, rdb, adminCacheKey)
		}
	}

	response.Success(c, user)
}

// Get Detail Receptionist
func (u UserController) GetReceptionistByID(c *gin.Context) {
	var user models.User
	id := c.Param("id")

	err := u.DB.Table("users").
		Where("users.id = ?", id).
		First(&user).Error

	if err != nil {
		response.NotFound(c)
		return
	}

	var banks []dto.Bank
	u.DB.Where("user_id = ?", id).Find(&banks)

	var accommodations []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	var ids []int64
	for _, v := range user.AccommodationIDs {
		ids = append(ids, v)
	}

	if len(ids) > 0 {
		u.DB.Table("accommodations").
			Select("id, name").
			Where("id IN (?)", ids).
			Find(&accommodations)
	}

	userResponse := dto.UserResponse{
		ID:               user.ID,
		Name:             user.Name,
		Email:            user.Email,
		PhoneNumber:      user.PhoneNumber,
		Role:             user.Role,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        user.UpdatedAt.Format(time.RFC3339),
		Banks:            banks,
		Status:           user.Status,
		IsVerified:       user.IsVerified,
		Avatar:           user.Avatar,
		Amount:           user.Amount,
		AccommodationIDs: user.AccommodationIDs,
	}

	response.Success(c, gin.H{
		"user":           userResponse,
		"accommodations": accommodations,
	})
}

// Get Bank SA
func (u UserController) GetBankSuperAdmin(c *gin.Context) {
	var user models.User

	err := u.DB.Table("users").
		Where("users.role = ?", 1).
		First(&user).Error

	if err != nil {
		response.NotFound(c)
		return
	}

	var bank []dto.Bank
	u.DB.Where("user_id = ?", user.ID).Find(&bank)

	response.Success(c, gin.H{
		"sabank": bank,
	})
}

// get Profile
func (u UserController) GetProfile(c *gin.Context) {
	var user models.User
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	id, _, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	if err := u.DB.First(&user, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	var banks []dto.Bank
	for _, bank := range user.Banks {
		banks = append(banks, dto.Bank{
			BankName:      bank.BankName,
			AccountNumber: bank.AccountNumber,
			BankShortName: bank.BankShortName,
		})
	}

	userResponse := dto.UserResponse{
		ID:               user.ID,
		Name:             user.Name,
		Email:            user.Email,
		PhoneNumber:      user.PhoneNumber,
		Role:             user.Role,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        user.UpdatedAt.Format(time.RFC3339),
		Banks:            banks,
		Status:           user.Status,
		IsVerified:       user.IsVerified,
		Avatar:           user.Avatar,
		Amount:           user.Amount,
		AccommodationIDs: user.AccommodationIDs,
	}

	response.Success(c, userResponse)
}

func (u UserController) GetUserList(c *gin.Context) {
	var users []models.User
	var total int64

	// Lấy thông tin phân trang
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	// Lấy tổng số user
	if err := config.DB.Model(&models.User{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get total users"})
		return
	}

	// Lấy danh sách user với phân trang
	if err := config.DB.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
		return
	}

	// Chuyển đổi sang DTO
	var userResponses []dto.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, dto.UserResponse{
			ID:          user.ID,
			Name:        user.Name,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
			Role:        user.Role,
			CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
			Status:      user.Status,
			IsVerified:  user.IsVerified,
			Avatar:      user.Avatar,
			DateOfBirth: user.DateOfBirth,
			Amount:      user.Amount,
		})
	}

	response := dto.UserListResponse{
		Data:  userResponses,
		Page:  page,
		Limit: limit,
		Total: total,
	}

	c.JSON(http.StatusOK, response)
}

func (u UserController) GetUserByEmail(c *gin.Context) {
	email := c.Param("email")
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	userResponse := dto.UserResponse{
		ID:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		PhoneNumber: user.PhoneNumber,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
		Status:      user.Status,
		IsVerified:  user.IsVerified,
		Avatar:      user.Avatar,
		DateOfBirth: user.DateOfBirth,
		Amount:      user.Amount,
	}

	c.JSON(http.StatusOK, userResponse)
}
