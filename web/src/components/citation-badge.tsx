"use client";

import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ExternalLink } from "lucide-react";
import type { Citation } from "@/lib/api";

interface CitationBadgeProps {
  citation: Citation;
}

export function CitationBadge({ citation }: CitationBadgeProps) {
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge
            variant="secondary"
            className="inline-flex items-center gap-0.5 cursor-pointer text-xs font-mono px-1.5 py-0.5 hover:bg-primary hover:text-primary-foreground transition-colors"
          >
            [{citation.index}]
          </Badge>
        </TooltipTrigger>
        <TooltipContent
          side="top"
          className="max-w-xs p-3 space-y-2"
        >
          <p className="font-medium text-sm leading-tight">
            {citation.title}
          </p>
          {citation.snippet && (
            <p className="text-xs text-muted-foreground line-clamp-3">
              {citation.snippet}
            </p>
          )}
          {citation.url && (
            <a
              href={citation.url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
            >
              <ExternalLink className="h-3 w-3" />
              <span className="truncate max-w-[200px]">{citation.url}</span>
            </a>
          )}
          <p className="text-[10px] text-muted-foreground uppercase tracking-wider">
            {citation.source}
          </p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
