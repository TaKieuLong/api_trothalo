package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"new/config"
	_ "new/docs"
	"new/routes"
	"new/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	"github.com/robfig/cron/v3"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func recreateUserTable() {
	// Nếu cần, thực hiện AutoMigrate ở đây
}

func main() {
	router := gin.Default()

	// Load biến môi trường
	if err := config.LoadEnv(); err != nil {
		panic("Failed to load .env file")
	}

	// Kết nối database và Cloudinary
	config.ConnectDB()
	config.ConnectCloudinary()

	// Khởi tạo WebSocket (Melody)
	m := melody.New()

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	fmt.Println("Current time:", now)

	// Cron job đã có (ví dụ: chạy lúc 0h mỗi ngày)
	c := cron.New()
	_, err := c.AddFunc("0 0 * * *", func() {
		now := time.Now()
		fmt.Println("Running UpdateUserAmounts at:", now)
		services.UpdateUserAmounts(m)
	})
	if err != nil {
		panic(fmt.Sprintf("Cron job error: %v", err))
	}
	c.Start()

	recreateUserTable()

	// Kết nối Redis
	redisCli, err := config.ConnectRedis()
	if err != nil {
		panic("Failed to connect to Redis!")
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

	// Setup các routes của ứng dụng
	routes.SetupRoutes(router, config.DB, redisCli, config.Cloudinary, m)

	// Endpoint WebSocket
	router.GET("/ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})

	// Endpoint Swagger
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Goroutine tự động gọi endpoint /ping mỗi 5 phút
	go func() {
		pingURL := "https://backend.trothalo.click/ping"
		for {
			resp, err := http.Get(pingURL)
			if err != nil {
				fmt.Println("Error pinging /ping endpoint:", err)
			} else {
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				fmt.Println("Ping response:", string(body))
			}
			time.Sleep(5 * time.Minute)
		}
	}()

	// Chạy server
	router.Run(":8083")
}
