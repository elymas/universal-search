"use client";

import { useState, useCallback, useRef } from "react";
import { SearchInput } from "@/components/search-input";
import { ResultsPanel } from "@/components/results-panel";
import { Badge } from "@/components/ui/badge";
import {
  searchQuery,
  searchStream,
  type Citation,
  type SearchResult,
} from "@/lib/api";
import { createSSEConnection } from "@/lib/sse-client";

type FormatMode = "text" | "markdown";

export default function HomePage() {
  const [answer, setAnswer] = useState("");
  const [citations, setCitations] = useState<Citation[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [elapsedMs, setElapsedMs] = useState<number | undefined>();
  const [format, setFormat] = useState<FormatMode>("text");
  const [error, setError] = useState<string | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);

  const handleSearch = useCallback(
    async (query: string, sources: string[]) => {
      // Reset state
      setAnswer("");
      setCitations([]);
      setElapsedMs(undefined);
      setError(null);
      setIsStreaming(true);

      // Clean up any existing SSE connection
      cleanupRef.current?.();
      cleanupRef.current = null;

      try {
        // Try streaming first
        const eventSource = searchStream(query, sources);
        const cleanup = createSSEConnection(eventSource, {
          onSentence: (text) => {
            setAnswer((prev) => prev + text);
          },
          onCitation: (citation) => {
            setCitations((prev) => {
              // Avoid duplicates
              if (prev.some((c) => c.index === citation.index)) return prev;
              return [...prev, citation].sort((a, b) => a.index - b.index);
            });
          },
          onComplete: (ms) => {
            setIsStreaming(false);
            setElapsedMs(ms);
          },
          onError: (msg) => {
            setError(msg);
          },
        });

        cleanupRef.current = cleanup;

        // Fallback timeout: if no data received in 15s, fall back to buffered
        const fallbackTimer = setTimeout(async () => {
          if (!answer && isStreaming) {
            cleanupRef.current?.();
            cleanupRef.current = null;

            try {
              const res = await searchQuery(query, sources);
              if (!res.ok) throw new Error(`HTTP ${res.status}`);
              const data: SearchResult = await res.json();
              setAnswer(data.answer);
              setCitations(data.citations ?? []);
              setElapsedMs(data.elapsed_ms);
              setIsStreaming(false);
            } catch (fallbackErr) {
              setIsStreaming(false);
              setError(
                fallbackErr instanceof Error
                  ? fallbackErr.message
                  : "Search failed",
              );
            }
          }
        }, 15000);

        // Clear fallback timer on complete
        const origOnComplete = cleanup;
        eventSource.addEventListener("complete", () => {
          clearTimeout(fallbackTimer);
        });
        eventSource.addEventListener("error", () => {
          clearTimeout(fallbackTimer);
        });
      } catch (err) {
        setIsStreaming(false);
        setError(
          err instanceof Error ? err.message : "Failed to connect to server",
        );
      }
    },
    [answer, isStreaming],
  );

  return (
    <div className="flex flex-col min-h-full">
      {/* Header */}
      <header className="border-b border-border px-6 py-4 md:px-12">
        <h1 className="text-xl font-semibold pl-10 md:pl-0">Search</h1>
      </header>

      <div className="flex-1 px-6 py-6 md:px-12 space-y-6">
        {/* Search input */}
        <SearchInput onSubmit={handleSearch} isLoading={isStreaming} />

        {/* Format toggle */}
        <div className="flex items-center gap-2 max-w-3xl mx-auto">
          <span className="text-xs text-muted-foreground mr-1">Format:</span>
          <Badge
            variant={format === "text" ? "default" : "outline"}
            className="cursor-pointer text-xs"
            onClick={() => setFormat("text")}
          >
            Text
          </Badge>
          <Badge
            variant={format === "markdown" ? "default" : "outline"}
            className="cursor-pointer text-xs"
            onClick={() => setFormat("markdown")}
          >
            Markdown
          </Badge>
        </div>

        {/* Error display */}
        {error && (
          <div className="max-w-3xl mx-auto rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}

        {/* Results */}
        <div className="max-w-3xl mx-auto">
          <ResultsPanel
            answer={answer}
            citations={citations}
            isStreaming={isStreaming}
            elapsedMs={elapsedMs}
            format={format}
          />
        </div>
      </div>
    </div>
  );
}
