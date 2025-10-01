package controllers

import (
	"AkuAI/middleware"
	"AkuAI/models"
	"AkuAI/pkg/services"
	utils "AkuAI/pkg/utills"
	"log"
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
			storage := services.NewObjectStorageService()
			imageURL := storage.GenerateImageURL(user.ProfileImageURL)

			c.JSON(http.StatusOK, gin.H{
				"id":                user.ID,
				"email":             user.Email,
				"username":          user.Username,
				"profile_image_url": imageURL,
				"has_profile_image": user.ProfileImageURL != "",
			})
			return
		}

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

		if newEmail != user.Email {
			var t models.User
			if err := db.Where("email = ?", newEmail).First(&t).Error; err == nil {
				c.JSON(http.StatusConflict, gin.H{"msg": "Email already exists"})
				return
			}
		}

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

		c.JSON(http.StatusOK, gin.H{
			"msg":               "Profile updated successfully",
			"profile_image_url": user.ProfileImageURL,
		})
	}
}

func ProfileImageUploadToken(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr, _ := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		var body struct {
			FileExtension string `json:"file_extension"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "file_extension is required"})
			return
		}

		ext := strings.ToLower(body.FileExtension)
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Invalid file extension. Allowed: .jpg, .jpeg, .png, .gif, .webp"})
			return
		}

		storage := services.NewObjectStorageService()
		response, err := storage.GenerateUploadToken(uint(uid), ext)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to generate upload token"})
			return
		}

		c.JSON(http.StatusOK, response)
	}
}

func ProfileImageUpload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr, _ := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		token := c.PostForm("upload_token")
		if token == "" {
			token = c.GetHeader("X-Upload-Token")
		}
		if token == "" {
			log.Printf("[PROFILE_IMAGE_UPLOAD] Missing upload token for user %d", uid)
			c.JSON(http.StatusBadRequest, gin.H{"msg": "upload_token is required"})
			return
		}

		file, header, err := c.Request.FormFile("image")
		if err != nil {
			log.Printf("[PROFILE_IMAGE_UPLOAD] Failed to get image file for user %d: %v", uid, err)
			c.JSON(http.StatusBadRequest, gin.H{"msg": "No image file provided"})
			return
		}
		defer file.Close()

		log.Printf("[PROFILE_IMAGE_UPLOAD] Processing upload for user %d, filename: %s, size: %d, token: %s", uid, header.Filename, header.Size, token)

		storage := services.NewObjectStorageService()
		response, err := storage.SaveUploadedImage(uint(uid), file, header, token)
		if err != nil {
			log.Printf("[PROFILE_IMAGE_UPLOAD] Failed to save image for user %d: %v", uid, err)
			c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
			return
		}

		var user models.User
		if err := db.First(&user, uid).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "User not found"})
			return
		}

		if user.ProfileImageURL != "" {
			if oldPath := extractImagePath(user.ProfileImageURL); oldPath != "" {
				storage.DeleteImage(oldPath)
			}
		}

		user.ProfileImageURL = response.FilePath
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to update profile"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"msg":       "Profile image uploaded successfully",
			"image_url": response.PublicURL,
			"file_size": response.FileSize,
		})
	}
}

func ProfileImageURL(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr, _ := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		var user models.User
		if err := db.First(&user, uid).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "User not found"})
			return
		}

		storage := services.NewObjectStorageService()
		imageURL := storage.GenerateImageURL(user.ProfileImageURL)

		c.JSON(http.StatusOK, gin.H{
			"image_url": imageURL,
			"has_image": user.ProfileImageURL != "",
		})
	}
}

func DeleteProfileImage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr, _ := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		var user models.User
		if err := db.First(&user, uid).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "User not found"})
			return
		}

		if user.ProfileImageURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "No profile image to delete"})
			return
		}

		storage := services.NewObjectStorageService()
		if err := storage.DeleteImage(user.ProfileImageURL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to delete image file"})
			return
		}

		user.ProfileImageURL = ""
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to update profile"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": "Profile image deleted successfully"})
	}
}

func extractImagePath(imageURL string) string {
	parts := strings.Split(imageURL, "/uploads/profiles/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
