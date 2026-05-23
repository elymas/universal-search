"use client";

import { useState, useEffect, useCallback } from "react";
import {
  fetchAdminAdapters,
  toggleAdapter,
  resyncAdapter,
  type AdminAdapter,
} from "@/lib/api";
import { ApiKeyRow } from "./api-key-row";
import { Skeleton } from "@/components/ui/skeleton";

// @MX:NOTE: [AUTO] Adapter status panel for SPEC-UI-002 Phase C3
export function AdapterStatusPanel() {
  const [adapters, setAdapters] = useState<AdminAdapter[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const data = await fetchAdminAdapters();
        setAdapters(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load adapters");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const handleToggle = useCallback(async (id: string, enabled: boolean) => {
    setBusyId(id);
    try {
      const updated = await toggleAdapter(id, enabled);
      setAdapters((prev) =>
        prev.map((a) => (a.id === id ? updated : a))
      );
    } finally {
      setBusyId(null);
    }
  }, []);

  const handleResync = useCallback(async (id: string) => {
    setBusyId(id);
    try {
      const updated = await resyncAdapter(id);
      setAdapters((prev) =>
        prev.map((a) => (a.id === id ? updated : a))
      );
    } finally {
      setBusyId(null);
    }
  }, []);

  return (
    <section>
      <h2 className="text-lg font-semibold mb-3">Adapter Status</h2>

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-10 rounded" />
          ))}
        </div>
      ) : error ? (
        <p className="text-sm text-destructive">{error}</p>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-border bg-muted/50">
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Name</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Status</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Last Sync</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">OK / Fail</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Last Error</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Secret</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Enabled</th>
                <th scope="col" className="px-3 py-2 text-xs font-medium text-muted-foreground">Action</th>
              </tr>
            </thead>
            <tbody>
              {adapters.map((adapter) => (
                <ApiKeyRow
                  key={adapter.id}
                  adapter={adapter}
                  onToggle={handleToggle}
                  onResync={handleResync}
                  busy={busyId === adapter.id}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
