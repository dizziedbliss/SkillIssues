# Root multi-target Dockerfile — build context is the repo root.
#
# The web target needs to see packages/markdown (shared with the desktop
# client), which the old ./web-scoped build context could never reach. That
# is the only reason this file exists at the root instead of staying split
# across backend/cmd/*/Dockerfile and web/Dockerfile.prod.
#
# Build one target at a time:
#   docker build --target api        -t skillissues-api        .
#   docker build --target ingestd    -t skillissues-ingestd    .
#   docker build --target flowd      -t skillissues-flowd      .
#   docker build --target migrate    -t skillissues-migrate    .
#   docker build --target web        -t skillissues-web        .
#   docker build --target repoimport -t skillissues-repoimport .

# ---------------------------------------------------------------------------
# Shared Go builder — compiles every backend binary plus healthcheck.
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS go-builder
WORKDIR /build

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/ingestd ./cmd/ingestd && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/flowd ./cmd/flowd && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/repoimport ./cmd/repoimport

# ---------------------------------------------------------------------------
# api — distroless, nonroot. Published images need their own HEALTHCHECK;
# compose having one is not enough once these ship to a registry.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS api
COPY --from=go-builder /out/api /api
COPY --from=go-builder /out/healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8000
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=3s CMD ["/healthcheck"]
ENTRYPOINT ["/api"]

# ---------------------------------------------------------------------------
# ingestd — distroless, nonroot.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS ingestd
COPY --from=go-builder /out/ingestd /ingestd
COPY --from=go-builder /out/healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8002
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=3s CMD ["/healthcheck"]
ENTRYPOINT ["/ingestd"]

# ---------------------------------------------------------------------------
# flowd — distroless, nonroot. Relays the live visualizer's stream.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS flowd
COPY --from=go-builder /out/flowd /flowd
COPY --from=go-builder /out/healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8003
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=3s CMD ["/healthcheck"]
ENTRYPOINT ["/flowd"]

# ---------------------------------------------------------------------------
# migrate — one-shot, distroless, nonroot. No HEALTHCHECK: it runs to
# completion and exits, it never serves traffic.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS migrate
COPY --from=go-builder /out/migrate /migrate
USER nonroot:nonroot
ENTRYPOINT ["/migrate"]

# ---------------------------------------------------------------------------
# repoimport — one-shot, distroless, nonroot. Not part of the running stack
# and not published by deploy.yml; run by hand against the Kaggle dataset
# (mount it at /data/repo_metadata.json, or set IMPORT_FILE/--file to
# another path). Resumable and idempotent — a re-run is how a refreshed
# dataset or a loosened eligibility threshold lands (see CLAUDE.md).
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS repoimport
COPY --from=go-builder /out/repoimport /repoimport
USER nonroot:nonroot
# cmd/repoimport's --file/IMPORT_FILE default is the relative path
# "data/repo_metadata.json" (written for running from backend/ in dev) —
# WORKDIR / keeps that default resolving to /data/repo_metadata.json,
# which is where the dataset should be mounted.
WORKDIR /
ENTRYPOINT ["/repoimport"]

# ---------------------------------------------------------------------------
# web — built from the repo root, which is also the npm workspace root
# (web, desktop, packages/*), so @skillissues/markdown and friends resolve
# as real workspace symlinks under node_modules instead of a relative
# import reaching outside web/; served by nginx.
# ---------------------------------------------------------------------------
FROM node:22-alpine@sha256:0a7108bf6c7bf5de370ffb1a3ed6be93d405b43ff159f681a8d18c0e2bc2e402 AS web-builder
# Alpine's bundled npm 10.9.9 has a known bug (npm/cli#7708-class) where
# `npm ci` hard-fails on an unrelated optionalDependency's platform
# mismatch (e.g. @esbuild/aix-ppc64 on linux/amd64) instead of skipping it,
# once a lockfile has enough workspace members pulling in enough
# platform-specific optional deps (electron-builder's, here). Newer npm
# doesn't have the bug.
RUN npm install -g npm@11
WORKDIR /app

# Manifests only, so `npm ci` gets its own cache layer independent of
# source changes. `npm ci` installs every workspace's dependencies from the
# one root lockfile — there is no partial per-workspace install with `ci` —
# so desktop's devDependencies (Electron) are pulled down here too even
# though this image only builds web. That cost is this intermediate
# stage's build time/bandwidth, never the final nginx image, which only
# ever copies web/dist.
COPY package.json package-lock.json ./
COPY web/package.json web/
COPY desktop/package.json desktop/
COPY packages/markdown/package.json packages/markdown/
COPY packages/api-client/package.json packages/api-client/
COPY packages/ui/package.json packages/ui/
RUN npm ci

COPY packages/ packages/
COPY web/ web/
WORKDIR /app/web

# Same-origin deployment: Caddy proxies /api/* to go-api, so the SPA never
# needs an absolute API origin baked in at build time.
ARG VITE_API_URL=/api
ENV VITE_API_URL=${VITE_API_URL}
RUN npm run build

FROM nginx:alpine@sha256:892bb2152464f0c5d8972593a5f5afaf8054552580b699d95f8ff4af08c2d33d AS web
COPY --from=web-builder /app/web/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=3s \
    CMD wget -q -O- http://127.0.0.1:80/ >/dev/null 2>&1 || exit 1
CMD ["nginx", "-g", "daemon off;"]
