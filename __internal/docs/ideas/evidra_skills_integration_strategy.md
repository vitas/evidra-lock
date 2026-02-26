# Evidra Skills — Product Analysis & Integration Strategy

**Date:** 2026-02-26
**Status:** Draft
**Scope:** What skills solve, where they integrate, what to build first

---

## What Skills Actually Solve

Skills solve one concrete problem: **the client doesn't want to assemble `ToolInvocation` by hand**. Instead of knowing Evidra's internal format (actions, kind, payload, risk_tags), the client registers a template once and then sends only business parameters.

**Without skills:**
```json
{
  "actor": {"type":"agent","id":"bot","origin":"api"},
  "tool": "kubectl",
  "operation": "apply",
  "params": {
    "target": {"namespace": "prod"},
    "payload": {"containers": [...]},
    "risk_tags": ["breakglass"]
  }
}
```

**With skills:**
```json
{"namespace": "prod", "manifest": "..."}
```

Skills are an **abstraction over policy input** — an SDK without an SDK.

---

## Integration Opportunities

If you look at skills as "named operations with validation and audit trail," they plug into several environments beyond the raw API.

### 1. CI/CD Pipeline Steps (GitHub Actions, GitLab CI)

**Scenario:** A pipeline has a `terraform apply` step. Before it — `evidra evaluate`. Today the developer must assemble a ToolInvocation from `terraform plan -json` output. With skills:

```yaml
- uses: evidra/action@v1
  with:
    skill: terraform-apply
    input:
      plan_file: tfplan.json
```

The skill knows how to turn `plan_file` into the correct ToolInvocation. **A GitHub Action is a wrapper around a skill.** This is the shortest path-to-value after MCP.

### 2. Platform Engineering — Internal Developer Portals (Backstage, Port, Humanitec)

Platform teams build "golden paths" — approved ways to do things. Skill = golden path with policy check:

- **"Deploy to production"** — skill with `input_schema: {service, version, namespace}`, hardcoded risk_tags, forced `environment=prod`
- **"Create S3 bucket"** — skill with `input_schema: {name, region}`, `default_target` includes encryption and versioning

**Backstage plugin flow:** developer clicks a button → Backstage calls the skill → Evidra checks policy → proceed or block. **Skill = policy-checked action inside an internal developer platform.**

### 3. Slack/Teams Bot Commands (ChatOps)

ChatOps pattern: `/deploy myapp production`. The bot parses the command, calls skill `deploy` with input `{app: "myapp", env: "production"}`. The skill validates, checks policy, returns evidence. The bot replies: "✅ Approved, deploying" or "❌ Blocked: production requires change-approved tag."

Skills provide double value here: `input_schema` validates the bot passed sane data, and policy checks whether the operation is permitted.

### 4. Terraform Cloud / Spacelift / Atlantis — Run Task Integration

Terraform Cloud supports "run tasks" — a webhook called between plan and apply. Skill `terraform-plan-review` receives plan JSON, checks policy, returns pass/fail. This is a direct competitor to OPA + Conftest, but with an evidence chain and signature.

Spacelift and Atlantis have analogous hooks. One skill = one webhook handler.

### 5. MCP Tool Wrapping — "Every Tool = Skill"

This is closest to the current focus. Claude Code / Cursor / Windsurf call MCP tools. Each tool (kubectl, terraform, aws cli) can be wrapped in a skill. The Evidra MCP server exposes not one generic `evaluate` tool, but **N tools matching registered skills**:

```
Available tools:
  - evidra_k8s_deploy (namespace, manifest)
  - evidra_terraform_apply (plan_file)
  - evidra_s3_create (name, region)
```

The AI agent sees concrete actions with descriptions, not an abstract "evaluate policy." This dramatically improves tool use quality — LLMs choose the right tool much better when it has a specific name and schema.

**This is the strongest argument for skills.** Without skills, the MCP server exposes one generic tool. With skills, every registered operation becomes a separate tool with a name and schema. The difference between "AI has a policy checker" and "AI has type-safe infrastructure operations with built-in policy."

### 6. Argo Workflows / Temporal / Step Functions — Policy Gate

Workflow orchestrators often need an "approval gate" — a step between "plan" and "execute." A skill = an HTTP call to Evidra that returns allow/deny + signed evidence. The workflow engine checks `decision.allow` and either continues or aborts. Evidence goes into the workflow's audit log.

---

## Priority Matrix

| Integration | Effort | Impact | When | Notes |
|---|---|---|---|---|
| **MCP dynamic tools** (skills → MCP tools) | Low — MCP server already exists | High — UX breakthrough for AI agents | Phase 2, with skills launch | Main argument for skills |
| **GitHub Action** | Medium — shell wrapper | High — massive audience | Right after Phase 1 | Works on Phase 1 validate; 10× better with skills |
| **Terraform Cloud run task** | Medium — webhook endpoint | High — direct competitive answer | Phase 2 | One skill = one run task |
| **Backstage plugin** | High — TypeScript, Backstage API | Medium — narrow audience, but premium | Phase 3 | Platform engineering play |
| **ChatOps bot** (Slack/Teams) | Medium — Slack API | Medium — visual wow-effect for demos | Phase 3 | Good for marketing |
| **Workflow gates** (Argo/Temporal) | Low — just HTTP call | Medium — enterprise use case | Phase 3 | Evidence fits naturally into workflow audit |

---

## Key Insight

Skills are not just a "convenience API." They are a **distribution mechanism**. Every integration above becomes simpler with skills because the integration only needs to know `skill_id + input`, not the full ToolInvocation format.

The pattern: **register once, call from everywhere.**

- MCP server reads skills → exposes as tools
- GitHub Action reads skill name → calls execute
- Backstage reads skill schema → renders form
- Slack bot reads skill name → maps to `/command`
- Terraform Cloud sends plan → skill parses it

Without skills, each integration needs its own ToolInvocation builder. With skills, each integration is a thin adapter over the same `POST /v1/skills/{id}:execute` endpoint.
