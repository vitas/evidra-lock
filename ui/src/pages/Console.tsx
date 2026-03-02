import { useState, useRef, useEffect } from "react";
import { createKey, ApiError } from "../api/client";
import type { KeyResponse } from "../types/api";
import { CopyButton } from "../components/CopyButton";
import { CodeBlock } from "../components/CodeBlock";
import { InlineError } from "../components/InlineError";
import "../styles/console.css";

type Track = "mcp" | "api";
type SetupPath = "hosted" | "local-api" | "offline";
type EditorTab = "claude-code" | "claude-desktop" | "cursor" | "codex" | "gemini";

interface ConsoleProps {
  onKeyCreated: () => void;
}

const KEY_PLACEHOLDER = "ev1_YOUR_KEY_HERE";
const HOSTED_URL = "https://evidra.samebits.com/mcp";
const API_URL = "https://api.evidra.rest";

// ── Config generators ──────────────────────────────────

function hostedClaudeCode(key: string) {
  return `claude mcp add evidra --url ${HOSTED_URL} --header "Authorization: Bearer ${key}"`;
}

function hostedJson(key: string) {
  return `{
  "mcpServers": {
    "evidra": {
      "url": "${HOSTED_URL}",
      "headers": {
        "Authorization": "Bearer ${key}"
      }
    }
  }
}`;
}

function hostedCodex(key: string) {
  return `[mcp_servers.evidra]
url = "${HOSTED_URL}"

[mcp_servers.evidra.headers]
Authorization = "Bearer ${key}"`;
}

function localApiClaudeCode(key: string) {
  return `claude mcp add evidra evidra-mcp \\
  -e EVIDRA_URL=${API_URL} \\
  -e EVIDRA_API_KEY=${key} \\
  -e EVIDRA_ENVIRONMENT=production \\
  -e EVIDRA_FALLBACK=offline
# Optional: enable deny-loop prevention
#   -e EVIDRA_DENY_CACHE=true`;
}

function localApiJson(key: string) {
  return `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "env": {
        "EVIDRA_URL": "${API_URL}",
        "EVIDRA_API_KEY": "${key}",
        "EVIDRA_ENVIRONMENT": "production",
        "EVIDRA_FALLBACK": "offline",
        "EVIDRA_BUNDLE_PATH": "",
        "EVIDRA_DENY_CACHE": "true"
      }
    }
  }
}`;
}

function localApiCodex(key: string) {
  return `[mcp_servers.evidra]
command = "evidra-mcp"

[mcp_servers.evidra.env]
EVIDRA_URL = "${API_URL}"
EVIDRA_API_KEY = "${key}"
EVIDRA_ENVIRONMENT = "production"
EVIDRA_FALLBACK = "offline"
EVIDRA_BUNDLE_PATH = ""
EVIDRA_DENY_CACHE = "true"`;
}

const offlineClaudeCode = `claude mcp add evidra evidra-mcp \\
  -e EVIDRA_ENVIRONMENT=production
# Optional: enable deny-loop prevention
#   -e EVIDRA_DENY_CACHE=true`;

const offlineJson = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "env": {
        "EVIDRA_ENVIRONMENT": "production",
        "EVIDRA_BUNDLE_PATH": "",
        "EVIDRA_DENY_CACHE": ""
      }
    }
  }
}`;

const offlineCodex = `[mcp_servers.evidra]
command = "evidra-mcp"

