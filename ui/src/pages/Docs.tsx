import { CodeBlock } from "../components/CodeBlock";
import "../styles/docs.css";

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

const curlGetKey = `curl -X POST https://api.evidra.rest/v1/keys \\
  -H "Content-Type: application/json" \\
  -d '{"label":"my-agent"}'`;

const curlValidateAllow = `curl -X POST https://api.evidra.rest/v1/validate \\
  -H "Authorization: Bearer ev1_YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "actor": {"type":"agent","id":"claude","origin":"claude-code"},
    "tool": "kubectl",
    "operation": "apply",
    "params": {
      "action": {
        "kind": "kubectl.apply",
        "target": {"namespace":"default"},
        "payload": {"resource":"configmap"}
      }
    },
    "environment": "staging"
  }'`;

const curlValidateDeny = `curl -X POST https://api.evidra.rest/v1/validate \\
  -H "Authorization: Bearer ev1_YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "actor": {"type":"agent","id":"claude","origin":"claude-code"},
    "tool": "kubectl",
    "operation": "delete",
    "params": {
      "action": {
        "kind": "kubectl.delete",
        "target": {"namespace":"kube-system"}
      }
    },
    "environment": "production"
  }'`;

const curlVerify = `# 1. Save the evidence record from the validate response
echo '$EVIDENCE_JSON' > evidence.json

# 2. Get the public key
curl https://api.evidra.rest/v1/evidence/pubkey | jq -r .pem > pubkey.pem

# 3. Extract signature and payload
jq -r .signature evidence.json | base64 -d > sig.bin
jq -r .signing_payload evidence.json > payload.txt

# 4. Verify with OpenSSL
openssl pkeyutl -verify -pubin -inkey pubkey.pem \\
  -rawin -in payload.txt -sigfile sig.bin`;

const validateReqExample = `{
  "actor": {
    "type": "agent",
    "id": "claude",
    "origin": "claude-code"
  },
  "tool": "kubectl",
  "operation": "apply",
  "params": {
    "action": {
      "kind": "kubectl.apply",
      "target": { "namespace": "default" },
      "payload": { "resource": "configmap" }
    }
  },
  "environment": "production"
}`;

const validateResExample = `{
  "event_id": "evt_01J...",
  "timestamp": "2026-02-26T14:23:01Z",
  "tenant_id": "01J...",
  "server_id": "evidra-api-1",
  "policy_ref": "bundle://evidra/default:0.1.0",
  "actor": { "type": "agent", "id": "claude", "origin": "claude-code" },
  "tool": "kubectl",
  "operation": "apply",
  "environment": "production",
  "input_hash": "sha256:...",
  "decision": {
    "allow": true,
    "risk_level": "low",
    "reason": "all checks passed",
    "reasons": [],
    "hints": [],
    "rule_ids": []
  },
  "signing_payload": "evidra.v1\\nevent_id=evt_01J...",
  "signature": "base64..."
}`;

const validateDenyExample = `{
  "event_id": "evt_01J...",
  "timestamp": "2026-02-26T14:24:00Z",
  "tenant_id": "01J...",
  "server_id": "evidra-api-1",
  "policy_ref": "bundle://evidra/default:0.1.0",
  "actor": { "type": "agent", "id": "claude", "origin": "claude-code" },
  "tool": "kubectl",
  "operation": "delete",
  "environment": "production",
  "input_hash": "sha256:...",
  "decision": {
    "allow": false,
    "risk_level": "high",
    "reason": "denied by policy",
    "reasons": [
      "ops.insufficient_context: destructive operation missing required payload fields",
      "namespace.protected: kube-system is a protected namespace"
    ],
    "hints": [
      "Include payload.resource and payload.name for delete operations",
      "Use a non-system namespace or add a breakglass risk_tag"
    ],
    "rule_ids": [
      "ops.insufficient_context",
      "namespace.protected"
    ]
  },
  "signing_payload": "evidra.v1\\nevent_id=evt_01J...",
  "signature": "base64..."
}`;

const signingPayloadExample = `evidra.v1
event_id=evt_01J...
timestamp=2026-02-26T14:23:01Z
server_id=evidra-api-1
policy_ref=bundle://evidra/default:0.1.0
actor_type=agent
actor_id=claude
tool=kubectl
operation=apply
environment=production
input_hash=sha256:...
allow=true
risk_level=low
reason=all checks passed
reasons=0:
hints=0:
rule_ids=0:`;

const cliUsageExample = `# Evaluate a scenario file
evidra validate -f scenario.yaml

# Run MCP server (local mode, no API key needed)
evidra-mcp

# Run MCP server in observe mode (policy evaluated but never blocks)
evidra-mcp --observe

# Inspect a stored evidence event
evidra-mcp get_event --event-id evt_01J...`;

