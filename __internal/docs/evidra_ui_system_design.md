# Evidra API — UI System Design

**Date:** 2026-02-26
**Status:** Technical specification
**Stack:** React 18 + TypeScript + Vite + Vitest + Playwright (same as evidra-gitops)

---

## 1. What the UI Does

Three pages. No more for MVP.

| Page | URL | Auth | Purpose |
|---|---|---|---|
| **Landing** | `/` | — | Product page: what Evidra does, MCP integration examples, use cases, CTA to console. |
| **Console** | `/#console` | — | Get API key. One form, one button. Quick start curl. |
| **Dashboard** | `/#dashboard` | API key in localStorage/sessionStorage | Test validate endpoint, see recent activity, public key. |
| **Docs** | `/#docs` | — | Static — quickstart, API reference, evidence guide. |

The UI is embedded in the `evidra-api` Go binary via `//go:embed`. No separate deployment. `GET /` serves the SPA, `GET /v1/*` serves the API.

---

## 2. Pages

### 2.1 Landing Page (`/`)

**Goal:** Developer or decision-maker lands here and understands what Evidra does in 10 seconds. Clear path to "Get started".

**Layout:**
```
┌─────────────────────────────────────────────────────┐
│  Evidra                Console  Dashboard  Docs      │
├─────────────────────────────────────────────────────┤
│                                                      │
│   Policy guardrails for AI agents.                   │
│   Every tool call evaluated. Every decision signed.  │
│   Store evidence wherever you want.                  │
│                                                      │
│   [ Get started → ]                                  │
│                                                      │
├─────────────────────────────────────────────────────┤
│                                                      │
│   HOW IT WORKS                                       │
│                                                      │
│   ┌─────────┐    ┌─────────┐    ┌──────────┐        │
│   │ AI Agent│ →  │ Evidra  │ →  │ Signed   │        │
│   │ calls   │    │ evaluate│    │ attesta- │        │
│   │ tool    │    │ policy  │    │ tion     │        │
│   └─────────┘    └─────────┘    └──────────┘        │
│                                                      │
│   Agent sends tool invocation → Evidra evaluates     │
│   against OPA policy → returns allow/deny + signed   │
│   attestation. You store it. We don't.               │
│                                                      │
├─────────────────────────────────────────────────────┤
│                                                      │
│   WORKS WITH YOUR STACK                              │
│                                                      │
│   ┌─ Claude Code / MCP ──────────────────────────┐  │
│   │ // MCP server config                          │  │
│   │ {                                             │  │
│   │   "mcpServers": {                             │  │
│   │     "evidra": {                               │  │
│   │       "command": "evidra-mcp",                │  │
│   │       "args": ["--api", "https://evidra.rest"]  │  │
│   │     }                                         │  │
│   │   }                                           │  │
│   │ }                                             │  │
│   └───────────────────────────────────────────────┘  │
│                                                      │
│   ┌─ GitHub Actions ─────────────────────────────┐  │
│   │ - name: Evaluate terraform plan               │  │
│   │   run: |                                      │  │
│   │     curl -X POST https://evidra.rest/...  │  │
│   │     -H "Authorization: Bearer $KEY"           │  │
│   │     -d @invocation.json                       │  │
│   └───────────────────────────────────────────────┘  │
│                                                      │
│   ┌─ Cursor / Windsurf / Any MCP client ─────────┐  │
│   │ Same MCP server. Drop-in. Zero code change.   │  │
│   └───────────────────────────────────────────────┘  │
│                                                      │
│   ┌─ CLI (policy development) ────────────────────┐  │
│   │ $ evidra validate invocation.json              │  │
│   │ For debugging policies and verifying           │  │
│   │ attestations locally.                          │  │
│   └───────────────────────────────────────────────┘  │
│                                                      │
├─────────────────────────────────────────────────────┤
│                                                      │
│   USE CASES                                          │
│                                                      │
│   • AI agent deploys to Kubernetes — Evidra blocks   │
│     deletes in production, allows in staging.        │
│                                                      │
│   • Terraform plan review — policy evaluates blast   │
│     radius before agent runs apply.                  │
│                                                      │
│   • Audit trail — every AI-initiated change has a    │
│     cryptographically signed, offline-verifiable     │
│     evidence record.                                 │
│                                                      │
│   [ Get started → ]          [ Read docs → ]         │
│                                                      │
├─────────────────────────────────────────────────────┤
│  Open source  │  Apache 2.0  │  Policy catalog  │   │
│  GitHub  │  evidra.rest                              │
└─────────────────────────────────────────────────────┘
```

**Content is static.** No API calls. Pure TSX. "Get started" button navigates to `/#console`.

**Sections:**
1. Hero — one-liner + CTA
2. How it works — 3-step diagram (agent → evaluate → attestation)
3. Integration examples — MCP config (primary, full width), GitHub Actions (primary, full width), Cursor/Windsurf (secondary), CLI (secondary, smaller card — "policy development" positioning)
4. Use cases — 3 concrete scenarios
5. Footer — links to GitHub, docs, license, policy catalog (links to published OPA bundle or docs page listing available rules)

---

### 2.2 Console (`/#console`)

**Goal:** Developer gets an API key in 10 seconds, copies it, starts using the API.

