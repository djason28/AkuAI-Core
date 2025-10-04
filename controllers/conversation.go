package controllers

import (
	"AkuAI/middleware"
	"AkuAI/models"
	"AkuAI/pkg/cache"
	"AkuAI/pkg/config"
	svc "AkuAI/pkg/services"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CreateOrAddMessage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		var body struct {
			Message        string `json:"message"`
			ConversationID *uint  `json:"conversation_id"`
			RequestImages  bool   `json:"request_images"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Message) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "message is required"})
			return
		}

		bypass := strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "1") ||
			strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "true")
		cacheKeyDup := cache.KeyFromStrings("chat-final", uidStr, strings.ToLower(strings.TrimSpace(body.Message)))
		_, cacheHit := cache.Default().GetChatResponse(cacheKeyDup)
		if !bypass && !cacheHit {
			if !middleware.DuplicateGuard(uidStr, body.Message) {
				c.JSON(http.StatusConflict, gin.H{"msg": "duplicate message"})
				return
			}
		}

		var conv models.Conversation
		if body.ConversationID != nil {
			if err := db.Preload("Messages").Where("id = ? AND user_id = ?", *body.ConversationID, uid).First(&conv).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"msg": "conversation not found"})
				return
			}
		} else {
			title := body.Message
			if len(title) > 30 {
				title = title[:30] + "..."
			}
			conv = models.Conversation{UserID: uint(uid), Title: title}
			if err := db.Create(&conv).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to create conversation"})
				return
			}
		}

		msgUser := models.Message{ConversationID: conv.ID, Sender: "user", Text: body.Message, Timestamp: time.Now()}
		if err := db.Create(&msgUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to save message"})
			return
		}

		var history []svc.ChatMessage
		if len(conv.Messages) > 0 {
			msgs := append([]models.Message(nil), conv.Messages...)
			sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].Timestamp.Before(msgs[j].Timestamp) })
			for _, m := range msgs {
				role := "user"
				if strings.ToLower(m.Sender) == "bot" {
					role = "model"
				}
				history = append(history, svc.ChatMessage{Role: role, Text: m.Text})
			}
		}
		history = append(history, svc.ChatMessage{Role: "user", Text: body.Message})

		release := middleware.AcquireUserSlot(uidStr)
		defer release()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()

		botReply := ""
		// Create cache key - add UIB indicator for better cache management
		cachePrefix := "chat-final"
		message := strings.ToLower(strings.TrimSpace(body.Message))

		// Check if this is UIB-related for cache key differentiation
		geminiService := svc.NewGeminiService()
		if geminiService != nil {
			// Add version identifier to ensure new UIB logic is used
			cachePrefix = "chat-uib-v2"
		}

		key := cache.KeyFromStrings(cachePrefix, uidStr, message)
		if cachedText, ok, cacheInfo := cache.Default().GetChatResponseWithInfo(key); ok {
			botReply = cachedText
			log.Printf("[conversation] 🟢 SERVING FROM CACHE - User: %s, Message: %.50s..., Cache Age: %v",
				uidStr, body.Message, time.Since(cacheInfo.CachedAt).Round(time.Second))
		}
		if strings.TrimSpace(botReply) == "" {
			log.Printf("[conversation] 🔵 GENERATING NEW RESPONSE - User: %s, Message: %.50s...", uidStr, body.Message)

			// Try UIB-enhanced method first for better UIB-related responses
			if resp, err := geminiService.AskCampusWithUIBContext(ctx, history); err == nil && strings.TrimSpace(resp) != "" {
				botReply = resp
				log.Printf("[conversation] ✅ UIB-enhanced response generated successfully")
			} else {
				// Fallback to regular method
				log.Printf("[conversation] ⚠️ UIB-enhanced failed (%v), trying regular method", err)
				if resp, err := geminiService.AskCampusWithChat(ctx, history); err == nil && strings.TrimSpace(resp) != "" {
					botReply = resp
					log.Printf("[conversation] ✅ Regular response generated successfully")
				}
			}
		}
		if strings.TrimSpace(botReply) == "" {
			botReply = svc.AskCampusWithChatLocal(ctx, history)
		}
		if strings.TrimSpace(botReply) != "" {
			cache.Default().SetChatResponse(key, botReply, cache.StatusCompleted, time.Duration(config.ChatCacheTTLSeconds)*time.Second)
		}

		msgBot := models.Message{ConversationID: conv.ID, Sender: "bot", Text: botReply, Timestamp: time.Now()}
		if err := db.Create(&msgBot).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to save bot reply"})
			return
		}

		if err := db.Preload("Messages").First(&conv, conv.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to load messages"})
			return
		}

		var messages []gin.H
		for _, m := range conv.Messages {
			messages = append(messages, gin.H{"id": m.ID, "sender": m.Sender, "text": m.Text, "timestamp": m.Timestamp})
		}

		c.JSON(http.StatusCreated, gin.H{"conversation_id": conv.ID, "messages": messages})
	}
}

func CreateOrAddMessageStream(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.String(http.StatusInternalServerError, "streaming unsupported")
			return
		}

		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		release := middleware.AcquireUserSlot(uidStr)
		defer release()

		var body struct {
			Message        string `json:"message"`
			ConversationID *uint  `json:"conversation_id"`
			RequestImages  bool   `json:"request_images"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Message) == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		bypass := strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "1") ||
			strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "true")
		cacheKeyDup := cache.KeyFromStrings("chat-uib-v2", uidStr, strings.ToLower(strings.TrimSpace(body.Message)))
		_, cacheHit := cache.Default().GetChatResponse(cacheKeyDup)
		if !bypass && !cacheHit {
			if !middleware.DuplicateGuard(uidStr, body.Message) {
				c.Status(http.StatusConflict)
				return
			}
		}

		var conv models.Conversation
		if body.ConversationID != nil {
			if err := db.Preload("Messages").Where("id = ? AND user_id = ?", *body.ConversationID, uid).First(&conv).Error; err != nil {
				c.Status(http.StatusNotFound)
				return
			}
		} else {
			title := body.Message
			if len(title) > 30 {
				title = title[:30] + "..."
			}
			conv = models.Conversation{UserID: uint(uid), Title: title}
			if err := db.Create(&conv).Error; err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
		}

		msgUser := models.Message{ConversationID: conv.ID, Sender: "user", Text: body.Message, Timestamp: time.Now()}
		if err := db.Create(&msgUser).Error; err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(c.Writer, "event: user_saved\n")
		fmt.Fprintf(c.Writer, "data: {\"conversation_id\": %d}\n\n", conv.ID)
		flusher.Flush()

		var history []svc.ChatMessage
		if len(conv.Messages) > 0 {
			msgs := append([]models.Message(nil), conv.Messages...)
			sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].Timestamp.Before(msgs[j].Timestamp) })
			for _, m := range msgs {
				role := "user"
				if strings.ToLower(m.Sender) == "bot" {
					role = "model"
				}
				history = append(history, svc.ChatMessage{Role: role, Text: m.Text})
			}
		}
		history = append(history, svc.ChatMessage{Role: "user", Text: body.Message})

		gsvc := svc.NewGeminiService()
		var full strings.Builder
		gotDelta := false
		onDelta := func(chunk string) {
			esc := strings.ReplaceAll(chunk, "\n", "\\n")
			fmt.Fprintf(c.Writer, "event: delta\n")
			fmt.Fprintf(c.Writer, "data: %s\n\n", esc)
			flusher.Flush()
			full.WriteString(chunk)
			gotDelta = true
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 75*time.Second)
		defer cancel()

		cacheKey := cache.KeyFromStrings("chat-uib-v2", uidStr, strings.ToLower(strings.TrimSpace(body.Message)))
		if v, ok := cache.Default().Get(cacheKey); ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				runes := []rune(s)
				chunk := 28
				for i := 0; i < len(runes); i += chunk {
					end := i + chunk
					if end > len(runes) {
						end = len(runes)
					}
					onDelta(string(runes[i:end]))
					time.Sleep(12 * time.Millisecond)
				}
				gotDelta = true
			}
		}

		if !gotDelta {
			// Use UIB-enhanced method instead of regular StreamCampusWithChat
			if response, err := gsvc.AskCampusWithUIBContext(ctx, history); err == nil && response != "" {
				// Simulate streaming for UIB response
				runes := []rune(response)
				chunk := 28
				for i := 0; i < len(runes); i += chunk {
					end := i + chunk
					if end > len(runes) {
						end = len(runes)
					}
					onDelta(string(runes[i:end]))
					time.Sleep(12 * time.Millisecond)
				}
				gotDelta = true
			} else {
				svc.StreamCampusWithChatLocal(c.Request.Context(), history, onDelta)
			}
		}

		if !gotDelta {
			svc.StreamCampusWithChatLocal(c.Request.Context(), history, onDelta)
		}

		botText := strings.TrimSpace(full.String())
		if botText == "" {
			botText = "Maaf, belum ada jawaban."
			msgBot := models.Message{ConversationID: conv.ID, Sender: "bot", Text: botText, Timestamp: time.Now()}
			_ = db.Create(&msgBot).Error
		} else {
			msgBot := models.Message{ConversationID: conv.ID, Sender: "bot", Text: botText, Timestamp: time.Now()}
			_ = db.Create(&msgBot).Error
			cache.Default().SetChatResponse(cacheKey, botText, cache.StatusCompleted, time.Duration(config.ChatCacheTTLSeconds)*time.Second)
		}

		// Handle image search if requested
		if body.RequestImages {
			fmt.Fprintf(c.Writer, "event: images_searching\n")
			fmt.Fprintf(c.Writer, "data: {\"message\": \"Mencari gambar Universitas International Batam...\"}\n\n")
			flusher.Flush()

			// Use Google Images API with UIB-specific search
			googleImageService := svc.NewGoogleImageService()
			if googleImageService.IsEnabled() {
				searchCtx, searchCancel := context.WithTimeout(ctx, 30*time.Second)
				defer searchCancel()

				// Search for UIB images using Google API
				images, err := googleImageService.SearchImagesForChat(searchCtx, body.Message)
				if err != nil {
					fmt.Fprintf(c.Writer, "event: images_error\n")
					fmt.Fprintf(c.Writer, "data: {\"error\": \"Gagal mencari gambar UIB: %s\"}\n\n", strings.ReplaceAll(err.Error(), `"`, `\"`))
					flusher.Flush()
				} else if len(images) > 0 {
					imagesJSON, _ := json.Marshal(images)
					fmt.Fprintf(c.Writer, "event: images_found\n")
					fmt.Fprintf(c.Writer, "data: {\"images\": %s, \"count\": %d}\n\n", string(imagesJSON), len(images))
					flusher.Flush()
				} else {
					fmt.Fprintf(c.Writer, "event: images_empty\n")
					fmt.Fprintf(c.Writer, "data: {\"message\": \"Tidak ada gambar UIB yang ditemukan\"}\n\n")
					flusher.Flush()
				}
			} else {
				fmt.Fprintf(c.Writer, "event: images_disabled\n")
				fmt.Fprintf(c.Writer, "data: {\"message\": \"Fitur pencarian gambar belum dikonfigurasi\"}\n\n")
				flusher.Flush()
			}
		}

		fmt.Fprintf(c.Writer, "event: done\n")
		fmt.Fprintf(c.Writer, "data: {\"ok\": true}\n\n")
		flusher.Flush()
	}
}

