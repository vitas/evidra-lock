# Evidra --- Entry Points Architecture

Evidra is an **API-first policy engine** that validates structured
actions before execution.

The REST API is the single source of truth. All integrations --- CLI,
CI, MCP --- are clients of the same API.

------------------------------------------------------------------------

# 1. API-First Core

                        ┌──────────────────────┐
                        │     Evidra API       │
                        │   (Policy Engine)    │
                        └──────────┬───────────┘
                                   │
                ┌──────────────────┼──────────────────┐
                │                                      │
             CLI / CI                              MCP Gateway
         (REST Client Mode)                 (Protocol Adapter → REST)

The policy engine does not care whether input came from AI or CI. It
validates structured intent via REST.

------------------------------------------------------------------------

# 2. CLI / CI Mode (Deterministic)

Used for:

-   GitHub Actions
-   GitLab CI
-   ArgoCD
-   Backstage
-   Air-gapped environments

Flow:

    terraform show -json
            ↓
    evidra-adapter-terraform
            ↓
    evidra CLI (API client)
            ↓
    POST /v1/validate

Characteristics:

-   Deterministic
-   No AI required
-   Fully REST-based
-   Stable for regulated environments

------------------------------------------------------------------------

# 3. MCP Mode (AI Integration)

Used for:

-   AI agents (Claude, GPT, internal LLMs)
-   Autonomous workflows
-   AI tool guardrails

Flow:

    AI Agent
       ↓
    MCP Gateway
       ↓
    REST call to Evidra API

The MCP server is a **protocol gateway**. It does not contain policy
logic. It forwards requests to the REST API.

------------------------------------------------------------------------

# 4. Core Principle

> Evidra is API-first. CLI and MCP are integration entry points. Policy
> logic lives only in the core.
