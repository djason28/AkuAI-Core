package cache

import (
	"testing"
	"time"
)

func TestSetGetAndExpire(t *testing.T) {
	c := Default()
	key := KeyFromStrings("unit", "expire", time.Now().String())

	// ensure no value
	if _, ok := c.Get(key); ok {
		t.Fatalf("expected no value initially")
	}

	// set with ttl
	c.Set(key, "hello", 50*time.Millisecond)
	if v, ok := c.Get(key); !ok || v.(string) != "hello" {
		t.Fatalf("expected value 'hello', got %v ok=%v", v, ok)
	}

	// wait for expiry
	time.Sleep(80 * time.Millisecond)
	if _, ok := c.Get(key); ok {
		t.Fatalf("expected expired value to be gone")
	}
}

func TestDelete(t *testing.T) {
	c := Default()
	key := KeyFromStrings("unit", "delete", time.Now().String())
	c.Set(key, 42, time.Second)
	if v, ok := c.Get(key); !ok || v.(int) != 42 {
		t.Fatalf("expected 42 present before delete, got %v ok=%v", v, ok)
	}
	c.Delete(key)
	if _, ok := c.Get(key); ok {
		t.Fatalf("expected deleted value to be absent")
	}
}

func TestKeyFromStringsStability(t *testing.T) {
	k1 := KeyFromStrings("a", "b", "c")
	k2 := KeyFromStrings("a", "b", "c")
	if k1 != k2 {
		t.Fatalf("expected same inputs to yield same key")
	}
	k3 := KeyFromStrings("a", "b", "d")
	if k1 == k3 {
		t.Fatalf("expected different inputs to yield different key")
	}
}
