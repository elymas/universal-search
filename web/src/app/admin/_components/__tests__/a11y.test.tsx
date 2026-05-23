import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AdapterStatusPanel } from "@/app/admin/_components/adapter-status-panel";
import { AuditViewer } from "@/app/admin/_components/audit-viewer";
import { SidebarNav } from "@/components/sidebar-nav";
import type { AdminAdapter, AuditEntry } from "@/lib/api";

// Mock next/navigation for sidebar
const mockPathname = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname(),
}));

// Mock API
vi.mock("@/lib/api", () => ({
  fetchAdminAdapters: vi.fn(),
  fetchAdminAudit: vi.fn(),
  toggleAdapter: vi.fn(),
  resyncAdapter: vi.fn(),
}));

import { fetchAdminAdapters, fetchAdminAudit, toggleAdapter } from "@/lib/api";

const MOCK_ADAPTER: AdminAdapter = {
  id: "google",
  name: "Google",
  status: "connected",
  enabled: true,
  last_sync: "2026-01-01T00:00:00Z",
  success_count: 100,
  fail_count: 2,
  secret_source: "GOOGLE_API_KEY",
  secret_set: true,
};

const MOCK_AUDIT_ENTRY: AuditEntry = {
  id: "q1",
  timestamp: "2026-01-01T00:00:00Z",
  latency_ms: 150,
  tokens: 200,
  sources_count: 3,
  error: null,
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("Accessibility", () => {
  describe("Sidebar nav — aria-current", () => {
    it("marks Admin link with aria-current=page when pathname=/admin", () => {
      mockPathname.mockReturnValue("/admin");
      render(<SidebarNav />);

      const adminLink = screen
        .getAllByRole("link")
        .find((link) => link.textContent?.includes("Admin"));
      expect(adminLink).toHaveAttribute("aria-current", "page");
    });

    it("does not set aria-current on non-admin pages", () => {
      mockPathname.mockReturnValue("/");
      render(<SidebarNav />);

      const adminLink = screen
        .getAllByRole("link")
        .find((link) => link.textContent?.includes("Admin"));
      expect(adminLink).not.toHaveAttribute("aria-current", "page");
    });
  });

  describe("Toggle buttons — aria-pressed", () => {
    it("toggle button has aria-pressed=true when adapter is enabled", async () => {
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([
        { ...MOCK_ADAPTER, enabled: true },
      ]);

      render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("Google")).toBeInTheDocument();
      });

      const toggle = screen.getByLabelText(/toggle google/i);
      expect(toggle).toHaveAttribute("aria-pressed", "true");
    });

    it("toggle button has aria-pressed=false when adapter is disabled", async () => {
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([
        { ...MOCK_ADAPTER, enabled: false },
      ]);

      render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("Google")).toBeInTheDocument();
      });

      const toggle = screen.getByLabelText(/toggle google/i);
      expect(toggle).toHaveAttribute("aria-pressed", "false");
    });

    it("toggle updates aria-pressed after click", async () => {
      const user = userEvent.setup();
      const adapter = { ...MOCK_ADAPTER, enabled: true };
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([adapter]);
      vi.mocked(toggleAdapter).mockResolvedValueOnce({
        ...adapter,
        enabled: false,
      });

      render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("Google")).toBeInTheDocument();
      });

      const toggle = screen.getByLabelText(/toggle google/i);
      expect(toggle).toHaveAttribute("aria-pressed", "true");

      await user.click(toggle);

      await waitFor(() => {
        expect(toggle).toHaveAttribute("aria-pressed", "false");
      });
    });
  });

  describe("Audit table — th scope", () => {
    it("all table headers have scope=col", async () => {
      vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
        entries: [MOCK_AUDIT_ENTRY],
        total: 1,
        has_more: false,
      });

      render(<AuditViewer />);

      await waitFor(() => {
        expect(screen.getByText("q1")).toBeInTheDocument();
      });

      const headers = screen.getAllByRole("columnheader");
      expect(headers.length).toBeGreaterThan(0);
      for (const th of headers) {
        expect(th).toHaveAttribute("scope", "col");
      }
    });
  });

  describe("Re-sync button — accessible name", () => {
    it("re-sync button has descriptive aria-label", async () => {
      vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([MOCK_ADAPTER]);

      render(<AdapterStatusPanel />);

      await waitFor(() => {
        expect(screen.getByText("Google")).toBeInTheDocument();
      });

      const resyncBtn = screen.getByLabelText(/resync google/i);
      expect(resyncBtn).toBeInTheDocument();
      expect(resyncBtn.tagName).toBe("BUTTON");
    });
  });
});
