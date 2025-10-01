package main

import (
	"AkuAI/middleware"
	"AkuAI/models"
	"AkuAI/pkg/config"
	"AkuAI/routes"
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	log.Printf("Database Config - Host:%s Port:%s User:%s DB:%s",
		config.MySQLHost, config.MySQLPort, config.MySQLUser, config.MySQLDatabase)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.MySQLUser,
		config.MySQLPassword,
		config.MySQLHost,
		config.MySQLPort,
		config.MySQLDatabase,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to MySQL database: %v", err)
	}

	log.Printf("Connected to MySQL database: %s@%s:%s/%s",
		config.MySQLUser, config.MySQLHost, config.MySQLPort, config.MySQLDatabase)

	if err := db.AutoMigrate(&models.User{}, &models.Conversation{}, &models.Message{}); err != nil {
		log.Fatalf("failed migrate: %v", err)
	}

	middleware.SetRateLimitConfig(time.Duration(config.RateLimitWindowSeconds)*time.Second, config.RateLimitCapacity, config.UserConcurrencyLimit)
	middleware.SetDuplicateTTL(time.Duration(config.DuplicateWindowSeconds) * time.Second)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Bypass-Duplicate", "x-bypass-duplicate"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	routes.RegisterRoutes(r, db)
	r.Run(":" + config.Port)
}
