import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Import after mocking
import {
  fetchAdminAdapters,
  fetchAdminAudit,
  toggleAdapter,
  resyncAdapter,
  type AdminAdapter,
  type AuditEntry,
} from "@/lib/api";

beforeEach(() => {
  vi.clearAllMocks();
});

describe("fetchAdminAdapters", () => {
  it("returns typed data on 200", async () => {
    const mockData: AdminAdapter[] = [
      {
        id: "google",
        name: "Google",
        status: "connected",
        enabled: true,
        last_sync: "2026-01-01T00:00:00Z",
        success_count: 100,
        fail_count: 2,
        secret_source: "GOOGLE_API_KEY",
        secret_set: true,
      },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockData),
    });

    const result = await fetchAdminAdapters();
    expect(result).toEqual(mockData);
    expect(result[0].id).toBe("google");
    expect(result[0].status).toBe("connected");
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/admin/adapters"),
    );
  });

  it("throws on 403", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 403,
      statusText: "Forbidden",
    });

    await expect(fetchAdminAdapters()).rejects.toThrow("403");
  });
});

describe("toggleAdapter", () => {
  it("calls correct endpoint with POST and enabled param", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ id: "google", enabled: false }),
    });

    await toggleAdapter("google", false);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/admin/adapters/google/toggle"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
        }),
        body: JSON.stringify({ enabled: false }),
      }),
    );
  });
});

describe("resyncAdapter", () => {
  it("calls correct endpoint with POST", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ id: "slack", status: "syncing" }),
    });

    await resyncAdapter("slack");

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/admin/adapters/slack/resync"),
      expect.objectContaining({
        method: "POST",
      }),
    );
  });
});

describe("fetchAdminAudit", () => {
  it("passes query params correctly", async () => {
    const mockResponse = {
      entries: [
        {
          id: "q1",
          timestamp: "2026-01-01T00:00:00Z",
          latency_ms: 150,
          tokens: 200,
          sources_count: 3,
          error: null,
        } as AuditEntry,
      ],
      total: 1,
      has_more: false,
    };
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const result = await fetchAdminAudit({
      limit: 10,
      offset: 20,
    });

    expect(result).toEqual(mockResponse);
    const calledUrl = mockFetch.mock.calls[0][0] as string;
    expect(calledUrl).toContain("limit=10");
    expect(calledUrl).toContain("offset=20");
  });

  it("handles errors_only filter param", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ entries: [], total: 0, has_more: false }),
    });

    await fetchAdminAudit({ limit: 10, offset: 0, errors_only: true });

    const calledUrl = mockFetch.mock.calls[0][0] as string;
    expect(calledUrl).toContain("errors_only=true");
  });
});
