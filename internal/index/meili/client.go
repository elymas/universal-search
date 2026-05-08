// Package meili provides the Meilisearch sub-client for the hybrid index layer.
// SPEC-IDX-001 REQ-IDX-003 (scope item g).
package meili

import (
	"context"
	"fmt"

	"github.com/meilisearch/meilisearch-go"
)

// Config holds construction parameters for the Meilisearch client.
type Config struct {
	// Endpoint is the Meilisearch HTTP endpoint (e.g., "http://localhost:7700").
	Endpoint string
	// MasterKey is the Meilisearch master API key.
	MasterKey string
	// IndexName is the Meilisearch index to use (default "usearch_docs").
	IndexName string
}

// IndexSettings defines the schema configuration for a Meilisearch index.
type IndexSettings struct {
	SearchableAttributes []string
	FilterableAttributes []string
	DistinctAttribute    string
}

// Document is a single Meilisearch document with mandatory doc_id primary key.
type Document map[string]any

// SearchOptions encodes retrieval parameters for a Meilisearch text search.
type SearchOptions struct {
	Filter     string
	Limit      int64
	Attributes []string
}

// Client wraps the Meilisearch HTTP client with index-layer semantics.
type Client struct {
	ms        meilisearch.ServiceManager
	indexName string
}

// NewClient creates a Meilisearch client connected to cfg.Endpoint.
func NewClient(cfg Config) (*Client, error) {
	if cfg.IndexName == "" {
		cfg.IndexName = "usearch_docs"
	}

	ms := meilisearch.New(cfg.Endpoint, meilisearch.WithAPIKey(cfg.MasterKey))
	return &Client{ms: ms, indexName: cfg.IndexName}, nil
}

// EnsureIndex creates the Meilisearch index idempotently with the given settings.
// If the index already exists this is a no-op; settings are updated if they differ.
func (c *Client) EnsureIndex(ctx context.Context, name string, settings IndexSettings) error {
	// Check if index exists.
	_, err := c.ms.GetIndex(name)
	if err != nil {
		// Index does not exist — create it.
		task, createErr := c.ms.CreateIndex(&meilisearch.IndexConfig{
			Uid:        name,
			PrimaryKey: "doc_id",
		})
		if createErr != nil {
			return fmt.Errorf("meili: create index %q: %w", name, createErr)
		}
		if _, waitErr := c.ms.WaitForTask(task.TaskUID, 0); waitErr != nil {
			return fmt.Errorf("meili: wait for index creation %q: %w", name, waitErr)
		}
	}

	// Update settings.
	idx := c.ms.Index(name)
	distinctAttr := settings.DistinctAttribute

	settingsUpdate := &meilisearch.Settings{
		SearchableAttributes: settings.SearchableAttributes,
		FilterableAttributes: settings.FilterableAttributes,
		DistinctAttribute:    &distinctAttr,
	}
	task, err := idx.UpdateSettings(settingsUpdate)
	if err != nil {
		return fmt.Errorf("meili: update settings for %q: %w", name, err)
	}
	if _, err := c.ms.WaitForTask(task.TaskUID, 0); err != nil {
		return fmt.Errorf("meili: wait for settings update %q: %w", name, err)
	}
	return nil
}

// AddDocuments fires an async document add request (fire-and-forget in production).
// Returns the TaskInfo so callers (tests) can call WaitForTask to synchronise.
// D12: production callers do NOT block on indexing; tests MUST call WaitForTask.
func (c *Client) AddDocuments(ctx context.Context, name string, docs []Document) (*meilisearch.TaskInfo, error) {
	idx := c.ms.Index(name)
	// Convert []Document to []map[string]any for the SDK.
	items := make([]map[string]any, len(docs))
	for i, d := range docs {
		items[i] = map[string]any(d)
	}
	pk := "doc_id"
	task, err := idx.AddDocuments(items, &meilisearch.DocumentOptions{PrimaryKey: &pk})
	if err != nil {
		return nil, fmt.Errorf("meili: add documents to %q: %w", name, err)
	}
	return task, nil
}

// WaitForTask blocks until a Meilisearch background task completes.
// Used in tests to synchronise after AddDocuments.
func (c *Client) WaitForTask(taskUID int64) error {
	_, err := c.ms.WaitForTask(taskUID, 0)
	return err
}

// Search performs a full-text search on the named Meilisearch index.
func (c *Client) Search(ctx context.Context, name string, query string, opts SearchOptions) ([]Document, error) {
	idx := c.ms.Index(name)

	params := &meilisearch.SearchRequest{
		Query:  query,
		Limit:  opts.Limit,
		Filter: opts.Filter,
	}
	if opts.Limit == 0 {
		params.Limit = 50
	}

	result, err := idx.Search(query, params)
	if err != nil {
		return nil, fmt.Errorf("meili: search %q: %w", name, err)
	}

	docs := make([]Document, 0, len(result.Hits))
	for _, hit := range result.Hits {
		doc := make(Document, len(hit))
		if err := hit.DecodeInto(&doc); err == nil {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

// GetSettings returns the current settings for a Meilisearch index.
// Used in tests to verify settings were applied correctly.
func (c *Client) GetSettings(name string) (*meilisearch.Settings, error) {
	idx := c.ms.Index(name)
	settings, err := idx.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("meili: get settings %q: %w", name, err)
	}
	return settings, nil
}

// IndexName returns the configured Meilisearch index name.
func (c *Client) IndexName() string { return c.indexName }

// Close is a no-op for the HTTP client; present for interface symmetry.
func (c *Client) Close() error { return nil }
