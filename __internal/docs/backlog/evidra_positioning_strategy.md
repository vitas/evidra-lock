# Evidra --- Positioning & Architecture Strategy

## 1. What Evidra Is

Evidra is a **deterministic, API-first policy engine** that validates
structured intent before execution.

It is:

-   Tool-agnostic
-   AI-compatible
-   CI-compatible
-   Contract-driven

It is NOT:

-   A Terraform wrapper
-   An AI chatbot
-   A CI tool
-   A policy DSL runtime tied to one platform

------------------------------------------------------------------------

## 2. Strategic Positioning

Evidra sits between:

-   Intent (AI or CI)
-   Execution (apply, deploy, delete, mutate)

It acts as a **policy control plane**.

    Intent → Evidra → Decision → Execution

------------------------------------------------------------------------

## 3. API-First Architecture

REST API is the core.

All integrations are clients:

-   CLI → REST
-   GitHub Action → REST
-   Backstage Plugin → REST
-   MCP Gateway → REST

Single decision engine. Single audit surface. Single policy source of
truth.

------------------------------------------------------------------------

## 4. Why API-First Matters

This ensures:

-   Deterministic decisions
-   Clear separation of concerns
-   Protocol flexibility
-   Enterprise compatibility
-   AI interoperability

------------------------------------------------------------------------

## 5. Market Narrative

Instead of:

"Terraform policy tool"

Position as:

"Policy control plane for AI and CI."

Instead of:

"LLM guardrails"

Position as:

"Deterministic validation engine with AI and CI entry points."

------------------------------------------------------------------------

## 6. Long-Term Vision

Evidra becomes:

-   The approval engine for infrastructure
-   The safety layer for AI agents
-   The validation backend for platforms

One engine. Multiple entry points. Consistent decisions everywhere.