func ListConversations(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		q := strings.TrimSpace(c.Query("q"))

		var convs []models.Conversation
		if err := db.Preload("Messages").Where("user_id = ?", uid).Find(&convs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "db error"})
			return
		}

		filtered := convs[:0]
		if q == "" {
			filtered = convs
		} else {
			p := strings.ToLower(q)
			for _, conv := range convs {
				if strings.Contains(strings.ToLower(conv.Title), p) {
					filtered = append(filtered, conv)
					continue
				}
				matched := false
				for _, m := range conv.Messages {
					if strings.Contains(strings.ToLower(m.Text), p) {
						matched = true
						break
					}
				}
				if matched {
					filtered = append(filtered, conv)
				}
			}
		}

		sort.SliceStable(filtered, func(i, j int) bool {
			li := latestTimestamp(filtered[i].Messages)
			lj := latestTimestamp(filtered[j].Messages)
			return lj.Before(li)
		})

		result := make([]gin.H, 0, len(filtered))
		for _, conv := range filtered {
			createdAt := interface{}(nil)
			if len(conv.Messages) > 0 {
				createdAt = conv.Messages[0].Timestamp
			}
			result = append(result, gin.H{
				"id":             conv.ID,
				"title":          conv.Title,
				"created_at":     createdAt,
				"messages_count": len(conv.Messages),
			})
		}

		c.JSON(http.StatusOK, result)
	}
}

