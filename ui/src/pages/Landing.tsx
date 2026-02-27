import "../styles/landing.css";

interface LandingProps {
  onGetStarted: () => void;
}

const mcpExample = `{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": ["--api", "https://evidra.rest"]
    }
  }
}`;

const ghActionsExample = `- name: Evaluate terraform plan
  run: |
    curl -X POST https://evidra.rest/v1/validate \\
      -H "Authorization: Bearer \$KEY" \\
      -H "Content-Type: application/json" \\
      -d @invocation.json`;

export function Landing({ onGetStarted }: LandingProps) {
  return (
    <div className="landing">
      {/* Hero */}
      <section className="hero">
        <h1>Policy guardrails for AI agents.</h1>
        <p>
          Every tool call evaluated. Every decision signed.
          Store evidence wherever you want.
        </p>
        <a
          href="#console"
          className="hero-cta"
          onClick={(e) => {
            e.preventDefault();
            onGetStarted();
          }}
        >
          Get started &rarr;
        </a>
      </section>

      {/* How it works */}
      <section className="landing-section">
        <h2>How it works</h2>
        <div className="how-it-works">
          <div className="how-step">
            <div className="how-step-number">1</div>
            <div className="how-step-title">AI Agent calls tool</div>
            <div className="how-step-desc">
              Agent sends a tool invocation before executing.
            </div>
          </div>
          <div className="how-arrow">&rarr;</div>
          <div className="how-step">
            <div className="how-step-number">2</div>
            <div className="how-step-title">Evidra evaluates policy</div>
            <div className="how-step-desc">
              OPA policy checks the operation against your rules.
            </div>
          </div>
          <div className="how-arrow">&rarr;</div>
          <div className="how-step">
            <div className="how-step-number">3</div>
            <div className="how-step-title">Signed attestation</div>
            <div className="how-step-desc">
              Returns allow/deny + cryptographically signed evidence record.
            </div>
          </div>
        </div>
        <p className="how-summary">
          Agent sends tool invocation &rarr; Evidra evaluates against OPA policy
          &rarr; returns allow/deny + signed attestation. You store it. We don't.
        </p>
      </section>

      {/* Integration examples */}
      <section className="landing-section">
        <h2>Works with your stack</h2>

        <div className="integration-card">
          <div className="integration-card-header">Claude Code / MCP</div>
          <pre><code>{mcpExample}</code></pre>
        </div>

        <div className="integration-card">
          <div className="integration-card-header">GitHub Actions</div>
          <pre><code>{ghActionsExample}</code></pre>
        </div>

        <div className="integration-grid">
          <div className="integration-card integration-card--secondary">
            <div className="integration-card-header">Cursor / Windsurf / Any MCP client</div>
            <div className="integration-card-body">
              Same MCP server. Drop-in. Zero code change.
            </div>
          </div>
          <div className="integration-card integration-card--secondary">
            <div className="integration-card-header">CLI (policy development)</div>
            <div className="integration-card-body">
              <code>$ evidra validate invocation.json</code>
              <br />
              For debugging policies and verifying attestations locally.
            </div>
          </div>
        </div>
      </section>

      {/* Use cases */}
      <section className="landing-section">
        <h2>Use cases</h2>
        <div className="use-cases">
          <div className="use-case">
            <strong>Kubernetes guardrails</strong> &mdash; AI agent deploys to
            Kubernetes. Evidra blocks deletes in production, allows in staging.
          </div>
          <div className="use-case">
            <strong>Terraform plan review</strong> &mdash; Policy evaluates blast
            radius before agent runs apply.
          </div>
          <div className="use-case">
            <strong>Audit trail</strong> &mdash; Every AI-initiated change has a
            cryptographically signed, offline-verifiable evidence record.
          </div>
        </div>

        <div className="landing-footer-cta">
          <a
            href="#console"
            className="hero-cta"
            onClick={(e) => {
              e.preventDefault();
              onGetStarted();
            }}
          >
            Get started &rarr;
          </a>
          <a href="#docs">Read docs &rarr;</a>
        </div>
      </section>

      {/* Footer */}
      <footer className="landing-footer">
        <span>Open source</span>
        <span>Apache 2.0</span>
        <a href="#docs">Policy catalog</a>
        <a href="https://github.com/vitas/evidra" target="_blank" rel="noopener noreferrer">GitHub</a>
        <span>evidra.rest</span>
      </footer>
    </div>
  );
}
