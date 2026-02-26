import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { Dashboard } from "../../src/pages/Dashboard";

describe("Dashboard", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    global.fetch = vi.fn();
  });

  it("prompts for API key when not set", () => {
    render(<Dashboard />);
    expect(screen.getByPlaceholderText(/paste.*key/i)).toBeInTheDocument();
  });

  it("shows storage warning near key input", () => {
    render(<Dashboard />);
    expect(screen.getByText(/do not use on shared/i)).toBeInTheDocument();
  });

  it("shows validate form when key is set", () => {
    localStorage.setItem("evidra_api_key", "ev1_testkey12345678901234567890");
    // Mock pubkey fetch
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ pem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----" }),
    });

    render(<Dashboard />);
    expect(screen.getByText(/try validate/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /evaluate/i })).toBeInTheDocument();
  });

  it("shows simple/advanced tabs", () => {
    localStorage.setItem("evidra_api_key", "ev1_testkey12345678901234567890");
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ pem: "test" }),
    });

    render(<Dashboard />);
    expect(screen.getByRole("tab", { name: /simple/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /advanced/i })).toBeInTheDocument();
  });

  it("shows validate result on success", async () => {
    localStorage.setItem("evidra_api_key", "ev1_testkey12345678901234567890");
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          ok: true,
          decision: { allow: true, risk_level: "low", reason: "all checks passed" },
          evidence_record: {
            event_id: "evt_01JTEST",
            timestamp: "2026-02-26T14:23:01Z",
            server_id: "srv_test",
            policy_ref: "bundle://evidra/default:0.1.0",
            actor: { type: "agent", id: "claude" },
            tool: "kubectl",
            operation: "apply",
            environment: "production",
            input_hash: "sha256:abc",
            decision: { allow: true, risk_level: "low", reason: "all checks passed" },
            signing_payload: "test",
            signature: "sig",
          },
        }),
    });

    render(<Dashboard />);
    await userEvent.click(screen.getByRole("button", { name: /evaluate/i }));

    await waitFor(() => {
      expect(screen.getAllByText(/allow/i).length).toBeGreaterThan(0);
    });
  });

  it("shows error with change key action on 401", async () => {
    localStorage.setItem("evidra_api_key", "ev1_testkey12345678901234567890");
    // First call is pubkey fetch (success), second is validate (401)
    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ pem: "test" }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: "Unauthorized",
        json: () =>
          Promise.resolve({
            ok: false,
            error: { code: "unauthorized", message: "Invalid API key" },
          }),
      });

    render(<Dashboard />);
    await userEvent.click(screen.getByRole("button", { name: /evaluate/i }));

    await waitFor(() => {
      expect(screen.getByText(/invalid api key/i)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /change key/i })).toBeInTheDocument();
    });
  });
});
