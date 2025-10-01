package cache

import (
	"testing"
	"time"
)

func TestChatResponseCaching(t *testing.T) {
	c := &Cache{items: make(map[string]Item)}
	key := "test-key"

	t.Run("Cache completed response", func(t *testing.T) {
		c.SetChatResponse(key, "Hello, this is a complete response", StatusCompleted, 5*time.Minute)

		text, ok := c.GetChatResponse(key)
		if !ok {
			t.Fatal("Expected cached response to be found")
		}
		if text != "Hello, this is a complete response" {
			t.Errorf("Expected cached text to match, got: %s", text)
		}
	})

	t.Run("Don't cache canceled response", func(t *testing.T) {
		key2 := "test-key-canceled"
		c.SetChatResponse(key2, "Partial response", StatusCanceled, 5*time.Minute)

		_, ok := c.GetChatResponse(key2)
		if ok {
			t.Error("Canceled response should not be cached")
		}
	})

	t.Run("Don't cache error response", func(t *testing.T) {
		key3 := "test-key-error"
		c.SetChatResponse(key3, "Maaf, belum ada jawaban.", StatusCompleted, 5*time.Minute)

		_, ok := c.GetChatResponse(key3)
		if ok {
			t.Error("Default error message should not be cached")
		}
	})

	t.Run("Don't cache empty response", func(t *testing.T) {
		key4 := "test-key-empty"
		c.SetChatResponse(key4, "", StatusCompleted, 5*time.Minute)

		_, ok := c.GetChatResponse(key4)
		if ok {
			t.Error("Empty response should not be cached")
		}
	})

	t.Run("Backward compatibility with string cache", func(t *testing.T) {
		key5 := "test-key-old"
		c.Set(key5, "Old format response", 5*time.Minute)

		text, ok := c.GetChatResponse(key5)
		if !ok {
			t.Fatal("Expected old format response to be found")
		}
		if text != "Old format response" {
			t.Errorf("Expected old format text to match, got: %s", text)
		}
	})

	t.Run("Don't return old error messages", func(t *testing.T) {
		key6 := "test-key-old-error"
		c.Set(key6, "Maaf, belum ada jawaban.", 5*time.Minute)

		_, ok := c.GetChatResponse(key6)
		if ok {
			t.Error("Old error message should not be returned")
		}
	})

	t.Run("Cache invalidation works", func(t *testing.T) {
		key7 := "test-key-invalidate"
		c.SetChatResponse(key7, "Response to be invalidated", StatusCompleted, 5*time.Minute)

		_, ok := c.GetChatResponse(key7)
		if !ok {
			t.Fatal("Response should be cached initially")
		}

		c.InvalidateChatResponse(key7)

		_, ok = c.GetChatResponse(key7)
		if ok {
			t.Error("Response should be invalidated")
		}
	})
}
