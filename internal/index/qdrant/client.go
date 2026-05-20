// Package qdrant provides the Qdrant sub-client for the hybrid index layer.
// SPEC-IDX-001 REQ-IDX-002 (scope item f).
package qdrant

import (
	"context"
	"fmt"

	qdrantpb "github.com/qdrant/go-client/qdrant"
)

// Config holds construction parameters for the Qdrant client.
type Config struct {
	// Endpoint is the Qdrant gRPC endpoint (e.g., "localhost:6334").
	Endpoint string
	// APIKey is optional; used for Qdrant Cloud.
	APIKey string
	// CollectionName is the Qdrant collection to use (default "usearch_docs").
	CollectionName string
}

// Point is a single vector point for upsert.
type Point struct {
	// ID is the 16-hex doc_id; we left-pad to UUID form for Qdrant.
	ID      string
	Vector  []float32
	Payload map[string]any
}

// ScoredPoint is a retrieval result from Qdrant.
type ScoredPoint struct {
	ID      string
	Score   float32
	Payload map[string]any
}

// Filter encodes simple payload filter conditions.
type Filter struct {
	SourceID string
	Lang     string
	TeamID   string
	DocType  string
}

// Client wraps the Qdrant gRPC client with index-layer semantics.
//
// @MX:NOTE: [AUTO] UUID-shaped point_id transformation: 16-hex doc_id is left-padded to 32 hex chars and RFC 4122 dashes inserted.
// @MX:SPEC: SPEC-IDX-001
type Client struct {
	client *qdrantpb.Client
	cfg    Config
}

// NewClient creates a Qdrant gRPC client connected to cfg.Endpoint.
func NewClient(cfg Config) (*Client, error) {
	if cfg.CollectionName == "" {
		cfg.CollectionName = "usearch_docs"
	}

	c, err := qdrantpb.NewClient(&qdrantpb.Config{
		Host:   hostFromEndpoint(cfg.Endpoint),
		Port:   portFromEndpoint(cfg.Endpoint),
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant: new client: %w", err)
	}

	return &Client{client: c, cfg: cfg}, nil
}

// EnsureCollection creates the named Qdrant collection idempotently.
// If the collection already exists with matching parameters, this is a no-op.
// vectorSize should be 1024 for BGE-M3 (SPEC-IDX-002).
func (c *Client) EnsureCollection(ctx context.Context, name string, vectorSize uint64) error {
	// Check if collection already exists.
	exists, err := c.client.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("qdrant: check collection exists: %w", err)
	}
	if exists {
		return nil
	}

	onDisk := true
	err = c.client.CreateCollection(ctx, &qdrantpb.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrantpb.NewVectorsConfig(&qdrantpb.VectorParams{
			Size:     vectorSize,
			Distance: qdrantpb.Distance_Cosine,
		}),
		OnDiskPayload: &onDisk,
	})
	if err != nil {
		return fmt.Errorf("qdrant: create collection %q: %w", name, err)
	}
	return nil
}

// Upsert inserts or updates a batch of points in the Qdrant collection.
func (c *Client) Upsert(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}

	qpts := make([]*qdrantpb.PointStruct, 0, len(points))
	for _, p := range points {
		payload := qdrantpb.NewValueMap(p.Payload)
		qpts = append(qpts, &qdrantpb.PointStruct{
			Id:      qdrantpb.NewID(toQdrantID(p.ID)),
			Vectors: qdrantpb.NewVectors(p.Vector...),
			Payload: payload,
		})
	}

	waitUpsert := true
	_, err := c.client.Upsert(ctx, &qdrantpb.UpsertPoints{
		CollectionName: c.cfg.CollectionName,
		Points:         qpts,
		Wait:           &waitUpsert,
	})
	if err != nil {
		return fmt.Errorf("qdrant: upsert: %w", err)
	}
	return nil
}