**Early beta note:** In private beta, `POST /v1/keys` can be disabled via `EVIDRA_SELF_SERVE_KEYS=false` env var. When disabled, Console shows "Request access" mailto link instead of the form. Keys are issued manually via CLI. This avoids exposing a key-minting endpoint before rate limiting and abuse detection are battle-tested.

**Layout:**
```
┌─────────────────────────────────────────────────┐
│  Evidra              Console  Dashboard  Docs    │
├─────────────────────────────────────────────────┤
│                                                  │
│   Policy evaluation for AI agents.               │
│   Get a signed evidence record for every          │
│   decision. Store it wherever you want.            │
│                                                  │
│   ┌──────────────────────────┐  ┌──────────┐    │
│   │ Label (optional)         │  │ Get Key  │    │
│   └──────────────────────────┘  └──────────┘    │
│                                                  │
│   ┌─────────────────────────────────────────┐    │
│   │ ev1_a8Fk3mQ9x...                  📋   │    │
│   │                                         │    │
│   │ Save this key — it won't be shown again │    │
│   └─────────────────────────────────────────┘    │
│                                                  │
│   Quick start:                                   │
│   curl -X POST https://evidra.rest/v1/valid.. │
│                                                  │
├─────────────────────────────────────────────────┤
│  How it works   │   API Reference   │   GitHub   │
└─────────────────────────────────────────────────┘
```

**Behavior:**
1. User enters optional label, clicks "Get Key"
2. `POST /v1/keys {label}` → response `{key, prefix, tenant_id}`
3. Show key in a highlighted box with copy button
4. Show curl example pre-filled with the key
5. Key box has yellow/orange background — "save this, shown once"
6. After page refresh, key box is gone (key is NOT stored)

**State:** Minimal. No auth context. Just one API call.

### 2.3 Dashboard (`/#dashboard`)

**Goal:** Developer pastes their API key, sees usage stats, can test the validate endpoint interactively.

**Layout:**
```
┌──────────────────────────────────────────────────┐
│  Evidra                Dashboard    Docs    Key ▾ │
├──────────────────────────────────────────────────┤
│                                                   │
│  ┌─ API Key ─────────────────────────────────┐   │
│  │ ev1_a8Fk****                    Change ▾  │   │
│  │ ☐ Forget key on tab close                 │   │
│  │ ⚠ Key stored in browser. Not for shared   │   │
│  │   computers.                              │   │
│  └───────────────────────────────────────────┘   │
│                                                   │
│  ┌─ Try Validate ────────────────────────────┐   │
│  │  [Simple]  [Advanced/JSON]                 │   │
│  │                                            │   │
│  │ Simple mode:                               │   │
│  │ Tool:      [kubectl    ▾]                  │   │
│  │ Operation: [apply      ▾]                  │   │
│  │ Namespace: [default       ]                │   │
│  │ Actor:     [agent:claude  ]                │   │
│  │ Env:       [production    ]                │   │
│  │ + Add param                                │   │
│  │                                            │   │
│  │ Advanced mode (when toggled):              │   │
│  │ ┌──────────────────────────────────┐       │   │
│  │ │ { "tool": "kubectl", ...        │       │   │
│  │ │ }                    ✓ valid JSON│       │   │
│  │ └──────────────────────────────────┘       │   │
│  │                                            │   │
│  │            [  Evaluate  ]                  │   │
│  │                                            │   │
│  │ Result:  ✅ allow  │  risk: low            │   │
│  │                                            │   │
│  │ Evidence Record:                           │   │
│  │ ┌──────────────────────────────────┐       │   │
│  │ │ { "event_id": "evt_01J...",      │  📋   │   │
│  │ │   "decision": { "allow": true }, │       │   │
│  │ │   "signature": "base64..."       │       │   │
│  │ │ }                                │       │   │
│  │ └──────────────────────────────────┘       │   │
│  └────────────────────────────────────────────┘   │
│                                                   │
│  ┌─ Recent Evaluations ──────────────────────┐   │
│  │ Time       Tool     Op      Decision      │   │
│  │ 14:23:01   kubectl  apply   ✅ allow      │   │
│  │ 14:22:45   kubectl  delete  ❌ deny       │   │
│  │ 14:20:12   terraform plan   ✅ allow      │   │
│  └────────────────────────────────────────────┘   │
│                                                   │
│  ┌─ Public Key ──────────────────────────────┐   │
│  │ -----BEGIN PUBLIC KEY-----           📋   │   │
│  │ MCowBQYDK2VwAyEA...                      │   │
│  │ -----END PUBLIC KEY-----                  │   │
│  └────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────┘
```

**Behavior:**
1. On first visit, prompt for API key (paste input). Store per storage mode (see API Key Storage above).
2. "Forget key on tab close" toggle below key input. Warning text always visible.
3. **Validate form** has two modes, toggled by a tab/switch:
   - **Simple mode** (default): form fields — Tool, Operation, Namespace, Actor, Environment. Builds a `ToolInvocation` from form values.
   - **Advanced mode**: raw JSON editor (textarea with monospace font, syntax-error highlighting). Developer pastes or edits full `ToolInvocation` JSON. Validate JSON on blur, show inline parse errors.
