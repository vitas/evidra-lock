import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { InlineError } from "../../src/components/InlineError";

describe("InlineError", () => {
  it("shows error message", () => {
    render(<InlineError message="Invalid API key" />);
    expect(screen.getByText(/invalid api key/i)).toBeInTheDocument();
  });

  it("shows retry button when onRetry provided", () => {
    render(<InlineError message="Server error" onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("hides retry button when not provided", () => {
    render(<InlineError message="Bad input" />);
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });

  it("calls onRetry on click", async () => {
    const onRetry = vi.fn();
    render(<InlineError message="Error" onRetry={onRetry} />);
    await userEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("shows custom action button", async () => {
    const onAction = vi.fn();
    render(<InlineError message="401" action={{ label: "Change key", onClick: onAction }} />);
    await userEvent.click(screen.getByRole("button", { name: /change key/i }));
    expect(onAction).toHaveBeenCalledOnce();
  });
});
