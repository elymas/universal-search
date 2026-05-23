import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuditViewer } from "@/app/admin/_components/audit-viewer";
import type { AuditEntry, AuditResponse } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  fetchAdminAudit: vi.fn(),
}));

import { fetchAdminAudit } from "@/lib/api";

function makeEntries(count: number, opts?: { allErrors?: boolean }): AuditEntry[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `q${i}`,
    timestamp: `2026-01-${String(i % 28 + 1).padStart(2, "0")}T12:00:00Z`,
    latency_ms: 100 + i,
    tokens: 50 + i,
    sources_count: 3,
    config_snapshot: i === 0 ? '{"model":"gpt-4"}' : null,
    error: opts?.allErrors
      ? "timeout"
      : i % 5 === 0
        ? "rate limit exceeded"
        : null,
  }));
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("AuditViewer", () => {
  it("shows empty state with disabled pagination when no entries", async () => {
    vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
      entries: [],
      total: 0,
      has_more: false,
    });

    render(<AuditViewer />);

    await waitFor(() => {
      expect(screen.getByText("No queries yet")).toBeInTheDocument();
    });

    expect(screen.getByLabelText("Previous page")).toBeDisabled();
    expect(screen.getByLabelText("Next page")).toBeDisabled();
  });

  it("renders 50 entries in the table", async () => {
    const entries = makeEntries(50);
    vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
      entries,
      total: 100,
      has_more: true,
    });

    render(<AuditViewer />);

    await waitFor(() => {
      const rows = screen.getAllByRole("row");
      // 1 header + 50 data rows
      expect(rows).toHaveLength(51);
    });
  });

  it('filters to error rows when "Errors only" is ON', async () => {
    const entries = makeEntries(10);
    vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
      entries,
      total: 10,
      has_more: false,
    });

    // Second call for errors_only
    vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
      entries: entries.filter((e) => e.error !== null),
      total: 2,
      has_more: false,
    });

    render(<AuditViewer />);

    await waitFor(() => {
      expect(screen.getByText("q0")).toBeInTheDocument();
    });

    const errorToggle = screen.getByLabelText("Errors only");
    await userEvent.click(errorToggle);

    await waitFor(() => {
      expect(fetchAdminAudit).toHaveBeenLastCalledWith(
        expect.objectContaining({ errors_only: true })
      );
    });
  });

  it("next page increases offset and shows new entries", async () => {
    const page1 = makeEntries(20);
    const page2 = makeEntries(20).map((e) => ({
      ...e,
      id: `q_page2_${e.id}`,
    }));

    vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
      entries: page1,
      total: 40,
      has_more: true,
    });
    vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
      entries: page2,
      total: 40,
      has_more: false,
    });

    render(<AuditViewer />);

    await waitFor(() => {
      expect(screen.getByText("q0")).toBeInTheDocument();
    });

    const nextButton = screen.getByLabelText("Next page");
    await userEvent.click(nextButton);

    await waitFor(() => {
      expect(fetchAdminAudit).toHaveBeenLastCalledWith(
        expect.objectContaining({ offset: 20 })
      );
    });
  });

  it("table has proper th scope for accessibility", async () => {
    vi.mocked(fetchAdminAudit).mockResolvedValueOnce({
      entries: makeEntries(1),
      total: 1,
      has_more: false,
    });

    render(<AuditViewer />);

    await waitFor(() => {
      const thElements = screen.getAllByRole("columnheader");
      for (const th of thElements) {
        expect(th).toHaveAttribute("scope", "col");
      }
    });
  });
});
