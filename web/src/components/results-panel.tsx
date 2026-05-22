"use client";

import { useMemo } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { CitationBadge } from "@/components/citation-badge";
import type { Citation } from "@/lib/api";

interface ResultsPanelProps {
  /** Accumulated answer text (streaming or buffered) */
  answer: string;
  /** Citations collected so far */
  citations: Citation[];
  /** Whether the stream is still in progress */
  isStreaming?: boolean;
  /** Query elapsed time in ms */
  elapsedMs?: number;
  /** Format toggle: "text" or "markdown" */
  format?: "text" | "markdown";
}

/**
 * Renders search results with inline citation badges.
 * In streaming mode, the answer text grows progressively.
 * Citations like [1], [2] in the answer text are replaced with hoverable CitationBadge components.
 */
export function ResultsPanel({
  answer,
  citations,
  isStreaming,
  elapsedMs,
  format = "text",
}: ResultsPanelProps) {
  const citationMap = useMemo(() => {
    const map = new Map<number, Citation>();
    for (const c of citations) {
      map.set(c.index, c);
    }
    return map;
  }, [citations]);

  // Parse answer text to split around [N] citation markers
  const renderedContent = useMemo(() => {
    if (!answer) return null;

    // Split on citation patterns like [1], [2], [12], etc.
    const parts = answer.split(/(\[\d+\])/g);

    return parts.map((part, i) => {
      const match = part.match(/^\[(\d+)\]$/);
      if (match) {
        const idx = parseInt(match[1], 10);
        const citation = citationMap.get(idx);
        if (citation) {
          return <CitationBadge key={`cite-${idx}`} citation={citation} />;
        }
        // Fallback: render the raw [N] if citation data not yet received
        return (
          <span key={i} className="text-muted-foreground">
            {part}
          </span>
        );
      }

      if (format === "markdown") {
        // Simple markdown rendering: bold, italic, code, line breaks
        return (
          <span
            key={i}
            className="whitespace-pre-wrap"
            dangerouslySetInnerHTML={{
              __html: part
                .replace(/&/g, "&amp;")
                .replace(/</g, "&lt;")
                .replace(/>/g, "&gt;")
                .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
                .replace(/\*(.+?)\*/g, "<em>$1</em>")
                .replace(/`(.+?)`/g, "<code>$1</code>")
                .replace(/\n/g, "<br />"),
            }}
          />
        );
      }

      return (
        <span key={i} className="whitespace-pre-wrap">
          {part}
        </span>
      );
    });
  }, [answer, citationMap, format]);

  if (!answer && !isStreaming) {
    return (
      <div className="text-center text-muted-foreground py-12">
        <p className="text-lg">Search results will appear here</p>
        <p className="text-sm mt-1">
          Enter a query above to start searching
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Answer content */}
      <div className="text-base leading-relaxed">{renderedContent}</div>

      {/* Streaming indicator */}
      {isStreaming && (
        <div className="flex items-center gap-2">
          <div className="flex gap-1">
            <span className="w-1.5 h-1.5 bg-primary rounded-full animate-pulse" />
            <span
              className="w-1.5 h-1.5 bg-primary rounded-full animate-pulse"
              style={{ animationDelay: "0.2s" }}
            />
            <span
              className="w-1.5 h-1.5 bg-primary rounded-full animate-pulse"
              style={{ animationDelay: "0.4s" }}
            />
          </div>
          <span className="text-xs text-muted-foreground">
            Receiving results...
          </span>
        </div>
      )}

      {/* Elapsed time and source badges */}
      {!isStreaming && elapsedMs !== undefined && elapsedMs > 0 && (
        <>
          <Separator />
          <div className="flex items-center gap-3 flex-wrap">
            <span className="text-xs text-muted-foreground">
              {elapsedMs < 1000
                ? `${elapsedMs}ms`
                : `${(elapsedMs / 1000).toFixed(1)}s`}
            </span>
            {citations.length > 0 && (
              <Badge variant="outline" className="text-xs">
                {citations.length} source{citations.length !== 1 ? "s" : ""}
              </Badge>
            )}
          </div>
        </>
      )}

      {/* Loading skeleton when waiting for first content */}
      {!answer && isStreaming && (
        <div className="space-y-3">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
          <Skeleton className="h-4 w-4/6" />
        </div>
      )}
    </div>
  );
}
