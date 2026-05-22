// Package main — config subcommand tree for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-012: usearch config {path,show,init,get,set}.
// Manages XDG-compliant TOML config file via the koanf-based config loader.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/elymas/universal-search/internal/usearch/config"
)

// newConfigCmd creates the cobra command tree for the config subcommand.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage usearch configuration",
		Long: `Manage the usearch configuration file.

Configuration is loaded from an XDG-compliant TOML file with layered precedence:
  1. Command-line flags (highest)
  2. Environment variables (USEARCH_* prefix)
  3. Config file (~/.config/usearch/config.toml)
  4. Built-in defaults (lowest)`,
		Example: `  usearch config path
  usearch config show
  usearch config init
  usearch config get server.endpoint
  usearch config set server.endpoint http://localhost:9090`,
	}

	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())

	return cmd
}

// newConfigPathCmd returns the config path subcommand.
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), config.ConfigPath())
			return nil
		},
	}
}

// newConfigShowCmd returns the config show subcommand.
func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the effective merged configuration",
		Long:  "Print the effective merged configuration (after all precedence layers) in TOML format.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return exitError{code: ExitUserError, err: err}
			}
			// Output the config as TOML.
			toml, err := config.ToTOML(cfg)
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}
			fmt.Fprint(cmd.OutOrStdout(), toml)
			return nil
		},
	}
}

// newConfigInitCmd returns the config init subcommand.
func newConfigInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default config file",
		Long:  "Create a default config file at the XDG config path. Refuses to overwrite an existing file unless --force is set.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.ConfigPath()

			// Check if file already exists.
			if _, err := os.Stat(cfgPath); err == nil && !force {
				return exitError{
					code: ExitUserError,
					err:  fmt.Errorf("config file already exists at %s; use --force to overwrite", cfgPath),
				}
			}

			// Create parent directory.
			dir := filepath.Dir(cfgPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return exitError{code: ExitSystemError, err: fmt.Errorf("create config dir: %w", err)}
			}

			// Write default config.
			def := config.DefaultConfig()
			toml, err := config.ToTOML(&def)
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			if err := os.WriteFile(cfgPath, []byte(toml), 0o644); err != nil {
				return exitError{code: ExitSystemError, err: fmt.Errorf("write config: %w", err)}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Config file created at %s\n", cfgPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")

	return cmd
}

// newConfigGetCmd returns the config get subcommand.
func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print the value of a single config key",
		Long:  "Print the value of a single config key after precedence resolution. Key format: section.name (e.g. server.endpoint).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			cfg, err := config.Load()
			if err != nil {
				return exitError{code: ExitUserError, err: err}
			}
			val, err := config.GetKey(cfg, key)
			if err != nil {
				return exitError{code: ExitUserError, err: err}
			}
			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

// newConfigSetCmd returns the config set subcommand.
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config key value in the config file",
		Long:  "Write a key-value pair to the config file. Creates the file if missing. Refuses to write keys in the [auth] token namespace.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			// Refuse to write token keys (NFR-CLI2-005).
			if key == "auth.token" {
				return exitError{
					code: ExitUserError,
					err:  fmt.Errorf("usearch config: refusing to write 'auth.token' to config.toml; use the credentials file at %s instead", filepath.Join(config.ConfigHome(), "usearch", "credentials")),
				}
			}

			cfgPath := config.ConfigPath()

			// Create parent directory if needed.
			dir := filepath.Dir(cfgPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return exitError{code: ExitSystemError, err: fmt.Errorf("create config dir: %w", err)}
			}

			if err := config.SetKey(cfgPath, key, value); err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
			return nil
		},
	}
}
