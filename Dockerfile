# ─── Stage 1: Build React frontend ──────────────────────────────────────────
# REGISTRY lets a private build pass a pull-through cache prefix (ends with /).
ARG REGISTRY=
FROM ${REGISTRY}node:22-alpine AS frontend-builder

RUN corepack enable && corepack prepare pnpm@10 --activate

WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml* web/pnpm.yaml* web/pnpm-workspace.yaml* web/.npmrc* ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm run build


# ─── Stage 2: Build Go binary ────────────────────────────────────────────────
ARG REGISTRY=
FROM ${REGISTRY}golang:1.27-alpine AS go-builder

# SQLite (modernc.org/sqlite) is pure Go — no CGO needed.
ENV CGO_ENABLED=0 GOOS=linux

ARG APP_VERSION=dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /app/web/dist ./web/dist

# Empty, nonroot-owned /data seed for the distroless stage (no shell there).
RUN mkdir -p /app/data-empty

RUN go build \
      -buildvcs=false \
      -ldflags="-s -w -X main.version=${APP_VERSION}" \
      -o bin/shitmail \
      ./cmd/shitmail


# ─── Stage 3: Minimal runtime image ─────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot
ARG APP_VERSION=dev

WORKDIR /app
COPY --from=go-builder /app/bin/shitmail /app/shitmail
# Owned by the nonroot uid so a fresh volume is writable without an init step.
COPY --from=go-builder --chown=65532:65532 /app/data-empty /data

# Persistent data directory
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/app/shitmail", "health"]

# HTTP
EXPOSE 8080
# SMTP
EXPOSE 2525

LABEL org.opencontainers.image.title="shitmail"
LABEL org.opencontainers.image.description="Self-hosted disposable email service"
LABEL org.opencontainers.image.url="https://github.com/marc-schuetze/shitmail"
LABEL org.opencontainers.image.source="https://github.com/marc-schuetze/shitmail"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.vendor="DML Labs"

ENV APP_VERSION=${APP_VERSION} \
    PORT=8080 \
    SMTP_PORT=2525 \
    DATABASE_PATH=/data/shitmail.db \
    MAILTUB_DOMAIN=localhost \
    LOG_LEVEL=info

ENTRYPOINT ["/app/shitmail"]
