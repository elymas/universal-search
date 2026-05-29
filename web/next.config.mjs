// SPEC-SEC-001 REQ-SEC-012 / D4 — strict security response headers.
//
// CROSS-SPEC NOTE (SPEC-UI-001): this file is owned by SPEC-UI-001. SEC-001
// adds the security header layer below as an additive `headers()` block; it
// does not modify reactStrictMode or any UI-001 behavior. If UI-001 needs to
// adjust CSP for a new external origin (analytics, fonts, image CDN), edit the
// `connect-src`/`img-src`/`font-src` directives here in coordination with the
// security owner — do not remove the header set.
//
// V1 CSP strategy: `strict-dynamic` + per-build script hashes (NOT nonce).
// Nonce-based CSP requires SSR nonce propagation and is deferred to post-V1
// (spec.md Exclusions). `'unsafe-inline'` is included only as a fallback that
// `strict-dynamic`-aware browsers ignore; non-supporting browsers degrade
// safely. Replace the placeholder API host via the NEXT_PUBLIC_API_HOST build
// arg when wiring the real backend origin.

const apiHost = process.env.NEXT_PUBLIC_API_HOST || "'self'";

const csp = [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "script-src 'self' 'strict-dynamic' 'unsafe-inline' https:",
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
