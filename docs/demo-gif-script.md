# Demo GIF Recording Plan

Record a terminal GIF that shows Evidra blocking a dangerous operation in under 30 seconds.

## Prerequisites

- [asciinema](https://asciinema.org/) or [vhs](https://github.com/charmbracelet/vhs) installed
- `evidra` binary built and on PATH
- Terminal width: 90 columns, 24 rows
- Dark background, monospace font (JetBrains Mono or similar)

## Script

### Scene 1: DENY — delete pods in kube-system (~10s)

```bash
# Show what we're about to validate
cat examples/demo/kubernetes_kube_system_delete.json | jq .actions[0].intent

# Evaluate
evidra validate examples/demo/kubernetes_kube_system_delete.json
```

Pause 2 seconds. The FAIL output with `k8s.protected_namespace` and hints should be visible.

### Scene 2: DENY — public S3 bucket (~10s)

```bash
evidra validate examples/demo/terraform_public_s3.json
```

Pause 2 seconds. Two rules fire: `terraform.s3_public_access` and `aws_s3.no_encryption`.

### Scene 3: PASS — safe deployment (~5s)

```bash
evidra validate examples/demo/kubernetes_safe_apply.json
```

Pause 1 second. PASS, risk low, no rules matched.

### Scene 4: Evidence recorded (~5s)

```bash
evidra evidence report --last 3
```

Pause 2 seconds. Shows all three decisions recorded in the evidence chain.

## VHS Tape (alternative)

```tape
Output demo.gif
Set Width 960
Set Height 540
Set FontSize 16
Set Theme "Catppuccin Mocha"

Type "# AI agent wants to delete pods in kube-system..."
Enter
Sleep 1s

Type "evidra validate examples/demo/kubernetes_kube_system_delete.json"
Enter
Sleep 3s

Type "# Now try a public S3 bucket..."
Enter
Sleep 1s

Type "evidra validate examples/demo/terraform_public_s3.json"
Enter
Sleep 3s

Type "# Safe deployment passes"
Enter
Sleep 1s

Type "evidra validate examples/demo/kubernetes_safe_apply.json"
Enter
Sleep 3s
```

## Output

- Save as `docs/assets/demo.gif`
- Target file size: under 2 MB
- Embed in README: `![Demo](docs/assets/demo.gif)`