4. `POST /v1/validate` with Bearer token → show result.
5. Result display: allow/deny badge, risk level, reason text, full evidence record JSON (collapsible).
6. "Recent Evaluations" — client-side list of evaluations done in this session (stored in React state, not persistent).
7. "Public Key" — fetched from `GET /v1/evidence/pubkey` on page load.
8. "Change" button for API key — clears storage, prompts again.

**Simple mode fields:**
| Field | Type | Default | Notes |
|---|---|---|---|
| Tool | select | `kubectl` | Options: kubectl, terraform, helm, custom (free text) |
| Operation | select | `apply` | Options depend on tool, or free text |
| Namespace | text | `default` | Free text |
| Actor | text | `agent:claude` | Free text, format `type:id` |
| Environment | text | `production` | Free text |
| Extra params | key-value pairs | — | Optional, "Add param" button. Becomes `params` object. |

**Advanced mode:**
```
┌─ JSON Editor ─────────────────────────────────┐
│ {                                              │
│   "actor": {"type":"agent","id":"claude",      │
│             "origin":"claude-code"},            │
│   "tool": "kubectl",                           │
│   "operation": "apply",                        │
│   "params": {"namespace":"default"},           │
│   "environment": "production"                  │
│ }                                              │
│                                ⚠ valid JSON ✓  │
└────────────────────────────────────────────────┘
```

---

## 2.5 Error States & Edge Cases

Every API interaction has three visual states: **loading**, **success**, **error**.

### Error types and UI response

| Error | HTTP | UI behavior |
|---|---|---|
| **Network error** | — | Inline error: "Cannot reach API server. Check that evidra-api is running." Retry button. |
| **Auth failure** | 401 | Inline error: "Invalid API key." Clear key button + re-prompt. Do NOT auto-clear — user may have a typo and want to retry. |
| **Rate limited** | 429 | Inline error: "Too many requests. Retry in {retry-after} seconds." Disabled button with countdown. |
| **Validation error** | 400 | Inline error: show `error.message` from response. If `error.details` contains field names (e.g. `{"fields":{"namespace":"required"}}`), highlight those fields with red border + per-field error text. `ApiError.details` carries this data. |
| **Server error** | 500 | Inline error: "Server error. Check evidra-api logs." Retry button. |
| **Timeout** | — | After 10s: "Request timed out." Retry button. |

### Error display pattern

Errors are shown **inline** next to the action that caused them, not as toasts or modals. This is a dev tool — the developer needs to see the error in context.

```
┌─ Validate Form ─────────────────────────────┐
│  [Tool: kubectl]  [Op: apply]               │
│  [Namespace: kube-system]                   │
│                                             │
│  [ Evaluate ]                               │
│                                             │
│  ┌─ error ────────────────────────────────┐ │
│  │ ❌ 401 — Invalid API key               │ │
│  │ The provided key was not recognized.    │ │
│  │ [Change key]  [Retry]                   │ │
│  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

### Retry behavior

- Network errors and 5xx: show "Retry" button. No auto-retry.
- 401: show "Change key" + "Retry" buttons.
- 429: disable "Evaluate" button, show countdown timer based on `Retry-After` header (default 60s).
- 400: no retry button — fix the input.

### Loading state

- "Evaluate" button shows spinner and disables during request.
- Public key section shows skeleton/placeholder while loading.
- No full-page loading screens — each section loads independently.

**API Key Storage:**
- Default: `localStorage` — persists across tabs and sessions.
- Option: "Forget key on tab close" toggle → switches to `sessionStorage`. Key cleared when last tab closes.
- UI always shows a warning below the key input:
  ```
  ⚠ API key stored in browser storage. Do not use on shared computers.
  ```
- "Clear key" button always visible in header when key is set.
- On key clear: remove from both `localStorage` and `sessionStorage`, redirect to key prompt.

**State:** Recent evaluations in React state (lost on refresh — intentional, they're just for the interactive testing UX).

### 2.4 Docs Page (`/#docs`)

Static content. Three sections:

1. **Quickstart** — curl examples for get key, validate (allow), validate (deny), verify evidence
2. **API Reference** — endpoint table with request/response shapes (condensed from architecture doc §2)
3. **Evidence Guide** — how to store evidence, verify offline, signing payload format

Can be hardcoded TSX or rendered from markdown strings. No build-time markdown processing needed for MVP.

---

## 3. Project Structure

