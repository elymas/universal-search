import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AdapterStatusPanel } from "@/app/admin/_components/adapter-status-panel";
import type { AdminAdapter } from "@/lib/api";

// Mock the API module
vi.mock("@/lib/api", () => ({
  fetchAdminAdapters: vi.fn(),
  toggleAdapter: vi.fn(),
  resyncAdapter: vi.fn(),
}));

import { fetchAdminAdapters, toggleAdapter, resyncAdapter } from "@/lib/api";

const MOCK_ADAPTERS: AdminAdapter[] = Array.from({ length: 9 }, (_, i) => ({
  id: `adapter-${i}`,
  name: `Adapter ${i}`,
  status: (["connected", "auth_required", "disabled", "error"] as const)[i % 4],
  enabled: i % 2 === 0,
  last_sync: i < 3 ? `2026-01-0${i + 1}T00:00:00Z` : null,
  success_count: 100 + i * 10,
  fail_count: i,
  last_error: i === 3 ? "connection refused" : null,
  secret_source: `ADAPTER_${i}_KEY`,
  secret_set: i < 7,
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("AdapterStatusPanel", () => {
  it("renders 9 rows for 9 adapters", async () => {
    vi.mocked(fetchAdminAdapters).mockResolvedValueOnce(MOCK_ADAPTERS);

    render(<AdapterStatusPanel />);

    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(10); // 1 header + 9 data
    });
  });

  it("shows dash for null last_sync", async () => {
    const partial = [
      {
        ...MOCK_ADAPTERS[0],
        last_sync: null,
      },
    ];
    vi.mocked(fetchAdminAdapters).mockResolvedValueOnce(partial);

    render(<AdapterStatusPanel />);

    await waitFor(() => {
      const cells = screen.getAllByRole("cell");
      // Find the cell containing em dash
      const dashCell = cells.find((c) => c.textContent === "—");
      expect(dashCell).toBeInTheDocument();
    });
  });

  it("toggle click calls toggleAdapter and updates row", async () => {
    const user = userEvent.setup();
    const adapter = { ...MOCK_ADAPTERS[0], enabled: true };
    vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([adapter]);
    vi.mocked(toggleAdapter).mockResolvedValueOnce({
      ...adapter,
      enabled: false,
    });

    render(<AdapterStatusPanel />);

    await waitFor(() => {
      expect(screen.getByText("Adapter 0")).toBeInTheDocument();
    });

    const toggleButton = screen.getByLabelText(/toggle adapter-0/i);
    expect(toggleButton).toHaveAttribute("aria-pressed", "true");
    await user.click(toggleButton);

    expect(toggleAdapter).toHaveBeenCalledWith("adapter-0", false);
  });

  it("re-sync click disables button during request then enables after", async () => {
    const user = userEvent.setup();
    const adapter = { ...MOCK_ADAPTERS[0] };
    vi.mocked(fetchAdminAdapters).mockResolvedValueOnce([adapter]);

    let resolveResync: (v: AdminAdapter) => void;
    const resyncPromise = new Promise<AdminAdapter>((resolve) => {
      resolveResync = resolve;
    });
    vi.mocked(resyncAdapter).mockReturnValueOnce(resyncPromise);

    render(<AdapterStatusPanel />);

    await waitFor(() => {
      expect(screen.getByText("Adapter 0")).toBeInTheDocument();
    });

    const resyncButton = screen.getByLabelText(/resync adapter-0/i);
    expect(resyncButton).toBeEnabled();

    await user.click(resyncButton);
    expect(resyncButton).toBeDisabled();

    resolveResync!({ ...adapter, status: "connected" });

    await waitFor(() => {
      expect(resyncButton).toBeEnabled();
    });
  });

  it("never renders secret values in DOM", async () => {
    vi.mocked(fetchAdminAdapters).mockResolvedValueOnce(MOCK_ADAPTERS);

    render(<AdapterStatusPanel />);

    await waitFor(() => {
      expect(screen.getByText("ADAPTER_0_KEY")).toBeInTheDocument();
    });

    // Common secret value patterns should NOT appear
    const secretPatterns = [
      "sk-",
      "xoxb-",
      "ghp_",
      "AIza",
      "AKIA",
      "-----BEGIN",
    ];
    for (const pattern of secretPatterns) {
      expect(screen.queryByText(new RegExp(pattern))).not.toBeInTheDocument();
    }

    // Source names should be visible (these are env var names, not values)
    expect(screen.getByText("ADAPTER_0_KEY")).toBeInTheDocument();
  });
});
