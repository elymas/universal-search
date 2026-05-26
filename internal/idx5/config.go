package idx5

// Config holds the configuration for the IDX-005 lookup pipeline.
// REQ-IDX5-002 D1: similarity threshold default 0.92 with per-team override.
type Config struct {
	// SimilarityThreshold is the minimum cosine similarity for a cache hit.
	SimilarityThreshold float64
	// TeamThresholdOverrides allows per-team threshold customization.
	TeamThresholdOverrides map[string]float64
	// CategoryTTLs maps category names to TTL in seconds.
	CategoryTTLs map[string]int
	// CitationRevalidationMode: "lazy" (default), "eager_top_n", "eager_all".
	CitationRevalidationMode string
	// EagerTopN: number of top citations to re-validate in eager mode.
	EagerTopN int
}

// DefaultConfig returns the default IDX-005 configuration.
func DefaultConfig() Config {
	return Config{
		SimilarityThreshold:    0.92,
		TeamThresholdOverrides: make(map[string]float64),
		CategoryTTLs: map[string]int{
			"web":      3600,
			"social":   1800,
			"academic": 2592000,
			"korean":   3600,
			"mixed":    7200,
			"unknown":  7200,
		},
		CitationRevalidationMode: "lazy",
		EagerTopN:                3,
	}
}

// GetThreshold returns the effective similarity threshold for a team.
func (c Config) GetThreshold(teamID string) float64 {
	if t, ok := c.TeamThresholdOverrides[teamID]; ok {
		return t
	}
	return c.SimilarityThreshold
}
