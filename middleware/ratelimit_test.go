package middleware

import (
	"testing"
	"time"
)

func TestDuplicateGuard(t *testing.T) {
	SetDuplicateTTL(50 * time.Millisecond)
	uid := "user-123"
	text := "Hello"

	if ok := DuplicateGuard(uid, text); !ok {
		t.Fatalf("expected first call to pass duplicate guard")
	}
	if ok := DuplicateGuard(uid, text); ok {
		t.Fatalf("expected immediate duplicate to be blocked")
	}
	if ok := DuplicateGuard(uid, text+"!"); !ok {
		t.Fatalf("expected different text to pass within TTL")
	}
	time.Sleep(70 * time.Millisecond)
	if ok := DuplicateGuard(uid, text); !ok {
		t.Fatalf("expected same text to pass after TTL")
	}
}
