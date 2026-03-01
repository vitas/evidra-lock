# Evidra Development Guide

## UI Dev Mock Mode (No Backend)

If you want to review the UI locally without running `evidra-api`, start the UI with:

```bash
cd ui
VITE_MOCK_API=1 npm run dev
```

This enables a mocked API for:

- `POST /v1/keys`
- `POST /v1/validate`
- `GET /v1/evidence/pubkey`

Safety guardrails:

- Mock mode is enabled only when **both** are true:
  - `VITE_MOCK_API=1`
  - Vite mode is `development`
- Production builds (`npm --prefix ui run build`) do **not** enable mock mode.
- To disable locally, unset `VITE_MOCK_API` (or set it to `0`).
