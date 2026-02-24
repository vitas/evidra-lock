FROM golang:1.24.6-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/evidra ./cmd/evidra \
 && go build -o /out/evidra-mcp ./cmd/evidra-mcp

FROM alpine:3.22
RUN adduser -D evidra
USER evidra
COPY --from=builder /out/evidra /usr/local/bin/evidra
COPY --from=builder /out/evidra-mcp /usr/local/bin/evidra-mcp
ENTRYPOINT ["/usr/local/bin/evidra-mcp"]
