// Package mcpserver implements the MCP server for Universal Search.
//
// SPEC-MCP-001: Exposes search, deep_research, list_sources, and get_citation
// tools over stdio (default) and Streamable HTTP (opt-in) transports per the
// 2025-06-18 MCP spec revision.
package mcpserver

// Config holds the MCP server configuration loaded from koanf mcp.* keys.
type Config struct {
	// Transport is the transport mode: "stdio" (default) or "http".
	Transport string `json:"transport" yaml:"transport" koanf:"transport"`

	// HTTP configures the Streamable HTTP transport (used when Transport == "http").
	HTTP HTTPConfig `json:"http,omitempty" yaml:"http,omitempty" koanf:"http"`

	// Tools configures which MCP tools are enabled.
	Tools ToolsConfig `json:"tools,omitempty" yaml:"tools,omitempty" koanf:"tools"`

	// Shutdown configures graceful shutdown behaviour.
	Shutdown ShutdownConfig `json:"shutdown,omitempty" yaml:"shutdown,omitempty" koanf:"shutdown"`
}

// HTTPConfig holds Streamable HTTP transport settings.
type HTTPConfig struct {
	// ListenAddr is the bind address (default "127.0.0.1:7080").
	ListenAddr string `json:"listen_addr" yaml:"listen_addr" koanf:"listen_addr"`

	// BindPublic allows binding to non-loopback addresses.
	BindPublic bool `json:"bind_public" yaml:"bind_public" koanf:"bind_public"`

	// AuthMode is one of "none", "trust-headers", "jwt".
	AuthMode string `json:"auth_mode" yaml:"auth_mode" koanf:"auth_mode"`

	// AllowedOrigins is the list of allowed Origin header values.
	AllowedOrigins []string `json:"allowed_origins" yaml:"allowed_origins" koanf:"allowed_origins"`

	// EndpointPath is the MCP endpoint path (default "/mcp").
	EndpointPath string `json:"endpoint_path" yaml:"endpoint_path" koanf:"endpoint_path"`
}

// ToolsConfig configures which MCP tools are enabled.
type ToolsConfig struct {
	// Enabled is the list of enabled tool names. Empty means all tools enabled.
	Enabled []string `json:"enabled" yaml:"enabled" koanf:"enabled"`
}

// ShutdownConfig configures graceful shutdown.
type ShutdownConfig struct {
	// GracePeriodSeconds is the time to allow in-flight requests to complete (default 30).
	GracePeriodSeconds int `json:"grace_period_seconds" yaml:"grace_period_seconds" koanf:"grace_period_seconds"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Transport: "stdio",
		HTTP: HTTPConfig{
			ListenAddr:   "127.0.0.1:7080",
			AuthMode:     "trust-headers",
			EndpointPath: "/mcp",
		},
		Shutdown: ShutdownConfig{
			GracePeriodSeconds: 30,
		},
	}
}
