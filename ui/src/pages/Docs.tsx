import { CodeBlock } from "../components/CodeBlock";
import "../styles/docs.css";

const mcpClaudeDesktop = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-lock-mcp",
      "args": []
    }
  }
}`;

const mcpClaudeCode = `claude mcp add evidra evidra-lock-mcp`;

const mcpCursor = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-lock-mcp",
      "args": []
    }
  }
}`;

const mcpCodex = `[mcp_servers.evidra]
command = "evidra-lock-mcp"`;

const mcpCodexCustom = `[mcp_servers.evidra]
command = "evidra-lock-mcp"
args = ["--offline", "--evidence-dir", "/path/to/evidence"]`;

const mcpGemini = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-lock-mcp",
      "args": []
    }
  }
}`;

const mcpGeminiCustom = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-lock-mcp",
      "args": ["--offline", "--evidence-dir", "/path/to/evidence"]
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
  "server_id": "evidra-lock-api-1",
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
  "server_id": "evidra-lock-api-1",
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
      "k8s.protected_namespace: kube-system is a protected namespace"
    ],
    "hints": [
      "Include payload.resource and payload.name for delete operations",
      "Use a non-system namespace or add a breakglass risk_tag"
    ],
    "rule_ids": [
      "ops.insufficient_context",
      "k8s.protected_namespace"
    ]
  },
  "signing_payload": "evidra.v1\\nevent_id=evt_01J...",
  "signature": "base64..."
}`;

const signingPayloadExample = `evidra.v1
event_id=evt_01J...
timestamp=2026-02-26T14:23:01Z
server_id=evidra-lock-api-1
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
evidra-lock-mcp

# Run MCP server with deny-loop prevention
evidra-lock-mcp --deny-cache

# Inspect a stored evidence event
evidra-lock-mcp get_event --event-id evt_01J...`;

