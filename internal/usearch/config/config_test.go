// Package config_test validates the koanf-based configuration loader.
// SPEC-CLI-002 REQ-CLI2-007: XDG-compliant config file with layered precedence.
package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/usearch/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigDefaultValues verifies that defaults are applied when no config file exists.
func TestConfigDefaultValues(t *testing.T) {

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:8080", cfg.Server.Endpoint)
	assert.Equal(t, 30, cfg.Server.TimeoutSeconds)
	assert.Equal(t, "human", cfg.Output.DefaultFormat)
	assert.Equal(t, "auto", cfg.Output.Color)
	assert.True(t, cfg.History.Enabled)
	assert.Equal(t, "jsonl", cfg.History.Backend)
	assert.Equal(t, 1000, cfg.History.MaxEntries)
	assert.Equal(t, 90, cfg.History.RetentionDays)
	assert.False(t, cfg.Deep.AllowDegrade)
	assert.False(t, cfg.Deep.ForceOverride)
}

// TestConfigLoadsFromXDGPath verifies config is read from XDG_CONFIG_HOME.
func TestConfigLoadsFromXDGPath(t *testing.T) {

	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "usearch")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))

	cfgPath := filepath.Join(cfgDir, "config.toml")
	tomlContent := []byte(`
[server]
endpoint = "http://custom:9090"
timeout_seconds = 60

[output]
default_format = "json"

[history]
max_entries = 500
`)
	require.NoError(t, os.WriteFile(cfgPath, tomlContent, 0o644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "http://custom:9090", cfg.Server.Endpoint)
	assert.Equal(t, 60, cfg.Server.TimeoutSeconds)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 500, cfg.History.MaxEntries)
}

// TestConfigPrecedenceFlagOverEnvOverFileOverDefault verifies layered precedence.
// REQ-CLI2-007: flag > env > file > default.
func TestConfigPrecedenceFlagOverEnvOverFileOverDefault(t *testing.T) {

	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "usearch")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))

	cfgPath := filepath.Join(cfgDir, "config.toml")
	tomlContent := []byte(`
[server]
endpoint = "http://from-file:8080"
`)
	require.NoError(t, os.WriteFile(cfgPath, tomlContent, 0o644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)
	// Env overrides file.
	t.Setenv("USEARCH_SERVER_ENDPOINT", "http://from-env:8081")

	cfg, err := config.Load()
	require.NoError(t, err)

	// Env wins over file.
	assert.Equal(t, "http://from-env:8081", cfg.Server.Endpoint)
}

// TestConfigMalformedExitsOne verifies that a malformed TOML file produces an error.
func TestConfigMalformedExitsOne(t *testing.T) {

	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "usearch")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))

	cfgPath := filepath.Join(cfgDir, "config.toml")
	badContent := []byte(`[server
endpoint = "unclosed bracket`)
	require.NoError(t, os.WriteFile(cfgPath, badContent, 0o644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	_, err := config.Load()
	assert.Error(t, err, "malformed TOML should produce an error")
}

// TestConfigSchemaAllSections verifies all config sections are populated.
func TestConfigSchemaAllSections(t *testing.T) {

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	cfg, err := config.Load()
	require.NoError(t, err)

	// Server section
	assert.NotEmpty(t, cfg.Server.Endpoint)
	assert.Greater(t, cfg.Server.TimeoutSeconds, 0)

	// Auth section (empty by default)
	assert.Empty(t, cfg.Auth.UserID)
	assert.Empty(t, cfg.Auth.TenantID)

	// Output section
	assert.NotEmpty(t, cfg.Output.DefaultFormat)
	assert.NotEmpty(t, cfg.Output.Color)

	// History section
	assert.True(t, cfg.History.Enabled)
	assert.NotEmpty(t, cfg.History.Backend)
	assert.Greater(t, cfg.History.MaxEntries, 0)
	assert.Greater(t, cfg.History.RetentionDays, 0)

	// Deep section
	assert.NotNil(t, cfg.Deep)

	// Sources section
	assert.NotNil(t, cfg.Sources)

	// TUI section
	assert.NotNil(t, cfg.TUI)
}

// TestConfigPathReturnsXDGPath verifies that ConfigPath resolves XDG paths correctly.
func TestConfigPathReturnsXDGPath(t *testing.T) {

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	path := config.ConfigPath()
	assert.Equal(t, filepath.Join(home, ".config", "usearch", "config.toml"), path)
}

// TestConfigUnknownKeysSilentlyIgnored verifies that unknown keys in config
// do not cause errors (koanf default behaviour). Edge case 5.6.
func TestConfigUnknownKeysSilentlyIgnored(t *testing.T) {

	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "usearch")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))

	cfgPath := filepath.Join(cfgDir, "config.toml")
	tomlContent := []byte(`
[banana]
flavor = "x"

[server]
endpoint = "http://test:8080"
`)
	require.NoError(t, os.WriteFile(cfgPath, tomlContent, 0o644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "http://test:8080", cfg.Server.Endpoint)
}

