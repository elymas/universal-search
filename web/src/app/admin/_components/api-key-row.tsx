"use client";

import type { AdminAdapter } from "@/lib/api";

// @MX:NOTE: [AUTO] Per-adapter API key row - shows secret source and toggle for SPEC-UI-002
interface ApiKeyRowProps {
  adapter: AdminAdapter;
  onToggle: (id: string, enabled: boolean) => Promise<void>;
  onResync: (id: string) => Promise<void>;
  busy: boolean;
}

export function ApiKeyRow({ adapter, onToggle, onResync, busy }: ApiKeyRowProps) {
  const statusColors: Record<string, string> = {
    connected: "bg-green-500/10 text-green-500",
    auth_required: "bg-yellow-500/10 text-yellow-500",
    disabled: "bg-gray-500/10 text-gray-500",
    error: "bg-red-500/10 text-red-500",
  };

  return (
    <tr className="border-b border-border hover:bg-accent/30 transition-colors">
      {/* Adapter ID + Name */}
      <td className="px-3 py-2 text-sm font-medium">{adapter.name}</td>

      {/* Status badge */}
      <td className="px-3 py-2">
        <span
          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
            statusColors[adapter.status] ?? statusColors.disabled
          }`}
        >
          {adapter.status}
        </span>
      </td>

      {/* Last sync */}
      <td className="px-3 py-2 text-sm text-muted-foreground">
        {adapter.last_sync
          ? new Date(adapter.last_sync).toLocaleString()
          : "—"}
      </td>

      {/* Success / Fail */}
      <td className="px-3 py-2 text-sm text-muted-foreground">
        {adapter.success_count} / {adapter.fail_count}
      </td>

      {/* Last error */}
      <td className="px-3 py-2 text-xs text-destructive max-w-48 truncate">
        {adapter.last_error ?? "—"}
      </td>

      {/* Secret source */}
      <td className="px-3 py-2 text-sm">
        <span className="font-mono text-xs">{adapter.secret_source}</span>
        <span
          className={`ml-2 text-xs ${
            adapter.secret_set ? "text-green-500" : "text-muted-foreground"
          }`}
        >
          {adapter.secret_set ? "set" : "unset"}
        </span>
      </td>

      {/* Toggle */}
      <td className="px-3 py-2">
        <button
          type="button"
          aria-label={`Toggle ${adapter.id}`}
          aria-pressed={adapter.enabled}
          disabled={busy}
          onClick={() => onToggle(adapter.id, !adapter.enabled)}
          className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
            adapter.enabled ? "bg-primary" : "bg-muted"
          } ${busy ? "opacity-50 cursor-not-allowed" : ""}`}
        >
          <span
            className={`pointer-events-none inline-block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform ${
              adapter.enabled ? "translate-x-4" : "translate-x-0"
            }`}
          />
        </button>
      </td>

      {/* Re-sync */}
      <td className="px-3 py-2">
        <button
          type="button"
          aria-label={`Resync ${adapter.id}`}
          disabled={busy}
          onClick={() => onResync(adapter.id)}
          className="inline-flex items-center rounded-md border border-border px-2 py-1 text-xs font-medium hover:bg-accent disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          Re-sync
        </button>
      </td>
    </tr>
  );
}
