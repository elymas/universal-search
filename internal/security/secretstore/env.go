package secretstore

import (
	"context"
	"fmt"
	"os"
)

// EnvResolver resolves secrets from process environment variables via
// os.Getenv. It is the default backend for dev and CI. Behaviour is identical
// to a direct os.Getenv call: an unset variable yields an empty value and an
// error, preserving the existing "not set" semantics at call sites.
type EnvResolver struct{}

// NewEnvResolver constructs an EnvResolver.
func NewEnvResolver() *EnvResolver { return &EnvResolver{} }

// Get returns os.Getenv(key). If the variable is unset (empty), it returns an
// error so callers can distinguish "present" from "absent" without inspecting
// the (necessarily empty) value.
func (r *EnvResolver) Get(_ context.Context, key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("secretstore: env %q not set", key)
	}
	return v, nil
}
