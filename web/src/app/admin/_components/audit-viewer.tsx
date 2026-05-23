"use client";

import { useState, useEffect, useCallback } from "react";
import { fetchAdminAudit, type AuditEntry } from "@/lib/api";

// @MX:NOTE: [AUTO] Audit viewer with pagination for SPEC-UI-002 Phase C4

const PAGE_SIZE = 20;

export function AuditViewer() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [offset, setOffset] = useState(0);
  const [errorsOnly, setErrorsOnly] = useState(false);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const loadEntries = useCallback(async (newOffset: number, errOnly: boolean) => {
    setLoading(true);
    try {
      const res = await fetchAdminAudit({
        limit: PAGE_SIZE,
        offset: newOffset,
        errors_only: errOnly || undefined,
      });
      setEntries(res.entries);
      setTotal(res.total);
      setHasMore(res.has_more);
    } catch {
      // Silently handle — empty state will show
      setEntries([]);
      setTotal(0);
      setHasMore(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line -- data fetch on dependency change triggers setState
    loadEntries(offset, errorsOnly);
  }, [offset, errorsOnly, loadEntries]);

  const handleNext = () => {
    setOffset((prev) => prev + PAGE_SIZE);
  };

  const handlePrev = () => {
    setOffset((prev) => Math.max(0, prev - PAGE_SIZE));
  };

  const toggleErrorsOnly = () => {
    setErrorsOnly((prev) => !prev);
    setOffset(0);
  };

  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <section>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-lg font-semibold">Query Audit</h2>
        <label className="inline-flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="checkbox"
            checked={errorsOnly}
            onChange={toggleErrorsOnly}
            className="rounded border-border"
          />
          <span>Errors only</span>
        </label>
      </div>

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 bg-muted animate-pulse rounded" />
          ))}
        </div>
      ) : entries.length === 0 ? (
        <div className="text-sm text-muted-foreground py-8 text-center">
          No queries yet
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-border bg-muted/50">
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">ID</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Timestamp</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Latency</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Tokens</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Sources</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Config</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Error</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <tr
                  key={entry.id}
                  className={`border-b border-border hover:bg-accent/30 transition-colors ${
                    entry.error ? "bg-destructive/5" : ""
                  }`}
                >
                  <td className="px-3 py-2 text-sm font-mono">{entry.id}</td>
                  <td className="px-3 py-2 text-sm text-muted-foreground">
                    {new Date(entry.timestamp).toISOString()}
                  </td>
                  <td className="px-3 py-2 text-sm">{entry.latency_ms}ms</td>
                  <td className="px-3 py-2 text-sm">{entry.tokens}</td>
                  <td className="px-3 py-2 text-sm">{entry.sources_count}</td>
                  <td className="px-3 py-2 text-sm">
                    {entry.config_snapshot ? (
                      <button
                        type="button"
                        className="text-xs text-primary hover:underline"
                        onClick={() =>
                          setExpandedId(
                            expandedId === entry.id ? null : entry.id
                          )
                        }
                      >
                        {expandedId === entry.id ? "Hide" : "Show"}
                      </button>
                    ) : (
                      "—"
                    )}
                  </td>
                  <td className="px-3 py-2 text-sm">
                    {entry.error ? (
                      <span className="text-destructive">{entry.error}</span>
                    ) : (
                      "—"
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {expandedId && (
        <div className="mt-2 rounded-md border border-border bg-muted/30 p-3">
          {(() => {
            const entry = entries.find((e) => e.id === expandedId);
            if (!entry?.config_snapshot) return null;
            try {
              const parsed = JSON.parse(entry.config_snapshot);
              return (
                <pre className="text-xs font-mono whitespace-pre-wrap">
                  {JSON.stringify(parsed, null, 2)}
                </pre>
              );
            } catch {
              return (
                <pre className="text-xs font-mono whitespace-pre-wrap">
                  {entry.config_snapshot}
                </pre>
              );
            }
          })()}
        </div>
      )}

      {/* Pagination - always visible */}
      {!loading && (
        <div className="flex items-center justify-between mt-3">
          <span className="text-xs text-muted-foreground">
            Page {currentPage} of {totalPages} ({total} total)
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              aria-label="Previous page"
              disabled={offset === 0}
              onClick={handlePrev}
              className="inline-flex items-center rounded-md border border-border px-3 py-1 text-xs font-medium hover:bg-accent disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Previous
            </button>
            <button
              type="button"
              aria-label="Next page"
              disabled={!hasMore}
              onClick={handleNext}
              className="inline-flex items-center rounded-md border border-border px-3 py-1 text-xs font-medium hover:bg-accent disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
