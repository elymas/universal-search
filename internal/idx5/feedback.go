package idx5

import (
	"container/list"
	"sync"
	"time"
)

// FeedbackResult is the outcome of a feedback operation.
type FeedbackResult struct {
	Status        string `json:"status"`
	UnmappedCount int    `json:"unmapped_count,omitempty"`
}

// FeedbackStorer is the interface for marking docs as stale.
type FeedbackStorer interface {
	MarkStale(docID, teamID string) error
}

// FeedbackHandler processes thumbs-down feedback.
// REQ-IDX5-008: POST /feedback {score: -1} -> force_stale=TRUE.
type FeedbackHandler struct {
	store       FeedbackStorer
	lru         *RequestLRU
	unmappedMu  sync.Mutex
	unmappedCount int
}

// NewFeedbackHandler creates a new FeedbackHandler.
func NewFeedbackHandler(store FeedbackStorer, lru *RequestLRU) *FeedbackHandler {
	return &FeedbackHandler{store: store, lru: lru}
}

// HandleFeedback processes a feedback request.
// REQ-IDX5-008: single thumbs-down -> immediate force_stale.
// REQ-IDX5-007: team boundary enforced.
func (fh *FeedbackHandler) HandleFeedback(requestID, teamID string, score int) FeedbackResult {
	if score >= 0 {
		return FeedbackResult{Status: "ignored"}
	}

	// Look up the request mapping
	mapping, ok := fh.lru.Get(requestID)
	if !ok {
		fh.unmappedMu.Lock()
		fh.unmappedCount++
		fh.unmappedMu.Unlock()
		return FeedbackResult{Status: "unmapped", UnmappedCount: 1}
	}

	// REQ-IDX5-007: tenant boundary check
	if mapping.TeamID != teamID {
		return FeedbackResult{Status: "tenant_mismatch"}
	}

	// Mark as stale (idempotent)
	if err := fh.store.MarkStale(mapping.DocID, teamID); err != nil {
		return FeedbackResult{Status: "error"}
	}

	return FeedbackResult{Status: "marked_stale"}
}

// RequestMapping stores the request_id -> (doc_id, team_id) mapping.
type RequestMapping struct {
	DocID  string
	TeamID string
}

// RequestLRU is a time-bounded LRU cache for request_id -> RequestMapping.
// REQ-IDX5-008: 24h TTL, in-memory.
type RequestLRU struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	items    map[string]*list.Element
	order    *list.List
}

type lruEntry struct {
	key       string
	mapping   RequestMapping
	expiresAt time.Time
}

// NewRequestLRU creates a new LRU with the given TTL in seconds.
func NewRequestLRU(ttlSeconds int) *RequestLRU {
	return &RequestLRU{
		capacity: 100000, // ~10MB for 100k entries
		ttl:      time.Duration(ttlSeconds) * time.Second,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Set stores a request_id -> (doc_id, team_id) mapping.
func (l *RequestLRU) Set(requestID, docID, teamID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Update if exists
	if elem, ok := l.items[requestID]; ok {
		entry := elem.Value.(*lruEntry)
		entry.mapping = RequestMapping{DocID: docID, TeamID: teamID}
		entry.expiresAt = time.Now().Add(l.ttl)
		l.order.MoveToFront(elem)
		return
	}

	// Evict oldest if at capacity
	if l.order.Len() >= l.capacity {
		oldest := l.order.Back()
		if oldest != nil {
			l.order.Remove(oldest)
			delete(l.items, oldest.Value.(*lruEntry).key)
		}
	}

	entry := &lruEntry{
		key:       requestID,
		mapping:   RequestMapping{DocID: docID, TeamID: teamID},
		expiresAt: time.Now().Add(l.ttl),
	}
	elem := l.order.PushFront(entry)
	l.items[requestID] = elem
}

// Get retrieves a mapping by request_id. Returns false if expired or missing.
func (l *RequestLRU) Get(requestID string) (RequestMapping, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	elem, ok := l.items[requestID]
	if !ok {
		return RequestMapping{}, false
	}

	entry := elem.Value.(*lruEntry)
	if time.Now().After(entry.expiresAt) {
		l.order.Remove(elem)
		delete(l.items, requestID)
		return RequestMapping{}, false
	}

	l.order.MoveToFront(elem)
	return entry.mapping, true
}
