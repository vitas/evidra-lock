# Client-Side Evidence Guide

### Overview

Every `POST /v1/validate` and `POST /v1/skills/{id}:execute` response includes
an `evidence_record` field. This is a complete, server-signed JSON object that
the client must persist to maintain an audit trail.

The server does not store evidence records. If the client discards the response,
the evidence is lost.

**Important:** For `execute` requests with an `idempotency_key`, the server
returns the full `evidence_record` only on the **first** call. Subsequent calls
with the same key return a minimal replay response without the evidence record.
Clients must persist the evidence record from the first response.

### Option 1: Append to JSONL File

The simplest approach. Pipe the evidence record to a file:

```bash
# Using curl and jq
curl -s -X POST https://api.evidra.rest/v1/validate \
  -H "Authorization: Bearer ev1_..." \
  -H "Content-Type: application/json" \
  -d '{"actor":{"type":"agent","id":"bot","origin":"api"},"tool":"kubectl","operation":"get","params":{"target":{},"payload":{}}}' \
  | jq -c '.evidence_record' >> evidence.jsonl
```

Each line in `evidence.jsonl` is a self-contained signed record. Verify any
record later:

```bash
# Verify a single record (no API key required)
head -1 evidence.jsonl | \
  curl -s -X POST https://api.evidra.rest/v1/evidence/verify \
    -H "Content-Type: application/json" \
    -d "{\"evidence_record\": $(cat)}"
```

### Option 2: Use the Evidra CLI Local Evidence Store

The `evidra` CLI binary includes a local evidence store with hash-linked chain
validation. Clients can write server-signed records into the local store:

```bash
# In your agent/platform code:
# 1. Call the API
response=$(curl -s -X POST https://api.evidra.rest/v1/validate ...)

# 2. Extract and store the evidence record
echo "$response" | jq -c '.evidence_record' | evidra evidence import --stdin

# 3. Verify chain integrity
evidra evidence verify
```

This gives you the best of both worlds: server-signed records (proving the
server evaluated the policy) stored in a hash-linked chain (proving no records
were deleted or reordered).

### Option 3: Send to External System

Forward evidence records to S3, Elasticsearch, Splunk, or any append-only store:

```python
# Python example
import requests, json

resp = requests.post(
    "https://api.evidra.rest/v1/validate",
    headers={"Authorization": "Bearer ev1_..."},
    json=invocation,
)
data = resp.json()

# Forward to S3
s3.put_object(
    Bucket="evidra-audit",
    Key=f"evidence/{data['event_id']}.json",
    Body=json.dumps(data["evidence_record"]),
)
```

### Verification

Clients can verify evidence records in two ways:

**Online** — Send the record to `POST /v1/evidence/verify` (no API key
required). The server checks the signature and confirms the signing payload
matches the structured fields:

```bash
curl -s -X POST https://api.evidra.rest/v1/evidence/verify \
  -H "Content-Type: application/json" \
  -d '{"evidence_record": ...}'
```

**Offline** — Fetch the public key from `GET /v1/evidence/pubkey` once, then
verify the Ed25519 signature over the `signing_payload` string:

```go
// Go example
pubKey := fetchPublicKey() // one-time fetch from GET /v1/evidence/pubkey
rec := loadEvidenceRecord()

sigBytes, _ := base64.StdEncoding.DecodeString(rec.Signature)
valid := ed25519.Verify(pubKey, []byte(rec.SigningPayload), sigBytes)
```

```python
# Python example (using PyNaCl)
from nacl.signing import VerifyKey
import base64

verify_key = VerifyKey(public_key_bytes)
sig = base64.b64decode(record["signature"])
# signing_payload is the exact bytes that were signed — no JSON re-serialization
verify_key.verify(record["signing_payload"].encode("utf-8"), sig)
```

No JSON canonicalization needed. The `signing_payload` string is the exact
input that was signed — just verify the signature over that string.

---
