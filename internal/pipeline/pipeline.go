// Package pipeline provides the shared search pipeline assembly used by both
// cmd/usearch (CLI) and cmd/usearch-api (HTTP server).
//
// REQ-API-002: The system shall reuse the exact CLI search pipeline assembly as the
// single source of truth, sharing one extracted internal/ package between cmd/usearch
// and cmd/usearch-api so the two entry points cannot diverge.
//
// @MX:NOTE: [AUTO] Shared pipeline assembly extracted from cmd/usearch/query.go.
// Both cmd packages import from here to prevent duplication.
// @MX:SPEC: SPEC-API-001
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/adapters/arxiv"
	"github.com/elymas/universal-search/internal/adapters/github"
	"github.com/elymas/universal-search/internal/adapters/hn"
	"github.com/elymas/universal-search/internal/adapters/koreanews"
	"github.com/elymas/universal-search/internal/adapters/naver"
	redditrss "github.com/elymas/universal-search/internal/adapters/reddit_rss"
	"github.com/elymas/universal-search/internal/adapters/reddit"
	"github.com/elymas/universal-search/internal/adapters/searxng"
	"github.com/elymas/universal-search/internal/adapters/social"
	"github.com/elymas/universal-search/internal/adapters/youtube"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/internal/security/secretstore"
	"github.com/elymas/universal-search/internal/synthesis"
	"github.com/elymas/universal-search/pkg/types"
)

// ErrSynthUnavailable is a sentinel for the nop synthesis client (degraded mode).
var ErrSynthUnavailable = errors.New("synthesis: client unavailable")

// SynthResult is the internal representation of a synthesis result.
type SynthResult struct {
	Text      string
	Citations []SynthCitation
}

// SynthCitation is a single citation from the synthesis client.
type SynthCitation struct {
	Marker int
	DocID  string
	URL    string
	Title  string
}

// SynthClient is the interface for calling the synthesis service.
type SynthClient interface {
	Synthesize(ctx context.Context, query, lang string, docs []types.NormalizedDoc) (SynthResult, error)
}

// Assembly holds the fully wired production search pipeline components.
type Assembly struct {
	Registry *adapters.Registry
	Router   *router.Router
	Fanout   *fanout.Fanout
	Synth    SynthClient
}

// BuildProductionAssembly constructs the full production pipeline using the
// default (env) Resolver. This is the backward-compatible entry point.
//
// @MX:ANCHOR: [AUTO] Production pipeline assembly; callers: cmd/usearch, cmd/usearch-api
// @MX:REASON: fan_in >= 3 (CLI main, API main, integration tests). Single source of truth
// for pipeline wiring prevents divergence between entry points.
// @MX:SPEC: SPEC-API-001 REQ-API-002
func BuildProductionAssembly() (*Assembly, error) {
	return BuildProductionAssemblyWithResolver(nil)
}

// BuildProductionAssemblyWithResolver constructs the full production pipeline
// with an injected secretstore.Resolver (SPEC-SEC-002 REQ-SEC2-001).
func BuildProductionAssemblyWithResolver(resolver secretstore.Resolver) (*Assembly, error) {
	reg, err := BuildProductionRegistryWithResolverAndError(resolver)
	if err != nil {
		return nil, err
	}

	rtr, err := BuildRouter(reg)
	if err != nil {
		return nil, err
	}

	f, err := fanout.New(fanout.Options{Registry: reg})
	if err != nil {
		return nil, err
	}

	synth := BuildProductionSynth()

	return &Assembly{
		Registry: reg,
		Router:   rtr,
		Fanout:   f,
		Synth:    synth,
	}, nil
}

// BuildProductionRegistry constructs the adapter registry for production use
// using the default (env) Resolver. This is the backward-compatible entry
// point — callers that need a specific backend should use
// BuildProductionRegistryWithResolver.
//
// @MX:NOTE: [AUTO] Backward-compat wrapper. Resolver-aware callers should use
// BuildProductionRegistryWithResolver instead.
// @MX:SPEC: SPEC-CLI-001 SPEC-API-001 SPEC-SEC-002
func BuildProductionRegistry() *adapters.Registry {
	return BuildProductionRegistryWithResolver(nil)
}