```
ui/
├── package.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts
├── index.html
├── public/
│   └── favicon.svg
├── src/
│   ├── main.tsx                    # React root
│   ├── App.tsx                     # Router: /, /dashboard, /docs
│   ├── api/
│   │   └── client.ts              # fetch wrapper: baseUrl, auth header, error handling
│   ├── components/
│   │   ├── Layout.tsx              # Header + nav + content shell
│   │   ├── CopyButton.tsx          # Copy to clipboard with feedback
│   │   ├── CodeBlock.tsx           # Syntax-highlighted JSON/curl display
│   │   ├── Badge.tsx               # Allow/Deny/Risk level badge
│   │   ├── KeyPrompt.tsx           # API key input dialog + ephemeral toggle
│   │   ├── InlineError.tsx         # Error display with retry/action buttons
│   │   ├── JsonEditor.tsx          # Textarea with JSON validation on blur
│   │   └── KeyValueEditor.tsx      # Dynamic key-value pairs for extra params
│   ├── pages/
│   │   ├── Landing.tsx             # Product page: hero, integrations, use cases
│   │   ├── Console.tsx             # GET /v1/keys form (was "Landing" in earlier drafts)
│   │   ├── Dashboard.tsx           # Try Validate + Recent + PubKey
│   │   └── Docs.tsx                # Static documentation
│   ├── hooks/
│   │   ├── useApiKey.ts            # get/set/clear API key, ephemeral toggle (localStorage ↔ sessionStorage)
│   │   └── useApi.ts               # Generic fetch hook with loading/error/retry state
│   ├── styles/
│   │   ├── global.css              # CSS variables, reset, typography
│   │   ├── landing.css             # Product page: hero, integration cards, use cases
│   │   ├── console.css             # Get API key form
│   │   ├── dashboard.css
│   │   └── docs.css
│   └── types/
│       └── api.ts                  # TypeScript types matching API response shapes
├── test/
│   ├── components/
│   │   ├── CopyButton.test.tsx
│   │   ├── Badge.test.tsx
│   │   ├── KeyPrompt.test.tsx
│   │   ├── InlineError.test.tsx
│   │   └── JsonEditor.test.tsx
│   ├── pages/
│   │   ├── Landing.test.tsx
│   │   ├── Console.test.tsx
│   │   └── Dashboard.test.tsx
│   ├── hooks/
│   │   └── useApiKey.test.ts
│   └── setup.ts                    # Vitest setup: jsdom, testing-library matchers
├── e2e/
│   ├── landing.spec.ts             # Playwright: product page renders, CTA works
│   ├── console.spec.ts             # Playwright: get key flow
│   ├── dashboard.spec.ts           # Playwright: validate flow, copy evidence
│   └── playwright.config.ts
└── CLAUDE.md                       # UI module instructions for Claude Code
```

---

## 4. TypeScript Types

```typescript
// src/types/api.ts

export interface Actor {
  type: string;
  id: string;
  origin?: string;
}

export interface ToolInvocation {
  actor: Actor;
  tool: string;
  operation: string;
  params: Record<string, unknown>;
  context?: Record<string, unknown>;
  environment?: string;
}

export interface PolicyDecision {
  allow: boolean;
  risk_level: "low" | "medium" | "high";
  reason: string;
  reasons?: string[];
  hints?: string[];
  rule_ids?: string[];
}

export interface EvidenceRecord {
  event_id: string;
  timestamp: string;
  server_id: string;
  policy_ref: string;
  actor: Actor;
  tool: string;
  operation: string;
  environment: string;
  input_hash: string;
  decision: PolicyDecision;
  signing_payload: string;
  signature: string;
}

export interface ValidateResponse {
  ok: boolean;
  decision: PolicyDecision;
  evidence_record: EvidenceRecord;
  // event_id lives inside evidence_record only — no top-level duplicate.
  // NOTE: Hosted validate ALWAYS returns evidence_record (signed attestation).
  // The server generates and signs, but does NOT store — the client owns the record.
  // There is no "decision-only" mode in the hosted API.
}

export interface KeyResponse {
  key: string;
  prefix: string;
  tenant_id: string;
}

export interface ErrorResponse {
  ok: false;
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
}
```

---

## 5. API Client

```typescript
// src/api/client.ts

const BASE_URL = import.meta.env.VITE_API_URL || "";

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public details?: Record<string, unknown>,
  ) {
    super(message);
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {},
  apiKey?: string,
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) || {}),
  };

  if (apiKey) {
    headers["Authorization"] = `Bearer ${apiKey}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  const data = await res.json();

  if (!res.ok || data.ok === false) {
    throw new ApiError(
      res.status,
      data.error?.code || "unknown",
      data.error?.message || res.statusText,
      data.error?.details,
    );
  }

  return data as T;
}

export function createKey(label?: string) {
  return request<KeyResponse>("/v1/keys", {
    method: "POST",
    body: JSON.stringify({ label: label || undefined }),
  });
}

export function validate(invocation: ToolInvocation, apiKey: string) {
  return request<ValidateResponse>(
    "/v1/validate",
    { method: "POST", body: JSON.stringify(invocation) },
    apiKey,
  );
}

export function getPublicKey() {
  return request<{ pem: string }>("/v1/evidence/pubkey");
}
```

---

## 6. Routing

No React Router needed for 4 pages. Hash-based routing:

```typescript
// src/App.tsx

import { useState } from "react";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { Console } from "./pages/Console";
import { Dashboard } from "./pages/Dashboard";
import { Docs } from "./pages/Docs";

type Page = "landing" | "console" | "dashboard" | "docs";

export function App() {
  const [page, setPage] = useState<Page>(() => {
    const hash = window.location.hash.slice(1);
    if (hash === "console") return "console";
    if (hash === "dashboard") return "dashboard";
    if (hash === "docs") return "docs";
    return "landing";
  });

  const navigate = (p: Page) => {
    window.location.hash = p === "landing" ? "" : p;
    setPage(p);
  };

  return (
    <Layout currentPage={page} onNavigate={navigate}>
      {page === "landing" && <Landing onGetStarted={() => navigate("console")} />}
      {page === "console" && <Console onKeyCreated={() => navigate("dashboard")} />}
      {page === "dashboard" && <Dashboard />}
      {page === "docs" && <Docs />}
    </Layout>
  );
}
```

**Routing evolution path:** Hash routing is acceptable for MVP (3 pages, dev tool, embedded SPA). If the UI becomes a public-facing entry point with docs that need SEO, migrate to History API (`pushState`) with Go-side SPA fallback already implemented in `uiHandler()`. The SPA fallback handler is ready — only the JS-side routing needs to change.

---

## 7. Embedding in Go Binary

The built UI is embedded in the API server:

```go
// cmd/evidra-api/ui.go

