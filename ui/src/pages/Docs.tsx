import { CodeBlock } from "../components/CodeBlock";
import "../styles/docs.css";

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
    "params": {"namespace":"default"},
    "environment": "staging"
  }'`;

const curlValidateDeny = `curl -X POST https://api.evidra.rest/v1/validate \\
  -H "Authorization: Bearer ev1_YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "actor": {"type":"agent","id":"claude"},
    "tool": "kubectl",
    "operation": "delete",
    "params": {"namespace":"kube-system"},
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
    "namespace": "default"
  },
  "environment": "production"
}`;

const validateResExample = `{
  "ok": true,
  "decision": {
    "allow": true,
    "risk_level": "low",
    "reason": "all checks passed",
    "reasons": [],
    "hints": []
  },
  "evidence_record": {
    "event_id": "evt_01J...",
    "timestamp": "2026-02-26T14:23:01Z",
    "server_id": "evidra-api-1",
    "policy_ref": "bundle://evidra/default:0.1.0",
    "actor": {"type":"agent","id":"claude"},
    "tool": "kubectl",
    "operation": "apply",
    "environment": "production",
    "input_hash": "sha256:...",
    "decision": {"allow":true,"risk_level":"low","reason":"all checks passed"},
    "signing_payload": "evidra.v1\\nversion=...",
    "signature": "base64..."
  }
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

export function Docs() {
  return (
    <div className="docs">
      <h1>Documentation</h1>

      {/* Quickstart */}
      <section className="docs-section">
        <h2>Quickstart</h2>

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

        <h3>4. Verify evidence offline</h3>
        <p>
          Evidence records are cryptographically signed with Ed25519. Verify
          with the public key — no server contact needed:
        </p>
        <CodeBlock code={curlVerify} />
      </section>

      {/* API Reference */}
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
              <td>Evaluate policy, return signed evidence record</td>
            </tr>
            <tr>
              <td><span className="method-badge method-badge--post">POST</span></td>
              <td><code>/v1/keys</code></td>
              <td>&mdash;</td>
              <td>Create API key (returns key once)</td>
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

        <p>
          Response (<code>ValidateResponse</code>). Note: deny is HTTP 200 — check{" "}
          <code>decision.allow</code>, not HTTP status.
        </p>
        <CodeBlock code={validateResExample} />
      </section>

      {/* Evidence Guide */}
      <section className="docs-section">
        <h2>Evidence Guide</h2>

        <h3>What is an evidence record?</h3>
        <p>
          Every <code>/v1/validate</code> response includes an{" "}
          <code>evidence_record</code> — a cryptographically signed attestation
          of the policy decision. The server signs it but does not store it.
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
    </div>
  );
}
