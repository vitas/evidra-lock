import type { KeyResponse, ToolInvocation, ValidateResponse } from "../types/api";

const BASE_URL = import.meta.env.VITE_API_URL || "";
const MOCK_API_REQUESTED = import.meta.env.VITE_MOCK_API === "1";
// Safety guardrail: mock API is development-only even if VITE_MOCK_API is set in other modes.
const MOCK_API_ENABLED = import.meta.env.MODE === "development" && MOCK_API_REQUESTED;

const MOCK_PUBLIC_KEY_PEM = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA7mock7mock7mock7mock7mock7mock7mock7mock7m0=
-----END PUBLIC KEY-----`;

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

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function parseInvocation(body: BodyInit | null | undefined): ToolInvocation {
  if (typeof body !== "string") {
    throw new ApiError(400, "invalid_request", "mock validate expects a JSON body");
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch {
    throw new ApiError(400, "invalid_json", "invalid JSON request body");
  }

  if (!isRecord(parsed)) {
    throw new ApiError(400, "invalid_request", "request body must be an object");
  }

  const actor = isRecord(parsed.actor) ? parsed.actor : {};
  const invocation: ToolInvocation = {
    actor: {
      type: typeof actor.type === "string" ? actor.type : "agent",
      id: typeof actor.id === "string" ? actor.id : "mock-agent",
      origin: typeof actor.origin === "string" ? actor.origin : "web-ui",
    },
    tool: typeof parsed.tool === "string" ? parsed.tool : "unknown",
    operation: typeof parsed.operation === "string" ? parsed.operation : "unknown",
    params: isRecord(parsed.params) ? parsed.params : {},
    context: isRecord(parsed.context) ? parsed.context : undefined,
    environment: typeof parsed.environment === "string" ? parsed.environment : "development",
  };
  return invocation;
}

function readNamespace(invocation: ToolInvocation): string {
  const action = isRecord(invocation.params.action) ? invocation.params.action : undefined;
  const target = action && isRecord(action.target) ? action.target : undefined;
  return typeof target?.namespace === "string" ? target.namespace : "";
}

function buildMockDecision(invocation: ToolInvocation): ValidateResponse["decision"] {
  const ns = readNamespace(invocation);

  if (
    invocation.tool === "kubectl" &&
    invocation.operation === "delete" &&
    (ns === "kube-system" || ns === "kube-public")
  ) {
    return {
      allow: false,
      risk_level: "high",
      reason: "denied by k8s.protected_namespace",
      reasons: [`k8s.protected_namespace: ${ns} is protected`],
      hints: ["Use a non-system namespace or add breakglass risk tag"],
      rule_ids: ["k8s.protected_namespace"],
    };
  }

  if (
    invocation.tool === "terraform" &&
    invocation.operation === "destroy" &&
    invocation.environment === "production"
  ) {
    return {
      allow: false,
      risk_level: "high",
      reason: "denied by ops.mass_delete",
      reasons: ["ops.mass_delete: terraform destroy blocked in production mock profile"],
      hints: ["Run in non-production environment or require explicit breakglass"],
      rule_ids: ["ops.mass_delete"],
    };
  }

  return {
    allow: true,
    risk_level: "low",
    reason: "allowed by mock policy",
    reasons: [],
    hints: [],
    rule_ids: [],
  };
}

async function mockRequest<T>(
  path: string,
  options: RequestInit = {},
  apiKey?: string,
): Promise<T> {
  const method = (options.method || "GET").toUpperCase();
  await sleep(120);

  if (path === "/v1/keys" && method === "POST") {
    const key = `ev1_mock_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`;
    const response: KeyResponse = {
      key,
      prefix: key.slice(0, 12),
      tenant_id: "tenant_mock_dev",
    };
    return response as T;
  }

  if (path === "/v1/evidence/pubkey" && method === "GET") {
    return { pem: MOCK_PUBLIC_KEY_PEM } as T;
  }

  if (path === "/v1/validate" && method === "POST") {
    if (!apiKey) {
      throw new ApiError(401, "unauthorized", "Missing API key");
    }

    const invocation = parseInvocation(options.body);
    const decision = buildMockDecision(invocation);
    const timestamp = new Date().toISOString();
    const eventID = `evt_mock_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}`;
    const signingPayload = [
      "evidra.v1",
      `event_id=${eventID}`,
      `timestamp=${timestamp}`,
      `tool=${invocation.tool}`,
      `operation=${invocation.operation}`,
      `allow=${String(decision.allow)}`,
      `risk_level=${decision.risk_level}`,
    ].join("\n");

    const response: ValidateResponse = {
      event_id: eventID,
      timestamp,
      tenant_id: "tenant_mock_dev",
      server_id: "evidra-api-mock",
      policy_ref: "bundle://evidra/mock:dev",
      actor: invocation.actor,
      tool: invocation.tool,
      operation: invocation.operation,
      environment: invocation.environment || "development",
      input_hash: `sha256:mock-${Math.random().toString(16).slice(2, 18)}`,
      decision,
      signing_payload: signingPayload,
      signature: "mock-signature-dev-only",
    };
    return response as T;
  }

  throw new ApiError(404, "mock_route_not_found", `Mock route not found: ${method} ${path}`);
}

async function request<T>(
  path: string,
  options: RequestInit = {},
  apiKey?: string,
): Promise<T> {
  if (MOCK_API_ENABLED) {
    return mockRequest<T>(path, options, apiKey);
  }

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