// BuildProductionRegistryWithResolver constructs the adapter registry with an
// injected secretstore.Resolver for credential resolution. When resolver is
// nil, an EnvResolver is constructed per-lookup (preserving backward compat).
// Credentialed adapters are registered with SkipAuthCheck: true so the
// registry's env-only auth gate does not reject adapters whose secrets come
// from a non-env backend (SPEC-SEC-002 REQ-SEC2-007).
//
// Returns an error if the Resolver returns ErrNotImplemented (vault stub)
// while resolving a credentialed adapter's key (REQ-SEC2-004). For the
// non-error variant, use BuildProductionRegistryWithResolver which silently
// skips adapters whose creds are not available.
//
// @MX:ANCHOR: [AUTO] Resolver-aware production registry builder; callers:
// cmd/usearch, cmd/usearch-api, tests.
// @MX:REASON: fan_in >= 3; single source of truth for adapter credential wiring.
// Adapter credentials flow exclusively through the injected Resolver (F-07 fix).
// The registry's env-only auth gate is bypassed for credentialed adapters
// because the CLI already validated presence via the Resolver (REQ-SEC2-007).
// @MX:SPEC: SPEC-SEC-002
func BuildProductionRegistryWithResolver(resolver secretstore.Resolver) *adapters.Registry {
	reg, _ := BuildProductionRegistryWithResolverAndError(resolver)
	return reg
}