//go:embed ui/dist/*
var uiFS embed.FS

func uiHandler() http.Handler {
    sub, _ := fs.Sub(uiFS, "ui/dist")
    fileServer := http.FileServer(http.FS(sub))
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // fs.Sub expects paths without leading "/".
        path := strings.TrimPrefix(r.URL.Path, "/")
        if path == "" {
            path = "index.html"
        }
        f, err := sub.Open(path)
        if err != nil {
            // SPA fallback: serve index.html for unknown paths (client-side routing).
            r.URL.Path = "/"
            fileServer.ServeHTTP(w, r)
            return
        }
        f.Close()
        fileServer.ServeHTTP(w, r)
    })
}
```

Router priority: `/v1/*` → API handlers, everything else → embedded UI.

Build pipeline:
```bash
cd ui && npm run build    # → ui/dist/
cd .. && go build ./cmd/evidra-api  # embeds ui/dist/
```

---

## 8. Styling

Pure CSS, no framework. CSS custom properties for theming. Matches evidra-gitops approach (no Tailwind, no MUI).

```css
/* src/styles/global.css */

:root {
  --color-bg: #0f1117;
  --color-surface: #1a1d27;
  --color-surface-hover: #242836;
  --color-border: #2e3345;
  --color-text: #e1e4ed;
  --color-text-muted: #8b91a5;
  --color-primary: #6366f1;
  --color-primary-hover: #818cf8;
  --color-success: #22c55e;
  --color-danger: #ef4444;
  --color-warning: #f59e0b;
  --color-code-bg: #12141d;

  --radius: 8px;
  --font-mono: "JetBrains Mono", "Fira Code", monospace;
  --font-sans: "Inter", -apple-system, sans-serif;

  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 16px;
  --space-lg: 24px;
  --space-xl: 40px;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: var(--font-sans);
  background: var(--color-bg);
  color: var(--color-text);
  line-height: 1.6;
}

