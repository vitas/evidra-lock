package mcpserver

import (
	"fmt"
	"sync"
	"time"
)

// DenyCache tracks recently denied intent keys to prevent infinite retry loops.
// It is in-memory, per-process. No persistence.
type DenyCache struct {
	mu      sync.Mutex
	entries map[string]*DenyEntry
	ttl     time.Duration
}

// DenyEntry records metadata about a denied intent.
type DenyEntry struct {
	DeniedAt  time.Time
	DenyCount int
	Reason    string
	RuleIDs   []string
	EventID   string
}

// NewDenyCache creates a new deny cache with the given TTL.
func NewDenyCache(ttl time.Duration) *DenyCache {
	return &DenyCache{
		entries: make(map[string]*DenyEntry),
		ttl:     ttl,
	}
}

// CheckDenyLoop returns an ErrorSummary if the intent key is in the cache
// and not expired. Returns nil if the key is not cached or has expired.
func (dc *DenyCache) CheckDenyLoop(intentKey string) *ErrorSummary {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	entry, ok := dc.entries[intentKey]
	if !ok {
		return nil
	}

	if time.Since(entry.DeniedAt) > dc.ttl {
		delete(dc.entries, intentKey)
		return nil
	}

	return &ErrorSummary{
		Code:    ErrCodeStopAfterDeny,
		Message: fmt.Sprintf("This intent was already denied (count: %d). Original reason: %s", entry.DenyCount, entry.Reason),
	}
}

// RecordDeny upserts a deny entry for the given intent key.
func (dc *DenyCache) RecordDeny(intentKey, reason string, ruleIDs []string, eventID string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if entry, ok := dc.entries[intentKey]; ok {
		entry.DenyCount++
		entry.DeniedAt = time.Now()
		return
	}

	dc.entries[intentKey] = &DenyEntry{
		DeniedAt:  time.Now(),
		DenyCount: 1,
		Reason:    reason,
		RuleIDs:   ruleIDs,
		EventID:   eventID,
	}
}

// ClearIntent removes a deny entry (called on allow).
func (dc *DenyCache) ClearIntent(intentKey string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	delete(dc.entries, intentKey)
}

// Cleanup removes all expired entries.
func (dc *DenyCache) Cleanup() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	now := time.Now()
	for k, e := range dc.entries {
		if now.Sub(e.DeniedAt) > dc.ttl {
			delete(dc.entries, k)
		}
	}
}
