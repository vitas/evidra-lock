import { useState, useEffect, useRef } from "react";
import { CodeBlock } from "../components/CodeBlock";
import "../styles/landing.css";

interface LandingProps {
  onGetStarted: () => void;
}

function PromptRow({
  prompt,
  result,
  rule,
  variant,
}: {
  prompt: string;
  result: string;
  rule: string;
  variant: "deny" | "allow";
}) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(prompt);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <tr className={`prompt-row prompt-row--${variant}`} onClick={handleCopy}>
      <td className="prompt-text">
        <code>{prompt}</code>
        <span className={`prompt-copy-hint${copied ? " prompt-copy-hint--copied" : ""}`}>
          {copied ? "Copied!" : "Click to copy"}
        </span>
      </td>
      <td>
        <span
          className={`badge badge--${variant === "deny" ? "danger" : "success"}`}
        >
          {result}
        </span>
      </td>
      <td className="prompt-rule">{rule}</td>
    </tr>
  );
}

export function Landing({ onGetStarted }: LandingProps) {
  const landingRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add("visible");
          }
        });
      },
      { threshold: 0.1 }
    );

    const els = landingRef.current?.querySelectorAll(".reveal");
    els?.forEach((el) => observer.observe(el));

    return () => observer.disconnect();
  }, []);

  // suppress unused warning — onGetStarted kept for programmatic nav
  void onGetStarted;

  return (
    <div className="landing" ref={landingRef}>
      {/* 1. Hero + Terminal demo (A2) */}
      <section className="hero">
        <div className="container">
          <div className="hero-label">
            your infrastructure &middot; your rules &middot; your evidence
          </div>
          <h1>
            Your AI agent is one wrong guess away from{" "}
            <span className="danger">kubectl delete --all</span>
          </h1>
          <p className="hero-sub">
            Evidra is a kill-switch for AI agents managing infrastructure.
            Blocks dangerous ops. Allows safe ones. Every decision logged.
          </p>
          <p className="hero-tagline">
            Designed for AI agents operating your infrastructure.
          </p>

          {/* Terminal moved here from terminal-section */}
          <div className="terminal hero-terminal">
            <div className="terminal-bar">
              <div className="terminal-dot"></div>
              <div className="terminal-dot"></div>
              <div className="terminal-dot"></div>
              <span className="terminal-title">
                claude-code — ~/infrastructure
              </span>
            </div>
            <div className="terminal-body">
              <div className="t-comment">
                # Agent wants to delete pods in kube-system
              </div>
              <div>
                <span className="t-prompt"></span>
                <span className="t-cmd">evidra validate</span>{" "}
                <span className="t-flag">--tool</span> kubectl{" "}
                <span className="t-flag">--op</span> delete{" "}
                <span className="t-flag">--payload</span>{" "}
                <span className="t-string">
                  {'\u0027{"namespace":"kube-system"}\u0027'}
                </span>
              </div>
              <br />
              <div>
                <span className="t-key">allow:</span>{" "}
                <span className="t-deny">false</span>
              </div>
              <div>
                <span className="t-key">risk:</span>{"   "}
                <span className="t-deny">high</span>
              </div>
              <div>
                <span className="t-key">rule:</span>{"   "}
                k8s.protected_namespace
              </div>
              <div>
                <span className="t-key">hint:</span>{"   "}
                <span className="t-string">
                  "Apply changes outside kube-system"
                </span>
              </div>
              <br />
              <div className="t-comment">
                # Agent sees deny &rarr; stops &rarr; asks user for guidance
              </div>
              <br />
              <div className="t-comment"># Same agent, safe namespace</div>
              <div>
                <span className="t-prompt"></span>
                <span className="t-cmd">evidra validate</span>{" "}
                <span className="t-flag">--tool</span> kubectl{" "}
                <span className="t-flag">--op</span> delete{" "}
                <span className="t-flag">--payload</span>{" "}
                <span className="t-string">
                  {'\u0027{"namespace":"staging"}\u0027'}
                </span>
              </div>
              <br />
              <div>
                <span className="t-key">allow:</span>{" "}
                <span className="t-allow">true</span>
              </div>
              <div>
                <span className="t-key">risk:</span>{"   "}low
              </div>
            </div>
          </div>

          <div className="hero-cta">
            <a href="#hosted-mcp" className="btn btn-primary">
              Try it now &rarr;
            </a>
            <a
              href="https://github.com/vitas/evidra"
              className="btn btn-ghost"
              target="_blank"
              rel="noopener noreferrer"
            >
              GitHub
            </a>
          </div>
        </div>
      </section>

      {/* 2. Hosted MCP config — CTA target (A4) */}
      <section id="hosted-mcp" className="hosted-section reveal">
        <div className="container">
          <h2>Add to Claude Code — 10 seconds</h2>
          <CodeBlock
            code={`claude mcp add evidra --url https://evidra.samebits.com/mcp`}
          />
          <p className="hosted-note">
            No install. Designed for AI agents operating your infrastructure.
            <br />
            <a href="#console">
              Other editors (Cursor, Claude Desktop, Codex, Gemini)
            </a>
            &nbsp;&middot;&nbsp;
            <a href="#console">Install locally for offline / CI use</a>
          </p>
          <p className="hosted-links">
            <a
              href="https://github.com/vitas/evidra"
              target="_blank"
              rel="noopener noreferrer"
            >
              GitHub
            </a>
            &nbsp;&middot;&nbsp;
            <a href="#docs">Docs</a>
            &nbsp;&middot;&nbsp;
            Apache 2.0
          </p>
        </div>
      </section>

      {/* 3. Prompts table (A3) */}
      <section className="prompts-section reveal">
        <div className="container">
          <h2>Test these in Claude Code</h2>
          <p className="prompts-intro">
            Add Evidra, then click any prompt to copy it. Paste into Claude
            Code.
          </p>
          <table className="prompts-table">
            <thead>
              <tr>
                <th>Prompt</th>
                <th>Result</th>
                <th>Rule</th>
              </tr>
            </thead>
            <tbody>
              <PromptRow
                prompt="Delete all pods in kube-system"
                result="DENY"
                rule="k8s.protected_namespace"
                variant="deny"
              />
              <PromptRow
                prompt="Create a public S3 bucket"
                result="DENY"
                rule="terraform.s3_public_access"
                variant="deny"
              />
              <PromptRow
                prompt="Run a privileged container"
                result="DENY"
                rule="k8s.privileged_container"
                variant="deny"
              />
              <PromptRow
                prompt="Open SSH to 0.0.0.0/0"
                result="DENY"
                rule="terraform.sg_open_world"
                variant="deny"
              />
              <PromptRow
                prompt="Deploy nginx to default namespace"
                result="ALLOW"
                rule="safe operation"
                variant="allow"
              />
            </tbody>
          </table>
          <p className="prompts-note">
            The last one passes.{" "}
            <strong>Evidra blocks danger, not work.</strong>
          </p>
        </div>
      </section>

      <div className="repeat-cta">
        <a href="#hosted-mcp">Add Evidra to Claude Code &rarr;</a> &middot; 10
        seconds, no install
      </div>

      {/* 4. Scenarios */}
      <section className="scenarios reveal">
        <div className="container">
          <h2>What goes wrong without a kill-switch</h2>
          <div className="scenario-grid">
            <div className="scenario">
              <div className="scenario-icon deny">&times;</div>
              <div className="scenario-desc">
                <strong>Agent deletes pods in kube-system.</strong> It needed to
                restart a service, picked the wrong namespace. DNS resolves to
                nothing. Everything goes down.
              </div>
              <div className="scenario-cmd">kubectl.delete</div>
            </div>
            <div className="scenario">
              <div className="scenario-icon deny">&times;</div>
              <div className="scenario-desc">
                <strong>Terraform apply with 47 resource deletions.</strong>{" "}
                Agent said "looks good" without understanding the plan. Three
                databases gone. No backup was recent enough.
              </div>
              <div className="scenario-cmd">terraform.apply</div>
            </div>
            <div className="scenario">
              <div className="scenario-icon deny">&times;</div>
              <div className="scenario-desc">
                <strong>Security group opened to 0.0.0.0/0 on port 22.</strong>{" "}
                The agent "fixed" connectivity by making SSH public. You find out
                from a security alert. Or worse.
              </div>
              <div className="scenario-cmd">terraform.apply</div>
            </div>
            <div className="scenario">
              <div className="scenario-icon deny">&times;</div>
              <div className="scenario-desc">
                <strong>IAM policy with Action:* Resource:*.</strong> The
                quickest way to "make it work." Also the quickest way to give the
                internet admin access to your AWS account.
              </div>
              <div className="scenario-cmd">terraform.apply</div>
            </div>
          </div>
        </div>
      </section>

      {/* 5. MCP + OPA note merged (A4) */}
      <section className="mcp-section reveal">
        <div className="container">
          <h2>Built for AI agents. Not a CLI wrapper.</h2>
          <p className="mcp-sub">
            Evidra runs as a standard MCP (Model Context Protocol) server. AI
            agents discover it automatically and call <code>validate</code>{" "}
            before destructive operations. No wrapper scripts. No custom
            plugins. No patching the agent.
          </p>
          <div className="mcp-flow">
            <div className="mcp-node">
              AI agent
              <span className="mcp-label">Claude Code, Cursor, etc.</span>
            </div>
            <div className="mcp-arrow">
              &darr; <span className="mcp-proto">MCP: validate</span>
            </div>
            <div className="mcp-node active">
              Evidra
              <span className="mcp-label">allow / deny + evidence</span>
            </div>
            <div className="mcp-arrow">
              &darr; <span className="mcp-proto">only if allowed</span>
            </div>
            <div className="mcp-node">
              kubectl &middot; terraform &middot; helm
              <span className="mcp-label">actual execution</span>
            </div>
          </div>
          <div className="mcp-points">
            <div className="mcp-point">
              <strong>Standard protocol</strong> — MCP, not a proprietary
              integration. Works with Claude Code, Cursor, and any
              MCP-compatible agent.
            </div>
            <div className="mcp-point">
              <strong>Pre-execution</strong> — policy runs before any command
              reaches your infrastructure. Not after.
            </div>
          </div>
          <p className="mcp-note">
            Not a replacement for OPA or Gatekeeper — Evidra runs before
            execution, across tools. They're complementary.
          </p>
        </div>
      </section>

      {/* 6. Levels */}
      <section className="levels-section reveal">
        <div className="container">
          <h2>Two levels. Default is maximum safety.</h2>
          <div className="levels-grid">
            <div className="level-card">
              <div className="level-name">baseline</div>
              <div className="level-tag caution">kill-switch only</div>
              <div className="level-desc">
                Blocks destructive operations with missing context. Guards
                against unknown tools. No opinions on what's "bad config."
              </div>
              <ul className="level-rules">
                <li>Empty payload on destructive op</li>
                <li>Unknown/unregistered tools</li>
                <li>Protected namespaces (kube-system)</li>
                <li>Mass deletions above threshold</li>
                <li>Truncated/incomplete data</li>
              </ul>
            </div>
            <div className="level-card">
              <div className="level-name">ops</div>
              <div className="level-tag default">
                default — full protection
              </div>
              <div className="level-desc">
                Everything in baseline plus curated ops rules targeting configs
                that cause real outages. Extensible with your own policies.
              </div>
              <ul className="level-rules">
                <li>Privileged containers</li>
                <li>Host namespace / hostPath mounts</li>
                <li>Wildcard IAM (Action:* Resource:*)</li>
                <li>Public S3 without encryption</li>
                <li>Security groups open to 0.0.0.0/0</li>
                <li>Dangerous ArgoCD sync settings</li>
                <li>Capability escalation (SYS_ADMIN)</li>
                <li>+ Extensible with your own policies</li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      {/* ---- fold ---- */}

      {/* 7. CI / GitOps */}
      <section className="ci-section reveal">
        <div className="container">
          <h2>Also works in CI and GitOps pipelines</h2>
          <p className="ci-sub">
            Same policy engine. Same evidence chain. Evidra validates Terraform
            plans, rendered manifests, or change bundles before merge — not just
            in AI workflows.
          </p>
          <div className="terminal">
            <div className="terminal-bar">
              <div className="terminal-dot"></div>
              <div className="terminal-dot"></div>
              <div className="terminal-dot"></div>
              <span className="terminal-title">
                github-actions — ci/validate
              </span>
            </div>
            <div className="terminal-body">
              <div className="t-comment">
                # Validate terraform plan in CI before merge
              </div>
              <div>
                <span className="t-prompt"></span>
                <span className="t-cmd">terraform show -json tfplan</span> |{" "}
                <span className="t-cmd">evidra validate</span>{" "}
                <span className="t-flag">--tool</span> terraform{" "}
                <span className="t-flag">--op</span> apply
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 8. Evidence */}
      <section className="evidence-section reveal">
        <div className="container">
          <h2>When something goes wrong, you'll have proof</h2>
          <p className="evidence-sub">
            3 databases deleted. Incident review starts. With Evidra, you have
            the exact decision record: who triggered it, what policy was active,
            whether it was approved or bypassed. No guessing. No "we think it
            was..."
          </p>
          <div className="evidence-grid">
            <div className="evidence-card">
              <div className="evidence-q">"Who ran this?"</div>
              <div className="evidence-a">
                actor.type &middot; actor.id &middot; actor.origin
              </div>
            </div>
            <div className="evidence-card">
              <div className="evidence-q">"Was it approved?"</div>
              <div className="evidence-a">
                policy_decision &middot; rule_ids &middot; hints
              </div>
            </div>
            <div className="evidence-card">
              <div className="evidence-q">"Was breakglass used?"</div>
              <div className="evidence-a">
                breakglass_used &middot; evidence chain
              </div>
            </div>
            <div className="evidence-card">
              <div className="evidence-q">"What policy was active?"</div>
              <div className="evidence-a">
                policy_ref &middot; bundle_revision
              </div>
            </div>
            <div className="evidence-card">
              <div className="evidence-q">"Can I verify the input?"</div>
              <div className="evidence-a">
                payload_digest &middot; target_digest
              </div>
            </div>
            <div className="evidence-card">
              <div className="evidence-q">"Is the chain intact?"</div>
              <div className="evidence-a">evidra evidence verify ✓</div>
            </div>
          </div>
        </div>
      </section>

      {/* 9. Install details — below fold */}
      <section className="how-section reveal">
        <div className="container">
          <h2>Three steps. Zero config.</h2>
          <div className="steps">
            <div className="step">
              <div className="step-num">1</div>
              <div>
                <div className="step-title">
                  Install — 30 seconds, any environment
                </div>
                <div className="step-desc">
                  Runs as an MCP server. Agents discover it automatically. Pick
                  your method:
                </div>
                <div className="step-code">
                  <span className="t-prompt"></span>go install
                  samebits.com/evidra@latest
                  <br />
                  <br />
                  <span className="t-prompt"></span>brew install evidra
                  <br />
                  <br />
                  <span className="t-prompt"></span>docker run -v
                  $(pwd):/config ghcr.io/samebits/evidra
                </div>
              </div>
            </div>
            <div className="step">
              <div className="step-num">2</div>
              <div>
                <div className="step-title">Agent calls validate</div>
                <div className="step-desc">
                  Before any destructive operation — kubectl apply, terraform
                  destroy, helm upgrade — the agent asks Evidra: "is this safe?"
                </div>
                <div className="step-code">
                  <span className="t-key">tool:</span> kubectl{" "}
                  <span className="t-key">op:</span> delete{" "}
                  <span className="t-key">payload:</span>{" "}
                  {'{namespace: "production", resource: "pod"}'}
                </div>
              </div>
            </div>
            <div className="step">
              <div className="step-num">3</div>
              <div>
                <div className="step-title">Evidra decides</div>
                <div className="step-desc">
                  Allow, deny, or "I need more context." Every decision is
                  recorded to a tamper-evident evidence chain. The agent sees
                  actionable hints on deny — not just "no."
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="landing-footer">
        <div className="container">
          <p className="footer-line">
            Open source. Apache 2.0 license. No SaaS. No telemetry.
            <br />
            Runs locally or on-prem. Your infrastructure, your rules, your
            evidence.
          </p>
          <div className="footer-links">
            <a
              href="https://github.com/vitas/evidra"
              target="_blank"
              rel="noopener noreferrer"
            >
              GitHub
            </a>
            <a href="#docs">Documentation</a>
            <a href="#docs">Policy catalog</a>
            <a
              href="https://discord.gg/evidra"
              target="_blank"
              rel="noopener noreferrer"
            >
              Discord
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
