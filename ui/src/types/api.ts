export interface Actor {
  type: string;
  id: string;
  origin: string;
}

export interface ActionPayload {
  kind: string;
  target?: Record<string, unknown>;
  payload?: Record<string, unknown>;
  risk_tags?: string[];
}

export interface ToolInvocation {
  actor: Actor;
  tool: string;
  operation: string;
  params: Record<string, unknown>;
  context?: Record<string, unknown>;
  environment?: string;
}

export interface PolicyDecision {
  allow: boolean;
  risk_level: "low" | "medium" | "high";
  reason: string;
  reasons: string[];
  hints: string[];
  rule_ids: string[];
}

export interface ActionResult {
  index: number;
  kind: string;
  pass: boolean;
  risk_level: "low" | "medium" | "high";
  rule_ids: string[];
  reasons: string[];
  hints: string[];
}

// ValidateResponse is a flat EvidenceRecord — the API returns the record directly,
// not wrapped in {ok, evidence_record}.
export interface ValidateResponse {
  event_id: string;
  timestamp: string;
  tenant_id: string;
  server_id: string;
  policy_ref: string;
  actor: Actor;
  tool: string;
  operation: string;
  environment: string;
  input_hash: string;
  decision: PolicyDecision;
  action_results?: ActionResult[];
  signing_payload: string;
  signature: string;
}

export interface KeyResponse {
  key: string;
  prefix: string;
  tenant_id: string;
}

export interface ErrorResponse {
  error: string | {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
}
