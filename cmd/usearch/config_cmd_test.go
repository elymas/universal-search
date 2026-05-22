// Package main — tests for the config subcommand tree.
// SPEC-CLI-002 REQ-CLI2-012: config {path,show,init,get,set}.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/usearch/config"
)

// TestConfigPathSubcommand verifies "config path" outputs the XDG path.
func TestConfigPathSubcommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "path"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	expected := filepath.Join(home, ".config", "usearch", "config.toml")
	got := strings.TrimSpace(stdout.String())
	if got != expected {
		t.Errorf("config path = %q, want %q", got, expected)
	}
}

// TestConfigShowSubcommand verifies "config show" outputs TOML with defaults.
func TestConfigShowSubcommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "show"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "http://localhost:8080") {
		t.Errorf("config show missing default endpoint: %q", output)
	}
}

// TestConfigInitCreatesFile verifies "config init" creates a TOML file.
func TestConfigInitCreatesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "init"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	cfgPath := config.ConfigPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}
}

// TestConfigInitRefusesOverwrite verifies "config init" refuses to overwrite.
func TestConfigInitRefusesOverwrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	// Create the file first.
	cfgDir := filepath.Dir(config.ConfigPath())
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(), []byte("[server]\nendpoint = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "init"})

	if code != ExitUserError {
		t.Errorf("exit code = %d, want %d (ExitUserError)", code, ExitUserError)
	}
}

// TestConfigInitForceOverwrites verifies "config init --force" overwrites.
func TestConfigInitForceOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	// Create an existing file.
	cfgDir := filepath.Dir(config.ConfigPath())
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(), []byte("[server]\nendpoint = \"old\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "init", "--force"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	// Verify the file was overwritten with defaults.
	content, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "http://localhost:8080") {
		t.Error("config file was not overwritten with defaults")
	}
	if strings.Contains(string(content), "endpoint = \"old\"") || strings.Contains(string(content), "endpoint = 'old'") {
		t.Error("config file still contains old endpoint value")
	}
}

// TestConfigGetKnownKey verifies "config get server.endpoint" works.
func TestConfigGetKnownKey(t *testing.T) {
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "usearch")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tomlContent := []byte(`[server]
endpoint = "http://test:8080"
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), tomlContent, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "get", "server.endpoint"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := strings.TrimSpace(stdout.String())
	if got != "http://test:8080" {
		t.Errorf("config get = %q, want %q", got, "http://test:8080")
	}
}

// TestConfigGetUnknownKeyExitsOne verifies unknown key returns exit 1.
func TestConfigGetUnknownKeyExitsOne(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "get", "nosuch.key"})

	if code != ExitUserError {
		t.Errorf("exit code = %d, want %d (ExitUserError)", code, ExitUserError)
	}
}

// TestConfigSetWritesToFile verifies "config set" writes to the config file.
func TestConfigSetWritesToFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "set", "server.endpoint", "http://new:9090"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	// Verify by reading back.
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Endpoint != "http://new:9090" {
		t.Errorf("server.endpoint = %q, want %q", cfg.Server.Endpoint, "http://new:9090")
	}
}

// TestConfigSetRefusesTokenKey verifies "config set auth.token" is rejected.
func TestConfigSetRefusesTokenKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"config", "set", "auth.token", "sk_secret"})

	if code != ExitUserError {
		t.Errorf("exit code = %d, want %d (ExitUserError)", code, ExitUserError)
	}
}
