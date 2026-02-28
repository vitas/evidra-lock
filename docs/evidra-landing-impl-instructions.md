# Evidra Landing Page — Implementation Instructions

These instructions replace the current landing page with the new
kill-switch positioning. The existing React/Vite UI architecture stays
unchanged — we're only replacing `Landing.tsx` and `landing.css`.

Reference design: `evidra-landing.html` (standalone HTML prototype).

---

## Project context

- UI app: `ui/` (React + Vite + TypeScript)
- Entry: `ui/src/main.tsx` → `App.tsx` → `pages/Landing.tsx`
- Landing page: `ui/src/pages/Landing.tsx`
- Landing styles: `ui/src/styles/landing.css`
- Global CSS vars: `ui/src/styles/global.css`
- Layout wrapper: `ui/src/components/Layout.tsx`
- Navigation: hash-based (`#console`, `#dashboard`, `#docs`)
- Existing theme: dark green (vars in global.css)
- Component convention: functional components, named exports

---

## What changes

| file | action |
|---|---|
| `ui/src/pages/Landing.tsx` | Replace entirely |
| `ui/src/styles/landing.css` | Replace entirely |
| `ui/src/styles/global.css` | Update color vars to light theme |

Everything else stays the same — `App.tsx`, `Layout.tsx`, other pages,
components, build config.

---

## Task 1: Update global theme to light

**File:** `ui/src/styles/global.css`

Replace the `:root` color variables with a light theme:

```css
:root {
  --color-bg: #fafafa;
  --color-surface: #ffffff;
  --color-surface-hover: #f5f5f5;
  --color-border: #e5e5e5;
  --color-text: #1a1a1a;
  --color-text-muted: #555;
  --color-text-dim: #999;
  --color-primary: #16a34a;
  --color-primary-hover: #15803d;
  --color-success: #16a34a;
  --color-danger: #dc2626;
  --color-warning: #d97706;
  --color-code-bg: #f0f0f0;

  --color-terminal-bg: #2a2f35;
  --color-terminal-border: #363b42;
  --color-terminal-text: #ccc;
  --color-terminal-muted: #666;

  --radius: 8px;
  --font-mono: "JetBrains Mono", "Fira Code", monospace;
  --font-serif: "Source Serif 4", Georgia, serif;
  --font-sans: "Inter", -apple-system, sans-serif;

  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 16px;
  --space-lg: 24px;
  --space-xl: 40px;
}
```

Also add Google Fonts import in `ui/index.html` `<head>`:

```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;700&family=Source+Serif+4:opsz,wght@8..60,400;8..60,600;8..60,700&display=swap" rel="stylesheet">
```

**Important:** this changes the global theme. Other pages (Console,
Dashboard, Docs) may need minor color adjustments. Check them after
the landing page is done. If they break badly, scope the light theme
to `.landing` class only and keep the dark vars for other pages.

**Done when:** `npm run dev` shows light background, dark text on the
landing page.

---

## Task 2: Replace Landing.tsx

**File:** `ui/src/pages/Landing.tsx`

Replace the entire file. The new component must:

1. Keep the same interface: `{ onGetStarted: () => void }`
2. Keep the same export: `export function Landing({ onGetStarted }: LandingProps)`
3. Use the same `import "../styles/landing.css"`

### Component structure

Convert `evidra-landing.html` to React JSX. The sections in order:

1. **Hero** — tagline label, h1 with danger span, subhead, adopt tagline, CTA buttons
2. **Terminal demo** — deny/allow flow for kubectl
3. **Scenarios** — "What goes wrong without a kill-switch" (4 cards)
4. **How it works** — 3 steps with install options
5. **MCP section** — flow diagram + 2 points
6. **CI section** — one terraform example in terminal
7. **Not OPA** — lightweight 3-line differentiation
8. **Levels** — baseline (amber) / ops (green) cards
9. **Evidence** — scenario text + 6 question cards
10. **Footer** — Apache 2.0, runs locally/on-prem, links

### Key adaptations from HTML to React

- `class` → `className`
- The "Get started" button uses `onGetStarted` prop (navigates to console):
  ```tsx
  <a href="#console" className="btn btn-primary" onClick={(e) => {
    e.preventDefault();
    onGetStarted();
  }}>Get started →</a>
  ```
- The "View on GitHub" button links to `https://github.com/vitas/evidra`
- Scroll reveal: use IntersectionObserver in a `useEffect` hook, or
  use a simple `useRef` + observer pattern. Target elements with class `reveal`.
