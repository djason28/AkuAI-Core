package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ObjectStorageService struct {
	basePath  string
	baseURL   string
	secretKey string
}

func NewObjectStorageService() *ObjectStorageService {
	basePath := "./uploads/profiles"
	baseURL := "http://127.0.0.1:5000/uploads/profiles"
	secretKey := "your-secret-key-for-signing"

	os.MkdirAll(basePath, 0755)

	return &ObjectStorageService{
		basePath:  basePath,
		baseURL:   baseURL,
		secretKey: secretKey,
	}
}

func (s *ObjectStorageService) GenerateUploadToken(userID uint, fileExtension string) (*UploadTokenResponse, error) {
	timestamp := time.Now().Unix()

	token := s.generateSimpleSignedToken(userID, timestamp)

	return &UploadTokenResponse{
		UploadToken: token,
		Filename:    "",
		FilePath:    "",
		ExpiresAt:   time.Now().Add(15 * time.Minute),
	}, nil
}

func (s *ObjectStorageService) SaveUploadedImage(userID uint, file multipart.File, header *multipart.FileHeader, token string) (*SaveImageResponse, error) {
	if !s.validateUploadToken(token, userID) {
		return nil, fmt.Errorf("invalid upload token")
	}

	if !s.isValidImageType(header.Filename) {
		return nil, fmt.Errorf("invalid file type. Only JPG, PNG, GIF, WEBP allowed")
	}

	if header.Size > 5*1024*1024 {
		return nil, fmt.Errorf("file too large. Maximum size is 5MB")
	}

	userDir := filepath.Join(s.basePath, strconv.Itoa(int(userID)))
	os.MkdirAll(userDir, 0755)

	timestamp := time.Now().Unix()
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("avatar_%d%s", timestamp, ext)
	filePath := filepath.Join(userDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	relativePath := fmt.Sprintf("%d/%s", userID, filename)
	publicURL := fmt.Sprintf("%s/%s", s.baseURL, relativePath)

	return &SaveImageResponse{
		Filename:  filename,
		FilePath:  relativePath,
		PublicURL: publicURL,
		FileSize:  header.Size,
	}, nil
}

func (s *ObjectStorageService) GenerateImageURL(imagePath string) string {
	if imagePath == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", s.baseURL, imagePath)
}

func (s *ObjectStorageService) DeleteImage(imagePath string) error {
	if imagePath == "" {
		return nil
	}

	fullPath := filepath.Join(s.basePath, imagePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(fullPath)
}

func (s *ObjectStorageService) generateSimpleSignedToken(userID uint, timestamp int64) string {
	message := fmt.Sprintf("%d:%d", userID, timestamp)
	h := hmac.New(sha256.New, []byte(s.secretKey))
	h.Write([]byte(message))
	signature := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%d.%d.%s", userID, timestamp, signature)
}

func (s *ObjectStorageService) validateUploadToken(token string, userID uint) bool {
	log.Printf("[STORAGE_TOKEN_VALIDATION] Validating token for user %d: %s", userID, token)

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		log.Printf("[STORAGE_TOKEN_VALIDATION] Invalid token format - expected 3 parts, got %d", len(parts))
		return false
	}

	tokenUserID, _ := strconv.ParseUint(parts[0], 10, 32)
	timestamp, _ := strconv.ParseInt(parts[1], 10, 64)
	providedSignature := parts[2]

	log.Printf("[STORAGE_TOKEN_VALIDATION] Parsed token - userID: %d, timestamp: %d, signature: %s", tokenUserID, timestamp, providedSignature)

	if uint(tokenUserID) != userID {
		log.Printf("[STORAGE_TOKEN_VALIDATION] User ID mismatch - token: %d, expected: %d", tokenUserID, userID)
		return false
	}

	currentTime := time.Now().Unix()
	if currentTime-timestamp > 15*60 {
		log.Printf("[STORAGE_TOKEN_VALIDATION] Token expired - current: %d, token: %d, diff: %d seconds", currentTime, timestamp, currentTime-timestamp)
		return false
	}

	expectedSignature := s.generateSimpleSignedToken(userID, timestamp)
	expectedParts := strings.Split(expectedSignature, ".")
	if len(expectedParts) != 3 {
		log.Printf("[STORAGE_TOKEN_VALIDATION] Failed to generate expected signature")
		return false
	}

	log.Printf("[STORAGE_TOKEN_VALIDATION] Signature comparison - provided: %s, expected: %s", providedSignature, expectedParts[2])

	isValid := providedSignature == expectedParts[2]
	log.Printf("[STORAGE_TOKEN_VALIDATION] Token validation result: %t", isValid)

	return isValid
}

func (s *ObjectStorageService) isValidImageType(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}

	for _, validExt := range validExts {
		if ext == validExt {
			return true
		}
	}
	return false
}

type UploadTokenResponse struct {
	UploadToken string    `json:"upload_token"`
	Filename    string    `json:"filename"`
	FilePath    string    `json:"file_path"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type SaveImageResponse struct {
	Filename  string `json:"filename"`
	FilePath  string `json:"file_path"`
	PublicURL string `json:"public_url"`
	FileSize  int64  `json:"file_size"`
}

type ProfileImageResponse struct {
	ImageURL  string    `json:"image_url"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}
