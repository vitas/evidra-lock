# Build from Source

This guide explains how to build Evidra locally.

## Prerequisites

-   Go (see go.mod for required version)
-   Git

Verify:

``` bash
go version
git --version
```

------------------------------------------------------------------------

## Clone repository

``` bash
git clone <REPO_URL>
cd <REPO_DIR>
```

------------------------------------------------------------------------

## Build binaries

CLI:

``` bash
go build ./cmd/evidra
```

MCP server:

``` bash
go build ./cmd/evidra-mcp
```

------------------------------------------------------------------------

## Run

``` bash
./evidra --help
./evidra-mcp --help
```

------------------------------------------------------------------------

## Install globally (optional)

``` bash
go install ./cmd/evidra
go install ./cmd/evidra-mcp
```

------------------------------------------------------------------------

## Notes

Default evidence store: `~/.evidra/evidence`
