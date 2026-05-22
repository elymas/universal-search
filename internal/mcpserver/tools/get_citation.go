package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/elymas/universal-search/pkg/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrCitationNotFound is returned when a doc_id is not found in the cache.
var ErrCitationNotFound = errors.New("citation not found")

// DocCache is a thread-safe in-memory cache for NormalizedDoc lookup by DocID.
// V1: uses in-memory fanout result cache for doc_id resolution.
// TBD: IDX-001 GetByDocID integration.
type DocCache struct {
	mu   sync.RWMutex
	docs map[string]types.NormalizedDoc
}

// NewDocCache creates an empty DocCache.
func NewDocCache() *DocCache {
	return &DocCache{docs: make(map[string]types.NormalizedDoc)}
}

// Store adds documents to the cache.
func (c *DocCache) Store(docs []types.NormalizedDoc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, doc := range docs {
		c.docs[doc.ID] = doc
	}
}

// Get retrieves a document by ID. Returns false if not found.
func (c *DocCache) Get(docID string) (types.NormalizedDoc, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	doc, ok := c.docs[docID]
	return doc, ok
}

// GetCitationHandler returns a ToolHandlerFor that resolves doc_id to citation.
func GetCitationHandler(cache *DocCache) func(_ context.Context, _ *mcp.CallToolRequest, input GetCitationInput) (*mcp.CallToolResult, GetCitationOutput, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, input GetCitationInput) (*mcp.CallToolResult, GetCitationOutput, error) {
		doc, ok := cache.Get(input.DocID)
		if !ok {
			return nil, GetCitationOutput{}, fmt.Errorf("%w: %s", ErrCitationNotFound, input.DocID)
		}

		return nil, GetCitationOutput{
			DocID:       doc.ID,
			Title:       doc.Title,
			URL:         doc.URL,
			Source:      doc.SourceID,
			Snippet:     doc.Snippet,
			Score:       doc.Score,
			RetrievedAt: doc.RetrievedAt,
		}, nil
	}
}

// GetCitationTool returns the MCP tool definition for get_citation.
func GetCitationTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_citation",
		Description: "Resolve a doc_id to its full citation details",
	}
}

// StoreDocs is a convenience to add fanout results to the cache.
func StoreDocs(cache *DocCache, docs []types.NormalizedDoc) {
	cache.Store(docs)
}
