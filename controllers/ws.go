package controllers

import (
	"AkuAI/middleware"
	"AkuAI/models"
	"AkuAI/pkg/cache"
	"AkuAI/pkg/config"
	svc "AkuAI/pkg/services"
	tokenstore "AkuAI/pkg/token"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsStartPayload struct {
	Type           string `json:"type"`
	Message        string `json:"message"`
	ConversationID *uint  `json:"conversation_id"`
	RequestImages  bool   `json:"request_images,omitempty"`
}

func ChatWS(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := strings.TrimSpace(c.Query("token"))
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "missing token query"})
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return []byte(config.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "invalid token"})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "invalid token claims"})
			return
		}
		jtiVal, _ := claims["jti"].(string)
		if tokenstore.IsRevoked(jtiVal) {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "Token has been revoked (logout)"})
			return
		}
		var userIDStr string
		if sub, ok := claims["sub"].(string); ok {
			userIDStr = sub
		} else if subf, ok := claims["sub"].(float64); ok {
			userIDStr = strconv.Itoa(int(subf))
		}
		if userIDStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "invalid subject in token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[ws] upgrade error: %v", err)
			return
		}
		defer conn.Close()

		conn.SetReadLimit(1 << 20)
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		})

		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ws] read message error: %v", err)
			return
		}
		var start wsStartPayload
		if err := json.Unmarshal(msgBytes, &start); err != nil || strings.ToLower(start.Type) != "start" || strings.TrimSpace(start.Message) == "" {
			_ = conn.WriteJSON(gin.H{"type": "error", "error": "invalid start payload"})
			return
		}

		uid64, _ := strconv.ParseUint(userIDStr, 10, 64)
		uid := uint(uid64)

		var conv models.Conversation
		if start.ConversationID != nil {
			if err := db.Preload("Messages").Where("id = ? AND user_id = ?", *start.ConversationID, uid).First(&conv).Error; err != nil {
				_ = conn.WriteJSON(gin.H{"type": "error", "error": "conversation not found"})
				return
			}
		} else {
			title := start.Message
			if len(title) > 30 {
				title = title[:30] + "..."
			}
			conv = models.Conversation{UserID: uid, Title: title}
			if err := db.Create(&conv).Error; err != nil {
				_ = conn.WriteJSON(gin.H{"type": "error", "error": "failed to create conversation"})
				return
			}
		}

		release := middleware.AcquireUserSlot(userIDStr)
		defer release()

		msgUser := models.Message{ConversationID: conv.ID, Sender: "user", Text: start.Message, Timestamp: time.Now()}
		if err := db.Create(&msgUser).Error; err != nil {
			_ = conn.WriteJSON(gin.H{"type": "error", "error": "failed to save message"})
			return
		}

		_ = conn.WriteJSON(gin.H{"type": "user_saved", "conversation_id": conv.ID})

		var history []svc.ChatMessage
		if len(conv.Messages) > 0 {
			for _, m := range conv.Messages {
				role := "user"
				if strings.ToLower(m.Sender) == "bot" {
					role = "model"
				}
				text := m.Text
				if len(text) > 200 {
					text = text[:200] + "..."
				}
				history = append(history, svc.ChatMessage{Role: role, Text: text})
			}
			if len(history) > 6 {
				history = history[len(history)-6:]
			}
		}
		history = append(history, svc.ChatMessage{Role: "user", Text: start.Message})

		gsvc := svc.NewGeminiService()
		var full strings.Builder

		writeDelta := func(chunk string) {
			_ = conn.WriteJSON(gin.H{"type": "delta", "data": chunk})
		}

		parentCtx, cancelTimeout := context.WithTimeout(c.Request.Context(), 75*time.Second)
		ctx, cancel := context.WithCancel(parentCtx)
		defer func() {
			cancel()
			cancelTimeout()
		}()

		stopCh := make(chan struct{})
		readErrCh := make(chan error, 1)
		go func() {
			defer close(readErrCh)
			for {
				if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
					readErrCh <- err
					return
				}
				mt, msg, err := conn.ReadMessage()
				if err != nil {
					readErrCh <- err
					return
				}
				if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
					continue
				}
				var obj struct {
					Type string `json:"type"`
				}
				_ = json.Unmarshal(msg, &obj)
				if strings.ToLower(strings.TrimSpace(obj.Type)) == "stop" {
					select {
					case <-stopCh:
					default:
						close(stopCh)
					}
					return
				}
			}
		}()

		isStopped := func() bool {
			select {
			case <-stopCh:
				return true
			default:
				return false
			}
		}

		ck := cache.KeyFromStrings("chat-final", userIDStr, strings.ToLower(strings.TrimSpace(start.Message)))
		if cachedText, ok, cacheInfo := cache.Default().GetChatResponseWithInfo(ck); ok {
			log.Printf("[ws] 🟢 SERVING FROM CACHE - User: %s, Message: %.50s..., Cache Age: %v",
				userIDStr, start.Message, time.Since(cacheInfo.CachedAt).Round(time.Second))

			runes := []rune(cachedText)
			chunk := 32
			for i := 0; i < len(runes); i += chunk {
				if isStopped() {
					break
				}
				end := i + chunk
				if end > len(runes) {
					end = len(runes)
				}

				if end < len(runes) && end > i {
					for j := end; j >= i+chunk-5 && j < len(runes); j-- {
						if runes[j] == ' ' || runes[j] == '\n' || runes[j] == '.' || runes[j] == ',' || runes[j] == ':' {
							end = j + 1
							break
						}
					}
				}

				chunkText := string(runes[i:end])
				full.WriteString(chunkText)
				writeDelta(chunkText)
				time.Sleep(12 * time.Millisecond)
			}
		}

		if full.Len() == 0 && !isStopped() {
			log.Printf("[ws] 🔵 GENERATING NEW RESPONSE - User: %s, Message: %.50s...", userIDStr, start.Message)

			if _, err := gsvc.StreamCampusWithChat(ctx, history, func(s string) {
				if isStopped() {
					return
				}
				full.WriteString(s)
				writeDelta(s)
			}); err != nil && !isStopped() {
				log.Printf("[ws] stream failed: %v", err)
				if resp, err2 := gsvc.AskCampusWithChat(ctx, history); err2 == nil && strings.TrimSpace(resp) != "" && !isStopped() {
					full.WriteString(resp)
					runes := []rune(resp)
					chunk := 28
					for i := 0; i < len(runes); i += chunk {
						if isStopped() {
							break
						}
						end := i + chunk
						if end > len(runes) {
							end = len(runes)
						}
						chunkText := string(runes[i:end])
						writeDelta(chunkText)
						time.Sleep(15 * time.Millisecond)
					}
				} else if !isStopped() {
					svc.StreamCampusWithChatLocal(ctx, history, func(s string) {
						if isStopped() {
							return
						}
						full.WriteString(s)
						writeDelta(s)
					})
				}
			}
		}

		if isStopped() {
			cancel()
		}

		botText := strings.TrimSpace(full.String())

		if isStopped() {
			cache.Default().InvalidateChatResponse(ck)

			if botText == "" {
				_ = conn.WriteJSON(gin.H{"type": "done", "ok": true, "stopped": true})
				return
			}
			_ = db.Create(&models.Message{ConversationID: conv.ID, Sender: "bot", Text: botText, Timestamp: time.Now()}).Error
			_ = conn.WriteJSON(gin.H{"type": "done", "ok": true, "stopped": true})
			return
		}

		if botText == "" {
			botText = "Maaf, belum ada jawaban."
			_ = db.Create(&models.Message{ConversationID: conv.ID, Sender: "bot", Text: botText, Timestamp: time.Now()}).Error
		} else {
			_ = db.Create(&models.Message{ConversationID: conv.ID, Sender: "bot", Text: botText, Timestamp: time.Now()}).Error
			cache.Default().SetChatResponse(ck, botText, cache.StatusCompleted, time.Duration(config.ChatCacheTTLSeconds)*time.Second)
		}

		// Handle image search if requested
		if start.RequestImages {
			_ = conn.WriteJSON(gin.H{"type": "images_searching", "message": "Mencari gambar yang relevan..."})

			imageService := svc.NewGoogleImageService()
			if imageService.IsEnabled() {
				searchCtx, searchCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer searchCancel()

				// Search for images based on the message context
				images, err := imageService.SearchImagesForChat(searchCtx, start.Message)
				if err != nil {
					_ = conn.WriteJSON(gin.H{"type": "images_error", "error": "Gagal mencari gambar: " + err.Error()})
				} else if len(images) > 0 {
					_ = conn.WriteJSON(gin.H{"type": "images_found", "images": images, "count": len(images)})
				} else {
					_ = conn.WriteJSON(gin.H{"type": "images_empty", "message": "Tidak ada gambar yang ditemukan"})
				}
			} else {
				_ = conn.WriteJSON(gin.H{"type": "images_disabled", "message": "Fitur pencarian gambar belum dikonfigurasi"})
			}
		}

		_ = conn.WriteJSON(gin.H{"type": "done", "ok": true})
	}
}