code, pre {
  font-family: var(--font-mono);
}
```

Dark theme by default. Professional, developer-focused. No gradients, no illustrations — clean information density.

### Content Security Policy

Since the UI is embedded via `go:embed` and served from the same origin as the API, CSP is straightforward. Set via Go middleware on all UI responses (not on `/v1/*` API routes):

```
Content-Security-Policy:
  default-src 'self';
  script-src 'self';
  style-src 'self' 'unsafe-inline';
  img-src 'self' data:;
  font-src 'self';
  connect-src 'self';
  frame-ancestors 'none';
  base-uri 'self';
  form-action 'self'
```

Notes:
- `'unsafe-inline'` for styles — **temporary, verify after first production build.** Vite production builds typically extract CSS to files (no inline styles), unlike dev/HMR mode. After build, test with `style-src 'self'` only. If no violations in browser console → remove `'unsafe-inline'`. Keep this TODO in implementation backlog.
- No `'unsafe-eval'` — no eval/Function needed.
- `connect-src 'self'` — all API calls are same-origin.
- `frame-ancestors 'none'` — equivalent to X-Frame-Options: DENY but CSP-native.
- Traefik adds HSTS, X-Content-Type-Options, X-Frame-Options on top (see deployment doc). CSP is the defense-in-depth layer.

---

## 9. Tests

### 9.1 Unit Tests (Vitest + Testing Library)

```typescript
// test/setup.ts
import "@testing-library/jest-dom/vitest";

// test/components/CopyButton.test.tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { CopyButton } from "../../src/components/CopyButton";

describe("CopyButton", () => {
  it("copies text to clipboard on click", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });

    render(<CopyButton text="ev1_test_key" />);
    await userEvent.click(screen.getByRole("button"));

    expect(writeText).toHaveBeenCalledWith("ev1_test_key");
  });

  it("shows feedback after copy", async () => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });

    render(<CopyButton text="test" />);
    await userEvent.click(screen.getByRole("button"));

    expect(screen.getByText(/copied/i)).toBeInTheDocument();
  });
});

// test/pages/Landing.test.tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { Landing } from "../../src/pages/Landing";

describe("Landing (product page)", () => {
  it("renders hero with product description", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.getByText(/policy/i)).toBeInTheDocument();
  });

  it("shows MCP integration example", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    expect(screen.getByText(/evidra-mcp/i)).toBeInTheDocument();
  });

  it("shows GitHub Actions integration example", () => {
    render(<Landing onGetStarted={vi.fn()} />);
    expect(screen.getByText(/github actions/i)).toBeInTheDocument();
  });

  it("Get started button calls onGetStarted", async () => {
    const onGetStarted = vi.fn();
    render(<Landing onGetStarted={onGetStarted} />);
    await userEvent.click(screen.getByRole("link", { name: /get started/i }));
    expect(onGetStarted).toHaveBeenCalledOnce();
  });
});

// test/pages/Console.test.tsx
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { Console } from "../../src/pages/Console";

describe("Console", () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  it("shows key after successful creation", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          ok: true,
          key: "ev1_testkey123456789012345678901234567890",
          prefix: "ev1_testkey1",
          tenant_id: "01J...",
        }),
    });

    render(<Console onKeyCreated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /get key/i }));

    await waitFor(() => {
      expect(screen.getByText(/ev1_testkey/)).toBeInTheDocument();
    });
  });

  it("shows error on rate limit", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 429,
      statusText: "Too Many Requests",
      json: () =>
        Promise.resolve({
          ok: false,
          error: { code: "rate_limited", message: "Too many requests" },
        }),
    });

    render(<Console onKeyCreated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /get key/i }));

    await waitFor(() => {
      expect(screen.getByText(/too many/i)).toBeInTheDocument();
    });
  });

  it("does not store key anywhere after showing", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          ok: true,
          key: "ev1_secret",
          prefix: "ev1_secr",
          tenant_id: "t1",
        }),
    });

    render(<Console onKeyCreated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /get key/i }));

    expect(localStorage.getItem("evidra_api_key")).toBeNull();
  });
});

// test/hooks/useApiKey.test.ts
import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, beforeEach } from "vitest";
import { useApiKey } from "../../src/hooks/useApiKey";

describe("useApiKey", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it("returns null when no key stored", () => {
    const { result } = renderHook(() => useApiKey());
    expect(result.current.apiKey).toBeNull();
  });

  it("persists key to localStorage by default", () => {
    const { result } = renderHook(() => useApiKey());
    act(() => result.current.setApiKey("ev1_test"));
    expect(localStorage.getItem("evidra_api_key")).toBe("ev1_test");
    expect(result.current.apiKey).toBe("ev1_test");
  });

  it("uses sessionStorage when ephemeral mode enabled", () => {
    const { result } = renderHook(() => useApiKey());
    act(() => result.current.setEphemeral(true));
    act(() => result.current.setApiKey("ev1_session"));
    expect(sessionStorage.getItem("evidra_api_key")).toBe("ev1_session");
    expect(localStorage.getItem("evidra_api_key")).toBeNull();
  });

  it("clears key from both storages", () => {
    localStorage.setItem("evidra_api_key", "ev1_old");
    sessionStorage.setItem("evidra_api_key", "ev1_old");
    const { result } = renderHook(() => useApiKey());
    act(() => result.current.clearApiKey());
    expect(result.current.apiKey).toBeNull();
    expect(localStorage.getItem("evidra_api_key")).toBeNull();
    expect(sessionStorage.getItem("evidra_api_key")).toBeNull();
  });
});
// test/components/InlineError.test.tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { InlineError } from "../../src/components/InlineError";

describe("InlineError", () => {
  it("shows error message", () => {
    render(<InlineError message="Invalid API key" />);
    expect(screen.getByText(/invalid api key/i)).toBeInTheDocument();
  });

  it("shows retry button when onRetry provided", () => {
    render(<InlineError message="Server error" onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("hides retry button when not provided", () => {
    render(<InlineError message="Bad input" />);
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });

  it("calls onRetry on click", async () => {
    const onRetry = vi.fn();
    render(<InlineError message="Error" onRetry={onRetry} />);
    await userEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("shows custom action button", async () => {
    const onAction = vi.fn();
    render(<InlineError message="401" action={{ label: "Change key", onClick: onAction }} />);
    await userEvent.click(screen.getByRole("button", { name: /change key/i }));
    expect(onAction).toHaveBeenCalledOnce();
  });
});

// test/components/JsonEditor.test.tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { JsonEditor } from "../../src/components/JsonEditor";

describe("JsonEditor", () => {
  it("renders textarea with initial value", () => {
    render(<JsonEditor value='{"tool":"kubectl"}' onChange={vi.fn()} />);
    expect(screen.getByRole("textbox")).toHaveValue('{"tool":"kubectl"}');
  });

  it("shows valid indicator for valid JSON on blur", () => {
    render(<JsonEditor value='{"tool":"kubectl"}' onChange={vi.fn()} />);
    fireEvent.blur(screen.getByRole("textbox"));
    expect(screen.getByText(/valid/i)).toBeInTheDocument();
  });

  it("shows error indicator for invalid JSON on blur", () => {
    const onChange = vi.fn();
    render(<JsonEditor value="{broken" onChange={onChange} />);
    fireEvent.blur(screen.getByRole("textbox"));
    expect(screen.getByText(/invalid/i)).toBeInTheDocument();
  });

  it("calls onChange with parsed object for valid JSON", () => {
    const onChange = vi.fn();
    render(<JsonEditor value="{}" onChange={onChange} />);
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: '{"tool":"helm"}' } });
    fireEvent.blur(textarea);
    expect(onChange).toHaveBeenCalledWith({ tool: "helm" }, true);
  });

  it("calls onChange with null for invalid JSON", () => {
    const onChange = vi.fn();
    render(<JsonEditor value="{}" onChange={onChange} />);
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "{bad" } });
    fireEvent.blur(textarea);
    expect(onChange).toHaveBeenCalledWith(null, false);
  });
});
```

### 9.2 E2E Tests (Playwright)

```typescript
// e2e/playwright.config.ts
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  webServer: {
    command: "npm run build && npm run preview",
    port: 4173,
    reuseExistingServer: !process.env.CI,
  },
  use: {
    baseURL: "http://localhost:4173",
  },
});

// e2e/landing.spec.ts
import { test, expect } from "@playwright/test";

test.describe("Landing (product page)", () => {
  test("shows hero and get started CTA", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByRole("link", { name: /get started/i })).toBeVisible();
  });

  test("shows MCP integration example", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByText(/evidra-mcp/)).toBeVisible();
  });

  test("shows GitHub Actions integration example", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByText(/github actions/i)).toBeVisible();
  });

  test("Get started navigates to console", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: /get started/i }).first().click();
    await expect(page).toHaveURL(/#console/);
    await expect(page.getByRole("button", { name: /get key/i })).toBeVisible();
  });
});

// e2e/console.spec.ts
import { test, expect } from "@playwright/test";

test.describe("Console (get API key)", () => {
  test("shows get key form", async ({ page }) => {
    await page.goto("/#console");
    await expect(page.getByRole("button", { name: /get key/i })).toBeVisible();
  });

  test("get key flow — shows key and curl example", async ({ page }) => {
    await page.route("**/v1/keys", (route) =>
      route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          key: "ev1_mockedkey1234567890123456789012345678",
          prefix: "ev1_mockedke",
          tenant_id: "01JTEST",
        }),
      }),
    );

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();

    await expect(page.getByText("ev1_mockedkey")).toBeVisible();
    await expect(page.getByText("curl")).toBeVisible();
    await expect(page.getByText(/won.*shown again/i)).toBeVisible();
  });

  test("copy button copies key to clipboard", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);

    await page.route("**/v1/keys", (route) =>
      route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          key: "ev1_clipboardtest12345678901234567890123",
          prefix: "ev1_clipboar",
          tenant_id: "01JTEST",
        }),
      }),
    );

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();
    await page.getByRole("button", { name: /copy/i }).first().click();

    const clipboard = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboard).toContain("ev1_clipboardtest");
  });

  test("shows error on network failure", async ({ page }) => {
    await page.route("**/v1/keys", (route) => route.abort("connectionrefused"));

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();

    await expect(page.getByText(/cannot reach/i)).toBeVisible();
  });

  test("shows error on rate limit", async ({ page }) => {
    await page.route("**/v1/keys", (route) =>
      route.fulfill({
        status: 429,
        contentType: "application/json",
        body: JSON.stringify({
          ok: false,
          error: { code: "rate_limited", message: "Too many requests" },
        }),
      }),
    );

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();

    await expect(page.getByText(/too many/i)).toBeVisible();
  });
});

