import type { KeyResponse, ToolInvocation, ValidateResponse } from "../types/api";

const BASE_URL = import.meta.env.VITE_API_URL || "";

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public details?: Record<string, unknown>,
  ) {
    super(message);
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {},
  apiKey?: string,
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) || {}),
  };

  if (apiKey) {
    headers["Authorization"] = `Bearer ${apiKey}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  let data: Record<string, unknown>;
  try {
    data = await res.json();
  } catch {
    // Server returned non-JSON (e.g. HTML error page or network proxy error).
    throw new ApiError(res.status, "parse_error", res.statusText || "unexpected response from server");
  }

  if (!res.ok) {
    // Server returns either {"error": "msg"} (flat) or {"error": {"code":"...","message":"..."}} (object).
    const errField = data.error;
    const msg =
      typeof errField === "string"
        ? errField
        : (errField as Record<string, string>)?.message || res.statusText;
    const code =
      typeof errField === "object" && errField !== null
        ? ((errField as Record<string, string>).code || "unknown")
        : "unknown";
    throw new ApiError(res.status, code, msg, (errField as Record<string, unknown>)?.details as Record<string, unknown>);
  }

  return data as T;
}

export function createKey(label?: string) {
  return request<KeyResponse>("/v1/keys", {
    method: "POST",
    body: JSON.stringify({ label: label || undefined }),
  });
}

export function validate(invocation: ToolInvocation, apiKey: string) {
  return request<ValidateResponse>(
    "/v1/validate",
    { method: "POST", body: JSON.stringify(invocation) },
    apiKey,
  );
}

export function getPublicKey() {
  return request<{ pem: string }>("/v1/evidence/pubkey");
}
