package controllers

import (
	"AkuAI/middleware"
	"AkuAI/models"
	"AkuAI/pkg/cache"
	"AkuAI/pkg/config"
	svc "AkuAI/pkg/services"
	"context"
	"fmt"
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
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Message) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "message is required"})
			return
		}

		// Duplicate guard with bypass/header and cache-aware allowance
		bypass := strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "1") ||
			strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "true")
		cacheKeyDup := cache.KeyFromStrings("chat-final", uidStr, strings.ToLower(strings.TrimSpace(body.Message)))
		_, cacheHit := cache.Default().Get(cacheKeyDup)
		if !bypass && !cacheHit {
			if !middleware.DuplicateGuard(uidStr, body.Message) {
				c.JSON(http.StatusConflict, gin.H{"msg": "duplicate message"})
				return
			}
		}

		// Create or find conversation
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

		// Save user message
		msgUser := models.Message{ConversationID: conv.ID, Sender: "user", Text: body.Message, Timestamp: time.Now()}
		if err := db.Create(&msgUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to save message"})
			return
		}

		// Build history
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

		// Concurrency guard + timeout for upstream
		release := middleware.AcquireUserSlot(uidStr)
		defer release()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()

		// Cache lookup
		botReply := ""
		key := cache.KeyFromStrings("chat-final", uidStr, strings.ToLower(strings.TrimSpace(body.Message)))
		if v, ok := cache.Default().Get(key); ok {
			if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
				botReply = s
			}
		}
		if strings.TrimSpace(botReply) == "" {
			if resp, err := svc.NewGeminiService().AskCampusWithChat(ctx, history); err == nil && strings.TrimSpace(resp) != "" {
				botReply = resp
			}
		}
		if strings.TrimSpace(botReply) == "" {
			botReply = svc.AskCampusWithChatLocal(ctx, history)
		}
		if strings.TrimSpace(botReply) != "" {
			cache.Default().Set(key, botReply, time.Duration(config.ChatCacheTTLSeconds)*time.Second)
		}

		// Save bot message
		msgBot := models.Message{ConversationID: conv.ID, Sender: "bot", Text: botReply, Timestamp: time.Now()}
		if err := db.Create(&msgBot).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to save bot reply"})
			return
		}

		// Reload conversation with messages
		if err := db.Preload("Messages").First(&conv, conv.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to load messages"})
			return
		}

		// Map messages
		var messages []gin.H
		for _, m := range conv.Messages {
			messages = append(messages, gin.H{"id": m.ID, "sender": m.Sender, "text": m.Text, "timestamp": m.Timestamp})
		}

		c.JSON(http.StatusCreated, gin.H{"conversation_id": conv.ID, "messages": messages})
	}
}

// CreateOrAddMessageStream streams the bot reply using SSE.
// Client will receive:
// - event: user_saved (once) with conversation_id
// - event: delta (multiple) with partial text chunks
// - event: done (once) when finished
func CreateOrAddMessageStream(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no") // nginx buffering off

		// Ensure streaming flush is available
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.String(http.StatusInternalServerError, "streaming unsupported")
			return
		}

		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		// Concurrency guard per user
		release := middleware.AcquireUserSlot(uidStr)
		defer release()

		var body struct {
			Message        string `json:"message"`
			ConversationID *uint  `json:"conversation_id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Message) == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		// Duplicate guard (after binding body) with bypass and cache-aware allowance
		bypass := strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "1") ||
			strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Bypass-Duplicate")), "true")
		cacheKeyDup := cache.KeyFromStrings("chat-final", uidStr, strings.ToLower(strings.TrimSpace(body.Message)))
		_, cacheHit := cache.Default().Get(cacheKeyDup)
		if !bypass && !cacheHit {
			if !middleware.DuplicateGuard(uidStr, body.Message) {
				c.Status(http.StatusConflict)
				return
			}
		}

		// create or find conversation
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

		// save user message
		msgUser := models.Message{ConversationID: conv.ID, Sender: "user", Text: body.Message, Timestamp: time.Now()}
		if err := db.Create(&msgUser).Error; err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		// Notify client that user message saved and provide conversation_id
		fmt.Fprintf(c.Writer, "event: user_saved\n")
		fmt.Fprintf(c.Writer, "data: {\"conversation_id\": %d}\n\n", conv.ID)
		flusher.Flush()

		// Prepare chat history for contextual streaming
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

		// stream bot reply using Gemini service with chat history
		gsvc := svc.NewGeminiService()
		var full strings.Builder
		gotDelta := false
		onDelta := func(chunk string) {
			// forward partial text as SSE delta event
			esc := strings.ReplaceAll(chunk, "\n", "\\n")
			fmt.Fprintf(c.Writer, "event: delta\n")
			fmt.Fprintf(c.Writer, "data: %s\n\n", esc)
			flusher.Flush()
			full.WriteString(chunk)
			gotDelta = true
		}

		// Context timeout to avoid hanging streams
		ctx, cancel := context.WithTimeout(c.Request.Context(), 75*time.Second)
		defer cancel()

		// Cache check first
		cacheKey := cache.KeyFromStrings("chat-final", uidStr, strings.ToLower(strings.TrimSpace(body.Message)))
		if v, ok := cache.Default().Get(cacheKey); ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				// Stream cached text while PRESERVING whitespace and newlines
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
			if _, err := gsvc.StreamCampusWithChat(ctx, history, onDelta); err != nil {
				// fallback to local streaming when Gemini fails (quota/overload/etc.)
				svc.StreamCampusWithChatLocal(c.Request.Context(), history, onDelta)
			}
		}

		// If no chunks were received from Gemini (empty or silent success), fall back to local mock
		if !gotDelta {
			svc.StreamCampusWithChatLocal(c.Request.Context(), history, onDelta)
		}

		botText := strings.TrimSpace(full.String())
		if botText == "" {
			botText = "Maaf, belum ada jawaban."
		}

		// persist bot message (best-effort) and set cache
		msgBot := models.Message{ConversationID: conv.ID, Sender: "bot", Text: botText, Timestamp: time.Now()}
		_ = db.Create(&msgBot).Error
		if botText != "" {
			cache.Default().Set(cacheKey, botText, time.Duration(config.ChatCacheTTLSeconds)*time.Second)
		}

		// final done event
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

		// filter by q (in-memory)
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

		// sort by latest message timestamp desc
		sort.SliceStable(filtered, func(i, j int) bool {
			li := latestTimestamp(filtered[i].Messages)
			lj := latestTimestamp(filtered[j].Messages)
			return lj.Before(li) // want descending
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

		// Delete cascade is enabled; can delete conversation and messages will be removed
		if err := db.Delete(&conv).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "failed to delete conversation"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"msg": "conversation deleted"})
	}
}

// DeleteAllConversations deletes all conversations for the authenticated user
func DeleteAllConversations(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, _ := c.Get(middleware.ContextUserIDKey)
		uidStr := userIDStr.(string)
		uid, _ := strconv.Atoi(uidStr)

		// Use a transaction to safely delete user's conversations and cascading messages
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
