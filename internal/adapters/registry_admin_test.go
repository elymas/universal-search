package adapters_test

// Behavior tests for the Registry admin operations (Resync, ToggleEnabled) and
// the package error wrapper types (RegistryError, UpstreamError). These exercise
// the SPEC-UI-002 admin surface that the HTTP admin handlers depend on.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
)

func TestRegistryResync_Success(t *testing.T) {
	reg := adapters.NewRegistry(initObs(t, io.Discard))
	fake := newFake("healthy")
	if err := reg.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}

	view, err := reg.Resync(context.Background(), "healthy")
	if err != nil {
		t.Fatalf("Resync: %v", err)
	}
	if view.ID != "healthy" {
		t.Errorf("view.ID = %q, want healthy", view.ID)
	}
	if view.Status != "connected" {
		t.Errorf("view.Status = %q, want connected", view.Status)
	}
	if fake.healthCalls.Load() != 1 {
		t.Errorf("Healthcheck calls = %d, want 1", fake.healthCalls.Load())
	}
}

func TestRegistryResync_NotFound(t *testing.T) {
	reg := adapters.NewRegistry(initObs(t, io.Discard))
	_, err := reg.Resync(context.Background(), "missing")
	if !errors.Is(err, adapters.ErrAdapterNotFound) {
		t.Fatalf("Resync(missing) err = %v, want ErrAdapterNotFound", err)
	}
}

func TestRegistryResync_UpstreamError(t *testing.T) {
	reg := adapters.NewRegistry(initObs(t, io.Discard))
	fake := newFake("sick")
	fake.healthErr = errors.New("connection refused")
	if err := reg.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err := reg.Resync(context.Background(), "sick")
	if err == nil {
		t.Fatal("expected upstream error, got nil")
	}
	var upErr *adapters.UpstreamError
	if !errors.As(err, &upErr) {
		t.Fatalf("errors.As(*UpstreamError) = false; err = %v", err)
	}
	if upErr.AdapterID != "sick" {
		t.Errorf("UpstreamError.AdapterID = %q, want sick", upErr.AdapterID)
	}
	// Unwrap must expose the original cause.
	if !errors.Is(err, fake.healthErr) {
		t.Error("UpstreamError must unwrap to the underlying health error")
	}
}

func TestRegistryToggleEnabled(t *testing.T) {
	reg := adapters.NewRegistry(initObs(t, io.Discard))
	if err := reg.Register(newFake("togglable")); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// First toggle -> disabled.
	view, err := reg.ToggleEnabled(context.Background(), "togglable")
	if err != nil {
		t.Fatalf("ToggleEnabled: %v", err)
	}
	if view.Status != "disabled" {
		t.Errorf("after first toggle Status = %q, want disabled", view.Status)
	}

	// Second toggle -> back to connected.
	view, err = reg.ToggleEnabled(context.Background(), "togglable")
	if err != nil {
		t.Fatalf("ToggleEnabled (2nd): %v", err)
	}
	if view.Status != "connected" {
		t.Errorf("after second toggle Status = %q, want connected", view.Status)
	}
}

func TestRegistryToggleEnabled_NotFound(t *testing.T) {
	reg := adapters.NewRegistry(initObs(t, io.Discard))
	_, err := reg.ToggleEnabled(context.Background(), "missing")
	if !errors.Is(err, adapters.ErrAdapterNotFound) {
		t.Fatalf("ToggleEnabled(missing) err = %v, want ErrAdapterNotFound", err)
	}
}

func TestRegistryResync_AuthAdapterKeySet(t *testing.T) {
	reg := adapters.NewRegistry(initObs(t, io.Discard))
	fake := newFake("authed")
	fake.caps.RequiresAuth = true
	fake.caps.AuthEnvVars = []string{"USEARCH_TEST_SECRET_REL001"}
	// Register skipping the auth check so we can test the Resync view logic
	// independent of whether the env var happens to be set.
	if err := reg.RegisterWithOptions(fake, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Without the env var, KeySet must be false and SecretSource populated.
	view, err := reg.Resync(context.Background(), "authed")
	if err != nil {
		t.Fatalf("Resync: %v", err)
	}
	if view.SecretSource != "USEARCH_TEST_SECRET_REL001" {
		t.Errorf("SecretSource = %q, want the auth env var name", view.SecretSource)
	}
	if view.KeySet {
		t.Error("KeySet should be false when the auth env var is unset")
	}

	// With the env var set, KeySet must be true.
	t.Setenv("USEARCH_TEST_SECRET_REL001", "value")
	view, err = reg.Resync(context.Background(), "authed")
	if err != nil {
		t.Fatalf("Resync (with secret): %v", err)
	}
	if !view.KeySet {
		t.Error("KeySet should be true when the auth env var is set")
	}
}

func TestRegistryError_FormatAndUnwrap(t *testing.T) {
	base := adapters.ErrDuplicateAdapter
	re := &adapters.RegistryError{Op: "register", Name: "x", Cause: base}
	if got := re.Error(); got == "" || !strings.Contains(got, "register") || !strings.Contains(got, `"x"`) {
		t.Errorf("RegistryError.Error() = %q, missing op/name", got)
	}
	if !errors.Is(re, base) {
		t.Error("RegistryError must unwrap to its Cause")
	}

	var nilRE *adapters.RegistryError
	if nilRE.Error() != "<nil>" {
		t.Errorf("nil RegistryError.Error() = %q, want <nil>", nilRE.Error())
	}
}

func TestUpstreamError_FormatAndUnwrap(t *testing.T) {
	cause := errors.New("boom")
	ue := &adapters.UpstreamError{AdapterID: "svc", Err: cause}
	if got := ue.Error(); !strings.Contains(got, "svc") || !strings.Contains(got, "boom") {
		t.Errorf("UpstreamError.Error() = %q, missing id/cause", got)
	}
	if !errors.Is(ue, cause) {
		t.Error("UpstreamError must unwrap to its Err")
	}

	var nilUE *adapters.UpstreamError
	if nilUE.Error() != "<nil>" {
		t.Errorf("nil UpstreamError.Error() = %q, want <nil>", nilUE.Error())
	}
}