[mcp_servers.evidra.env]
EVIDRA_ENVIRONMENT = "production"
EVIDRA_BUNDLE_PATH = ""
EVIDRA_DENY_CACHE = "true"`;

// ── Tabs & paths ────────────────────────────────────────

const editorTabs: { id: EditorTab; label: string }[] = [
  { id: "claude-code", label: "Claude Code" },
  { id: "claude-desktop", label: "Claude Desktop" },
  { id: "cursor", label: "Cursor" },
  { id: "codex", label: "Codex" },
  { id: "gemini", label: "Gemini CLI" },
];

const setupPaths: { id: SetupPath; label: string }[] = [
  { id: "hosted", label: "Hosted (no install)" },
  { id: "local-api", label: "Local + Remote API" },
  { id: "offline", label: "Fully Offline" },
];

// ── EditorConfig component ──────────────────────────────

function EditorConfig({ editor, setupPath, keyValue }: {
  editor: EditorTab;
  setupPath: SetupPath;
  keyValue: string;
}) {
  // Path note per editor
  const pathNote = (() => {
    switch (editor) {
      case "claude-desktop":
        return (
          <p className="config-path-note">
            Config file location:<br />
            <code>macOS: ~/Library/Application Support/Claude/claude_desktop_config.json</code><br />
            <code>Linux: ~/.config/Claude/claude_desktop_config.json</code><br />
            <code>Windows: %APPDATA%\Claude\claude_desktop_config.json</code>
          </p>
        );
      case "cursor":
        return <p>Add to <code>.cursor/mcp.json</code>:</p>;
      case "codex":
        return <p>Edit <code>~/.codex/config.toml</code>:</p>;
      case "gemini":
        return <p>Add to <code>~/.gemini/settings.json</code>:</p>;
      default:
        return null;
    }
  })();

  // Claude Code — one-liner for all paths
  if (editor === "claude-code") {
    let code: string;
    if (setupPath === "hosted") code = hostedClaudeCode(keyValue);
    else if (setupPath === "local-api") code = localApiClaudeCode(keyValue);
    else code = offlineClaudeCode;
    return (
      <>
        <p>Run in your terminal:</p>
        <CodeBlock code={code} />
      </>
    );
  }

  // Codex — TOML format
  if (editor === "codex") {
    let code: string;
    if (setupPath === "hosted") code = hostedCodex(keyValue);
    else if (setupPath === "local-api") code = localApiCodex(keyValue);
    else code = offlineCodex;
    return (
      <>
        {pathNote}
        <CodeBlock code={code} />
      </>
    );
  }

  // Claude Desktop, Cursor, Gemini — JSON format
  let code: string;
  if (setupPath === "hosted") code = hostedJson(keyValue);
  else if (setupPath === "local-api") code = localApiJson(keyValue);
  else code = offlineJson;
  return (
    <>
      {pathNote}
      <CodeBlock code={code} />
    </>
  );
}

// ── Console component ───────────────────────────────────

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function Console({ onKeyCreated: _onKeyCreated }: ConsoleProps) {
  const [track, setTrack] = useState<Track>("mcp");
  const [setupPath, setSetupPath] = useState<SetupPath>("hosted");
  const [editor, setEditor] = useState<EditorTab>("claude-code");
  const [label, setLabel] = useState("");
  const [loading, setLoading] = useState(false);
  const [keyData, setKeyData] = useState<KeyResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [manualKey, setManualKey] = useState("");
  const [keyInputOpen, setKeyInputOpen] = useState(false);
  const keyInputRef = useRef<HTMLInputElement>(null);

  const handleGetKey = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await createKey(label || undefined);
      setKeyData(data);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Cannot reach API server. Check that evidra-api is running.");
      }
    } finally {
      setLoading(false);
    }
  };

  const keyValue = keyData?.key || manualKey || KEY_PLACEHOLDER;
  const needsKey = setupPath !== "offline";
  const needsInstall = setupPath !== "hosted";

  useEffect(() => {
    if (keyInputOpen && keyInputRef.current) {
      keyInputRef.current.focus();
    }
  }, [keyInputOpen]);

  const curlExample = keyData
    ? `curl -X POST ${API_URL}/v1/validate \\
  -H "Authorization: Bearer ${keyData.key}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "actor": {"type":"agent","id":"claude","origin":"ci"},
    "tool": "kubectl",
    "operation": "apply",
    "params": {
      "action": {
        "kind": "kubectl.apply",
        "target": {"namespace":"default"},
        "payload": {"resource":"configmap"}
      }
    }
  }'`
    : "";

  // Step numbering: hosted has no install step, so numbering shifts
  const editorStepNum = needsInstall ? (needsKey ? 3 : 2) : (needsKey ? 2 : 1);
  const verifyStepNum = editorStepNum + 1;

  return (
    <div className="console">
      <div className="console-intro">
        <h1>Console</h1>
        <p>
          Policy evaluation for AI agents. Get a signed evidence record for
          every decision. Store it wherever you want.
        </p>
      </div>

      {/* Track selector */}
      <div className="track-selector">
        <button
          type="button"
          className={`track-btn${track === "mcp" ? " track-btn--active" : ""}`}
          onClick={() => setTrack("mcp")}
        >
          AI Agent (MCP)
        </button>
        <button
          type="button"
          className={`track-btn${track === "api" ? " track-btn--active" : ""}`}
          onClick={() => setTrack("api")}
        >
          API / CI
        </button>
      </div>

      {track === "mcp" ? (
        <div className="track-content">
          <h2>MCP Setup</h2>

          {/* Setup path tabs */}
          <div className="setup-tabs">
            {setupPaths.map((p) => (
              <button
                key={p.id}
                type="button"
                className={`setup-tab${setupPath === p.id ? " setup-tab--active" : ""}`}
                onClick={() => setSetupPath(p.id)}
              >
                {p.label}
              </button>
            ))}
          </div>

          {/* Path description */}
          {setupPath === "hosted" && (
            <p className="setup-desc">
              No install needed. Evaluations run on the hosted endpoint. <strong>Requires an API key.</strong>
            </p>
          )}
          {setupPath === "local-api" && (
            <p className="setup-desc">
              Install the binary locally. Evaluations sent to hosted API.
              Evidence stored server-side. <strong>Requires an API key.</strong>
            </p>
          )}
          {setupPath === "offline" && (
            <p className="setup-desc">
              Install the binary locally. Evaluations run entirely offline using the
              embedded policy bundle. Evidence stored to <code>~/.evidra/</code>. <strong>No API key needed.</strong>
            </p>
          )}

          {/* Install step (paths 2 & 3) */}
          {needsInstall && (
            <>
              <h3>1. Install</h3>
              <CodeBlock code="brew install samebits/tap/evidra-mcp" />
              <p>
                Or: <code>go install samebits.com/evidra/cmd/evidra-mcp@latest</code>
              </p>
            </>
          )}

          {/* Key hint (paths 1 & 2) */}
          {needsKey && (
            <div className="setup-key-hint">
              {keyData && !keyInputOpen ? (
                <p>
                  Using key: <code>{keyData.key.slice(0, 12)}...</code>{" "}
                  <a
                    href="#"
                    onClick={(e) => {
                      e.preventDefault();
                      setKeyData(null);
                      setManualKey("");
                    }}
                  >
                    Clear
                  </a>
                </p>
              ) : keyInputOpen ? (
                <div className="key-input-inline">
                  <div className="key-input-row">
                    <span className="key-input-label">Paste your API key</span>
                    <input
                      id="manual-key-input"
                      ref={keyInputRef}
                      type="text"
                      placeholder="ev1_..."
                      value={manualKey}
                      onChange={(e) => setManualKey(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Escape") {
                          setKeyInputOpen(false);
                          setManualKey("");
                        }
                      }}
                    />
                    <span className="key-hint-actions">
                      {manualKey ? (
                        <a
                          href="#"
                          onClick={(e) => {
                            e.preventDefault();
                            setManualKey("");
                            setKeyInputOpen(false);
                          }}
                        >
                          Clear
                        </a>
                      ) : (
                        <a
                          href="#console"
                          onClick={(e) => {
                            e.preventDefault();
                            setKeyInputOpen(false);
                            setTrack("api");
                          }}
                        >
                          Get a key
                        </a>
                      )}
                    </span>
                  </div>
                </div>
              ) : (
                <p className="key-hint-row">
                  <span>Replace <code
                      className="key-placeholder-clickable"
                      onClick={() => setKeyInputOpen(true)}
                    >{KEY_PLACEHOLDER}</code> with your API key.</span>
                  <span className="key-hint-actions">
                    <a
                      href="#"
                      onClick={(e) => {
                        e.preventDefault();
                        setKeyInputOpen(true);
                      }}
                    >
                      I have one
                    </a>
                    {" "}&middot;{" "}
                    <a
                      href="#console"
                      onClick={(e) => {
                        e.preventDefault();
                        setTrack("api");
                      }}
                    >
                      Get a key
                    </a>
                  </span>
                </p>
              )}
            </div>
          )}

          {/* Editor tabs */}
          <h3>
            {needsInstall
              ? `${needsKey ? "2" : "2"}. Add to your editor`
              : `${needsKey ? "1" : "1"}. Add to your editor`}
          </h3>
          <div className="editor-tabs">
            {editorTabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                className={`editor-tab${editor === tab.id ? " editor-tab--active" : ""}`}
                onClick={() => setEditor(tab.id)}
              >
                {tab.label}
              </button>
            ))}
          </div>
          <div className="editor-tab-content">
            <EditorConfig editor={editor} setupPath={setupPath} keyValue={keyValue} />
          </div>

          <div className="env-legend">
            <div className="env-legend-title">Environment variables</div>
            <dl className="env-legend-list">
              {needsKey && (
                <>
                  <dt><code>EVIDRA_API_KEY</code></dt>
                  <dd>Bearer token for API authentication</dd>
                </>
              )}
              <dt><code>EVIDRA_ENVIRONMENT</code></dt>
              <dd>Policy evaluation context: <code>production</code>, <code>staging</code>, or <code>development</code>. Affects environment-dependent rules.</dd>
              {setupPath === "local-api" && (
                <>
                  <dt><code>EVIDRA_FALLBACK</code></dt>
                  <dd><code>offline</code> = evaluate locally when API is unreachable. <code>closed</code> (default) = deny all if API is down.</dd>
                </>
              )}
              {setupPath !== "hosted" && (
                <>
                  <dt><code>EVIDRA_BUNDLE_PATH</code></dt>
                  <dd>Custom OPA bundle directory. Leave empty to use the embedded <code>ops-v0.1</code> bundle.</dd>
                  <dt><code>EVIDRA_DENY_CACHE</code></dt>
                  <dd>Enable deny-loop prevention for agent/CI actors (<code>true</code>/<code>false</code>, default: <code>false</code>). Blocks repeated identical denied requests without re-evaluating policy.</dd>
                </>
              )}
            </dl>
          </div>

          {/* Verify step */}
          <h3>{verifyStepNum}. Verify</h3>
          <p>
            Restart your editor. Try: <em>"Delete all pods in kube-system"</em> — should
            return <code>allow: false</code>.
          </p>
          <p>
            Try: <em>"Deploy nginx to default namespace"</em> — should
            return <code>allow: true</code>.
          </p>
        </div>
      ) : (
        <div className="track-content">
          <h2>API Setup</h2>

          {!keyData && (
            <>
              <ol className="onboarding-steps">
                <li>
                  <strong>Generate a key</strong>
                  <p>Click <em>Get Key</em> below. The key is shown once — copy it immediately.</p>
                </li>
                <li>
                  <strong>Add it to your agent</strong>
                  <p>
                    Set <code>EVIDRA_API_KEY=&lt;key&gt;</code> in your environment,
                    or pass it as <code>Authorization: Bearer &lt;key&gt;</code> on every request.
                  </p>
                </li>
                <li>
                  <strong>Call validate before every apply</strong>
                  <p>
                    Send <code>POST /v1/validate</code> before <code>kubectl apply</code> or{" "}
                    <code>terraform apply</code>. Receive a signed evidence record and store it
                    alongside your change log.
                  </p>
                </li>
              </ol>

              <div className="key-form">
                <input
                  type="text"
                  placeholder="Label (optional, e.g. prod-agent)"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                />
                <button type="button" onClick={handleGetKey} disabled={loading}>
                  {loading ? "Creating..." : "Get Key"}
                </button>
              </div>
              <p className="rate-limit-note">3 keys per hour per IP address.</p>
            </>
          )}

          {error && (
            <InlineError
              message={error}
              onRetry={handleGetKey}
            />
          )}

          {keyData && (
            <>
              <div className="key-result">
                <div className="key-value">
                  <code>{keyData.key}</code>
                  <CopyButton text={keyData.key} />
                </div>
                <div className="key-warning">
                  Save this key — it won't be shown again
                </div>
              </div>

              <div className="quick-start">
                <h3>Quick start:</h3>
                <CodeBlock code={curlExample} />
              </div>
            </>
          )}
        </div>
      )}

      <footer className="console-footer">
        <a href="#docs">How it works</a>
        <a href="#docs">API Reference</a>
        <a
          href="https://github.com/vitas/evidra"
          target="_blank"
          rel="noopener noreferrer"
        >
          GitHub
        </a>
      </footer>
    </div>
  );
}
