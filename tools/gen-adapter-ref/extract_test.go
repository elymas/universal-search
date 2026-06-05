package main

import (
	"path/filepath"
	"testing"
)

// TestExtractStandard tests extraction from a standard Capabilities() method.
func TestExtractStandard(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "standard.go")
	got, err := extract(path, "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFields(t, got, capabilitiesFields{
		SourceID:          "testadapter",
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   42,
		DefaultMaxResults: 10,
	})
}

// TestExtractAuthAdapter tests extraction with RequiresAuth=true and AuthEnvVars.
func TestExtractAuthAdapter(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "auth_adapter.go")
	got, err := extract(path, "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFields(t, got, capabilitiesFields{
		SourceID:          "testauthsrc",
		RequiresAuth:      true,
		AuthEnvVars:       []string{"TEST_API_KEY", "TEST_API_SECRET"},
		RateLimitPerMin:   30,
		DefaultMaxResults: 25,
	})
}

// TestExtractSocialHelper tests extraction of package-level helper func
// (the social.go pattern for bluesky and x).
func TestExtractSocialHelperAlpha(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "social_helpers.go")
	got, err := extract(path, "alphaCapabilities")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFields(t, got, capabilitiesFields{
		SourceID:          "alpha",
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   600,
		DefaultMaxResults: 25,
	})
}

// TestExtractSocialHelperBeta tests the disabled-stub helper extraction.
func TestExtractSocialHelperBeta(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "social_helpers.go")
	got, err := extract(path, "betaCapabilities")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFields(t, got, capabilitiesFields{
		SourceID:          "beta",
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   0,
		DefaultMaxResults: 0,
	})
}

// TestExtractRealAdapters is a table-driven test over the 10 real adapters.
// It verifies KNOWN values extracted from the live source files.
func TestExtractRealAdapters(t *testing.T) {
	t.Parallel()
	// projectRoot is two levels up from tools/gen-adapter-ref/
	projectRoot := filepath.Join("..", "..")

	type want struct {
		requiresAuth      bool
		authEnvVars       []string
		rateLimitPerMin   int
		defaultMaxResults int
	}
	tests := []struct {
		sourceID string
		want     want
	}{
		{
			sourceID: "arxiv",
			want:     want{requiresAuth: false, rateLimitPerMin: 20, defaultMaxResults: 25},
		},
		{
			sourceID: "reddit",
			want: want{
				requiresAuth:      true,
				authEnvVars:       []string{"REDDIT_CLIENT_ID", "REDDIT_CLIENT_SECRET"},
				rateLimitPerMin:   60,
				defaultMaxResults: 25,
			},
		},
		{
			sourceID: "hackernews",
			want:     want{requiresAuth: false, rateLimitPerMin: 60, defaultMaxResults: 25},
		},
		{
			sourceID: "github",
			want: want{
				requiresAuth:      true,
				authEnvVars:       []string{"USEARCH_GITHUB_TOKEN"},
				rateLimitPerMin:   30,
				defaultMaxResults: 25,
			},
		},
		{
			sourceID: "youtube",
			want:     want{requiresAuth: false, rateLimitPerMin: 30, defaultMaxResults: 25},
		},
		{
			sourceID: "searxng",
			want:     want{requiresAuth: false, rateLimitPerMin: 0, defaultMaxResults: 10},
		},
		{
			sourceID: "naver",
			want: want{
				requiresAuth:      true,
				authEnvVars:       []string{"NAVER_CLIENT_ID", "NAVER_CLIENT_SECRET"},
				rateLimitPerMin:   10,
				defaultMaxResults: 25,
			},
		},
		{
			sourceID: "koreanews",
			want:     want{requiresAuth: false, rateLimitPerMin: 0, defaultMaxResults: 20},
		},
		{
			sourceID: "bluesky",
			want:     want{requiresAuth: false, rateLimitPerMin: 600, defaultMaxResults: 25},
		},
		{
			sourceID: "x",
			want:     want{requiresAuth: false, rateLimitPerMin: 0, defaultMaxResults: 0},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.sourceID, func(t *testing.T) {
			t.Parallel()
			spec, ok := registry[tc.sourceID]
			if !ok {
				t.Fatalf("sourceID %q not in registry", tc.sourceID)
			}
			absFile := filepath.Join(projectRoot, "internal/adapters", spec.pkgDir, spec.primaryFile)
			got, err := extract(absFile, spec.funcName)
			if err != nil {
				t.Fatalf("extract(%s): %v", absFile, err)
			}
			if got.SourceID != tc.sourceID {
				t.Errorf("SourceID: got %q want %q", got.SourceID, tc.sourceID)
			}
			if got.RequiresAuth != tc.want.requiresAuth {
				t.Errorf("RequiresAuth: got %v want %v", got.RequiresAuth, tc.want.requiresAuth)
			}
			if got.RateLimitPerMin != tc.want.rateLimitPerMin {
				t.Errorf("RateLimitPerMin: got %d want %d", got.RateLimitPerMin, tc.want.rateLimitPerMin)
			}
			if got.DefaultMaxResults != tc.want.defaultMaxResults {
				t.Errorf("DefaultMaxResults: got %d want %d", got.DefaultMaxResults, tc.want.defaultMaxResults)
			}
			if tc.want.requiresAuth {
				if len(got.AuthEnvVars) != len(tc.want.authEnvVars) {
					t.Errorf("AuthEnvVars length: got %v want %v", got.AuthEnvVars, tc.want.authEnvVars)
				} else {
					for i, v := range tc.want.authEnvVars {
						if got.AuthEnvVars[i] != v {
							t.Errorf("AuthEnvVars[%d]: got %q want %q", i, got.AuthEnvVars[i], v)
						}
					}
				}
			}
			if got.SourceLine == 0 {
				t.Error("SourceLine should be > 0")
			}
		})
	}
}

