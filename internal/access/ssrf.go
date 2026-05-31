// Package access — SSRF guard functions.
//
// REQ-CACHE-013: Four HARD-rule SSRF guards.
// D6: Scheme allowlist, private/loopback deny-by-default, DNS-rebind
// mitigation (pinnedIPDialer), redirect re-validation with hop cap.
//
// SPEC-SEC-001 REQ-SEC-007 (DDD IMPROVE): the guards now DELEGATE to the
// generic internal/security/ssrf package. These thin wrappers preserve the
// CACHE-001 call-site signatures (access.Options / access.FetchOptions) and
// translate the generic ssrf.Error into the *FetchError{CategoryBlocked} that
// cascade callers expect — behavior is byte-identical to the original guards,
// plus the new cloud-metadata hostname blocklist (REQ-SEC-008). A block also
// increments usearch_security_ssrf_blocks_total{reason, component="access"}.
package access

import (
	"context"
	"net"
	"net/url"

	secssrf "github.com/elymas/universal-search/internal/security/ssrf"
)

// toSecOptions maps access.Options onto the generic ssrf.Options. The default
// hostname blocklist (cloud metadata) and scheme allowlist (http/https) are
// applied by the ssrf package when left zero.
func toSecOptions(opts Options) secssrf.Options {
	return secssrf.Options{
		AllowPrivateNetworks: opts.AllowPrivateNetworks,
		RedirectMaxHops:      opts.RedirectMaxHops,
	}
}

func toSecFetchOptions(fopts FetchOptions) secssrf.FetchOptions {
	return secssrf.FetchOptions{AllowPrivateNetworks: fopts.AllowPrivateNetworks}
}

// fromSecError translates a generic ssrf.Error into the access *FetchError
// contract (CategoryBlocked + preserved reason message) and records the SSRF
// block metric. Non-ssrf errors are returned unchanged.
func fromSecError(err error) error {
	if err == nil {
		return nil
	}
	reason := secssrf.ReasonOf(err)
	if reason == "" {
		// Not an ssrf.Error — pass through unchanged.
		return err
	}
	recordSSRFBlock(string(reason))
	return &FetchError{
		Category: CategoryBlocked,
		Reason:   err.Error(),
	}
}

// validateScheme checks that the URL scheme is http or https.
func validateScheme(u *url.URL) error {
	return fromSecError(secssrf.ValidateScheme(u))
}

// validateHost resolves the hostname and blocks private/loopback/link-local
// IPs (unless AllowPrivateNetworks is set) and cloud-metadata hostnames.
func validateHost(ctx context.Context, u *url.URL, opts Options, fopts FetchOptions) error {
	return fromSecError(secssrf.ValidateHost(ctx, u, toSecOptions(opts), toSecFetchOptions(fopts)))
}

// validateRedirect re-runs scheme + host checks for a redirect destination
// and enforces the redirect hop count cap.
func validateRedirect(next *url.URL, opts Options, fopts FetchOptions, hopCount int) error {
	return fromSecError(secssrf.ValidateRedirect(next, toSecOptions(opts), toSecFetchOptions(fopts), hopCount))
}

// isPrivateOrLoopback reports whether ip is private, loopback, or link-local.
func isPrivateOrLoopback(ip net.IP) bool {
	return secssrf.IsPrivateOrLoopback(ip)
}