// Search queries the Qdrant collection by vector similarity.
// filter may be nil for unfiltered searches.
func (c *Client) Search(ctx context.Context, vector []float32, filter *Filter, limit uint64) ([]ScoredPoint, error) {
	var qdrantFilter *qdrantpb.Filter
	if filter != nil {
		qdrantFilter = buildFilter(filter)
	}

	results, err := c.client.Query(ctx, &qdrantpb.QueryPoints{
		CollectionName: c.cfg.CollectionName,
		Query:          qdrantpb.NewQuery(vector...),
		Limit:          &limit,
		Filter:         qdrantFilter,
		WithPayload:    qdrantpb.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant: search: %w", err)
	}

	out := make([]ScoredPoint, 0, len(results))
	for _, r := range results {
		payload := make(map[string]any, len(r.Payload))
		for k, v := range r.Payload {
			payload[k] = payloadValue(v)
		}
		out = append(out, ScoredPoint{
			ID:      fromQdrantID(r.Id.GetUuid()),
			Score:   r.Score,
			Payload: payload,
		})
	}
	return out, nil
}

// payloadValue converts a Qdrant Value to a Go native type.
func payloadValue(v *qdrantpb.Value) any {
	if v == nil {
		return nil
	}
	switch k := v.GetKind().(type) {
	case *qdrantpb.Value_StringValue:
		return k.StringValue
	case *qdrantpb.Value_IntegerValue:
		return k.IntegerValue
	case *qdrantpb.Value_DoubleValue:
		return k.DoubleValue
	case *qdrantpb.Value_BoolValue:
		return k.BoolValue
	default:
		return v.GetStringValue()
	}
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// toQdrantID converts a 16-hex doc_id to a Qdrant UUID string (RFC 4122 form).
// Left-pads to 32 hex chars then inserts dashes: 8-4-4-4-12.
func toQdrantID(docID string) string {
	// Pad to 32 chars.
	padded := fmt.Sprintf("%032s", docID)
	// Insert RFC 4122 dashes: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return padded[0:8] + "-" + padded[8:12] + "-" + padded[12:16] + "-" + padded[16:20] + "-" + padded[20:32]
}

// fromQdrantID extracts the 16-hex doc_id from a Qdrant UUID string.
func fromQdrantID(uuid string) string {
	// Remove dashes and take last 16 chars (the original 16 hex chars, zero-padded).
	clean := ""
	for _, r := range uuid {
		if r != '-' {
			clean += string(r)
		}
	}
	if len(clean) >= 16 {
		return clean[len(clean)-16:]
	}
	return clean
}

// buildFilter converts a Filter into a Qdrant filter condition.
func buildFilter(f *Filter) *qdrantpb.Filter {
	var conditions []*qdrantpb.Condition

	if f.SourceID != "" {
		conditions = append(conditions, &qdrantpb.Condition{
			ConditionOneOf: &qdrantpb.Condition_Field{
				Field: &qdrantpb.FieldCondition{
					Key: "source_id",
					Match: &qdrantpb.Match{
						MatchValue: &qdrantpb.Match_Keyword{Keyword: f.SourceID},
					},
				},
			},
		})
	}
	if f.Lang != "" {
		conditions = append(conditions, &qdrantpb.Condition{
			ConditionOneOf: &qdrantpb.Condition_Field{
				Field: &qdrantpb.FieldCondition{
					Key: "lang",
					Match: &qdrantpb.Match{
						MatchValue: &qdrantpb.Match_Keyword{Keyword: f.Lang},
					},
				},
			},
		})
	}
	if f.TeamID != "" {
		conditions = append(conditions, &qdrantpb.Condition{
			ConditionOneOf: &qdrantpb.Condition_Field{
				Field: &qdrantpb.FieldCondition{
					Key: "team_id",
					Match: &qdrantpb.Match{
						MatchValue: &qdrantpb.Match_Keyword{Keyword: f.TeamID},
					},
				},
			},
		})
	}

	if len(conditions) == 0 {
		return nil
	}
	return &qdrantpb.Filter{Must: conditions}
}

// hostFromEndpoint extracts the hostname from "host:port".
func hostFromEndpoint(endpoint string) string {
	for i := len(endpoint) - 1; i >= 0; i-- {
		if endpoint[i] == ':' {
			return endpoint[:i]
		}
	}
	return endpoint
}

// portFromEndpoint extracts the port number from "host:port".
func portFromEndpoint(endpoint string) int {
	for i := len(endpoint) - 1; i >= 0; i-- {
		if endpoint[i] == ':' {
			port := 0
			for _, ch := range endpoint[i+1:] {
				if ch >= '0' && ch <= '9' {
					port = port*10 + int(ch-'0')
				}
			}
			return port
		}
	}
	return 6334
}
