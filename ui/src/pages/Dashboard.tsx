import { useState, useEffect, useCallback } from "react";
import { validate, getPublicKey, ApiError } from "../api/client";
import type { ToolInvocation, ValidateResponse } from "../types/api";
import { useApiKey } from "../hooks/useApiKey";
import { KeyPrompt } from "../components/KeyPrompt";
import { Badge } from "../components/Badge";
import { CodeBlock } from "../components/CodeBlock";
import { InlineError } from "../components/InlineError";
import { JsonEditor } from "../components/JsonEditor";
import { KeyValueEditor } from "../components/KeyValueEditor";
import "../styles/dashboard.css";

type Mode = "simple" | "advanced";

interface RecentEval {
  time: string;
  tool: string;
  operation: string;
  allow: boolean;
}

const TOOL_OPTIONS = ["kubectl", "terraform", "helm"];

const OP_OPTIONS: Record<string, string[]> = {
  kubectl: ["apply", "delete", "get", "patch"],
  terraform: ["plan", "apply", "destroy"],
  helm: ["install", "upgrade", "uninstall"],
};

function parseActor(input: string) {
  const [type, ...rest] = input.split(":");
  return { type: type || "agent", id: rest.join(":") || input };
}

export function Dashboard() {
  const { apiKey, ephemeral, setApiKey, clearApiKey, setEphemeral } = useApiKey();

  // Validate form state
  const [mode, setMode] = useState<Mode>("simple");
  const [tool, setTool] = useState("kubectl");
  const [customTool, setCustomTool] = useState("");
  const [operation, setOperation] = useState("apply");
  const [namespace, setNamespace] = useState("default");
  const [actor, setActor] = useState("agent:claude");
  const [environment, setEnvironment] = useState("production");
  const [extraParams, setExtraParams] = useState<{ key: string; value: string }[]>([]);

  // Advanced mode
  const [jsonText, setJsonText] = useState(
    JSON.stringify(
      {
        actor: { type: "agent", id: "claude", origin: "claude-code" },
        tool: "kubectl",
        operation: "apply",
        params: { namespace: "default" },
        environment: "production",
      },
      null,
      2,
    ),
  );
  const [jsonParsed, setJsonParsed] = useState<Record<string, unknown> | null>(null);
  const [jsonValid, setJsonValid] = useState(true);

  // Request state
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ValidateResponse | null>(null);
  const [error, setError] = useState<{ message: string; status?: number } | null>(null);
  const [showEvidence, setShowEvidence] = useState(false);

  // Recent evaluations
  const [recent, setRecent] = useState<RecentEval[]>([]);

  // Public key
  const [pubKey, setPubKey] = useState<string | null>(null);
  const [pubKeyLoading, setPubKeyLoading] = useState(false);

  // Fetch public key on mount
  useEffect(() => {
    if (!apiKey) return;
    setPubKeyLoading(true);
    getPublicKey()
      .then((data) => setPubKey(data.pem))
      .catch(() => setPubKey(null))
      .finally(() => setPubKeyLoading(false));
  }, [apiKey]);

  const buildInvocation = useCallback((): ToolInvocation | null => {
    if (mode === "advanced") {
      return jsonValid && jsonParsed ? (jsonParsed as unknown as ToolInvocation) : null;
    }

    const resolvedTool = tool === "custom" ? customTool : tool;
    const params: Record<string, unknown> = { namespace };
    for (const p of extraParams) {
      if (p.key.trim()) {
        params[p.key.trim()] = p.value;
      }
    }

    return {
      actor: parseActor(actor),
      tool: resolvedTool,
      operation,
      params,
      environment,
    };
  }, [mode, tool, customTool, operation, namespace, actor, environment, extraParams, jsonParsed, jsonValid]);

  const handleEvaluate = async () => {
    if (!apiKey) return;
    const invocation = buildInvocation();
    if (!invocation) return;

    setLoading(true);
    setError(null);
    setResult(null);
    setShowEvidence(false);

    try {
      const res = await validate(invocation, apiKey);
      setResult(res);
      setRecent((prev) => [
        {
          time: new Date().toLocaleTimeString(),
          tool: invocation.tool,
          operation: invocation.operation,
          allow: res.decision.allow,
        },
        ...prev,
      ]);
    } catch (err) {
      if (err instanceof ApiError) {
        setError({ message: err.message, status: err.status });
      } else {
        setError({ message: "Cannot reach API server. Check that evidra-api is running." });
      }
    } finally {
      setLoading(false);
    }
  };

  // If no API key, show prompt
  if (!apiKey) {
    return (
      <div className="dashboard">
        <div className="dash-section">
          <div className="dash-section-header">API Key</div>
          <KeyPrompt
            onSubmit={setApiKey}
            ephemeral={ephemeral}
            onEphemeralChange={setEphemeral}
          />
        </div>
      </div>
    );
  }

  const effectiveTool = tool === "custom" ? customTool : tool;
  const ops = OP_OPTIONS[effectiveTool] || [];

  return (
    <div className="dashboard">
      {/* API Key section */}
      <div className="dash-section">
        <div className="dash-section-header">API Key</div>
        <div className="dash-section-body">
          <div className="api-key-display">
            <code>{apiKey.slice(0, 12)}****</code>
            <button
              type="button"
              className="api-key-change"
              onClick={clearApiKey}
            >
              Change
            </button>
          </div>
          <div className="api-key-meta">
            API key stored in browser storage. Do not use on shared computers.
          </div>
        </div>
      </div>

      {/* Try Validate */}
      <div className="dash-section">
        <div className="dash-section-header">Try Validate</div>
        <div className="dash-section-body">
          {/* Tabs */}
          <div className="tabs" role="tablist">
            <button
              type="button"
              role="tab"
              className={`tab${mode === "simple" ? " tab--active" : ""}`}
              aria-selected={mode === "simple"}
              onClick={() => setMode("simple")}
            >
              Simple
            </button>
            <button
              type="button"
              role="tab"
              className={`tab${mode === "advanced" ? " tab--active" : ""}`}
              aria-selected={mode === "advanced"}
              onClick={() => setMode("advanced")}
            >
              Advanced / JSON
            </button>
          </div>

          {mode === "simple" ? (
            <div className="validate-form">
              <div className="form-field">
                <label htmlFor="dash-tool">Tool</label>
                <select
                  id="dash-tool"
                  value={tool}
                  onChange={(e) => {
                    setTool(e.target.value);
                    // Reset operation when tool changes
                    const newOps = OP_OPTIONS[e.target.value];
                    if (newOps && !newOps.includes(operation)) {
                      setOperation(newOps[0] || "apply");
                    }
                  }}
                >
                  {TOOL_OPTIONS.map((t) => (
                    <option key={t} value={t}>{t}</option>
                  ))}
                  <option value="custom">custom</option>
                </select>
                {tool === "custom" && (
                  <input
                    type="text"
                    placeholder="Tool name"
                    value={customTool}
                    onChange={(e) => setCustomTool(e.target.value)}
                  />
                )}
              </div>

              <div className="form-field">
                <label htmlFor="dash-operation">Operation</label>
                {ops.length > 0 ? (
                  <select
                    id="dash-operation"
                    value={operation}
                    onChange={(e) => setOperation(e.target.value)}
                  >
                    {ops.map((o) => (
                      <option key={o} value={o}>{o}</option>
                    ))}
                  </select>
                ) : (
                  <input
                    id="dash-operation"
                    type="text"
                    value={operation}
                    onChange={(e) => setOperation(e.target.value)}
                  />
                )}
              </div>

              <div className="form-field">
                <label htmlFor="dash-namespace">Namespace</label>
                <input
                  id="dash-namespace"
                  type="text"
                  value={namespace}
                  onChange={(e) => setNamespace(e.target.value)}
                />
              </div>

              <div className="form-field">
                <label htmlFor="dash-actor">Actor</label>
                <input
                  id="dash-actor"
                  type="text"
                  value={actor}
                  onChange={(e) => setActor(e.target.value)}
                />
              </div>

              <div className="form-field">
                <label htmlFor="dash-env">Environment</label>
                <input
                  id="dash-env"
                  type="text"
                  value={environment}
                  onChange={(e) => setEnvironment(e.target.value)}
                />
              </div>

              <KeyValueEditor pairs={extraParams} onChange={setExtraParams} />
            </div>
          ) : (
            <JsonEditor
              value={jsonText}
              onChange={(parsed, valid) => {
                setJsonParsed(parsed);
                setJsonValid(valid);
                if (valid && parsed) {
                  setJsonText(JSON.stringify(parsed, null, 2));
                }
              }}
            />
          )}

          <div style={{ marginTop: "var(--space-md)" }}>
            <button
              type="button"
              className="evaluate-btn"
              onClick={handleEvaluate}
              disabled={loading}
            >
              {loading ? "Evaluating..." : "Evaluate"}
            </button>
          </div>

          {/* Error */}
          {error && (
            <InlineError
              message={error.message}
              onRetry={error.status !== 400 ? handleEvaluate : undefined}
              action={
                error.status === 401
                  ? { label: "Change key", onClick: clearApiKey }
                  : undefined
              }
            />
          )}

          {/* Result */}
          {result && (
            <div className="validate-result">
              <div className="result-summary">
                <Badge variant={result.decision.allow ? "allow" : "deny"}>
                  {result.decision.allow ? "Allow" : "Deny"}
                </Badge>
                <Badge variant={result.decision.risk_level}>
                  risk: {result.decision.risk_level}
                </Badge>
              </div>
              <div className="result-reason">{result.decision.reason}</div>
              <button
                type="button"
                className="result-evidence-toggle"
                onClick={() => setShowEvidence(!showEvidence)}
              >
                {showEvidence ? "Hide" : "Show"} Evidence Record
              </button>
              {showEvidence && (
                <CodeBlock
                  code={JSON.stringify(result.evidence_record, null, 2)}
                />
              )}
            </div>
          )}
        </div>
      </div>

      {/* Recent Evaluations */}
      <div className="dash-section">
        <div className="dash-section-header">Recent Evaluations</div>
        <div className="dash-section-body">
          {recent.length === 0 ? (
            <p className="recent-empty">No evaluations yet. Try the form above.</p>
          ) : (
            <table className="recent-table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Tool</th>
                  <th>Op</th>
                  <th>Decision</th>
                </tr>
              </thead>
              <tbody>
                {recent.map((r, i) => (
                  <tr key={i}>
                    <td>{r.time}</td>
                    <td>{r.tool}</td>
                    <td>{r.operation}</td>
                    <td>
                      <Badge variant={r.allow ? "allow" : "deny"}>
                        {r.allow ? "Allow" : "Deny"}
                      </Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Public Key */}
      <div className="dash-section">
        <div className="dash-section-header">Public Key</div>
        <div className="dash-section-body">
          {pubKeyLoading ? (
            <p className="pubkey-loading">Loading public key...</p>
          ) : pubKey ? (
            <CodeBlock code={pubKey} />
          ) : (
            <p className="pubkey-loading">Could not load public key.</p>
          )}
        </div>
      </div>
    </div>
  );
}
