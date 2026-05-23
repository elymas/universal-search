import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SidebarNav } from "@/components/sidebar-nav";

// Mock next/navigation
const mockPathname = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname(),
}));

describe("SidebarNav", () => {
  it("renders 4 nav items with Admin as last", () => {
    mockPathname.mockReturnValue("/");
    render(<SidebarNav />);

    const links = screen.getAllByRole("link");
    // 4 nav links in the sidebar
    const navLinks = links.filter(
      (link) => link.closest("nav") !== null
    );
    expect(navLinks).toHaveLength(4);
    expect(navLinks[3]).toHaveTextContent("Admin");
  });

  it('marks Admin as active when pathname="/admin"', () => {
    mockPathname.mockReturnValue("/admin");
    render(<SidebarNav />);

    const adminLink = screen
      .getAllByRole("link")
      .find((link) => link.textContent?.includes("Admin"));
    expect(adminLink).toHaveAttribute("aria-current", "page");
  });

  it("other 3 items remain unchanged when admin is active", () => {
    mockPathname.mockReturnValue("/admin");
    render(<SidebarNav />);

    const navLinks = screen
      .getAllByRole("link")
      .filter((link) => link.closest("nav") !== null);

    expect(navLinks[0]).toHaveTextContent("Search");
    expect(navLinks[0]).not.toHaveAttribute("aria-current", "page");
    expect(navLinks[1]).toHaveTextContent("History");
    expect(navLinks[1]).not.toHaveAttribute("aria-current", "page");
    expect(navLinks[2]).toHaveTextContent("Sources");
    expect(navLinks[2]).not.toHaveAttribute("aria-current", "page");
  });

  it("mobile click on Admin closes overlay", async () => {
    const user = userEvent.setup();
    mockPathname.mockReturnValue("/");
    render(<SidebarNav />);

    // Open mobile menu
    const menuButton = screen.getByRole("button", {
      name: /open menu/i,
    });
    await user.click(menuButton);

    // Click Admin link
    const adminLink = screen
      .getAllByRole("link")
      .find((link) => link.textContent?.includes("Admin"));
    await user.click(adminLink!);

    // Overlay should be gone (close button should be back to open)
    expect(
      screen.queryByRole("button", { name: /close menu/i })
    ).not.toBeInTheDocument();
  });
});