// --- XDG path helpers ---

// TestDataHomeReturnsXDGPath verifies DataHome resolves XDG_DATA_HOME.
func TestDataHomeReturnsXDGPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("HOME", home)

	got := config.DataHome()
	assert.Equal(t, filepath.Join(home, ".local", "share"), got)
}

// TestDataPathReturnsUnderDataHome verifies DataPath appends "usearch".
func TestDataPathReturnsUnderDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".data"))
	t.Setenv("HOME", home)

	got := config.DataPath()
	assert.Equal(t, filepath.Join(home, ".data", "usearch"), got)
}

// TestHistoryPathJSONL verifies HistoryPath returns jsonl path by default.
func TestHistoryPathJSONL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".data"))
	t.Setenv("HOME", home)

	got := config.HistoryPath("jsonl")
	assert.Equal(t, filepath.Join(home, ".data", "usearch", "history.jsonl"), got)
}

// TestHistoryPathSQLite verifies HistoryPath returns db path for sqlite backend.
func TestHistoryPathSQLite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".data"))
	t.Setenv("HOME", home)

	got := config.HistoryPath("sqlite")
	assert.Equal(t, filepath.Join(home, ".data", "usearch", "history.db"), got)
}

// --- ToTOML ---

// TestToTOMLProducesValidTOML verifies ToTOML serializes all sections.
func TestToTOMLProducesValidTOML(t *testing.T) {
	cfg := config.DefaultConfig()
	toml, err := config.ToTOML(&cfg)
	require.NoError(t, err)
	assert.Contains(t, toml, "[server]")
	assert.Contains(t, toml, "http://localhost:8080")
	assert.Contains(t, toml, "[history]")
	assert.Contains(t, toml, "[deep]")
	assert.Contains(t, toml, "[tui]")
}

// --- GetKey ---

// TestGetKeyKnownKey verifies GetKey returns value for existing key.
func TestGetKeyKnownKey(t *testing.T) {
	cfg := config.DefaultConfig()
	val, err := config.GetKey(&cfg, "server.endpoint")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", val)
}

// TestGetKeyUnknownKey verifies GetKey returns error for missing key.
func TestGetKeyUnknownKey(t *testing.T) {
	cfg := config.DefaultConfig()
	_, err := config.GetKey(&cfg, "nonexistent.key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key")
}

// --- SetKey ---

// TestSetKeyCreatesFile verifies SetKey creates a new config file when missing.
func TestSetKeyCreatesFile(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, "config.toml")

	err := config.SetKey(cfgPath, "server.endpoint", "http://test:9999")
	require.NoError(t, err)

	// Verify the file was created.
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "http://test:9999")
}

// TestSetKeyUpdatesExisting verifies SetKey updates an existing key.
func TestSetKeyUpdatesExisting(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, "config.toml")

	// Write initial.
	require.NoError(t, config.SetKey(cfgPath, "server.endpoint", "http://first:8080"))
	// Update.
	require.NoError(t, config.SetKey(cfgPath, "server.endpoint", "http://second:9090"))

	// Verify the updated value.
	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "http://second:9090", cfg.Server.Endpoint)
}

// TestSetKeyNewSection verifies SetKey can set keys in sections not yet in file.
func TestSetKeyNewSection(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, "config.toml")

	err := config.SetKey(cfgPath, "output.default_format", "json")
	require.NoError(t, err)

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
}

// --- ConfigHome fallback ---

// TestConfigHomeFallsBackToHome verifies ConfigHome falls back to ~/.config when XDG unset.
func TestConfigHomeFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	got := config.ConfigHome()
	assert.Equal(t, filepath.Join(home, ".config"), got)
}

// TestDataHomeFallsBackToHome verifies DataHome falls back to ~/.local/share when XDG unset.
func TestDataHomeFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	got := config.DataHome()
	assert.Equal(t, filepath.Join(home, ".local", "share"), got)
}

// --- LoadFromPath edge cases ---

// TestLoadFromPathNonexistentFile verifies LoadFromPath succeeds with missing file.
func TestLoadFromPathNonexistentFile(t *testing.T) {
	cfg, err := config.LoadFromPath("/nonexistent/path/config.toml")
	require.NoError(t, err)
	// Defaults should be applied.
	assert.Equal(t, "http://localhost:8080", cfg.Server.Endpoint)
}

// TestLoadFromPathMalformedFile verifies LoadFromPath returns error for bad TOML.
func TestLoadFromPathMalformedFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`[broken`), 0o644))

	_, err := config.LoadFromPath(cfgPath)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "parse error"), "expected parse error, got: %v", err)
}
