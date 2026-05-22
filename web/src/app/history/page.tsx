"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Clock, Trash2, Search, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchHistory } from "@/lib/api";

interface HistoryEntry {
  query: string;
  timestamp: string;
  id: string;
}

export default function HistoryPage() {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const hasLoaded = useRef(false);

  useEffect(() => {
    if (hasLoaded.current) return;
    hasLoaded.current = true;

    let cancelled = false;

    async function load() {
      try {
        const data = await fetchHistory();
        if (!cancelled) setEntries(data);
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Failed to load history"
          );
          setEntries([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  const handleClear = useCallback(() => {
    setEntries([]);
  }, []);

  const handleReExecute = useCallback((query: string) => {
    window.location.href = `/?q=${encodeURIComponent(query)}`;
  }, []);

  const formatTimestamp = (ts: string) => {
    try {
      const date = new Date(ts);
      const now = new Date();
      const diffMs = now.getTime() - date.getTime();
      const diffMins = Math.floor(diffMs / 60000);
      const diffHours = Math.floor(diffMs / 3600000);
      const diffDays = Math.floor(diffMs / 86400000);

      if (diffMins < 1) return "Just now";
      if (diffMins < 60) return `${diffMins}m ago`;
      if (diffHours < 24) return `${diffHours}h ago`;
      if (diffDays < 7) return `${diffDays}d ago`;
      return date.toLocaleDateString();
    } catch {
      return ts;
    }
  };

  return (
    <div className="flex flex-col min-h-full">
      <header className="border-b border-border px-6 py-4 md:px-12">
        <div className="flex items-center justify-between pl-10 md:pl-0">
          <h1 className="text-xl font-semibold flex items-center gap-2">
            <Clock className="h-5 w-5" />
            History
          </h1>
          {entries.length > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleClear}
              className="text-destructive hover:text-destructive"
            >
              <Trash2 className="h-3 w-3 mr-1" />
              Clear
            </Button>
          )}
        </div>
      </header>

      <div className="flex-1 px-6 py-6 md:px-12">
        {loading ? (
          <div className="space-y-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-md" />
            ))}
          </div>
        ) : error && entries.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground">
            <Search className="h-10 w-10 mx-auto mb-3 opacity-50" />
            <p className="text-lg">No search history yet</p>
            <p className="text-sm mt-1">
              Your past searches will appear here
            </p>
          </div>
        ) : (
          <div className="space-y-2 max-w-3xl">
            {entries.map((entry, i) => (
              <Card key={entry.id ?? i} className="hover:bg-accent/50 transition-colors">
                <CardContent className="flex items-center justify-between py-3 px-4">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">
                      {entry.query}
                    </p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {formatTimestamp(entry.timestamp)}
                    </p>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleReExecute(entry.query)}
                    aria-label={`Re-execute: ${entry.query}`}
                  >
                    <RotateCcw className="h-4 w-4" />
                  </Button>
                </CardContent>
                {i < entries.length - 1 && <Separator />}
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
