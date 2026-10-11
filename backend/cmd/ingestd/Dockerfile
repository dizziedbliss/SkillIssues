FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /ingestd ./cmd/ingestd && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /ingestd /ingestd
COPY --from=builder /healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8002
ENTRYPOINT ["/ingestd"]
