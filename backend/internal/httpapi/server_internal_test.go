package httpapi

import (
	"testing"
	"time"
)

func TestCleanupLimitersEvictsExpiredEntries(t *testing.T) {
	server := NewServer(nil, Config{})
	now := time.Now()
	server.limiters["fresh"] = &limiterEntry{lastSeen: now.Add(-1 * time.Minute)}
	server.limiters["stale"] = &limiterEntry{lastSeen: now.Add(-20 * time.Minute)}

	server.cleanupLimiters(now)

	if _, ok := server.limiters["fresh"]; !ok {
		t.Fatalf("expected recent limiter entry to remain")
	}
	if _, ok := server.limiters["stale"]; ok {
		t.Fatalf("expected stale limiter entry to be evicted")
	}
}
