// SPEC-SEC-001 REQ-SEC-012 / D4 — strict security response headers.
//
// CROSS-SPEC NOTE (SPEC-UI-001): this file is owned by SPEC-UI-001. SEC-001
// adds the security header layer below as an additive `headers()` block; it
// does not modify reactStrictMode or any UI-001 behavior. If UI-001 needs to
// adjust CSP for a new external origin (analytics, fonts, image CDN), edit the
// `connect-src`/`img-src`/`font-src` directives here in coordination with the
// security owner — do not remove the header set.
//
// V1 CSP strategy: `'self' 'unsafe-inline'` for scripts. The original
// `'strict-dynamic'` strategy assumed per-build script hashes (or nonces)
// would be injected, but neither is wired up. With `'strict-dynamic'` and no
// hash/nonce the browser ignores `'self'`/`'unsafe-inline'`/`https:` and blocks
// EVERY script — Next.js chunks and the inline bootstrap included — leaving the
// client app completely non-interactive. Nonce-based CSP is deferred to post-V1
// (spec.md Exclusions); until then `'unsafe-inline'` is the working fallback.
// Replace the placeholder API host via the NEXT_PUBLIC_API_HOST build arg when
// wiring the real backend origin.

const apiHost = process.env.NEXT_PUBLIC_API_HOST || "'self'";

const csp = [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "script-src 'self' 'unsafe-inline' https:",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self'",
  `connect-src 'self' ${apiHost}`,
  "form-action 'self'",
  "upgrade-insecure-requests",
].join("; ");

const securityHeaders = [
  { key: "Content-Security-Policy", value: csp },
  {
    key: "Strict-Transport-Security",
    value: "max-age=31536000; includeSubDomains",
  },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  {
    key: "Permissions-Policy",
    value: "camera=(), microphone=(), geolocation=()",
  },
];

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
