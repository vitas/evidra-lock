# Evidra --- Kill Switch Minimal Concept (Draft v0.1)

## Core Idea

Introduce a minimal, fail-closed safety mechanism inside the normalized
policy input.

### Add Field

Add a boolean flag to normalized Input:

-   insufficient_context: true \| false

This flag indicates whether the adapter had enough reliable data to make
a safe policy decision.

------------------------------------------------------------------------

## Rule

If:

-   intent is destructive (apply, delete, replace, destroy)
-   AND insufficient_context == true

Then:

→ DENY the operation.

------------------------------------------------------------------------

## Why This Matters

AI agents may:

-   Provide incomplete artifacts
-   Omit required fields
-   Misconstruct payloads
-   Skip resolution steps (e.g., delete without fetching object)

Instead of guessing or allowing partial evaluation, the system
explicitly fails closed.

------------------------------------------------------------------------

## Design Principle

Destructive actions must require full context.

If context is incomplete: - No warning - No soft-fail - No
allow-with-risk

Only deny.

------------------------------------------------------------------------

## Outcome

This creates a lightweight but strong kill-switch behavior without:

-   Admission controllers
-   Token systems
-   Complex orchestration
-   Heavy enterprise features

It keeps the system minimal while safe by default.

------------------------------------------------------------------------

END OF DRAFT v0.1
