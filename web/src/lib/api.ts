// @MX:NOTE: [AUTO] API client for Universal Search backend
// @MX:SPEC: SPEC-UI-001

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface SearchResult {
  answer: string;
  citations: Citation[];
  query: string;
  sources_used: string[];
  elapsed_ms: number;
}

export interface Citation {
  index: number;
  title: string;
  url: string;
  snippet: string;
  source: string;
}

export interface AdapterInfo {
  name: string;
  category: string;
  enabled: boolean;
  latency_ms?: number;
}

// @MX:ANCHOR: [AUTO] Core search query function used by multiple components
// @MX:REASON: Both streaming and buffered search paths depend on this
export async function searchQuery(
  query: string,
  sources?: string[]
): Promise<Response> {
  const params = new URLSearchParams({ q: query });
  if (sources?.length) params.set("sources", sources.join(","));
  return fetch(`${API_BASE}/api/query?${params}`);
}

export function searchStream(
  query: string,
  sources?: string[]
): EventSource {
  const params = new URLSearchParams({ q: query });
  if (sources?.length) params.set("sources", sources.join(","));
  return new EventSource(`${API_BASE}/api/query/stream?${params}`);
}

export async function fetchSources(): Promise<AdapterInfo[]> {
  const res = await fetch(`${API_BASE}/api/sources`);
  if (!res.ok) throw new Error("Failed to fetch sources");
  return res.json();
}

export async function fetchHistory(): Promise<
  Array<{ query: string; timestamp: string; id: string }>
> {
  const res = await fetch(`${API_BASE}/api/history`);
  if (!res.ok) throw new Error("Failed to fetch history");
  return res.json();
}

// --- Admin API Types ---

// @MX:NOTE: [AUTO] Admin adapter status type for SPEC-UI-002
export interface AdminAdapter {
  id: string;
  name: string;
  status: "connected" | "auth_required" | "disabled" | "error";
  enabled: boolean;
  last_sync: string | null;
  success_count: number;
  fail_count: number;
  last_error?: string | null;
  secret_source: string;
  secret_set: boolean;
}

export interface AuditEntry {
  id: string;
  timestamp: string;
  latency_ms: number;
  tokens: number;
  sources_count: number;
  config_snapshot?: string | null;
  error: string | null;
}

export interface AuditQueryParams {
  limit: number;
  offset: number;
  cursor?: string;
  errors_only?: boolean;
}

export interface AuditResponse {
  entries: AuditEntry[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
}

// --- Admin API Functions ---

// @MX:ANCHOR: [AUTO] Admin adapter data fetching for status panel and audit viewer
// @MX:REASON: Used by adapter-status-panel and audit-viewer components
export async function fetchAdminAdapters(): Promise<AdminAdapter[]> {
  const res = await fetch(`${API_BASE}/api/admin/adapters`);
  if (!res.ok) {
    if (res.status === 403) {
      throw new Error("403: Admin access restricted to localhost");
    }
    throw new Error(`Failed to fetch adapters: ${res.status}`);
  }
  return res.json();
}

export async function toggleAdapter(
  id: string,
  enabled: boolean
): Promise<AdminAdapter> {
  const res = await fetch(`${API_BASE}/api/admin/adapters/${id}/toggle`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  });
  if (!res.ok) {
    throw new Error(`Failed to toggle adapter: ${res.status}`);
  }
  return res.json();
}

export async function resyncAdapter(id: string): Promise<AdminAdapter> {
  const res = await fetch(`${API_BASE}/api/admin/adapters/${id}/resync`, {
    method: "POST",
  });
  if (!res.ok) {
    throw new Error(`Failed to resync adapter: ${res.status}`);
  }
  return res.json();
}

export async function fetchAdminAudit(
  params: AuditQueryParams
): Promise<AuditResponse> {
  const searchParams = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });
  if (params.cursor) searchParams.set("cursor", params.cursor);
  if (params.errors_only) searchParams.set("errors_only", "true");

  const res = await fetch(
    `${API_BASE}/api/admin/audit/queries?${searchParams}`
  );
  if (!res.ok) {
    if (res.status === 403) {
      throw new Error("403: Admin access restricted to localhost");
    }
    throw new Error(`Failed to fetch audit log: ${res.status}`);
  }
  return res.json();
}
