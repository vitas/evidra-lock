# Stage 1: Build static binary with embedded OPA bundle.
# The go:embed directive in bundleembed.go bakes policy/bundles/ops-v0.1
# into the binary at compile time — no bundle copy needed in the final image.
FROM golang:1.24.6-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /evidra-lock-mcp ./cmd/evidra-mcp

# Stage 2: Distroless — no shell, no package manager, no debug tools.
# nonroot tag runs as uid 65534; satisfies non-root requirement without
# needing adduser or any OS package.
FROM gcr.io/distroless/static:nonroot
LABEL io.modelcontextprotocol.server.name="io.github.vitas/evidra-lock"
COPY --from=builder /evidra-lock-mcp /usr/local/bin/evidra-lock-mcp
ENTRYPOINT ["/usr/local/bin/evidra-lock-mcp"]
