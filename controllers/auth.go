package controllers

import (
	"AkuAI/middleware"
	"AkuAI/models"
	"AkuAI/pkg/config"
	tokenstore "AkuAI/pkg/token"
	utils "AkuAI/pkg/utills"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Register handler
func Register(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Email           string `json:"email"`
			Username        string `json:"username"`
			Password        string `json:"password"`
			ConfirmPassword string `json:"confirm_password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid request"})
			return
		}

		email := strings.TrimSpace(strings.ToLower(body.Email))
		username := strings.TrimSpace(body.Username)
		password := body.Password
		confirm := body.ConfirmPassword

		if email == "" || username == "" || password == "" || confirm == "" {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Email, username, password, and confirm password are required"})
			return
		}

		if password != confirm {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Passwords do not match"})
			return
		}

		// password validation: at least one letter and one number
		if !utils.HasLetter(password) || !utils.HasNumber(password) {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Password must contain at least one letter and one number"})
			return
		}

		var exists models.User
		if err := db.Where("email = ? OR username = ?", email, username).First(&exists).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"msg": "Email or username already exists"})
			return
		} else if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "db error"})
			return
		}

		user := models.User{
			Email:    email,
			Username: username,
		}
		if err := user.SetPassword(password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to set password"})
			return
		}
		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"msg": "User created", "username": user.Username, "email": user.Email})
	}
}

// Login handler
func Login(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid request"})
			return
		}
		email := strings.TrimSpace(strings.ToLower(body.Email))
		password := body.Password

		if email == "" || password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Email and password are required"})
			return
		}

		var user models.User
		if err := db.Where("email = ?", email).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "Invalid credentials"})
			return
		}

		if !user.CheckPassword(password) {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "Invalid credentials"})
			return
		}

		// create JWT with 1 day expiry
		jti := uuid.NewString()
		claims := jwt.MapClaims{
			"sub": strconv.Itoa(int(user.ID)),
			"exp": time.Now().Add(24 * time.Hour).Unix(),
			"jti": jti,
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err := token.SignedString([]byte(config.JWTSecret))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"access_token": tokenStr, "username": user.Username})
	}
}

// Logout handler
func Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		jti, _ := c.Get(middleware.ContextJTIKey)
		if s, ok := jti.(string); ok && s != "" {
			tokenstore.RevokeToken(s)
		}
		c.JSON(http.StatusOK, gin.H{"msg": "logged out"})
	}
}