// BuildProductionRegistryWithResolverAndError is like
// BuildProductionRegistryWithResolver but returns an error when the Resolver
// returns ErrNotImplemented (vault stub) for a credentialed adapter.
// This allows callers to surface vault-as-stub as a clear startup error
// rather than a silent missing-credential skip (REQ-SEC2-004).
func BuildProductionRegistryWithResolverAndError(resolver secretstore.Resolver) (*adapters.Registry, error) {
	reg := adapters.NewRegistry(nil)
	ctx := context.Background()

	// --- Non-credentialed adapters (unchanged from pre-SPEC) ---

	if a, err := hn.New(hn.Options{
		BaseURL: os.Getenv("HN_BASE_URL"),
	}); err == nil {
		_ = reg.Register(a)
	}
	if a, err := arxiv.New(arxiv.Options{
		BaseURL: os.Getenv("ARXIV_BASE_URL"),
	}); err == nil {
		_ = reg.Register(a)
	}
	if base := os.Getenv("YOUTUBE_BASE_URL"); base != "" {
		if a, err := youtube.New(youtube.Options{
			BaseURL: base,
		}); err == nil {
			_ = reg.Register(a)
		}
	}
	if a, err := searxng.New(searxng.Options{}); err == nil {
		_ = reg.Register(a)
	}
	// Bluesky: searchPosts requires an authenticated session (HTTP 403 otherwise).
	// Handle is config; the app password is a secret resolved via the Resolver
	// below. Missing creds → adapter builds but searches fail at query time.
	if a, err := social.NewBluesky(social.BlueskyOptions{
		BaseURL:     os.Getenv("BLUESKY_BASE_URL"),
		Handle:      os.Getenv("BLUESKY_HANDLE"),
		AppPassword: blueskyAppPassword(resolver),
	}); err == nil {
		_ = reg.Register(a)
	}
	if a, err := koreanews.New(koreanews.OptionsFromEnv()); err == nil {
		_ = reg.Register(a)
	}
	// reddit-rss: credential-free public RSS fallback (SPEC-ADP-001b REQ-ADP1B-020).
	// Always registered unconditionally; no credentials required.
	// REDDIT_RSS_BASE_URL overrides the production endpoint (parallel to HN_BASE_URL / ARXIV_BASE_URL).
	if a, err := redditrss.New(redditrss.Options{
		BaseURL: os.Getenv("REDDIT_RSS_BASE_URL"),
	}); err == nil {
		_ = reg.Register(a)
	}

	// --- Credentialed adapters via Resolver (SPEC-SEC-002) ---

	// Select resolver: injected → fallback to EnvResolver.
	r := resolver
	if r == nil {
		r = secretstore.NewEnvResolver()
	}

	// Naver: resolve NAVER_CLIENT_ID / NAVER_CLIENT_SECRET via Resolver.
	naverAdapter, naverErr := naver.New(naver.Options{Resolver: r})
	if naverErr == nil {
		_ = reg.RegisterWithOptions(naverAdapter, adapters.RegisterOptions{SkipAuthCheck: true})
	} else if errors.Is(naverErr, secretstore.ErrNotImplemented) {
		// @MX:WARN: [AUTO] Vault stub must not be silently skipped.
		// @MX:REASON: vault ErrNotImplemented indicates a misconfigured backend, not a missing key.
		// Surface as a loud startup error naming the unimplemented backend (REQ-SEC2-004).
		return nil, fmt.Errorf("credential resolution failed: vault backend is not implemented; use env or k8s backend: %w", naverErr)
	}
	// Other naver errors (missing creds under env/k8s) → silent skip (parity with pre-SPEC).

	// GitHub: resolve USEARCH_GITHUB_TOKEN (then GITHUB_TOKEN fallback) via Resolver.
	githubToken, githubErr := r.Get(ctx, "USEARCH_GITHUB_TOKEN")
	if githubErr != nil || githubToken == "" {
		// Try the GITHUB_TOKEN fallback alias.
		githubToken, githubErr = r.Get(ctx, "GITHUB_TOKEN")
	}
	if githubErr != nil {
		if errors.Is(githubErr, secretstore.ErrNotImplemented) {
			return nil, fmt.Errorf("credential resolution failed: vault backend is not implemented; use env or k8s backend: %w", githubErr)
		}
		// Other resolver errors → skip github (parity with pre-SPEC).
	}
	if githubToken != "" {
		if a, err := github.New(github.Options{
			BaseURL: os.Getenv("GITHUB_BASE_URL"),
			Token:   githubToken,
		}); err == nil {
			_ = reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true})
		}
	}

	// Reddit: resolve REDDIT_CLIENT_ID / REDDIT_CLIENT_SECRET via Resolver
	// (SPEC-SEC-002 REQ-SEC2-002). BASE_URL/OAUTH_URL are config, not secrets,
	// and stay on os.Getenv. Missing optional creds → silent skip (pre-SPEC parity).
	redditClientID, redditIDErr := r.Get(ctx, "REDDIT_CLIENT_ID")
	if errors.Is(redditIDErr, secretstore.ErrNotImplemented) {
		return nil, fmt.Errorf("credential resolution failed: vault backend is not implemented; use env or k8s backend: %w", redditIDErr)
	}
	redditClientSecret, redditSecretErr := r.Get(ctx, "REDDIT_CLIENT_SECRET")
	if errors.Is(redditSecretErr, secretstore.ErrNotImplemented) {
		return nil, fmt.Errorf("credential resolution failed: vault backend is not implemented; use env or k8s backend: %w", redditSecretErr)
	}
	if redditIDErr == nil && redditSecretErr == nil && redditClientID != "" && redditClientSecret != "" {
		if a, err := reddit.New(reddit.Options{
			BaseURL:      os.Getenv("REDDIT_BASE_URL"),
			ClientID:     redditClientID,
			ClientSecret: redditClientSecret,
			OAuthURL:     os.Getenv("REDDIT_OAUTH_URL"),
		}); err == nil {
			_ = reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true})
		}
	}

	// X (Twitter): gated by USEARCH_X_ENABLED (SPEC-ADP-006-XENABLE). Provider is
	// selected by which credential is present: TWITTERAPI_IO_KEY (twitterapi.io,
	// no X dev-console approval) is preferred; X_BEARER_TOKEN (official API v2) is
	// the fallback. Missing gate or both keys → silent skip.
	if os.Getenv("USEARCH_X_ENABLED") == "true" {
		if prov := buildXProvider(ctx, r); prov != nil {
			if a, aerr := social.NewX(social.XOptions{Provider: prov}); aerr == nil {
				_ = reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true})
			}
		}
	}

	return reg, nil
}

