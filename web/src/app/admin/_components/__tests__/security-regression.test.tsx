import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AdapterStatusPanel } from "@/app/admin/_components/adapter-status-panel";
import { AuditViewer } from "@/app/admin/_components/audit-viewer";
import { LocalhostGate } from "@/app/admin/_components/localhost-gate";
import type { AdminAdapter, AuditEntry } from "@/lib/api";

// Mock API
vi.mock("@/lib/api", () => ({
  fetchAdminAdapters: vi.fn(),
  fetchAdminAudit: vi.fn(),
  toggleAdapter: vi.fn(),
  resyncAdapter: vi.fn(),
}));

import { fetchAdminAdapters, fetchAdminAudit } from "@/lib/api";

beforeEach(() => {
  vi.clearAllMocks();
});

describe("Security Regression Tests", () => {
  describe("AK-2.3 — No secret input fields", () => {
    it("adapter panel has no password input fields", async () => {
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([
        makeAdapter("google", true),
      ]);

      const { container } = render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("Google")).toBeInTheDocument();
      });

      const passwordInputs = container.querySelectorAll(
        'input[type="password"]'
      );
      expect(passwordInputs).toHaveLength(0);
    });

    it("adapter panel has no inputs with secret-like names", async () => {
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([
        makeAdapter("slack", true),
      ]);

      const { container } = render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("Slack")).toBeInTheDocument();
      });

      const secretInputs = container.querySelectorAll(
        'input[name*="token"], input[name*="key"], input[name*="secret"], input[name*="password"]'
      );
      expect(secretInputs).toHaveLength(0);
    });

    it("adapter panel has no textarea or editable fields for secret values", async () => {
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([
        makeAdapter("google", true),
      ]);

      const { container } = render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("Google")).toBeInTheDocument();
      });

      const textareas = container.querySelectorAll("textarea");
      expect(textareas).toHaveLength(0);

      // All inputs in admin panel should be checkboxes (errors-only filter) or buttons, not text fields for secrets
      const textInputs = container.querySelectorAll(
        'input[type="text"], input[type="url"]'
      );
      expect(textInputs).toHaveLength(0);
    });

    it("audit viewer has no textarea or editable fields for secret values", async () => {
      vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
        entries: [makeAuditEntry()],
        total: 1,
        has_more: false,
      });

      const { container } = render(<AuditViewer />);

      await waitFor(() => {
        expect(screen.getByText("q1")).toBeInTheDocument();
      });

      const textareas = container.querySelectorAll("textarea");
      expect(textareas).toHaveLength(0);

      const textInputs = container.querySelectorAll(
        'input[type="text"], input[type="password"], input[type="url"]'
      );
      expect(textInputs).toHaveLength(0);
    });
  });

  describe("LH-4.4 — Localhost gate hides error details", () => {
    it("shows advisory message, not raw 403 on forbidden response", async () => {
      vi.mocked(fetchAdminAdapters).mockRejectedValueOnce(
        new Error("403: Admin access restricted to localhost")
      );

      render(
        <LocalhostGate>
          <div data-testid="child-content">Secret admin content</div>
        </LocalhostGate>
      );

      await waitFor(() => {
        expect(
          screen.getByText(/only accessible from localhost/i)
        ).toBeInTheDocument();
      });

      // Children should NOT render
      expect(screen.queryByTestId("child-content")).not.toBeInTheDocument();

      // Raw error details should NOT appear
      expect(screen.queryByText(/403/)).not.toBeInTheDocument();
      expect(screen.queryByText(/forbidden/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
    });

    it("shows advisory message, not raw error on network failure", async () => {
      vi.mocked(fetchAdminAdapters).mockRejectedValueOnce(
        new Error("Failed to fetch")
      );

      render(
        <LocalhostGate>
          <div data-testid="child-content">Admin panel</div>
        </LocalhostGate>
      );

      await waitFor(() => {
        expect(
          screen.getByText(/only accessible from localhost/i)
        ).toBeInTheDocument();
      });

      expect(screen.queryByTestId("child-content")).not.toBeInTheDocument();

      // No stack trace or raw error text
      expect(screen.queryByText(/failed to fetch/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
    });

    it("renders children on successful fetch", async () => {
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([]);

      render(
        <LocalhostGate>
          <div data-testid="child-content">Admin panel</div>
        </LocalhostGate>
      );

      await waitFor(() => {
        expect(screen.getByTestId("child-content")).toBeInTheDocument();
      });

      // Advisory should NOT appear
      expect(
        screen.queryByText(/only accessible from localhost/i)
      ).not.toBeInTheDocument();
    });
  });

  describe("Secret data not leaked into DOM", () => {
    it("mock response with secret-like values does not appear in DOM", async () => {
      const adapterWithSecret: AdminAdapter = {
        id: "openai",
        name: "OpenAI",
        status: "connected",
        enabled: true,
        last_sync: "2026-01-01T00:00:00Z",
        success_count: 50,
        fail_count: 0,
        secret_source: "OPENAI_API_KEY",
        secret_set: true,
      };

      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([
        adapterWithSecret,
      ]);

      render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("OpenAI")).toBeInTheDocument();
      });

      // These secret value patterns must never appear in the DOM
      const forbiddenPatterns = [
        "sk-",
        "sk-proj-",
        "xoxb-",
        "xoxp-",
        "ghp_",
        "AIza",
        "AKIA",
        "-----BEGIN",
      ];
      for (const pattern of forbiddenPatterns) {
        expect(
          screen.queryByText(new RegExp(pattern))
        ).not.toBeInTheDocument();
      }

      // Source names (env var names) ARE expected to be visible
      expect(screen.getByText("OPENAI_API_KEY")).toBeInTheDocument();
    });
  });
});

// Helpers
function makeAdapter(id: string, enabled: boolean): AdminAdapter {
  return {
    id,
    name: id.charAt(0).toUpperCase() + id.slice(1),
    status: "connected",
    enabled,
    last_sync: "2026-01-01T00:00:00Z",
    success_count: 100,
    fail_count: 2,
    secret_source: `${id.toUpperCase()}_API_KEY`,
    secret_set: true,
  };
}

function makeAuditEntry(): AuditEntry {
  return {
    id: "q1",
    timestamp: "2026-01-01T00:00:00Z",
    latency_ms: 150,
    tokens: 200,
    sources_count: 3,
    error: null,
  };
}
