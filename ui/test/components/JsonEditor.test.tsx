import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { JsonEditor } from "../../src/components/JsonEditor";

describe("JsonEditor", () => {
  it("renders textarea with initial value", () => {
    render(<JsonEditor value='{"tool":"kubectl"}' onChange={vi.fn()} />);
    expect(screen.getByRole("textbox")).toHaveValue('{"tool":"kubectl"}');
  });

  it("shows valid indicator for valid JSON on blur", () => {
    render(<JsonEditor value='{"tool":"kubectl"}' onChange={vi.fn()} />);
    fireEvent.blur(screen.getByRole("textbox"));
    expect(screen.getByText(/valid/i)).toBeInTheDocument();
  });

  it("shows error indicator for invalid JSON on blur", () => {
    const onChange = vi.fn();
    render(<JsonEditor value="{broken" onChange={onChange} />);
    fireEvent.blur(screen.getByRole("textbox"));
    expect(screen.getByText(/invalid/i)).toBeInTheDocument();
  });

  it("calls onChange with parsed object for valid JSON", () => {
    const onChange = vi.fn();
    render(<JsonEditor value="{}" onChange={onChange} />);
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: '{"tool":"helm"}' } });
    fireEvent.blur(textarea);
    expect(onChange).toHaveBeenCalledWith({ tool: "helm" }, true);
  });

  it("calls onChange with null for invalid JSON", () => {
    const onChange = vi.fn();
    render(<JsonEditor value="{}" onChange={onChange} />);
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "{bad" } });
    fireEvent.blur(textarea);
    expect(onChange).toHaveBeenCalledWith(null, false);
  });
});
