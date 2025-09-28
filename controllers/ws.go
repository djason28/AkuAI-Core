package controllers

import (
	"AkuAI/models"
	"AkuAI/pkg/config"
	svc "AkuAI/pkg/services"
	tokenstore "AkuAI/pkg/token"
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
		// CORS handled at HTTP level; allow WS here
		return true
	},
}

type wsStartPayload struct {
	Type           string `json:"type"`
	Message        string `json:"message"`
	ConversationID *uint  `json:"conversation_id"`
}

// ChatWS handles WebSocket chat streaming.
// Client protocol (JSON messages):
//
//	-> {type: "start", message: string, conversation_id?: number}
//	<- {type: "user_saved", conversation_id: number}
//	<- {type: "delta", data: string}
//	<- {type: "done", ok: true}
//	<- {type: "error", error: string}
func ChatWS(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Authenticate via ?token=JWT
		tokenStr := strings.TrimSpace(c.Query("token"))
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "missing token query"})
			return
		}
		// Validate JWT (similar to AuthMiddleware)
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

		// Upgrade to websocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[ws] upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Read exactly one start message for simplicity per connection
		conn.SetReadLimit(1 << 20) // 1MB
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			log.Printf("[ws] set read deadline: %v", err)
		}

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

		// Parse user id to uint
		uid64, _ := strconv.ParseUint(userIDStr, 10, 64)
		uid := uint(uid64)

		// Create or find conversation
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

		// Save user message
		msgUser := models.Message{ConversationID: conv.ID, Sender: "user", Text: start.Message, Timestamp: time.Now()}
		if err := db.Create(&msgUser).Error; err != nil {
			_ = conn.WriteJSON(gin.H{"type": "error", "error": "failed to save message"})
			return
		}

		// Notify user_saved
		_ = conn.WriteJSON(gin.H{"type": "user_saved", "conversation_id": conv.ID})

		// Build history (limit recent messages)
		var history []svc.ChatMessage
		if len(conv.Messages) > 0 {
			// sort by time ascending
			// gorm Model has CreatedAt, but we used Timestamp for display; rely on Timestamp order
			// We won't re-sort deeply here to keep it simple.
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
			// no escaping; send raw text as data
			_ = conn.WriteJSON(gin.H{"type": "delta", "data": chunk})
		}

		// Try streaming first
		if _, err := gsvc.StreamCampusWithChat(c.Request.Context(), history, func(s string) {
			full.WriteString(s)
			writeDelta(s)
		}); err != nil {
			log.Printf("[ws] stream failed: %v", err)
			// fallback non-streaming
			if resp, err2 := gsvc.AskCampusWithChat(c.Request.Context(), history); err2 == nil && strings.TrimSpace(resp) != "" {
				full.WriteString(resp)
				// simulate streaming by words
				words := strings.Fields(resp)
				for i, w := range words {
					ch := w
					if i < len(words)-1 {
						ch += " "
					}
					writeDelta(ch)
					time.Sleep(15 * time.Millisecond)
				}
			} else {
				// local fallback
				svc.StreamCampusWithChatLocal(c.Request.Context(), history, func(s string) {
					full.WriteString(s)
					writeDelta(s)
				})
			}
		}

		botText := strings.TrimSpace(full.String())
		if botText == "" {
			botText = "Maaf, belum ada jawaban."
		}
		// persist bot message (best-effort)
		_ = db.Create(&models.Message{ConversationID: conv.ID, Sender: "bot", Text: botText, Timestamp: time.Now()}).Error

		// done
		_ = conn.WriteJSON(gin.H{"type": "done", "ok": true})
	}
}
