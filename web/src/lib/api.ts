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
