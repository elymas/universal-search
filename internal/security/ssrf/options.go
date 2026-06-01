// Package ssrf provides generic, behavior-preserving SSRF guards extracted
// from the SPEC-CACHE-001 access cascade (REQ-CACHE-013) so they can be reused
// by the access cascade, OIDC discovery, and future user-provided-URL fetchers
// (RSS, webhooks).
//
// SPEC-SEC-001 REQ-SEC-007: the exported API PRESERVES the semantics of the
// original unexported access guards, in particular the per-call FetchOptions
// override (AllowPrivateNetworks) and the RedirectMaxHops option name.
package ssrf

const (
	// DefaultRedirectMaxHops is the redirect hop cap when Options.RedirectMaxHops
	// is zero. Matches the original access defaultRedirectMaxHops (5).
	DefaultRedirectMaxHops = 5
)

// DefaultSchemeAllowlist is the set of permitted URL schemes. http/https only,
// matching the original access validateScheme behavior.
var DefaultSchemeAllowlist = []string{"http", "https"}

// Options holds fetcher-level (constructor-time) SSRF configuration.
// Zero values are replaced by documented defaults at evaluation time.
type Options struct {
	// AllowPrivateNetworks permits fetching RFC1918/loopback/link-local
	// addresses. MUST be false in production; true only for httptest stubs.
	AllowPrivateNetworks bool

	// RedirectMaxHops caps redirect chains. Zero -> DefaultRedirectMaxHops.
	// NOTE: kept as "RedirectMaxHops" (NOT "MaxRedirects") to preserve the
	// existing access.Options field name and SPEC-SEC-007 contract.
	RedirectMaxHops int

	// HostnameBlocklist blocks requests to these hostnames (case-insensitive,
	// exact or "*.suffix" match). Nil -> DefaultHostnameBlocklist (cloud
	// metadata endpoints, REQ-SEC-008).
	HostnameBlocklist []string

	// SchemeAllowlist limits permitted URL schemes. Nil -> DefaultSchemeAllowlist.
	SchemeAllowlist []string
}

// FetchOptions holds per-call overrides that take precedence over the
// fetcher-level Options. Preserves the original access.FetchOptions dual
// AllowPrivateNetworks override (REQ-SEC-007).
type FetchOptions struct {
	// AllowPrivateNetworks overrides Options.AllowPrivateNetworks per-call.
	AllowPrivateNetworks bool
}

// allowPrivate reports whether either the fetcher-level or per-call override
// permits private networks. This is the exact `opts.AllowPrivateNetworks ||
// fopts.AllowPrivateNetworks` semantics from the original access guards.
func allowPrivate(opts Options, fopts FetchOptions) bool {
	return opts.AllowPrivateNetworks || fopts.AllowPrivateNetworks
}

// redirectMaxHops resolves the effective hop cap, applying the default.
func redirectMaxHops(opts Options) int {
	if opts.RedirectMaxHops == 0 {
		return DefaultRedirectMaxHops
	}
	return opts.RedirectMaxHops
}

// schemeAllowlist resolves the effective scheme allowlist, applying the default.
func schemeAllowlist(opts Options) []string {
	if len(opts.SchemeAllowlist) == 0 {
		return DefaultSchemeAllowlist
	}
	return opts.SchemeAllowlist
}

// hostnameBlocklist resolves the effective hostname blocklist, applying the
// cloud-metadata default when unset.
func hostnameBlocklist(opts Options) []string {
	if opts.HostnameBlocklist == nil {
		return DefaultHostnameBlocklist
	}
	return opts.HostnameBlocklist
}
