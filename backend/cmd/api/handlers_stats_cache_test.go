package main

import (
	"testing"
	"time"
)

func TestStatsCache(t *testing.T) {
	var c statsCache
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	if _, ok := c.get(10*time.Second, now); ok {
		t.Fatal("an empty cache answered")
	}
	c.put([]byte(`{"open_issues":1}`), now)
	if got, ok := c.get(10*time.Second, now.Add(9*time.Second)); !ok || string(got) != `{"open_issues":1}` {
		t.Fatalf("within its life: got %q, %v", got, ok)
	}
	if _, ok := c.get(10*time.Second, now.Add(10*time.Second)); ok {
		t.Fatal("answered at the end of its life")
	}
	// A max age of 0 turns off edge caching, not this one: still one read a second.
	if _, ok := c.get(0, now.Add(500*time.Millisecond)); !ok {
		t.Fatal("max age 0 left the database unguarded")
	}
	if _, ok := c.get(0, now.Add(time.Second)); ok {
		t.Fatal("max age 0 kept the answer past the floor")
	}
}
