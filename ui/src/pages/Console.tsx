import { useState } from "react";
import { createKey, ApiError } from "../api/client";
import type { KeyResponse } from "../types/api";
import { CopyButton } from "../components/CopyButton";
import { CodeBlock } from "../components/CodeBlock";
import { InlineError } from "../components/InlineError";
import "../styles/console.css";

type Track = "mcp" | "api";

interface ConsoleProps {
  onKeyCreated: () => void;
}

const mcpClaudeDesktop = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": ["--observe"]
    }
  }
}`;

const mcpClaudeCode = `# .claude/settings.json
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}`;

const mcpCursor = `# .cursor/mcp.json
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}`;

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function Console({ onKeyCreated: _onKeyCreated }: ConsoleProps) {
  const [track, setTrack] = useState<Track>("mcp");
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
          <p>
            The MCP server runs locally and evaluates policy before every
            destructive operation. No API key needed for local mode.
          </p>

          <h3>1. Install</h3>
          <CodeBlock code={`go install samebits.com/evidra/cmd/evidra-mcp@latest`} />

          <h3>2. Configure your editor</h3>

          <details className="editor-config">
            <summary>Claude Desktop</summary>
            <p>Add to <code>claude_desktop_config.json</code>:</p>
            <CodeBlock code={mcpClaudeDesktop} />
          </details>

          <details className="editor-config">
            <summary>Claude Code</summary>
            <p>Add to <code>.claude/settings.json</code>:</p>
            <CodeBlock code={mcpClaudeCode} />
          </details>

          <details className="editor-config">
            <summary>Cursor</summary>
            <p>Add to <code>.cursor/mcp.json</code>:</p>
            <CodeBlock code={mcpCursor} />
          </details>

          <h3>3. Verify</h3>
          <p>
            Ask the agent: <em>"What tools does evidra provide?"</em> — it should
            list <code>validate</code> and <code>get_event</code>.
          </p>
          <p>
            Then test a deny: <em>"Validate kubectl.delete in kube-system"</em> — it
            should return <code>allow: false</code>.
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
