import { useState } from "react";
import { createKey, ApiError } from "../api/client";
import type { KeyResponse } from "../types/api";
import { CopyButton } from "../components/CopyButton";
import { CodeBlock } from "../components/CodeBlock";
import { InlineError } from "../components/InlineError";
import "../styles/console.css";

type Track = "hosted" | "api";
type EditorTab = "claude-code" | "claude-desktop" | "cursor" | "codex" | "gemini";

interface ConsoleProps {
  onKeyCreated: () => void;
}

const HOSTED_URL = "https://evidra.samebits.com/mcp";

function hostedClaudeCode(apiKey?: string) {
  if (apiKey) {
    return `claude mcp add evidra --url ${HOSTED_URL} --header "Authorization: Bearer ${apiKey}"`;
  }
  return `claude mcp add evidra --url ${HOSTED_URL}`;
}

function hostedClaudeDesktop(apiKey?: string) {
  const headers = apiKey
    ? `\n      "headers": {\n        "Authorization": "Bearer ${apiKey}"\n      }`
    : "";
  return `{
  "mcpServers": {
    "evidra": {
      "url": "${HOSTED_URL}"${headers ? "," : ""}${headers}
    }
  }
}`;
}

function hostedCursor(apiKey?: string) {
  const headers = apiKey
    ? `\n      "headers": {\n        "Authorization": "Bearer ${apiKey}"\n      }`
    : "";
  return `{
  "mcpServers": {
    "evidra": {
      "url": "${HOSTED_URL}"${headers ? "," : ""}${headers}
    }
  }
}`;
}

function hostedCodex(apiKey?: string) {
  const headerLine = apiKey
    ? `\nheaders = { "Authorization" = "Bearer ${apiKey}" }`
    : "";
  return `[mcp_servers.evidra]
url = "${HOSTED_URL}"${headerLine}`;
}

function hostedGemini(apiKey?: string) {
  const headers = apiKey
    ? `\n      "headers": {\n        "Authorization": "Bearer ${apiKey}"\n      }`
    : "";
  return `{
  "mcpServers": {
    "evidra": {
      "url": "${HOSTED_URL}"${headers ? "," : ""}${headers}
    }
  }
}`;
}

const localClaudeCode = `claude mcp add evidra evidra-mcp`;

const localClaudeDesktop = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}`;

const localCursor = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}`;

const localCodex = `[mcp_servers.evidra]
command = "evidra-mcp"`;

