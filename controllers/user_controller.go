package controllers

import (
	"errors"
	"fmt"
	"log"
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

// filterUsers lọc danh sách users theo các tiêu chí
func filterUsers(users []models.User, statusStr, name, roleStr string) []models.User {
	var filteredUsers []models.User
	for _, user := range users {
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
	return filteredUsers
}

// convertToUserResponse chuyển đổi User model sang UserResponse DTO
func convertToUserResponse(user models.User) dto.UserResponse {
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
		childrenResponses = append(childrenResponses, convertToUserResponse(child))
	}

	return dto.UserResponse{
		ID:               user.ID,
		Name:             user.Name,
		Email:            user.Email,
		PhoneNumber:      user.PhoneNumber,
		Role:             user.Role,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        user.UpdatedAt.Format(time.RFC3339),
		Banks:            banks,
		Children:         childrenResponses,
		Status:           user.Status,
		IsVerified:       user.IsVerified,
		Avatar:           user.Avatar,
		Amount:           user.Amount,
		AccommodationIDs: user.AccommodationIDs,
		AdminId:          user.AdminId,
	}
}

// getUsersFromCacheOrDB lấy danh sách users từ cache hoặc database
func (u UserController) getUsersFromCacheOrDB(cacheKey string, currentUserID uint, currentUserRole int) ([]models.User, error) {
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
			var adminID uint
			if err := u.DB.Model(&models.User{}).Select("admin_id").Where("id = ?", currentUserID).Scan(&adminID).Error; err != nil {
				return nil, err
			}

			query = query.Where("role = 3 AND admin_id = ?", adminID)
		} else if currentUserRole == 2 {
			query = query.Where("role = 3 AND admin_id = ?", currentUserID)
		}

		if err := query.Find(&allUsers).Error; err != nil {
			return nil, err
		}

		// Lưu dữ liệu vào Redis
		if err := services.SetToRedis(config.Ctx, rdb, cacheKey, allUsers, 10*time.Minute); err != nil {
			log.Printf("Lỗi khi lưu danh sách người dùng vào Redis: %v", err)
		}
	}

	return allUsers, nil
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

	// Lấy danh sách users từ cache hoặc database
	allUsers, err := u.getUsersFromCacheOrDB(cacheKey, currentUserID, currentUserRole)
	if err != nil {
		response.ServerError(c)
		return
	}

	// Lọc users theo các tiêu chí
	filteredUsers := filterUsers(allUsers, statusStr, name, roleStr)

	// Chuyển đổi sang DTO và lọc theo quyền
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

		userResponses = append(userResponses, convertToUserResponse(user))
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

// handleError xử lý lỗi và trả về response phù hợp
func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.NotFound(c)
	case strings.Contains(err.Error(), "duplicate key"):
		response.BadRequest(c, "Resource already exists")
	case strings.Contains(err.Error(), "unauthorized"):
		response.Unauthorized(c)
	case strings.Contains(err.Error(), "forbidden"):
		response.Forbidden(c)
	default:
		response.ServerError(c)
	}
}

func (u UserController) CreateUser(c *gin.Context) {
	var input dto.CreateUserRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Kiểm tra email đã tồn tại chưa
	var existingUser models.User
	if err := config.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		response.BadRequest(c, "Email đã tồn tại")
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
		response.ServerError(c)
		return
	}
	user.Password = string(hashedPassword)

	// Tạo user trong database
	if err := config.DB.Create(&user).Error; err != nil {
		response.ServerError(c)
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
			response.ServerError(c)
			return
		}
	}

	response.Success(c, createUserResponse(user))
}

// createUserResponse tạo response cho user
func createUserResponse(user models.User) dto.UserResponse {
	var banks []dto.Bank
	for _, bank := range user.Banks {
		banks = append(banks, dto.Bank{
			BankName:      bank.BankName,
			AccountNumber: bank.AccountNumber,
			BankShortName: bank.BankShortName,
		})
	}

	return dto.UserResponse{
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
		AdminId:          user.AdminId,
	}
}

func (u UserController) GetUserByID(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := config.DB.Preload("Banks").First(&user, id).Error; err != nil {
		handleError(c, err)
		return
	}

	// Kiểm tra quyền truy cập
	if err := validateUserAccess(c, user.ID, user.Role); err != nil {
		handleError(c, err)
		return
	}

	response.Success(c, createUserResponse(user))
}

func (u UserController) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var input dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	var user models.User
	if err := config.DB.First(&user, id).Error; err != nil {
		handleError(c, err)
		return
	}

	// Cập nhật thông tin user
	updates := map[string]interface{}{
		"name":          input.Username,
		"phone_number":  input.Phone,
		"avatar":        input.Avatar,
		"date_of_birth": input.DateOfBirth,
		"gender":        input.Gender,
	}

	if err := config.DB.Model(&user).Updates(updates).Error; err != nil {
		response.ServerError(c)
		return
	}

	response.Success(c, createUserResponse(user))
}

// clearUserCache xóa cache của user dựa trên role
func (u UserController) clearUserCache(currentUserID uint, currentUserRole int) {
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

	// Xóa cache
	u.clearUserCache(currentUserID, currentUserRole)

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

// GetProfile lấy thông tin profile của user
func (u UserController) GetProfile(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	userID, _, err := services.GetUserIDFromToken(tokenString)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	var user models.User
	if err := u.DB.Preload("Banks").Preload("Children").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c)
			return
		}
		response.ServerError(c)
		return
	}

	response.Success(c, convertToUserResponse(user))
}

// paginateUsers phân trang danh sách users
func paginateUsers(users []dto.UserResponse, page, limit int) []dto.UserResponse {
	start := page * limit
	end := start + limit
	if end > len(users) {
		end = len(users)
	}
	return users[start:end]
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
		response.ServerError(c)
		return
	}

	// Lấy danh sách user với phân trang
	if err := config.DB.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		response.ServerError(c)
		return
	}

	// Chuyển đổi sang DTO
	var userResponses []dto.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, createUserResponse(user))
	}

	// Phân trang
	paginatedUsers := paginateUsers(userResponses, page, limit)

	response.Success(c, dto.UserListResponse{
		Data:  paginatedUsers,
		Page:  page,
		Limit: limit,
		Total: total,
	})
}

func (u UserController) GetUserByEmail(c *gin.Context) {
	email := c.Param("email")
	var user models.User
	if err := config.DB.Preload("Banks").Where("email = ?", email).First(&user).Error; err != nil {
		handleError(c, err)
		return
	}

	response.Success(c, createUserResponse(user))
}

// getCurrentUserInfo lấy thông tin user hiện tại từ token
func getCurrentUserInfo(c *gin.Context) (uint, int, error) {
	userID := c.GetUint("userID")
	userRole := c.GetInt("userRole")
	if userID == 0 || userRole == 0 {
		return 0, 0, errors.New("unauthorized")
	}
	return userID, userRole, nil
}

// validateUserAccess kiểm tra quyền truy cập của user
func validateUserAccess(c *gin.Context, targetUserID uint, targetUserRole int) error {
	userID, userRole, err := getCurrentUserInfo(c)
	if err != nil {
		return err
	}

	if userID != targetUserID && userRole != 1 {
		return errors.New("forbidden")
	}

	return nil
}
