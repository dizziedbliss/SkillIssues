# Root multi-target Dockerfile — build context is the repo root.
#
# The web target needs to see packages/markdown (shared with the desktop
# client), which the old ./web-scoped build context could never reach. That
# is the only reason this file exists at the root instead of staying split
# across backend/cmd/*/Dockerfile and web/Dockerfile.prod.
#
# Build one target at a time:
#   docker build --target api      -t skillissues-api      .
#   docker build --target ingestd  -t skillissues-ingestd  .
#   docker build --target migrate  -t skillissues-migrate  .
#   docker build --target web      -t skillissues-web      .

# ---------------------------------------------------------------------------
# Shared Go builder — compiles all three backend binaries plus healthcheck.
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS go-builder
WORKDIR /build

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/ingestd ./cmd/ingestd && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

# ---------------------------------------------------------------------------
# api — distroless, nonroot. Published images need their own HEALTHCHECK;
# compose having one is not enough once these ship to a registry.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=go-builder /out/api /api
COPY --from=go-builder /out/healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8000
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=3s CMD ["/healthcheck"]
ENTRYPOINT ["/api"]

# ---------------------------------------------------------------------------
# ingestd — distroless, nonroot.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS ingestd
COPY --from=go-builder /out/ingestd /ingestd
COPY --from=go-builder /out/healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8002
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=3s CMD ["/healthcheck"]
ENTRYPOINT ["/ingestd"]

# ---------------------------------------------------------------------------
# migrate — one-shot, distroless, nonroot. No HEALTHCHECK: it runs to
# completion and exits, it never serves traffic.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS migrate
COPY --from=go-builder /out/migrate /migrate
USER nonroot:nonroot
ENTRYPOINT ["/migrate"]

# ---------------------------------------------------------------------------
# web — built from the repo root so Vite can resolve the shared
# ../../packages/markdown import; served by nginx.
# ---------------------------------------------------------------------------
FROM node:22-alpine AS web-builder

# Shared markdown package needs to exist at ../../packages/markdown relative
# to web/, at the same relative path, with its own dependencies installed —
# TS module resolution walks up from the importing file's own directory, not
# web/'s, so web's node_modules alone can never satisfy this import.
WORKDIR /app/packages/markdown
COPY packages/markdown/package.json packages/markdown/package-lock.json* ./
RUN npm ci
COPY packages/markdown/ .

WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci

COPY web/ .

# Same-origin deployment: Caddy proxies /api/* to go-api, so the SPA never
# needs an absolute API origin baked in at build time.
ARG VITE_API_URL=/api
ENV VITE_API_URL=${VITE_API_URL}
RUN npm run build

FROM nginx:alpine AS web
COPY --from=web-builder /app/web/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=3s \
    CMD wget -q -O- http://localhost:80/ >/dev/null 2>&1 || exit 1
CMD ["nginx", "-g", "daemon off;"]
