import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
    expect(screen.getByText(/evidra-mcp/i)).toBeInTheDocument();
  });

  it("shows GitHub Actions integration example", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    expect(screen.getByText(/github actions/i)).toBeInTheDocument();
  });

  it("Get started button calls onGetStarted", async () => {
    const onGetStarted = vi.fn();
    render(<Landing onGetStarted={onGetStarted} />);
    const links = screen.getAllByRole("link", { name: /get started/i });
    await userEvent.click(links[0]);
    expect(onGetStarted).toHaveBeenCalledOnce();
  });
});
