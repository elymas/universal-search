"use client";

import { useState, useEffect } from "react";
import { Database, Globe, MessageSquare, GraduationCap, Flag } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchSources, type AdapterInfo } from "@/lib/api";

const CATEGORY_ICONS: Record<string, React.ElementType> = {
  web: Globe,
  social: MessageSquare,
  academic: GraduationCap,
  korean: Flag,
};

const CATEGORY_COLORS: Record<string, string> = {
  web: "bg-blue-500/10 text-blue-500",
  social: "bg-purple-500/10 text-purple-500",
  academic: "bg-green-500/10 text-green-500",
  korean: "bg-red-500/10 text-red-500",
};

export default function SourcesPage() {
  const [sources, setSources] = useState<AdapterInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const data = await fetchSources();
        setSources(data);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to load sources"
        );
        // Demo fallback: show placeholder sources
        setSources([
          { name: "Google", category: "web", enabled: true, latency_ms: 230 },
          { name: "Bing", category: "web", enabled: true, latency_ms: 180 },
          { name: "DuckDuckGo", category: "web", enabled: true, latency_ms: 310 },
          { name: "Twitter/X", category: "social", enabled: true, latency_ms: 420 },
          { name: "Reddit", category: "social", enabled: true, latency_ms: 350 },
          { name: "ArXiv", category: "academic", enabled: true, latency_ms: 540 },
          { name: "Semantic Scholar", category: "academic", enabled: true, latency_ms: 480 },
          { name: "Naver", category: "korean", enabled: true, latency_ms: 200 },
          { name: "Daum/Kakao", category: "korean", enabled: true, latency_ms: 190 },
        ]);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const grouped = sources.reduce(
    (acc, source) => {
      const cat = source.category ?? "web";
      if (!acc[cat]) acc[cat] = [];
      acc[cat].push(source);
      return acc;
    },
    {} as Record<string, AdapterInfo[]>
  );

  return (
    <div className="flex flex-col min-h-full">
      <header className="border-b border-border px-6 py-4 md:px-12">
        <h1 className="text-xl font-semibold flex items-center gap-2 pl-10 md:pl-0">
          <Database className="h-5 w-5" />
          Sources
        </h1>
        <p className="text-sm text-muted-foreground mt-1 pl-10 md:pl-0">
          Available search adapters and their status
        </p>
      </header>

      <div className="flex-1 px-6 py-6 md:px-12">
        {loading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-28 rounded-lg" />
            ))}
          </div>
        ) : (
          <div className="space-y-8 max-w-4xl">
            {Object.entries(grouped).map(([category, adapters]) => {
              const Icon = CATEGORY_ICONS[category] ?? Globe;
              const colorClass = CATEGORY_COLORS[category] ?? "bg-gray-500/10 text-gray-500";

              return (
                <section key={category}>
                  <div className="flex items-center gap-2 mb-3">
                    <Icon className="h-4 w-4" />
                    <h2 className="text-lg font-semibold capitalize">
                      {category}
                    </h2>
                    <Badge variant="outline" className="text-xs">
                      {adapters.length}
                    </Badge>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                    {adapters.map((adapter) => (
                      <Card
                        key={adapter.name}
                        className="hover:bg-accent/50 transition-colors"
                      >
                        <CardHeader className="pb-2 pt-4 px-4">
                          <CardTitle className="text-sm flex items-center justify-between">
                            <span className="truncate">{adapter.name}</span>
                            <Badge
                              variant={adapter.enabled ? "default" : "outline"}
                              className={`text-[10px] ml-2 shrink-0 ${
                                adapter.enabled ? "" : "text-muted-foreground"
                              }`}
                            >
                              {adapter.enabled ? "Active" : "Disabled"}
                            </Badge>
                          </CardTitle>
                        </CardHeader>
                        <CardContent className="px-4 pb-4">
                          <div className="flex items-center gap-2">
                            <Badge
                              variant="secondary"
                              className={`text-[10px] ${colorClass}`}
                            >
                              {category}
                            </Badge>
                            {adapter.latency_ms !== undefined && (
                              <span className="text-[10px] text-muted-foreground">
                                ~{adapter.latency_ms}ms
                              </span>
                            )}
                          </div>
                        </CardContent>
                      </Card>
                    ))}
                  </div>
                </section>
              );
            })}
          </div>
        )}

        {error && (
          <p className="text-xs text-muted-foreground mt-4">
            Using demo data (backend not reachable)
          </p>
        )}
      </div>
    </div>
  );
}