export function Docs() {
  return (
    <div className="docs">
      <h1>Documentation</h1>

      {/* MCP Setup — K1 */}
      <section className="docs-section">
        <h2>MCP Setup</h2>
        <p>
          The fastest way to use Evidra: run the MCP server locally. Your AI agent
          calls <code>validate</code> before every destructive operation. No API
          key needed for local mode.
        </p>

        <h3>Install</h3>
        <CodeBlock code={`go install samebits.com/evidra/cmd/evidra-mcp@latest`} />

        <h3>Claude Desktop</h3>
        <p>Add to <code>claude_desktop_config.json</code>:</p>
        <CodeBlock code={mcpClaudeDesktop} />

        <h3>Claude Code</h3>
        <p>Add to your project <code>.claude/settings.json</code>:</p>
        <CodeBlock code={mcpClaudeCode} />

        <h3>Cursor</h3>
        <p>Add to <code>.cursor/mcp.json</code>:</p>
        <CodeBlock code={mcpCursor} />

        <h3>Verify</h3>
        <ol>
          <li>
            Ask the agent: <em>"What tools does evidra provide?"</em> — it should
            list <code>validate</code> and <code>get_event</code>.
          </li>
          <li>
            Test a deny: <em>"Validate kubectl.delete in kube-system"</em> — the
            agent should report <code>allow: false</code> and stop.
          </li>
        </ol>
      </section>

      {/* Quickstart — K2, K3 */}
      <section className="docs-section">
        <h2>API Quickstart</h2>

        <h3>1. Get an API key</h3>
        <p>
          Create a key from the <a href="#console">Console</a> or via curl:
        </p>
        <CodeBlock code={curlGetKey} />

        <h3>2. Evaluate a policy (allow)</h3>
        <p>
          Send a tool invocation to get a policy decision and signed evidence record.
          A staging <code>kubectl apply</code> is typically allowed:
        </p>
        <CodeBlock code={curlValidateAllow} />

        <h3>3. Evaluate a policy (deny)</h3>
        <p>
          A <code>kubectl delete</code> in <code>kube-system</code> on production
          is denied by default policy:
        </p>
        <CodeBlock code={curlValidateDeny} />

        <div className="docs-warning">
          HTTP 200 does not mean allow. Policy deny returns HTTP 200 with{" "}
          <code>decision.allow: false</code>. Always check <code>decision.allow</code>,
          not the HTTP status code.
        </div>

        <h3>4. Verify evidence offline</h3>
        <p>
          Evidence records are cryptographically signed with Ed25519. Verify
          with the public key — no server contact needed:
        </p>
        <CodeBlock code={curlVerify} />
      </section>

      {/* API Reference — K8 */}
      <section className="docs-section">
        <h2>API Reference</h2>

        <table className="endpoint-table">
          <thead>
            <tr>
              <th>Method</th>
              <th>Path</th>
              <th>Auth</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><span className="method-badge method-badge--post">POST</span></td>
              <td><code>/v1/validate</code></td>
              <td>Bearer</td>
              <td>Evaluate policy, return signed evidence record. Deny = HTTP 200.</td>
            </tr>
            <tr>
              <td><span className="method-badge method-badge--post">POST</span></td>
              <td><code>/v1/keys</code></td>
              <td>&mdash;</td>
              <td>Create API key (returns key once, rate-limited)</td>
            </tr>
            <tr>
              <td><span className="method-badge method-badge--get">GET</span></td>
              <td><code>/v1/evidence/pubkey</code></td>
              <td>&mdash;</td>
              <td>Ed25519 public key (PEM)</td>
            </tr>
            <tr>
              <td><span className="method-badge method-badge--get">GET</span></td>
              <td><code>/healthz</code></td>
              <td>&mdash;</td>
              <td>Health check (always 200)</td>
            </tr>
          </tbody>
        </table>

        <h3>POST /v1/validate</h3>
        <p>Request body (<code>ToolInvocation</code>):</p>
        <CodeBlock code={validateReqExample} />

        <p>Response — the evidence record is returned directly (not wrapped):</p>
        <CodeBlock code={validateResExample} />
      </section>

      {/* Deny Example — K4 */}
      <section className="docs-section">
        <h2>Deny Response Example</h2>
        <p>
          When policy denies an operation, the response is still HTTP 200.
          The <code>decision.allow</code> field is <code>false</code>, and
          the response includes <code>reasons</code>, <code>hints</code>,
          and <code>rule_ids</code> for diagnosis:
        </p>
        <CodeBlock code={validateDenyExample} />
      </section>

      {/* Understanding Decisions — K5 */}
      <section className="docs-section">
        <h2>Understanding Decisions</h2>
        <p>Common rule IDs and what they mean:</p>
        <table className="rule-table">
          <thead>
            <tr>
              <th>Rule ID</th>
              <th>Meaning</th>
              <th>Fix</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><code>ops.insufficient_context</code></td>
              <td>Destructive operation is missing required payload fields</td>
              <td>Include <code>payload.resource</code>, <code>payload.name</code> or relevant fields</td>
            </tr>
            <tr>
              <td><code>ops.unknown_destructive</code></td>
              <td>Tool/operation is not in the known-safe list</td>
              <td>Use a recognized tool prefix or add a <code>breakglass</code> risk tag</td>
            </tr>
            <tr>
              <td><code>ops.truncated_context</code></td>
              <td>Adapter output was truncated</td>
              <td>Ensure adapter produces complete output, or increase output limits</td>
            </tr>
            <tr>
              <td><code>namespace.protected</code></td>
              <td>Target namespace is protected (kube-system, kube-public, etc.)</td>
              <td>Use a non-system namespace or add <code>breakglass</code> risk tag</td>
            </tr>
          </tbody>
        </table>
      </section>

      {/* CLI Usage — K6 */}
      <section className="docs-section">
        <h2>CLI Usage</h2>
        <p>
          Evidra ships three binaries: <code>evidra</code> (offline CLI),{" "}
          <code>evidra-mcp</code> (MCP server), and <code>evidra-api</code> (HTTP API).
        </p>
        <CodeBlock code={cliUsageExample} />
      </section>

      {/* Evidence Guide */}
      <section className="docs-section">
        <h2>Evidence Guide</h2>

        <h3>What is an evidence record?</h3>
        <p>
          Every <code>/v1/validate</code> response is a cryptographically signed
          evidence record. The server signs it but does not store it.
          You store it wherever you want: filesystem, S3, database, git.
        </p>

        <h3>Storage recommendations</h3>
        <p>
          Append evidence records to a JSONL file, store in S3 with the{" "}
          <code>event_id</code> as the key, or insert into a database. The
          records are self-contained — each one has the full decision context
          and can be verified independently.
        </p>

        <h3>Offline verification</h3>
        <p>
          The <code>signature</code> field is an Ed25519 signature over the{" "}
          <code>signing_payload</code> field. The signing payload is
          deterministic text (not JSON) — fields concatenated as{" "}
          <code>key=value\n</code> in fixed order with a{" "}
          <code>evidra.v1</code> version prefix:
        </p>
        <CodeBlock code={signingPayloadExample} />

        <p>
          List fields (reasons, hints, rule_ids) use length-prefixed encoding:{" "}
          <code>hints=3:foo,11:hello,world,0:</code>. This ensures
          deterministic serialization regardless of JSON formatting.
        </p>

        <p>
          Fetch the public key from <code>GET /v1/evidence/pubkey</code> once
          and cache it. Verify using standard Ed25519 tooling (OpenSSL, Go{" "}
          <code>crypto/ed25519</code>, Node <code>crypto</code>).
        </p>
      </section>

      {/* MCP Troubleshooting — K7 */}
      <section className="docs-section">
        <h2>MCP Troubleshooting</h2>

        <h3>Tool not showing in agent</h3>
        <p>
          Check that <code>evidra-mcp</code> is on your PATH. Run{" "}
          <code>which evidra-mcp</code> to verify. Restart the editor after
          adding the MCP config.
        </p>

        <h3>Permission denied</h3>
        <p>
          Ensure <code>evidra-mcp</code> is executable:{" "}
          <code>chmod +x $(which evidra-mcp)</code>. On macOS, you may need to
          allow it in System Settings &gt; Privacy &amp; Security.
        </p>

        <h3>Agent doesn't call validate</h3>
        <p>
          The <code>validate</code> tool description instructs the agent to call
          it before destructive operations. If the agent skips it, remind it:
          <em> "Always call evidra validate before kubectl apply or terraform apply."</em>
        </p>

        <h3>Insufficient context deny</h3>
        <p>
          The <code>ops.insufficient_context</code> rule requires payload fields
          for destructive operations. Ensure the agent provides{" "}
          <code>payload.resource</code> and <code>payload.name</code> for
          delete/apply operations.
        </p>

        <h3>Unknown destructive deny</h3>
        <p>
          The <code>ops.unknown_destructive</code> rule denies unrecognized
          tools. Known tools: kubectl, terraform, helm, argocd. For other tools,
          add <code>"risk_tags": ["breakglass"]</code> to override.
        </p>
      </section>
    </div>
  );
}
