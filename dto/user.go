package dto

// UserResponse định nghĩa response cho user
type UserResponse struct {
	ID               uint           `json:"id"`
	Name             string         `json:"name"`
	Email            string         `json:"email"`
	PhoneNumber      string         `json:"phoneNumber"`
	Role             int            `json:"role"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
	Banks            []Bank         `json:"banks"`
	Children         []UserResponse `json:"children,omitempty"`
	Status           int            `json:"status,omitempty"`
	IsVerified       bool           `json:"isVerified,omitempty"`
	Avatar           string         `json:"avatar,omitempty"`
	DateOfBirth      string         `json:"dateOfBirth,omitempty"`
	Amount           int64          `json:"amount,omitempty"`
	AccommodationIDs []int64        `json:"accommodationIds,omitempty"`
	AdminId          *uint          `json:"adminId,omitempty"`
}

// Bank định nghĩa thông tin ngân hàng
type Bank struct {
	BankName      string `json:"bankName"`
	AccountNumber string `json:"accountNumber"`
	BankShortName string `json:"bankShortName"`
}

// CreateUserRequest định nghĩa request tạo user
type CreateUserRequest struct {
	Username      string `json:"username"`
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password" binding:"required"`
	Phone         string `json:"phone" binding:"required"`
	Role          int    `json:"role"`
	BankID        int    `json:"bankId"`
	AccountNumber string `json:"accountNumber"`
	Amount        int64  `json:"amount"`
}

// UpdateUserRequest định nghĩa request cập nhật user
type UpdateUserRequest struct {
	Username    string `json:"username"`
	Phone       string `json:"phone"`
	Avatar      string `json:"avatar"`
	DateOfBirth string `json:"dateOfBirth"`
	Gender      int    `json:"gender"`
}

// StatusUserRequest định nghĩa request cập nhật trạng thái user
type UserStatusRequest struct {
	Status int  `json:"status"`
	ID     uint `json:"id" binding:"required"`
}

// LoginRequest định nghĩa request đăng nhập
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest định nghĩa request đăng ký
type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
	PhoneNumber string `json:"phoneNumber" binding:"required"`
}

// VerifyCodeRequest định nghĩa request xác thực mã
type VerifyCodeRequest struct {
	Email string `json:"email" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// UpdateBalanceRequest định nghĩa request cập nhật số dư
type UpdateBalanceRequest struct {
	UserID uint  `json:"userId" binding:"required"`
	Amount int64 `json:"amount" binding:"required"`
}

type UserListResponse struct {
	Data  []UserResponse `json:"data"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
	Total int64          `json:"total"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}
