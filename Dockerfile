# ----------------------------------------
# 1. Build du frontend Angular 21 (optionnel si servi par ng-web en dev)
# ----------------------------------------
FROM node:22-alpine AS ng-builder
ARG BUILD_WEB_UI=true
WORKDIR /app
COPY ng_web/package*.json ./
RUN if [ "${BUILD_WEB_UI}" = "true" ]; then npm install; fi
COPY ng_web/ ./
RUN mkdir -p /web-out && \
    if [ "${BUILD_WEB_UI}" = "true" ]; then \
      npm run build -- --configuration production && \
      cp -r dist/ng_web/browser/. /web-out/; \
    else \
      echo "Skipping Angular production build (BUILD_WEB_UI=${BUILD_WEB_UI})"; \
    fi

# ----------------------------------------
# 2. Build du Turbo Daemon Rust (typst-renderd, optionnel si service dédié en compose)
# ----------------------------------------
FROM rust:1.92-alpine AS rust-builder
ARG ENABLE_RUST_DAEMON=true
WORKDIR /src/rust/typst-renderd
COPY rust/typst-renderd/Cargo.toml rust/typst-renderd/Cargo.lock* ./
RUN mkdir -p /out && \
    if [ "${ENABLE_RUST_DAEMON}" = "true" ]; then \
      apk add --no-cache musl-dev pkgconfig && \
      mkdir -p src && echo 'fn main() {}' > src/main.rs && \
      cargo build --release && rm -rf src; \
    fi
COPY rust/typst-renderd/src ./src
RUN if [ "${ENABLE_RUST_DAEMON}" = "true" ]; then \
      touch src/main.rs && \
      cargo build --release && \
      cp target/release/typst-renderd /out/typst-renderd && \
      chmod +x /out/typst-renderd; \
    else \
      echo "Skipping Rust typst-renderd build (ENABLE_RUST_DAEMON=${ENABLE_RUST_DAEMON})"; \
    fi

# ----------------------------------------
# 3. Build du backend Go (fX v2)
# ----------------------------------------
FROM golang:1.25-alpine AS builder
ARG TARGETARCH

RUN apk add --no-cache git make ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -ldflags="-w -s" -o /bin/fx-server ./cmd/server

# ----------------------------------------
# 4. Image finale légère & durcie
# ----------------------------------------
FROM alpine:latest
ARG TYPST_VERSION=0.15.1
ARG TARGETARCH

# Dépendances runtime (libgcc requis par les binaires Rust musl) et polices standard :
# Liberation Sans/Serif/Mono, DejaVu Sans/Serif/Sans Mono, Open Sans, Noto Sans/Serif.
RUN apk add --no-cache ca-certificates tzdata libgcc fontconfig ttf-liberation ttf-dejavu ttf-opensans font-noto && \
    fc-cache -f

# Installer Typst CLI (fallback worker pool & PDF/A-3b CLI)
RUN set -eux; \
    case "${TARGETARCH:-amd64}" in \
      amd64) TYPST_ARCH="x86_64-unknown-linux-musl" ;; \
      arm64) TYPST_ARCH="aarch64-unknown-linux-musl" ;; \
      *) echo "Unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -q "https://github.com/typst/typst/releases/download/v${TYPST_VERSION}/typst-${TYPST_ARCH}.tar.xz"; \
    tar xf "typst-${TYPST_ARCH}.tar.xz"; \
    mv "typst-${TYPST_ARCH}/typst" /usr/local/bin/typst; \
    rm -rf typst-*; \
    chmod +x /usr/local/bin/typst

# Créer un utilisateur non-root (UID/GID 1000 aligné avec typst-renderd)
RUN addgroup -g 1000 fx && \
    adduser -D -u 1000 -G fx fx

# Copier les binaires Go, Rust (optionnel) et l'entrypoint
COPY --from=builder /bin/fx-server /usr/local/bin/fx-server
COPY --from=rust-builder /out/ /usr/local/bin/
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/fx-server /usr/local/bin/entrypoint.sh

# Copier les templates embarqués, le schéma JSON et le frontend Angular compilé (si activé)
COPY --from=builder /app/pkg/pdf/templates /templates
COPY --from=builder /app/docs/invoice.schema.json /docs/invoice.schema.json
COPY --from=ng-builder /web-out/ /web/ng/

# Créer les répertoires nécessaires avec permissions dédiées
RUN mkdir -p /storage /templates-custom /fonts /images /web/ng /docs /var/run/typst && \
    chown -R fx:fx /storage /templates-custom /fonts /images /web/ng /var/run/typst

# Changer vers l'utilisateur non-root
USER fx

# Variables d'environnement sécurisées par défaut (sandbox isolé --root <jobDir> par compilation)
ENV PORT=8080 \
    HOST=0.0.0.0 \
    STORAGE_TYPE=local \
    STORAGE_LOCAL_PATH=/storage \
    WEB_UI_ENABLED=true \
    WEB_UI_PATH=/web/ng \
    READ_TIMEOUT=30 \
    WRITE_TIMEOUT=30 \
    SHUTDOWN_TIMEOUT=5 \
    RENDER_BACKEND=auto \
    RENDER_DAEMON_SOCKET=/tmp/typst-renderd.sock \
    RENDER_TMP_DIR=/dev/shm/fx-render \
    STUDIO_ENABLED=true \
    ALLOW_CUSTOM_TEMPLATES=true \
    TEMPLATES_PATH=/templates-custom

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
