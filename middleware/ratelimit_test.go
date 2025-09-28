package middleware

import (
	"testing"
	"time"
)

func TestDuplicateGuard(t *testing.T) {
	// speed up TTL for test
	SetDuplicateTTL(50 * time.Millisecond)
	uid := "user-123"
	text := "Hello"

	// First call should allow
	if ok := DuplicateGuard(uid, text); !ok {
		t.Fatalf("expected first call to pass duplicate guard")
	}
	// Immediate repeat should block
	if ok := DuplicateGuard(uid, text); ok {
		t.Fatalf("expected immediate duplicate to be blocked")
	}
	// Different text should pass even within TTL
	if ok := DuplicateGuard(uid, text+"!"); !ok {
		t.Fatalf("expected different text to pass within TTL")
	}
	// After TTL, same text should pass
	time.Sleep(70 * time.Millisecond)
	if ok := DuplicateGuard(uid, text); !ok {
		t.Fatalf("expected same text to pass after TTL")
	}
}
