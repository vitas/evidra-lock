import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, beforeEach } from "vitest";
import { useApiKey } from "../../src/hooks/useApiKey";

describe("useApiKey", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it("returns null when no key stored", () => {
    const { result } = renderHook(() => useApiKey());
    expect(result.current.apiKey).toBeNull();
  });

  it("persists key to localStorage by default", () => {
    const { result } = renderHook(() => useApiKey());
    act(() => result.current.setApiKey("ev1_test"));
    expect(localStorage.getItem("evidra_api_key")).toBe("ev1_test");
    expect(result.current.apiKey).toBe("ev1_test");
  });

  it("uses sessionStorage when ephemeral mode enabled", () => {
    const { result } = renderHook(() => useApiKey());
    act(() => result.current.setEphemeral(true));
    act(() => result.current.setApiKey("ev1_session"));
    expect(sessionStorage.getItem("evidra_api_key")).toBe("ev1_session");
    expect(localStorage.getItem("evidra_api_key")).toBeNull();
  });

  it("clears key from both storages", () => {
    localStorage.setItem("evidra_api_key", "ev1_old");
    sessionStorage.setItem("evidra_api_key", "ev1_old");
    const { result } = renderHook(() => useApiKey());
    act(() => result.current.clearApiKey());
    expect(result.current.apiKey).toBeNull();
    expect(localStorage.getItem("evidra_api_key")).toBeNull();
    expect(sessionStorage.getItem("evidra_api_key")).toBeNull();
  });
});
