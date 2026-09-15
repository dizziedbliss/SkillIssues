# Build stage: compile the Go binary
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Download dependencies first (cached unless go.mod/go.sum changes)
COPY go.mod go.sum ./
RUN go mod download

# Copy all source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /api ./cmd/api

# ---------------------------------------------------------------------------
# Runtime stage: minimal distroless image (no shell, no package manager)
# Binary is the only thing in the image.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /api /api

# Run as nonroot user (uid 65532)
USER nonroot:nonroot

EXPOSE 8000

ENTRYPOINT ["/api"]
