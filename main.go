package main

import (
	"fmt"
	"new/config"
	_ "new/docs"
	"new/routes"
	"new/services"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	"github.com/robfig/cron/v3"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func recreateUserTable() {

	// if err := config.DB.AutoMigrate(&models.Room{}, &models.Benefit{}, &models.User{}, models.Rate{}, models.Order{}, models.Invoice{}, models.Bank{}, models.Accommodation{}, models.AccommodationStatus{}, models.BankFake{}, models.UserDiscount{}, models.Discount{}, models.Holiday{}, models.RoomStatus{}); err != nil {
	// 	panic("Failed to migrate tables: " + err.Error())
	// }

	println("abc.")
}

func main() {
	router := gin.Default()

	err := config.LoadEnv()
	if err != nil {
		panic("Failed to load .env file")
	}

	config.ConnectDB()

	// Khởi tạo Cloudinary
	config.ConnectCloudinary()

	// Khởi ws
	m := melody.New()

	loc := time.UTC
	// Khởi tạo cron job
	c := cron.New(cron.WithLocation(loc))

	time.Local = loc
	_, err = c.AddFunc("22 0 * * *", func() {
		now := time.Now().In(loc)
		fmt.Println("Đang chạy UpdateUserAmounts vào lúc:", now)
		services.UpdateUserAmounts(m)
	})

	if err != nil {
		panic(fmt.Sprintf("Lỗi khi thêm cron job: %v", err))
	}

	c.Start()

	recreateUserTable()

	redisCli, err := config.ConnectRedis()
	if err != nil {
		panic("Failed to connect to Redis!")
	}

	configCors := cors.DefaultConfig()
	configCors.AddAllowHeaders("Authorization")
	configCors.AllowCredentials = true
	configCors.AllowAllOrigins = false
	configCors.AllowOriginFunc = func(origin string) bool {
		return true
	}

	router.Use(cors.New(configCors))

	routes.SetupRoutes(router, config.DB, redisCli, config.Cloudinary, m)

	router.GET("/ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		m.Broadcast(msg)
	})

	router.Use(func(c *gin.Context) {
		c.Next()
		for key, value := range c.Writer.Header() {
			fmt.Println(key, value)
		}
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.Run(":8083")
}