// TestRegistryCompleteness verifies the registry has exactly the 10 expected SourceIDs.
func TestRegistryCompleteness(t *testing.T) {
	t.Parallel()
	expected := []string{
		"arxiv", "reddit", "hackernews", "github", "youtube",
		"searxng", "naver", "koreanews", "bluesky", "x",
	}
	if len(registry) != len(expected) {
		t.Errorf("registry size: got %d want %d", len(registry), len(expected))
	}
	for _, id := range expected {
		if _, ok := registry[id]; !ok {
			t.Errorf("registry missing SourceID %q", id)
		}
	}
	// Ensure noop/reference is NOT in registry.
	if _, ok := registry["reference"]; ok {
		t.Error("registry must not contain noop adapter SourceID 'reference'")
	}
	if _, ok := registry["noop"]; ok {
		t.Error("registry must not contain 'noop'")
	}
}

// assertFields compares two capabilitiesFields structs (ignoring SourcePath and SourceLine).
func assertFields(t *testing.T, got, want capabilitiesFields) {
	t.Helper()
	if got.SourceID != want.SourceID {
		t.Errorf("SourceID: got %q want %q", got.SourceID, want.SourceID)
	}
	if got.RequiresAuth != want.RequiresAuth {
		t.Errorf("RequiresAuth: got %v want %v", got.RequiresAuth, want.RequiresAuth)
	}
	if got.RateLimitPerMin != want.RateLimitPerMin {
		t.Errorf("RateLimitPerMin: got %d want %d", got.RateLimitPerMin, want.RateLimitPerMin)
	}
	if got.DefaultMaxResults != want.DefaultMaxResults {
		t.Errorf("DefaultMaxResults: got %d want %d", got.DefaultMaxResults, want.DefaultMaxResults)
	}
	if len(got.AuthEnvVars) != len(want.AuthEnvVars) {
		t.Errorf("AuthEnvVars len: got %v want %v", got.AuthEnvVars, want.AuthEnvVars)
		return
	}
	for i := range want.AuthEnvVars {
		if got.AuthEnvVars[i] != want.AuthEnvVars[i] {
			t.Errorf("AuthEnvVars[%d]: got %q want %q", i, got.AuthEnvVars[i], want.AuthEnvVars[i])
		}
	}
}
