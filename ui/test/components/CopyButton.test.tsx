import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { CopyButton } from "../../src/components/CopyButton";

describe("CopyButton", () => {
  it("copies text to clipboard on click", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });

    render(<CopyButton text="ev1_test_key" />);
    await userEvent.click(screen.getByRole("button"));

    expect(writeText).toHaveBeenCalledWith("ev1_test_key");
  });

  it("shows feedback after copy", async () => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });

    render(<CopyButton text="test" />);
    await userEvent.click(screen.getByRole("button"));

    expect(screen.getByText(/copied/i)).toBeInTheDocument();
  });
});