const localGemini = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}`;

const editorTabs: { id: EditorTab; label: string }[] = [
  { id: "claude-code", label: "Claude Code" },
  { id: "claude-desktop", label: "Claude Desktop" },
  { id: "cursor", label: "Cursor" },
  { id: "codex", label: "Codex" },
  { id: "gemini", label: "Gemini CLI" },
];

function HostedEditorConfig({ editor, apiKey }: { editor: EditorTab; apiKey?: string }) {
  switch (editor) {
    case "claude-code":
      return (
        <>
          <p>Run in your terminal:</p>
          <CodeBlock code={hostedClaudeCode(apiKey)} />
        </>
      );
    case "claude-desktop":
      return (
        <>
          <p className="config-path-note">
            Config file location:<br />
            <code>macOS: ~/Library/Application Support/Claude/claude_desktop_config.json</code><br />
            <code>Linux: ~/.config/Claude/claude_desktop_config.json</code><br />
            <code>Windows: %APPDATA%\Claude\claude_desktop_config.json</code>
          </p>
          <CodeBlock code={hostedClaudeDesktop(apiKey)} />
        </>
      );
    case "cursor":
      return (
        <>
          <p>Add to <code>.cursor/mcp.json</code>:</p>
          <CodeBlock code={hostedCursor(apiKey)} />
        </>
      );
    case "codex":
      return (
        <>
          <p>
            CLI: <code>codex mcp add evidra -- --url {HOSTED_URL}</code>
          </p>
          <p>Or edit <code>~/.codex/config.toml</code>:</p>
          <CodeBlock code={hostedCodex(apiKey)} />
        </>
      );
    case "gemini":
      return (
        <>
          <p>Add to <code>~/.gemini/settings.json</code>:</p>
          <CodeBlock code={hostedGemini(apiKey)} />
        </>
      );
  }
}

function LocalEditorConfig({ editor }: { editor: EditorTab }) {
  switch (editor) {
    case "claude-code":
      return (
        <>
          <p>Run in your terminal:</p>
          <CodeBlock code={localClaudeCode} />
        </>
      );
    case "claude-desktop":
      return (
        <>
          <p className="config-path-note">
            Config file location:<br />
            <code>macOS: ~/Library/Application Support/Claude/claude_desktop_config.json</code><br />
            <code>Linux: ~/.config/Claude/claude_desktop_config.json</code><br />
            <code>Windows: %APPDATA%\Claude\claude_desktop_config.json</code>
          </p>
          <CodeBlock code={localClaudeDesktop} />
        </>
      );
    case "cursor":
      return (
        <>
          <p>Add to <code>.cursor/mcp.json</code>:</p>
          <CodeBlock code={localCursor} />
        </>
      );
    case "codex":
      return (
        <>
          <p>
            CLI: <code>codex mcp add evidra -- evidra-mcp</code>
          </p>
          <p>Or edit <code>~/.codex/config.toml</code>:</p>
          <CodeBlock code={localCodex} />
        </>
      );
    case "gemini":
      return (
        <>
          <p>Add to <code>~/.gemini/settings.json</code>:</p>
          <CodeBlock code={localGemini} />
        </>
      );
  }
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function Console({ onKeyCreated: _onKeyCreated }: ConsoleProps) {
  const [track, setTrack] = useState<Track>("hosted");
  const [editor, setEditor] = useState<EditorTab>("claude-code");
  const [localEditor, setLocalEditor] = useState<EditorTab>("claude-code");
  const [label, setLabel] = useState("");
  const [loading, setLoading] = useState(false);
  const [keyData, setKeyData] = useState<KeyResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

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

  const curlExample = keyData
    ? `curl -X POST https://api.evidra.rest/v1/validate \\
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
          className={`track-btn${track === "hosted" ? " track-btn--active" : ""}`}
          onClick={() => setTrack("hosted")}
        >
          Hosted (no install)
        </button>
        <button
          type="button"
          className={`track-btn${track === "api" ? " track-btn--active" : ""}`}
          onClick={() => setTrack("api")}
        >
          API / CI
        </button>
      </div>

      {track === "hosted" ? (
        <div className="track-content">
          <h2>Hosted MCP</h2>
          <p>
            Connect your editor to the hosted Evidra endpoint. No install needed.
            Policy evaluates in the cloud — every decision signed and returned.
          </p>

          {!keyData ? (
            <>
              <p>Generate a key to connect your editor:</p>
              <div className="key-form">
                <input
                  type="text"
                  placeholder="Label (optional, e.g. my-laptop)"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                />
                <button type="button" onClick={handleGetKey} disabled={loading}>
                  {loading ? "Creating..." : "Get Key"}
                </button>
              </div>
              <p className="rate-limit-note">3 keys per hour per IP address.</p>
            </>
          ) : (
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

              <h3>Add to your editor</h3>
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
                <HostedEditorConfig editor={editor} apiKey={keyData.key} />
              </div>

              <p>
                Restart your editor. Try: <em>"Delete all pods in kube-system"</em>
              </p>
            </>
          )}

          {error && (
            <InlineError
              message={error}
              onRetry={handleGetKey}
            />
          )}

          <details className="local-fallback">
            <summary>Or install locally (offline, no key needed)</summary>
            <div className="local-fallback-content">
              <h3>1. Install</h3>
              <CodeBlock code={`brew install evidra/tap/evidra-mcp`} />
              <p>
                Or: <code>go install samebits.com/evidra/cmd/evidra-mcp@latest</code>
              </p>

              <h3>2. Add to your editor</h3>
              <div className="editor-tabs">
                {editorTabs.map((tab) => (
                  <button
                    key={tab.id}
                    type="button"
                    className={`editor-tab${localEditor === tab.id ? " editor-tab--active" : ""}`}
                    onClick={() => setLocalEditor(tab.id)}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
              <div className="editor-tab-content">
                <LocalEditorConfig editor={localEditor} />
              </div>

              <h3>3. Verify</h3>
              <p>
                Test a <strong>deny</strong>: <em>"Validate kubectl.delete in kube-system"</em> — should
                return <code>allow: false</code>.
              </p>
              <p>
                Test an <strong>allow</strong>: <em>"Validate kubectl.get pods in default"</em> — should
                return <code>allow: true</code>.
              </p>
              <p className="optional-note">
                Optional: store evidence locally with{" "}
                <code>evidra-mcp --offline --evidence-dir ~/.evidra</code>
              </p>
            </div>
          </details>
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
