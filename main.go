package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"new/config"
	_ "new/docs"
	"new/jobs"
	"new/routes"
	"new/services"
	"new/services/logger"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// @title           API Trothalo
// @version         1.0
// @description     API hệ thống đặt lưu trú
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Khởi tạo ứng dụng
	router, m, c, err := config.InitApp()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	// Khởi tạo các services
	userService := services.NewUserService(services.UserServiceOptions{
		DB:     config.DB,
		Logger: logger.NewDefaultLogger(logger.InfoLevel),
	})
	userServiceAdapter := services.NewUserServiceAdapter(userService)
	jobs.SetUserAmountUpdater(userServiceAdapter)

	// Khởi tạo cron jobs
	if err := jobs.InitCronJobs(c, m); err != nil {
		log.Fatalf("Failed to initialize cron jobs: %v", err)
	}

	// Khởi tạo WebSocket
	config.InitWebSocket(router, m)

	// Khởi tạo Swagger
	config.InitSwagger(router)

	// Setup các routes của ứng dụng
	routes.SetupRoutes(router, config.DB, config.RedisClient, config.Cloudinary, m)

	// Endpoint ping
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Goroutine tự động gọi endpoint /ping mỗi 5 phút
	go func() {
		pingURL := "https://backend.trothalo.click/ping"
		for {
			resp, err := http.Get(pingURL)
			if err != nil {
				log.Printf("Error pinging /ping endpoint: %v", err)
			} else {
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				log.Printf("Ping response: %s", string(body))
			}
			time.Sleep(5 * time.Minute)
		}
	}()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	log.Println("Server starting on port " + port + "...")
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