export function Docs() {
  return (
    <div className="docs">
      <h1>Documentation</h1>

      {/* MCP Setup — K1 */}
      <section className="docs-section">
        <h2>MCP Setup</h2>
        <p>
          The fastest way to use Evidra-Lock: run the MCP server locally. Your AI agent
          calls <code>validate</code> before every destructive operation. No API
          key needed for local mode.{" "}
          <a
            href="https://github.com/vitas/evidra-lock/blob/main/docs/mcp-setup.md"
            target="_blank"
            rel="noopener noreferrer"
          >
            Full MCP setup guide on GitHub
          </a>
          .
        </p>

        <h3>Install</h3>
        <CodeBlock code={`brew install samebits/tap/evidra-lock-mcp`} />
        <p>
          Or: <code>go install samebits.com/evidra/cmd/evidra-mcp@latest</code>
        </p>

        <h3>Claude Code</h3>
        <p>Run in your terminal:</p>
        <CodeBlock code={mcpClaudeCode} />

        <h3>Claude Desktop</h3>
        <p>
          Config file location (OS-specific):
        </p>
        <ul className="config-paths">
          <li><strong>macOS:</strong> <code>~/Library/Application Support/Claude/claude_desktop_config.json</code></li>
          <li><strong>Linux:</strong> <code>~/.config/Claude/claude_desktop_config.json</code></li>
          <li><strong>Windows:</strong> <code>%APPDATA%\Claude\claude_desktop_config.json</code></li>
        </ul>
        <CodeBlock code={mcpClaudeDesktop} />

        <h3>Cursor</h3>
        <p>Add to <code>.cursor/mcp.json</code>:</p>
        <CodeBlock code={mcpCursor} />

        <h3>Codex (OpenAI)</h3>
        <p>
          Codex CLI and VS Code extension share MCP config in{" "}
          <code>~/.codex/config.toml</code>.
        </p>
        <p>Quick setup (CLI):</p>
        <CodeBlock code={`codex mcp add evidra -- evidra-lock-mcp`} />
        <p>Or edit <code>~/.codex/config.toml</code> manually:</p>
        <CodeBlock code={mcpCodex} />
        <p>With custom flags:</p>
        <CodeBlock code={mcpCodexCustom} />

        <h3>Gemini CLI</h3>
        <p>
          Gemini CLI configures MCP servers in <code>~/.gemini/settings.json</code>{" "}
          (global) or <code>.gemini/settings.json</code> (project-scoped).
        </p>
        <CodeBlock code={mcpGemini} />
        <p>With custom flags:</p>
        <CodeBlock code={mcpGeminiCustom} />

        <h3>Verify</h3>
        <ol>
          <li>
            Test a <strong>deny</strong>: <em>"Validate kubectl.delete in kube-system"</em> — the
            agent should report <code>allow: false</code> and stop.
          </li>
          <li>
            Test an <strong>allow</strong>: <em>"Validate kubectl.get pods in default"</em> — the
            agent should report <code>allow: true</code>.
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
          <strong>HTTP 200 does not mean allow.</strong> Policy deny returns HTTP 200
          with <code>decision.allow: false</code>. HTTP 4xx/5xx means a request error
          (bad auth, invalid input, server failure) — not a policy deny. Always check
          the decision field:
        </div>
        <CodeBlock code={`jq '.decision.allow' evidence.json`} />

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
              <td><code>k8s.protected_namespace</code></td>
              <td>Target namespace is protected (kube-system, kube-public, etc.)</td>
              <td>Use a non-system namespace or add <code>breakglass</code> risk tag</td>
            </tr>
            <tr>
              <td><code>ops.mass_delete</code></td>
              <td>Delete count exceeds threshold (default: 5, production: 3)</td>
              <td>Reduce scope or add <code>breakglass</code> risk tag</td>
            </tr>
            <tr>
              <td><code>stop_after_deny</code></td>
              <td>Same intent already denied (agent/CI retry loop)</td>
              <td>Change plan parameters or escalate to human</td>
            </tr>
          </tbody>
        </table>
      </section>

      {/* Minimum Payload Reference — Fix 8 */}
      <section className="docs-section">
        <h2>Minimum Payload Reference</h2>
        <p>
          The <code>ops.insufficient_context</code> rule requires certain payload fields
          for destructive operations. This table shows the minimum fields needed per tool:
        </p>
        <table className="rule-table">
          <thead>
            <tr>
              <th>Kind</th>
              <th>Minimum fields</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><code>kubectl.apply</code> (workload)</td>
              <td><code>target.namespace</code> + <code>payload.containers[].image</code></td>
            </tr>
            <tr>
              <td><code>kubectl.apply</code> (non-workload)</td>
              <td><code>target.namespace</code></td>
            </tr>
            <tr>
              <td><code>kubectl.delete</code></td>
              <td><code>target.namespace</code></td>
            </tr>
            <tr>
              <td><code>terraform.apply</code></td>
              <td><code>payload.resource_types</code> (or <code>security_group_rules</code>, <code>iam_policy_statements</code>)</td>
            </tr>
            <tr>
              <td><code>terraform.destroy</code></td>
              <td><code>payload.destroy_count</code></td>
            </tr>
            <tr>
              <td><code>helm.upgrade</code> / <code>helm.uninstall</code></td>
              <td><code>target.namespace</code></td>
            </tr>
            <tr>
              <td><code>argocd.sync</code></td>
              <td><code>payload.app_name</code> or <code>payload.sync_policy</code></td>
            </tr>
          </tbody>
        </table>
      </section>

      {/* Supported Operations — Fix 10 */}
      <section className="docs-section">
        <h2>Supported Operations</h2>
        <p>
          Evidra-Lock validates destructive operations and automatically allows safe (read-only)
          operations. Custom tools can be added to <code>ops.destructive_operations</code>{" "}
          in policy <code>data.json</code>.
        </p>
        <table className="rule-table">
          <thead>
            <tr>
              <th>Tool</th>
              <th>Destructive (validated)</th>
              <th>Safe (bypass)</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><code>kubectl</code></td>
              <td>apply, delete, patch, rollout</td>
              <td>get, list, describe, show, diff</td>
            </tr>
            <tr>
              <td><code>terraform</code></td>
              <td>apply, destroy</td>
              <td>plan, show</td>
            </tr>
            <tr>
              <td><code>helm</code></td>
              <td>install, upgrade, uninstall, rollback</td>
              <td>list, status, show</td>
            </tr>
            <tr>
              <td><code>argocd</code></td>
              <td>sync, rollback, terminate-op</td>
              <td>get, list</td>
            </tr>
          </tbody>
        </table>
      </section>

      {/* CLI Usage — K6 */}
      <section className="docs-section">
        <h2>CLI Usage</h2>
        <p>
          Evidra-Lock ships three binaries: <code>evidra</code> (offline CLI),{" "}
          <code>evidra-lock-mcp</code> (MCP server), and <code>evidra-lock-api</code> (HTTP API).
        </p>
        <CodeBlock code={cliUsageExample} />
      </section>

      {/* Evidence Guide */}
      <section className="docs-section">
        <h2>Evidence Guide</h2>

        <h3>What is an evidence record?</h3>
        <p>
          Every <code>/v1/validate</code> response is a cryptographically signed
          evidence record. The API server signs it but <strong>does not store it</strong> —
          you store it wherever you want: filesystem, S3, database, git.
          There is no <code>GET /v1/evidence/&#123;event_id&#125;</code> endpoint
          on the API.
        </p>
        <p>
          The MCP server (<code>evidra-lock-mcp</code>) in offline mode writes evidence
          to a local JSONL chain and supports <code>get_event</code> for lookup.
          In online mode, the MCP server delegates to the API and does not store
          evidence locally.
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

      {/* MCP Troubleshooting — K7 + Fix 7 */}
      <section className="docs-section">
        <h2>MCP Troubleshooting</h2>

        <h3>Tool not visible in editor</h3>
        <p>
          Restart the editor/IDE completely (not just reload window).
          Verify the config file path is correct for your OS.
          Check that <code>evidra-lock-mcp --version</code> works in your terminal.
        </p>

        <h3>"command not found: evidra-lock-mcp"</h3>
        <p>
          Check that your PATH includes the install location:
        </p>
        <ul>
          <li><strong>brew:</strong> <code>/opt/homebrew/bin</code> (macOS ARM) or <code>/usr/local/bin</code> (Intel)</li>
          <li><strong>go install:</strong> <code>~/go/bin</code></li>
        </ul>
        <p>
          Run <code>which evidra-lock-mcp</code> — if empty, PATH is wrong.
        </p>

        <h3>Permission denied</h3>
        <p>
          Ensure <code>evidra-lock-mcp</code> is executable:{" "}
          <code>chmod +x $(which evidra-lock-mcp)</code>. On macOS, you may need to
          allow it in System Settings &gt; Privacy &amp; Security.
        </p>

        <h3>Deny: ops.insufficient_context</h3>
        <p>
          The policy needs more fields to evaluate. Check the{" "}
          <a href="#minimum-payload-reference">Minimum Payload Reference</a> table
          above for required fields per tool. Copy the skeleton from the hint
          in the deny response, fill in real values, and retry.
          This is the most common support question.
        </p>

        <h3>Agent doesn't call validate</h3>
        <p>
          The <code>validate</code> tool description instructs the agent to call
          it before destructive operations. If the agent skips it, remind it:
          <em> "Always call evidra validate before kubectl apply or terraform apply."</em>
        </p>

        <h3>Unknown destructive deny</h3>
        <p>
          The <code>ops.unknown_destructive</code> rule denies unrecognized
          tools. Known tools: kubectl, terraform, helm, argocd. For other tools,
          add <code>"risk_tags": ["breakglass"]</code> to override.
        </p>

        <h3>All requests return allow</h3>
        <p>
          Check that the policy bundle is loaded correctly. Run{" "}
          <code>evidra-lock-mcp --version</code> — it should show the bundle revision.
          If you use a custom <code>--bundle</code> path, verify the bundle contains{" "}
          <code>.manifest</code> and rule files. Without rules, all operations are allowed
          by default.
        </p>

        <h3>stop_after_deny (deny-loop prevention)</h3>
        <p>
          If the agent retries the same denied operation, the MCP server returns{" "}
          <code>stop_after_deny</code> immediately without re-evaluating policy. To resolve:
          change the operation parameters (namespace, image, security posture)
          or escalate to a human. Enable with <code>--deny-cache</code> or{" "}
          <code>EVIDRA_DENY_CACHE=true</code>.
        </p>

        <h3>Evidence directory permission error</h3>
        <CodeBlock code={`mkdir -p ~/.evidra && chmod 700 ~/.evidra
evidra-lock-mcp --offline --evidence-dir ~/.evidra`} />
      </section>

      <section className="docs-section">
        <h2>More Docs on GitHub</h2>
        <p>
          Full documentation set (architecture notes, release plans, and security docs):
          {" "}
          <a
            href="https://github.com/vitas/evidra-lock/tree/main/docs"
            target="_blank"
            rel="noopener noreferrer"
          >
            github.com/vitas/evidra-lock/docs
          </a>
        </p>
      </section>
    </div>
  );
}
