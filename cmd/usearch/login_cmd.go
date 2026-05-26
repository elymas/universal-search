// Package main — login subcommand for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-007: usearch login {status,logout} skeleton.
// Real OIDC authentication deferred to SPEC-AUTH-001.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newLoginCmd creates the cobra command tree for the login subcommand.
func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Manage authentication",
		Long: `Manage authentication state.

Status shows current authentication state.
Logout clears stored credentials.

Note: Full OIDC authentication is planned for a future release.`,
		Example: `  usearch login status
	  usearch login logout`,
	}

	cmd.AddCommand(newLoginStatusCmd())
	cmd.AddCommand(newLoginLogoutCmd())

	return cmd
}

// newLoginStatusCmd creates the login status subcommand.
func newLoginStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// @MX:TODO: [AUTO] Check credential file existence when SPEC-AUTH-001 lands.
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Not authenticated.")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Use 'usearch login' to authenticate (coming soon).")
			return nil
		},
	}

	return cmd
}

// newLoginLogoutCmd creates the login logout subcommand.
func newLoginLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			// @MX:TODO: [AUTO] Clear credential file when SPEC-AUTH-001 lands.
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No active session. Already logged out.")
			return nil
		},
	}

	return cmd
}