// e2e/dashboard.spec.ts
import { test, expect } from "@playwright/test";

/** Minimal valid EvidenceRecord for mocks — matches all required fields in the TS type. */
function mockEvidence(overrides: Record<string, unknown> = {}) {
  return {
    event_id: "evt_01JTEST",
    timestamp: "2026-02-26T14:23:01Z",
    server_id: "srv_test",
    policy_ref: "bundle://evidra/default:0.1.0",
    actor: { type: "agent", id: "claude" },
    tool: "kubectl",
    operation: "apply",
    environment: "production",
    input_hash: "sha256:abcdef1234567890",
    decision: { allow: true, risk_level: "low", reason: "all checks passed" },
    signing_payload: "test-payload",
    signature: "base64-test-signature",
    ...overrides,
  };
}

test.describe("Dashboard", () => {
  test.beforeEach(async ({ page }) => {
    // Set API key in localStorage before navigating
    await page.goto("/");
    await page.evaluate(() =>
      localStorage.setItem("evidra_api_key", "ev1_testdashboardkey12345678901234567"),
    );
    await page.goto("/#dashboard");
  });

  test("shows validate form with simple/advanced toggle", async ({ page }) => {
    await expect(page.getByText(/try validate/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /evaluate/i })).toBeVisible();
    // Simple mode visible by default
    await expect(page.getByLabel(/tool/i)).toBeVisible();
    // Advanced tab exists
    await expect(page.getByRole("tab", { name: /advanced|json/i })).toBeVisible();
  });

  test("advanced mode shows JSON editor", async ({ page }) => {
    await page.getByRole("tab", { name: /advanced|json/i }).click();
    await expect(page.getByRole("textbox")).toBeVisible();
  });

  test("validate allow — shows green badge", async ({ page }) => {
    await page.route("**/v1/validate", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          decision: { allow: true, risk_level: "low", reason: "all checks passed" },
          evidence_record: mockEvidence(),
        }),
      }),
    );

    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/allow/i).first()).toBeVisible();
  });

  test("validate deny — shows red badge", async ({ page }) => {
    await page.route("**/v1/validate", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          decision: {
            allow: false,
            risk_level: "high",
            reason: "denied by k8s.protected_namespace",
          },
          evidence_record: mockEvidence({
            event_id: "evt_01JDENY",
            decision: { allow: false, risk_level: "high", reason: "denied by k8s.protected_namespace" },
          }),
        }),
      }),
    );

    // Fill kube-system to trigger deny
    await page.getByLabel(/namespace/i).fill("kube-system");
    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/deny/i).first()).toBeVisible();
  });

  test("shows 401 error and change key action", async ({ page }) => {
    await page.route("**/v1/validate", (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({
          ok: false,
          error: { code: "unauthorized", message: "Invalid API key" },
        }),
      }),
    );

    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/invalid api key/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /change key/i })).toBeVisible();
  });

  test("shows network error with retry", async ({ page }) => {
    await page.route("**/v1/validate", (route) => route.abort("connectionrefused"));

    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/cannot reach/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /retry/i })).toBeVisible();
  });

  test("shows public key", async ({ page }) => {
    await page.route("**/v1/evidence/pubkey", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ pem: "-----BEGIN PUBLIC KEY-----\nMCowBQ...\n-----END PUBLIC KEY-----" }),
      }),
    );

    await page.goto("/#dashboard");
    await expect(page.getByText(/BEGIN PUBLIC KEY/)).toBeVisible();
  });

  test("prompts for API key when not set", async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem("evidra_api_key"));
    await page.goto("/#dashboard");
    await expect(page.getByPlaceholderText(/paste.*key/i)).toBeVisible();
  });

  test("shows storage warning near key input", async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem("evidra_api_key"));
    await page.goto("/#dashboard");
    await expect(page.getByText(/do not use on shared/i)).toBeVisible();
  });

  test("ephemeral toggle uses sessionStorage", async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem("evidra_api_key"));
    await page.goto("/#dashboard");

    // Enable ephemeral mode
    await page.getByLabel(/forget.*tab close/i).check();
    await page.getByPlaceholderText(/paste.*key/i).fill("ev1_ephemeral123456789012345678901234");
    await page.getByRole("button", { name: /save|connect/i }).click();

    const inSession = await page.evaluate(() => sessionStorage.getItem("evidra_api_key"));
    const inLocal = await page.evaluate(() => localStorage.getItem("evidra_api_key"));
    expect(inSession).toContain("ev1_ephemeral");
    expect(inLocal).toBeNull();
  });
});
```

---

## 10. Vite Config

```typescript
// ui/vite.config.ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/v1": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./test/setup.ts"],
  },
});
```

---

## 11. package.json

```json
{
  "name": "evidra-ui",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "e2e:install": "playwright install --with-deps chromium",
    "e2e": "playwright test",
    "e2e:headed": "playwright test --headed"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@playwright/test": "^1.54.2",
    "@testing-library/jest-dom": "^6.8.0",
    "@testing-library/react": "^16.1.0",
    "@testing-library/user-event": "^14.5.2",
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.4",
    "jsdom": "^26.0.0",
    "typescript": "^5.6.3",
    "vite": "^5.4.8",
    "vitest": "^2.1.8"
  }
}
```

---

## 12. Build & Integration

### Makefile additions

```makefile
# Add to existing Makefile

