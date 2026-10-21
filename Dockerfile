# Root multi-target Dockerfile — build context is the repo root.
#
# The site is not built here: it is deployed to Vercel (vercel.json).
#
# Build one target at a time:
#   docker build --target api        -t skillissues-api        .
#   docker build --target ingestd    -t skillissues-ingestd    .
#   docker build --target flowd      -t skillissues-flowd      .
#   docker build --target migrate    -t skillissues-migrate    .
#   docker build --target repoimport -t skillissues-repoimport .

# ---------------------------------------------------------------------------
# Shared Go builder — compiles every backend binary plus healthcheck.
# ---------------------------------------------------------------------------
FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS go-builder
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
