# ── Stage 1: build ─────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /agent ./cmd/agent

# ── Stage 2: minimal runtime ────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /agent /agent
COPY --from=builder /app/configs/config.yaml /configs/config.yaml

# Prometheus metrics endpoint
EXPOSE 9100

# NET_RAW capability is required for ICMP:
#   docker run --cap-add NET_RAW go-agent
# or in docker-compose: cap_add: [NET_RAW]
ENTRYPOINT ["/agent", "-config", "/configs/config.yaml"]