func latestTimestamp(msgs []models.Message) time.Time {
	var t time.Time
	for _, m := range msgs {
		if m.Timestamp.After(t) {
			t = m.Timestamp
		}
	}
	return t
}

func GetConversation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		convIDStr := c.Param("conversation_id")
		cid, _ := strconv.Atoi(convIDStr)

		var conv models.Conversation
		if err := db.Preload("Messages").Where("id = ? AND user_id = ?", cid, uid).First(&conv).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "conversation not found"})
			return
		}

		var messages []gin.H
		for _, m := range conv.Messages {
			messages = append(messages, gin.H{
				"id":        m.ID,
				"sender":    m.Sender,
				"text":      m.Text,
				"timestamp": m.Timestamp,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"conversation_id": conv.ID,
			"title":           conv.Title,
			"messages":        messages,
		})
	}
}

func DeleteConversation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		convIDStr := c.Param("conversation_id")
		cid, _ := strconv.Atoi(convIDStr)

		var conv models.Conversation
		if err := db.Where("id = ? AND user_id = ?", cid, uid).First(&conv).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "conversation not found"})
			return
		}

		if err := db.Delete(&conv).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to delete conversation"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"msg": "conversation deleted"})
	}
}

func DeleteAllConversations(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("user_id = ?", uid).Delete(&models.Conversation{}).Error; err != nil {
				return err
			}
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to delete all conversations"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": "all conversations deleted"})
	}
}
