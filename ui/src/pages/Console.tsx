import { useState } from "react";
import { createKey, ApiError } from "../api/client";
import type { KeyResponse } from "../types/api";
import { CopyButton } from "../components/CopyButton";
import { CodeBlock } from "../components/CodeBlock";
import { InlineError } from "../components/InlineError";
import "../styles/console.css";

type Track = "mcp" | "api";
type EditorTab = "claude-code" | "claude-desktop" | "cursor" | "codex" | "gemini";

interface ConsoleProps {
  onKeyCreated: () => void;
}

const mcpClaudeCode = `claude mcp add evidra evidra-mcp`;

const mcpClaudeDesktop = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}`;

const mcpCursor = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}`;

const mcpCodex = `[mcp_servers.evidra]
command = "evidra-mcp"`;

const mcpGemini = `{
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

function EditorConfig({ editor }: { editor: EditorTab }) {
  switch (editor) {
    case "claude-code":
      return (
        <>
          <p>Run in your terminal:</p>
          <CodeBlock code={mcpClaudeCode} />
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
          <CodeBlock code={mcpClaudeDesktop} />
        </>
      );
    case "cursor":
      return (
        <>
          <p>Add to <code>.cursor/mcp.json</code>:</p>
          <CodeBlock code={mcpCursor} />
        </>
      );
    case "codex":
      return (
        <>
          <p>
            CLI: <code>codex mcp add evidra -- evidra-mcp</code>
          </p>
          <p>Or edit <code>~/.codex/config.toml</code>:</p>
          <CodeBlock code={mcpCodex} />
        </>
      );
    case "gemini":
      return (
        <>
          <p>Add to <code>~/.gemini/settings.json</code>:</p>
          <CodeBlock code={mcpGemini} />
        </>
      );
  }
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function Console({ onKeyCreated: _onKeyCreated }: ConsoleProps) {
  const [track, setTrack] = useState<Track>("mcp");
  const [editor, setEditor] = useState<EditorTab>("claude-code");
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
                className={`editor-tab${editor === tab.id ? " editor-tab--active" : ""}`}
                onClick={() => setEditor(tab.id)}
              >
                {tab.label}
              </button>
            ))}
          </div>
          <div className="editor-tab-content">
            <EditorConfig editor={editor} />
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
          <p>
            Both correct? You're set. <a href="#docs">Full setup guide</a>
          </p>
          <p className="optional-note">
            Optional: store evidence locally with{" "}
            <code>evidra-mcp --offline --evidence-dir ~/.evidra</code>
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
