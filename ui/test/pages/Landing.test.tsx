import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Landing } from "../../src/pages/Landing";

describe("Landing (product page)", () => {
  it("renders hero with product description", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.getAllByText(/policy/i).length).toBeGreaterThan(0);
  });

  it("shows MCP integration example", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    expect(screen.getAllByText(/evidra-lock-mcp/i).length).toBeGreaterThan(0);
  });

  it("shows GitHub Actions integration example", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    expect(screen.getByText(/github-actions|github actions/i)).toBeInTheDocument();
  });

  it("shows primary CTA button", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    const cta = screen.getByRole("button", { name: /try it now/i });
    expect(cta).toBeInTheDocument();
  });
});
