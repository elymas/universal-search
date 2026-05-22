// Package config provides XDG-compliant configuration loading for usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-007: Configuration is loaded via koanf from an
// XDG-compliant TOML file with layered precedence:
//
//	(1) command-line flags (highest)
//	(2) environment variables with USEARCH_ prefix
//	(3) config file (TOML)
//	(4) built-in defaults (lowest)
//
// Missing config file is tolerated (defaults applied).
// Malformed config file produces an error.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Config holds the effective merged configuration for the usearch CLI.
type Config struct {
	Server  ServerConfig  `koanf:"server"`
	Auth    AuthConfig    `koanf:"auth"`
	Output  OutputConfig  `koanf:"output"`
	History HistoryConfig `koanf:"history"`
	Deep    DeepConfig    `koanf:"deep"`
	Sources SourcesConfig `koanf:"sources"`
	TUI     TUIConfig     `koanf:"tui"`
}

// ServerConfig holds the server connection settings.
type ServerConfig struct {
	Endpoint       string `koanf:"endpoint"`
	TimeoutSeconds int    `koanf:"timeout_seconds"`
}

// AuthConfig holds authentication settings.
// Tokens are NEVER stored here — they go to a separate credentials file (mode 0600).
type AuthConfig struct {
	TokenFile string `koanf:"token_file"`
	UserID    string `koanf:"user_id"`
	TenantID  string `koanf:"tenant_id"`
}

// OutputConfig holds output formatting settings.
type OutputConfig struct {
	DefaultFormat string `koanf:"default_format"`
	Color         string `koanf:"color"`
	Pager         string `koanf:"pager"`
}

// HistoryConfig holds history persistence settings.
type HistoryConfig struct {
	Enabled       bool   `koanf:"enabled"`
	Backend       string `koanf:"backend"`
	Path          string `koanf:"path"`
	MaxEntries    int    `koanf:"max_entries"`
	RetentionDays int    `koanf:"retention_days"`
}

// DeepConfig holds deep pipeline settings.
type DeepConfig struct {
	AllowDegrade  bool `koanf:"allow_degrade"`
	ForceOverride bool `koanf:"force_override"`
}

// SourcesConfig holds source/adapter default settings.
type SourcesConfig struct {
	Defaults []string `koanf:"defaults"`
}

// TUIConfig holds TUI/interactive mode settings.
type TUIConfig struct {
	Enabled bool   `koanf:"enabled"`
	Theme   string `koanf:"theme"`
}

// DefaultConfig returns the built-in default configuration.
// Exported for use by config init subcommand.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Endpoint:       "http://localhost:8080",
			TimeoutSeconds: 30,
		},
		Auth: AuthConfig{
			TokenFile: filepath.Join(ConfigHome(), "usearch", "credentials"),
		},
		Output: OutputConfig{
			DefaultFormat: "human",
			Color:         "auto",
			Pager:         "auto",
		},
		History: HistoryConfig{
			Enabled:       true,
			Backend:       "jsonl",
			MaxEntries:    1000,
			RetentionDays: 90,
		},
		Deep: DeepConfig{
			AllowDegrade:  false,
			ForceOverride: false,
		},
		Sources: SourcesConfig{
			Defaults: nil,
		},
		TUI: TUIConfig{
			Enabled: true,
			Theme:   "default",
		},
	}
}

// Load reads configuration from file, env, and defaults.
// Missing config file is tolerated; malformed config returns an error.
func Load() (*Config, error) {
	return LoadFromPath(ConfigPath())
}

