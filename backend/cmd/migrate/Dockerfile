# Build stage: compile the Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Download dependencies first (cached unless go.mod/go.sum changes)
COPY go.mod go.sum ./
RUN go mod download

# Copy all source and build. The schema is embedded via go:embed, so the
# runtime image needs no SQL files.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /migrate ./cmd/migrate

# ---------------------------------------------------------------------------
# Runtime stage: minimal distroless image (no shell, no package manager)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /migrate /migrate

# Run as nonroot user (uid 65532)
USER nonroot:nonroot

ENTRYPOINT ["/migrate"]
