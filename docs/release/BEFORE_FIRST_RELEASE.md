# Before First Release

Prerequisites to complete before tagging the first `v*` release.

## 1. Create the Homebrew tap repo

Create `vitas/homebrew-tap` on GitHub:

```bash
gh repo create vitas/homebrew-tap --public --description "Homebrew formulae for Evidra"
git clone git@github.com:vitas/homebrew-tap.git
cd homebrew-tap
mkdir Formula
touch Formula/.gitkeep
cat > README.md << 'EOF'
# homebrew-tap

Homebrew formulae for Evidra.

## Install

    brew install vitas/tap/evidra-mcp
EOF
git add -A && git commit -m "init tap repo"
git push origin main
```

GoReleaser will create/update `Formula/evidra-mcp.rb` automatically on each tagged release.

## 2. Create a fine-grained PAT for Homebrew formula push

GoReleaser needs a GitHub token to push the formula to the tap repo. `GITHUB_TOKEN` is scoped to the source repo only, so a separate PAT is required.

1. Go to **GitHub > Settings > Developer settings > Personal access tokens > Fine-grained tokens**
2. Resource owner: `vitas`
3. Repository access: select **Only select repositories** > `vitas/homebrew-tap`
4. Permissions: **Contents** > Read and write
5. Generate and copy the token

## 3. Add the PAT as a repository secret

1. Go to `github.com/vitas/evidra` > **Settings > Secrets and variables > Actions**
2. Click **New repository secret**
3. Name: `HOMEBREW_TAP_TOKEN`
4. Value: paste the PAT from step 2
5. Save

## 4. Tag and release

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow will:
- Run a GoReleaser snapshot dry-run (validates config)
- Build and push the Docker image to `ghcr.io/vitas/evidra-mcp`
- Create a GitHub Release with separate archives for `evidra-mcp` and `evidra`
- Push the Homebrew formula to `vitas/homebrew-tap`

## 5. Verify on a clean machine

```bash
# Remove any existing install
brew uninstall evidra-mcp 2>/dev/null || true
brew untap vitas/tap 2>/dev/null || true

# Install
brew tap vitas/tap
brew install vitas/tap/evidra-mcp

# Verify version
evidra-mcp --version
# Expected: evidra-mcp 0.1.0 (commit: <hash>, built: <date>)

# Verify zero-config embedded bundle
evidra-mcp 2>&1 | head -1
# Expected stderr: "using built-in ops-v0.1 bundle"
```

## Checklist

- [ ] `vitas/homebrew-tap` repo exists with `Formula/` directory
- [ ] `HOMEBREW_TAP_TOKEN` secret is set in `vitas/evidra`
- [ ] `git tag v0.1.0 && git push origin v0.1.0` triggers release
- [ ] GitHub Release contains `evidra-mcp_*` and `evidra_*` archives + checksums
- [ ] Docker image is live at `ghcr.io/vitas/evidra-mcp:<version>`
- [ ] `Formula/evidra-mcp.rb` appears in `vitas/homebrew-tap`
- [ ] `brew install vitas/tap/evidra-mcp` works on clean macOS
- [ ] `evidra-mcp --version` prints semver, commit, date
- [ ] `evidra-mcp` (no flags) prints "using built-in ops-v0.1 bundle"
