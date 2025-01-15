package main

import (
	"fmt"
	"log"
	"new/config"
	_ "new/docs"
	"new/routes"
	"os/exec"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func recreateUserTable() {
	// Example: recreate or migrate your tables here.
	println("abc.")
}

// Kiểm tra kết nối Redis
func checkRedisConnection(RDB *redis.Client) bool {
	_, err := RDB.Ping(config.Ctx).Result()
	if err != nil {
		fmt.Println("Không thể kết nối Redis:", err)
		return false
	}
	fmt.Println("Redis đang hoạt động bình thường.")
	return true
}

// Khởi động lại Redis (Chạy lệnh trên hệ thống)
func restartRedis() error {
	fmt.Println("Khởi động lại Redis Server...")
	cmd := exec.Command("sudo", "systemctl", "restart", "redis")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("lỗi khi khởi động lại Redis: %v", err)
	}
	fmt.Println("Redis đã được khởi động lại thành công.")
	return nil
}

func main() {
	router := gin.Default()

	// Nạp biến môi trường
	err := config.LoadEnv()
	if err != nil {
		panic("Failed to load .env file")
	}

	// Kết nối database
	config.ConnectDB()

	// Kết nối Cloudinary
	config.ConnectCloudinary()

	// Tạo lại bảng nếu cần
	recreateUserTable()

	// Kết nối Redis
	redisCli, err := config.ConnectRedis()
	if err != nil {
		panic("Failed to connect to Redis!")
	}

	// Kiểm tra kết nối Redis, nếu không thành công, khởi động lại Redis
	if !checkRedisConnection(redisCli) {
		err := restartRedis()
		if err != nil {
			log.Fatalf("Không thể khởi động lại Redis: %v\n", err)
		}

		// Sau khi khởi động lại Redis, chờ 10 giây và kiểm tra lại
		// Có thể thêm thời gian chờ ở đây nếu cần
		// Chờ 10 giây cho Redis khởi động lại
		// time.Sleep(10 * time.Second)

		// Kiểm tra lại kết nối Redis
		if !checkRedisConnection(redisCli) {
			log.Fatalf("Redis vẫn không thể kết nối sau khi khởi động lại.\n")
		}
	}

	// Cấu hình CORS
	configCors := cors.DefaultConfig()
	configCors.AddAllowHeaders("Authorization")
	configCors.AllowCredentials = true
	configCors.AllowAllOrigins = false
	configCors.AllowOriginFunc = func(origin string) bool {
		return true
	}

	router.Use(cors.New(configCors))

	// Cấu hình các routes
	routes.SetupRoutes(router, config.DB, redisCli, config.Cloudinary)

	// Swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Chạy server
	router.Run(":8082")
}
