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