- Terminal content can be JSX directly (no need for template strings)

### Content — copy exactly from evidra-landing.html

**Hero:**
- Label: `your infrastructure · your rules · your evidence`
- H1: `Your AI agent is one wrong guess away from` + danger span: `kubectl delete --all`
- Subhead: `Evidra is a kill-switch for AI agents managing infrastructure. AI can suggest. Evidra decides. Destructive changes are blocked unless proven safe. Every decision is recorded to a tamper-proof evidence chain.`
- Tagline: `Adopt AI in infrastructure — without giving it the keys to production.`

**Terminal demo:** kubectl delete kube-system → deny, staging → allow

**Scenarios (4):**
1. Agent deletes pods in kube-system — picked wrong namespace
2. Terraform apply with 47 deletions — agent didn't understand plan
3. Security group 0.0.0.0/0 on port 22 — agent "fixed" connectivity
4. IAM Action:* Resource:* — quickest way to make it work

**How it works (3 steps):**
1. Install — go install, brew, docker
2. Agent calls validate
3. Evidra decides

**MCP section:**
- Title: "Built for AI agents. Not a CLI wrapper."
- Flow: AI agent → MCP:validate → Evidra → kubectl/terraform/helm
- 2 points: Standard protocol, Pre-execution

**CI section:**
- Title: "Also works in CI and GitOps pipelines"
- One terminal: `terraform show -json tfplan | evidra validate --tool terraform --op apply`

**Not OPA:**
- Title: "Not a replacement for OPA"
- One line: Admission controllers run at deploy time... Evidra runs before execution.

**Levels:**
- baseline: amber tag "kill-switch only", 5 rules
- ops: green tag "default — full protection", 8 rules + "16 more"

**Evidence:**
- Title: "When something goes wrong, you'll have proof"
- Scenario text about 3 databases deleted
- 6 question cards

**Footer:**
- Apache 2.0, no SaaS, no telemetry, runs locally or on-prem
- Links: GitHub, Documentation, Policy catalog, Discord

**Done when:** `npm run dev` shows the full landing page matching the
HTML prototype.

---

## Task 3: Replace landing.css

**File:** `ui/src/styles/landing.css`

Replace entirely with the styles from `evidra-landing.html`. Adapt:

- Use CSS variables from global.css where possible (e.g. `var(--color-text)`
  instead of hardcoded `#1a1a1a`)
- Terminal colors use dedicated `--color-terminal-*` vars
- Keep the `@keyframes fadeIn` and `.reveal` animation
- Keep responsive breakpoints at 640px

The CSS from the HTML prototype is self-contained and can be copied
almost verbatim. Main changes:

- Wrap all styles under `.landing` parent selector to avoid conflicts
  with other pages (`.landing .hero`, `.landing .terminal`, etc.)
- Or keep as-is if the page routes are separate enough

**Done when:** styles match the HTML prototype — light bg, dark terminal,
proper typography, responsive layout.

---

## Task 4: Verify other pages

After the theme change, check:

1. `#console` page — does it look ok with light theme?
2. `#dashboard` page — charts and tables readable?
3. `#docs` page — code blocks still have contrast?

If other pages break, scope the light theme:

```css
/* In global.css — keep dark theme as default */
:root { /* dark vars */ }

/* Light theme only for landing */
.landing { /* override with light vars */ }
```

**Done when:** all 4 pages work — landing is light, others are
acceptable.

---

## Task 5: Build and test

```bash
cd ui
npm install
npm run build   # must succeed with zero errors
npm run dev     # visual check

# Verify:
# 1. Landing page matches HTML prototype
# 2. "Get started" navigates to console
# 3. "View on GitHub" opens in new tab
# 4. Scroll reveal animations work
# 5. Responsive layout at 640px breakpoint
# 6. Terminal blocks are dark on light background
```

---

## Files reference

| deliverable | source |
|---|---|
| HTML prototype | `evidra-landing.html` (final approved design) |
| Copy document | `evidra-landing-copy.md` (all text, tone rules) |
| Current Landing.tsx | `ui/src/pages/Landing.tsx` (to be replaced) |
| Current landing.css | `ui/src/styles/landing.css` (to be replaced) |
| Global CSS | `ui/src/styles/global.css` (vars to update) |

All copy/content comes from `evidra-landing.html`. Do not invent new
text — use exactly what's in the prototype.
