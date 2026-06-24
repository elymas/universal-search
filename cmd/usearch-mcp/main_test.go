package main

import (
	"context"
	"sync"
	"testing"

	"github.com/elymas/universal-search/internal/security/secretstore"
)

// fakeResolver is a test double for secretstore.Resolver. It returns scripted
// values per key and records which keys were requested, so tests can prove the
// Resolver seam (not os.Getenv) was consulted for each SECRET key.
type fakeResolver struct {
	mu     sync.Mutex
	values map[string]string
	asked  map[string]int
}

func newFakeResolver(values map[string]string) *fakeResolver {
	return &fakeResolver{values: values, asked: make(map[string]int)}
}

// Get records the key and returns the scripted value. A missing key yields an
// empty value with an error, matching EnvResolver's "absent" semantics.
func (f *fakeResolver) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked[key]++
	v, ok := f.values[key]
	if !ok || v == "" {
		return "", secretstore.ErrNotImplemented // any non-nil error; treated as "skip"
	}
	return v, nil
}

func (f *fakeResolver) askedFor(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[key] > 0
}

// withXEnabled overrides the non-secret USEARCH_X_ENABLED config seam for the
// duration of a test, restoring it afterward. This avoids os.Setenv/t.Setenv,
// keeping the test deterministic and -race safe.
func withXEnabled(t *testing.T, enabled bool) {
	t.Helper()
	prev := xEnabledLookup
	t.Cleanup(func() { xEnabledLookup = prev })
	xEnabledLookup = func(key string) string {
		if key == "USEARCH_X_ENABLED" && enabled {
			return "true"
		}
		return ""
	}
}

// TestThreadsRegisteredViaResolver proves the Threads adapter's access token is
// sourced from the injected Resolver (not os.Getenv) and that the adapter is
// registered when the token resolves.
func TestThreadsRegisteredViaResolver(t *testing.T) {
	// X disabled so this test isolates the Threads path.
	withXEnabled(t, false)

	fake := newFakeResolver(map[string]string{
		"THREADS_ACCESS_TOKEN": "threads-secret-token",
	})

	reg := buildProductionRegistryWithResolver(fake)

	if _, ok := reg.Get("threads"); !ok {
		t.Fatal("threads adapter not registered when Resolver supplies THREADS_ACCESS_TOKEN")
	}
	if !fake.askedFor("THREADS_ACCESS_TOKEN") {
		t.Error("Resolver was not consulted for THREADS_ACCESS_TOKEN — secret not routed through Resolver seam")
	}
}

// TestThreadsSkippedWhenTokenAbsent confirms the Threads adapter is skipped
// (silently) when the Resolver cannot supply the token, preserving
// optional-adapter parity.
func TestThreadsSkippedWhenTokenAbsent(t *testing.T) {
	withXEnabled(t, false)

	fake := newFakeResolver(map[string]string{}) // no THREADS_ACCESS_TOKEN

	reg := buildProductionRegistryWithResolver(fake)

	if _, ok := reg.Get("threads"); ok {
		t.Fatal("threads adapter registered despite Resolver having no token")
	}
	if !fake.askedFor("THREADS_ACCESS_TOKEN") {
		t.Error("Resolver was not consulted for THREADS_ACCESS_TOKEN")
	}
}

// TestXBearerViaResolver proves the X bearer token is sourced from the injected
// Resolver (not os.Getenv) and that X is registered when enabled + the token
// resolves. The USEARCH_X_ENABLED flag is config and is driven via the
// injectable seam, not the environment.
func TestXBearerViaResolver(t *testing.T) {
	withXEnabled(t, true)

	fake := newFakeResolver(map[string]string{
		"X_BEARER_TOKEN": "x-bearer-secret",
	})

	reg := buildProductionRegistryWithResolver(fake)

	if _, ok := reg.Get("x"); !ok {
		t.Fatal("x adapter not registered when enabled and Resolver supplies X_BEARER_TOKEN")
	}
	if !fake.askedFor("X_BEARER_TOKEN") {
		t.Error("Resolver was not consulted for X_BEARER_TOKEN — secret not routed through Resolver seam")
	}
}

// TestXSkippedWhenBearerAbsent confirms X is not registered (even when enabled)
// if the Resolver cannot supply the bearer token.
func TestXSkippedWhenBearerAbsent(t *testing.T) {
	withXEnabled(t, true)

	fake := newFakeResolver(map[string]string{}) // no X_BEARER_TOKEN

	reg := buildProductionRegistryWithResolver(fake)

	if _, ok := reg.Get("x"); ok {
		t.Fatal("x adapter registered despite Resolver having no bearer token")
	}
	if !fake.askedFor("X_BEARER_TOKEN") {
		t.Error("Resolver was not consulted for X_BEARER_TOKEN")
	}
}

// TestXSkippedWhenDisabled confirms the bearer token is not even resolved when
// the X enable flag is off (config gate precedes the secret lookup).
func TestXSkippedWhenDisabled(t *testing.T) {
	withXEnabled(t, false)

	fake := newFakeResolver(map[string]string{
		"X_BEARER_TOKEN": "x-bearer-secret",
	})

	reg := buildProductionRegistryWithResolver(fake)

	if _, ok := reg.Get("x"); ok {
		t.Fatal("x adapter registered while USEARCH_X_ENABLED is not true")
	}
	if fake.askedFor("X_BEARER_TOKEN") {
		t.Error("Resolver consulted for X_BEARER_TOKEN while X disabled — config gate must precede secret lookup")
	}
}