ui-install:
	cd ui && npm install

ui-dev:
	cd ui && npm run dev

ui-build:
	cd ui && npm run build

ui-test:
	cd ui && npm run test

ui-e2e:
	cd ui && npx playwright install --with-deps chromium && npm run e2e

# Build API with embedded UI
build-api-with-ui: ui-build
	go build -o bin/evidra-api ./cmd/evidra-api
```

### CI addition (add to existing ci.yml)

```yaml
  test-ui:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: "npm"
          cache-dependency-path: "ui/package-lock.json"
      - run: cd ui && npm ci
      - run: cd ui && npm run test
      - run: cd ui && npx playwright install --with-deps chromium
      - run: cd ui && npm run e2e
```

---

## 13. Implementation Steps for Claude Code

```
Step 1: "Create ui/ directory with package.json, tsconfig.json, vite.config.ts, index.html. 
         Match the stack from evidra-gitops: React 18, Vite, TypeScript, Vitest, Playwright."

Step 2: "Create src/types/api.ts with TypeScript types for all API responses. 
         Create src/api/client.ts with fetch wrapper."

Step 3: "Create src/styles/global.css with dark theme CSS variables. 
         Create src/components/Layout.tsx, CopyButton.tsx, CodeBlock.tsx, Badge.tsx."

Step 4: "Create src/pages/Landing.tsx — product page with hero, how-it-works diagram,
         MCP/GitHub Actions/CLI integration examples, use cases. Pure static TSX."

Step 5: "Create src/pages/Console.tsx — get API key form with POST /v1/keys. 
         Show key once with copy button and curl example."

Step 6: "Create src/pages/Dashboard.tsx — API key prompt, try validate form (simple + advanced), 
         result display with evidence JSON, public key display."

Step 7: "Create src/pages/Docs.tsx — static quickstart, API reference, evidence guide."

Step 8: "Create src/App.tsx with hash routing (/, /#console, /#dashboard, /#docs), 
         src/main.tsx entry point. Wire everything together."

Step 9: "Write unit tests: test/components/ and test/pages/ and test/hooks/. 
         Run vitest, all must pass."

Step 10: "Write E2E tests: e2e/landing.spec.ts, e2e/console.spec.ts, e2e/dashboard.spec.ts.
          Use Playwright route mocking for API calls. Run playwright test."

Step 11: "Add go:embed to cmd/evidra-api for ui/dist/. 
          Wire SPA fallback handler in router. Build and test."
```
