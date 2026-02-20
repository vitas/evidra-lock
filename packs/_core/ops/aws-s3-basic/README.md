# aws-s3-basic

Minimal declarative AWS CLI surface for S3 guardrailed operations in ops profile.

Supported operations:
- `s3-ls`
- `s3-rm-object`
- `s3-rm-recursive`

Why this is minimal:
- No generic AWS wrapper.
- No free-form args.
- Only explicit operations and fixed argv templates.

Environment behavior:
- `s3-rm-object` can be allowed in `dev` and `prod` only for allowlisted prefixes.
- `s3-rm-recursive` can be allowed only in `dev` and only for allowlisted recursive prefixes.
- `s3-rm-recursive` is denied in `prod` by policy.

Allowlists:
- `policy/data.example.json` contains:
  - `aws_s3.allowed_delete_prefixes`
  - `aws_s3.allowed_recursive_prefixes`
- Customize these prefixes to match your approved cleanup zones.

Example ToolInvocation (`s3-ls`):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"aws",
  "operation":"s3-ls",
  "params":{"uri":"s3://my-bucket/tmp/"},
  "context":{"environment":"dev"}
}
```

Example ToolInvocation (`s3-rm-object` in dev):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"aws",
  "operation":"s3-rm-object",
  "params":{"uri":"s3://my-bucket/tmp/file.txt"},
  "context":{"environment":"dev"}
}
```

Example ToolInvocation (denied `s3-rm-recursive` in prod):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"aws",
  "operation":"s3-rm-recursive",
  "params":{"uri":"s3://my-bucket/tmp/"},
  "context":{"environment":"prod"}
}
```
