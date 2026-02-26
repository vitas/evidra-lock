export interface Actor {
  type: string;
  id: string;
  origin?: string;
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
  reasons?: string[];
  hints?: string[];
  rule_ids?: string[];
}

export interface EvidenceRecord {
  event_id: string;
  timestamp: string;
  server_id: string;
  policy_ref: string;
  actor: Actor;
  tool: string;
  operation: string;
  environment: string;
  input_hash: string;
  decision: PolicyDecision;
  signing_payload: string;
  signature: string;
}

export interface ValidateResponse {
  ok: boolean;
  decision: PolicyDecision;
  evidence_record: EvidenceRecord;
}

export interface KeyResponse {
  key: string;
  prefix: string;
  tenant_id: string;
}

export interface ErrorResponse {
  ok: false;
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
}
