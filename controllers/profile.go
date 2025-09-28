package controllers

import (
	"AkuAI/middleware"
	"AkuAI/models"
	utils "AkuAI/pkg/utills"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Profile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr, _ := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		var user models.User
		if err := db.First(&user, uid).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "User not found"})
			return
		}

		if c.Request.Method == http.MethodGet {
			c.JSON(http.StatusOK, gin.H{
				"id":       user.ID,
				"email":    user.Email,
				"username": user.Username,
			})
			return
		}

		// PUT
		var body struct {
			Email    string `json:"email"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid request"})
			return
		}

		newEmail := strings.TrimSpace(strings.ToLower(body.Email))
		if newEmail == "" {
			newEmail = user.Email
		}
		newUsername := strings.TrimSpace(body.Username)
		if newUsername == "" {
			newUsername = user.Username
		}
		newPassword := body.Password

		// check email uniqueness
		if newEmail != user.Email {
			var t models.User
			if err := db.Where("email = ?", newEmail).First(&t).Error; err == nil {
				c.JSON(http.StatusConflict, gin.H{"msg": "Email already exists"})
				return
			}
		}
		// check username uniqueness
		if newUsername != user.Username {
			var t models.User
			if err := db.Where("username = ?", newUsername).First(&t).Error; err == nil {
				c.JSON(http.StatusConflict, gin.H{"msg": "Username already exists"})
				return
			}
		}

		user.Email = newEmail
		user.Username = newUsername
		if newPassword != "" {
			if !utils.HasLetter(newPassword) || !utils.HasNumber(newPassword) {
				c.JSON(http.StatusBadRequest, gin.H{"msg": "New password must contain at least one letter and one number"})
				return
			}
			if err := user.SetPassword(newPassword); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to set password"})
				return
			}
		}
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to update profile"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": "Profile updated successfully"})
	}
}
