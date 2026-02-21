# Releasing Evidra

## Prerequisites

- Clean main branch
- Passing CI
- Updated `CHANGELOG.md`

## Local checks

```bash
make fmt
make test
make lint
make build
```

## Versioning

Use semantic version tags (for example `v0.1.1`).

## Cut a release

1. Update `CHANGELOG.md`:
   - Move notable items from `Unreleased` to the new version heading.
2. Commit changes to `main`.
3. Tag and push:

```bash
git tag v0.1.1
git push origin v0.1.1
```

4. GitHub Actions `release.yml` runs GoReleaser and publishes archives + checksums.

## Version metadata

Release builds inject:

- `Version`
- `Commit`
- `Date`

`evidra version` prints these values.
