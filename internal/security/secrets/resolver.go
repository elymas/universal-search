// Package secrets provides a 3-tier secret resolution interface.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotImplemented = errors.New("secrets: backend not implemented in V1")
var ErrSecretNotFound = errors.New("secrets: secret not found")

type Resolver interface {
	Get(ctx context.Context, key string) (string, error)
}

type EnvResolver struct{}

func (r *EnvResolver) Get(_ context.Context, key string) (string, error) {
	envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	val, ok := os.LookupEnv(envKey)
	if !ok {
		return "", fmt.Errorf("%w: key %q (env %q)", ErrSecretNotFound, key, envKey)
	}
	return val, nil
}

type K8sResolver struct {
	MountPath string
}

func (r *K8sResolver) Get(_ context.Context, key string) (string, error) {
	mountPath := r.MountPath
	if mountPath == "" {
		mountPath = "/var/run/secrets"
	}
	cleanKey := filepath.Base(key)
	filePath := filepath.Join(mountPath, cleanKey)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: key %q (file %q)", ErrSecretNotFound, key, filePath)
		}
		return "", fmt.Errorf("secrets: failed to read %q: %w", filePath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

type VaultResolver struct{}

func (r *VaultResolver) Get(_ context.Context, _ string) (string, error) {
	return "", ErrNotImplemented
}

func NewResolver(backend string) (Resolver, error) {
	switch backend {
	case "env", "":
		return &EnvResolver{}, nil
	case "k8s":
		return &K8sResolver{}, nil
	case "vault":
		return &VaultResolver{}, nil
	default:
		return nil, fmt.Errorf("secrets: unknown backend %q", backend)
	}
}
