"use client";

import { useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Search, Loader2 } from "lucide-react";

const SOURCE_CATEGORIES = [
  { id: "web", label: "Web" },
  { id: "social", label: "Social" },
  { id: "academic", label: "Academic" },
  { id: "korean", label: "Korean" },
] as const;

interface SearchInputProps {
  onSubmit: (query: string, sources: string[]) => void;
  isLoading?: boolean;
}

export function SearchInput({ onSubmit, isLoading }: SearchInputProps) {
  const [query, setQuery] = useState("");
  const [selectedSources, setSelectedSources] = useState<string[]>([]);

  const toggleSource = useCallback((sourceId: string) => {
    setSelectedSources((prev) =>
      prev.includes(sourceId)
        ? prev.filter((s) => s !== sourceId)
        : [...prev, sourceId],
    );
  }, []);

  const handleSubmit = useCallback(() => {
    const trimmed = query.trim();
    if (!trimmed || isLoading) return;
    onSubmit(trimmed, selectedSources);
  }, [query, selectedSources, isLoading, onSubmit]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        handleSubmit();
      }
    },
    [handleSubmit],
  );

  return (
    <div className="w-full max-w-3xl mx-auto space-y-3">
      <div className="relative">
        <Textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Ask anything... (Enter to search, Shift+Enter for new line)"
          className="min-h-[100px] resize-none pr-14 text-base bg-background border-border focus-visible:ring-ring"
          disabled={isLoading}
          aria-label="Search query input"
        />
        <Button
          onClick={handleSubmit}
          disabled={!query.trim() || isLoading}
          size="icon"
          className="absolute bottom-3 right-3 h-9 w-9"
          aria-label="Submit search"
        >
          {isLoading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Search className="h-4 w-4" />
          )}
        </Button>
      </div>

      <div
        className="flex flex-wrap gap-2"
        role="group"
        aria-label="Source filters"
      >
        {SOURCE_CATEGORIES.map((cat) => {
          const active = selectedSources.includes(cat.id);
          return (
            <Badge
              key={cat.id}
              variant={active ? "default" : "outline"}
              className="cursor-pointer select-none transition-colors"
              onClick={() => toggleSource(cat.id)}
              role="checkbox"
              aria-checked={active}
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  toggleSource(cat.id);
                }
              }}
            >
              {cat.label}
            </Badge>
          );
        })}
      </div>
    </div>
  );
}
