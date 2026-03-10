import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Docs } from "../../src/pages/Docs";

describe("Docs", () => {
  it("shows stop_after_deny in rule table", () => {
    render(<Docs />);
    const matches = screen.getAllByText("stop_after_deny");
    expect(matches.length).toBeGreaterThanOrEqual(1);
  });

  it("shows deny-cache CLI flag in usage section", () => {
    render(<Docs />);
    expect(screen.getByText(/evidra-lock-mcp --deny-cache/)).toBeInTheDocument();
  });

  it("shows deny-loop prevention troubleshooting section", () => {
    render(<Docs />);
    expect(
      screen.getByText(/stop_after_deny \(deny-loop prevention\)/)
    ).toBeInTheDocument();
  });

  it("shows EVIDRA_DENY_CACHE env var in troubleshooting", () => {
    render(<Docs />);
    expect(screen.getByText("EVIDRA_DENY_CACHE=true")).toBeInTheDocument();
  });
});