// buildXProvider selects an X search provider from available credentials.
// Preference order: twitterapi.io (TWITTERAPI_IO_KEY) → official API (X_BEARER_TOKEN).
// Returns nil when no credential resolves.
func buildXProvider(ctx context.Context, r secretstore.Resolver) social.XProvider {
	if key, err := r.Get(ctx, "TWITTERAPI_IO_KEY"); err == nil && key != "" {
		if prov, perr := social.NewTwitterAPIProvider(social.TwitterAPIOptions{APIKey: key}); perr == nil {
			return prov
		}
	}
	if token, err := r.Get(ctx, "X_BEARER_TOKEN"); err == nil && token != "" {
		if prov, perr := social.NewXOfficialProvider(social.XOfficialOptions{BearerToken: token}); perr == nil {
			return prov
		}
	}
	return nil
}

// blueskyAppPassword resolves BLUESKY_APP_PASSWORD via the given Resolver,
// falling back to the env resolver when none is injected. Returns "" on any
// error so a missing secret degrades to an unauthenticated adapter.
func blueskyAppPassword(resolver secretstore.Resolver) string {
	r := resolver
	if r == nil {
		r = secretstore.NewEnvResolver()
	}
	v, err := r.Get(context.Background(), "BLUESKY_APP_PASSWORD")
	if err != nil {
		return ""
	}
	return v
}

// BuildRouter constructs the Intent Router from a registry.
func BuildRouter(reg *adapters.Registry) (*router.Router, error) {
	return router.New(router.Options{
		Registry: reg,
	})
}

// BuildProductionFanout constructs the fanout dispatcher from a registry.
func BuildProductionFanout(reg *adapters.Registry) (*fanout.Fanout, error) {
	return fanout.New(fanout.Options{Registry: reg})
}

// BuildProductionSynth constructs the synthesis client for production use.
//
// Falls back to nopSynthClient if config load fails or client construction errors.
// The nop fallback satisfies degraded-mode behavior.
func BuildProductionSynth() SynthClient {
	cfg, err := synthesis.LoadConfig()
	if err != nil {
		return &NopSynthClient{}
	}
	client, err := synthesis.New(cfg, nil)
	if err != nil {
		return &NopSynthClient{}
	}
	return &productionSynthAdapter{client: client}
}

// NopSynthClient is a no-op implementation of SynthClient for degraded mode.
// Exported for use in tests that need to simulate synthesis unavailability.
type NopSynthClient struct{}

func (n *NopSynthClient) Synthesize(_ context.Context, _ string, _ string, _ []types.NormalizedDoc) (SynthResult, error) {
	return SynthResult{}, ErrSynthUnavailable
}

// productionSynthAdapter bridges *synthesis.Client to SynthClient.
type productionSynthAdapter struct {
	client *synthesis.Client
}

func (a *productionSynthAdapter) Synthesize(ctx context.Context, query, lang string, docs []types.NormalizedDoc) (SynthResult, error) {
	res, err := a.client.Synthesize(ctx, query, lang, docs)
	if err != nil {
		return SynthResult{}, err
	}
	citations := make([]SynthCitation, len(res.Citations))
	for i, c := range res.Citations {
		citations[i] = SynthCitation{
			Marker: c.Marker,
			DocID:  c.DocID,
			URL:    c.URL,
			Title:  c.Title,
		}
	}
	return SynthResult{Text: res.Text, Citations: citations}, nil
}
