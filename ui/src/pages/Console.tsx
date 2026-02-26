import { useState } from "react";
import { createKey, ApiError } from "../api/client";
import type { KeyResponse } from "../types/api";
import { CopyButton } from "../components/CopyButton";
import { CodeBlock } from "../components/CodeBlock";
import { InlineError } from "../components/InlineError";
import "../styles/console.css";

interface ConsoleProps {
  onKeyCreated: () => void;
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function Console({ onKeyCreated: _onKeyCreated }: ConsoleProps) {
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
    ? `curl -X POST https://evidra.rest/v1/validate \\
  -H "Authorization: Bearer ${keyData.key}" \\
  -H "Content-Type: application/json" \\
  -d '{"actor":{"type":"agent","id":"claude"},"tool":"kubectl","operation":"apply","params":{"namespace":"default"}}'`
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

      {!keyData && (
        <div className="key-form">
          <input
            type="text"
            placeholder="Label (optional)"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
          />
          <button type="button" onClick={handleGetKey} disabled={loading}>
            {loading ? "Creating..." : "Get Key"}
          </button>
        </div>
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

      <footer className="console-footer">
        <a href="#docs">How it works</a>
        <a href="#docs">API Reference</a>
        <a
          href="https://github.com/evidra"
          target="_blank"
          rel="noopener noreferrer"
        >
          GitHub
        </a>
      </footer>
    </div>
  );
}
