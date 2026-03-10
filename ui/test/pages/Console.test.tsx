import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { Console } from "../../src/pages/Console";

describe("Console", () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  it("shows track selector", () => {
    render(<Console onKeyCreated={vi.fn()} />);
    expect(screen.getByText(/ai agent \(mcp\)/i)).toBeInTheDocument();
    expect(screen.getByText(/api \/ ci/i)).toBeInTheDocument();
  });

  it("shows MCP setup by default", () => {
    render(<Console onKeyCreated={vi.fn()} />);
    expect(screen.getByText(/mcp setup/i)).toBeInTheDocument();
  });

  it("shows key after successful creation in API track", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          key: "ev1_testkey123456789012345678901234567890",
          prefix: "ev1_testkey1",
          tenant_id: "01J...",
        }),
    });

    render(<Console onKeyCreated={vi.fn()} />);
    // Switch to API track
    await userEvent.click(screen.getByText(/api \/ ci/i));
    await userEvent.click(screen.getByRole("button", { name: /get key/i }));

    await waitFor(() => {
      expect(screen.getAllByText(/ev1_testkey/).length).toBeGreaterThan(0);
    });
  });

  it("shows error on rate limit in API track", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 429,
      statusText: "Too Many Requests",
      json: () =>
        Promise.resolve({
          error: { code: "rate_limited", message: "Too many requests" },
        }),
    });

    render(<Console onKeyCreated={vi.fn()} />);
    await userEvent.click(screen.getByText(/api \/ ci/i));
    await userEvent.click(screen.getByRole("button", { name: /get key/i }));

    await waitFor(() => {
      expect(screen.getByText(/too many/i)).toBeInTheDocument();
    });
  });

  it("shows EVIDRA_DENY_CACHE in env legend for local setups", async () => {
    render(<Console onKeyCreated={vi.fn()} />);
    // Switch to Local + Self-hosted API path
    await userEvent.click(screen.getByText(/local \+ self-hosted api/i));
    expect(screen.getByText("EVIDRA_DENY_CACHE")).toBeInTheDocument();
  });

  it("shows EVIDRA_DENY_CACHE in env legend for offline setup", async () => {
    render(<Console onKeyCreated={vi.fn()} />);
    await userEvent.click(screen.getByText(/fully offline/i));
    expect(screen.getByText("EVIDRA_DENY_CACHE")).toBeInTheDocument();
  });

  it("shows EVIDRA_DENY_CACHE in env legend for default (offline) setup", () => {
    render(<Console onKeyCreated={vi.fn()} />);
    // Offline is default — DENY_CACHE should be visible
    expect(screen.getByText("EVIDRA_DENY_CACHE")).toBeInTheDocument();
  });

  it("does not store key anywhere after showing", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          key: "ev1_secret",
          prefix: "ev1_secr",
          tenant_id: "t1",
        }),
    });

    render(<Console onKeyCreated={vi.fn()} />);
    await userEvent.click(screen.getByText(/api \/ ci/i));
    await userEvent.click(screen.getByRole("button", { name: /get key/i }));

    expect(localStorage.getItem("evidra_api_key")).toBeNull();
  });
});