// LoadFromPath reads configuration from a specific file path.
// Used by tests to inject temp paths.
func LoadFromPath(cfgPath string) (*Config, error) {
	k := koanf.New(".")

	// Layer 4: built-in defaults (lowest precedence).
	def := DefaultConfig()
	if err := k.Load(structs.Provider(&def, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}

	// Layer 3: config file (if it exists).
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		if err := k.Load(file.Provider(cfgPath), toml.Parser()); err != nil {
			return nil, fmt.Errorf("config: parse error at %s: %w", cfgPath, err)
		}
	}

	// Layer 2: environment variables with USEARCH_ prefix.
	// USEARCH_SERVER_ENDPOINT -> server.endpoint (single underscore = section separator)
	// USEARCH_DEEP_ALLOW_DEGRADE -> deep.allow_degrade
	if err := k.Load(env.Provider("USEARCH_", ".", func(s string) string {
		// USEARCH_SERVER_ENDPOINT -> server_endpoint
		key := strings.ToLower(strings.TrimPrefix(s, "USEARCH_"))
		// server_endpoint -> server.endpoint (first underscore splits section.key)
		// deep_allow_degrade -> deep.allow_degrade
		parts := strings.SplitN(key, "_", 2)
		if len(parts) == 2 {
			return parts[0] + "." + parts[1]
		}
		return key
	}), nil); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &cfg, nil
}

// ConfigHome returns the XDG_CONFIG_HOME directory, respecting the
// environment variable override. Falls back to OS defaults.
func ConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return os.Getenv("APPDATA")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// DataHome returns the XDG_DATA_HOME directory, respecting the
// environment variable override. Falls back to OS defaults.
func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return os.Getenv("LOCALAPPDATA")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

// ConfigPath returns the resolved config file path based on XDG conventions.
func ConfigPath() string {
	return filepath.Join(ConfigHome(), "usearch", "config.toml")
}

// DataPath returns the resolved data directory path based on XDG conventions.
func DataPath() string {
	return filepath.Join(DataHome(), "usearch")
}

// HistoryPath returns the resolved history file path.
func HistoryPath(backend string) string {
	dir := DataPath()
	switch backend {
	case "sqlite":
		return filepath.Join(dir, "history.db")
	default: // jsonl
		return filepath.Join(dir, "history.jsonl")
	}
}

// ToTOML serializes a Config to TOML format.
func ToTOML(cfg *Config) (string, error) {
	k := koanf.New(".")
	if err := k.Load(structs.Provider(cfg, "koanf"), nil); err != nil {
		return "", fmt.Errorf("config: marshal to koanf: %w", err)
	}
	b, err := k.Marshal(toml.Parser())
	if err != nil {
		return "", fmt.Errorf("config: marshal to TOML: %w", err)
	}
	return string(b), nil
}

// GetKey retrieves a single config value by dot-separated key.
// Returns an error if the key is not found.
func GetKey(cfg *Config, key string) (string, error) {
	k := koanf.New(".")
	if err := k.Load(structs.Provider(cfg, "koanf"), nil); err != nil {
		return "", fmt.Errorf("config: load for get: %w", err)
	}
	if !k.Exists(key) {
		return "", fmt.Errorf("usearch config: unknown key '%s'", key)
	}
	return fmt.Sprintf("%v", k.Get(key)), nil
}

// SetKey writes a key-value pair to the config file.
// Creates the file with defaults if it doesn't exist.
func SetKey(cfgPath, key, value string) error {
	k := koanf.New(".")

	// Load defaults first.
	def := DefaultConfig()
	if err := k.Load(structs.Provider(&def, "koanf"), nil); err != nil {
		return fmt.Errorf("config: load defaults: %w", err)
	}

	// Load existing file if present.
	if _, err := os.Stat(cfgPath); err == nil {
		if loadErr := k.Load(file.Provider(cfgPath), toml.Parser()); loadErr != nil {
			return fmt.Errorf("config: load existing: %w", loadErr)
		}
	}

	// Set the key.
	if err := k.Set(key, value); err != nil {
		return fmt.Errorf("config: set key: %w", err)
	}

	// Marshal back to TOML and write.
	b, err := k.Marshal(toml.Parser())
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}

	return nil
}
