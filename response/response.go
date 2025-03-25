package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response định nghĩa cấu trúc response
type Response struct {
	Code int         `json:"code"`
	Mess string      `json:"mess"`
	Data interface{} `json:"data,omitempty"`
}

// Success trả về response thành công
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 1,
		Mess: "Thành công",
		Data: data,
	})
}

// Error trả về response lỗi
func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code: code,
		Mess: message,
	})
}

// ServerError trả về response lỗi server
func ServerError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, Response{
		Code: 0,
		Mess: "Lỗi server",
	})
}

// Unauthorized trả về response chưa xác thực
func Unauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, Response{
		Code: 0,
		Mess: "Chưa xác thực",
	})
}

// Forbidden trả về response không có quyền
func Forbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, Response{
		Code: 0,
		Mess: "Không có quyền truy cập",
	})
}

// NotFound trả về response không tìm thấy
func NotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, Response{
		Code: 0,
		Mess: "Không tìm thấy",
	})
}

// ValidationError trả về response lỗi validation
func ValidationError(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code: 0,
		Mess: message,
	})
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(400, gin.H{
		"code": 0,
		"mess": message,
	})
}
