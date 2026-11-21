# ----------------------------------------
# 1. Build du Turbo Daemon Rust (typst-renderd)
# ----------------------------------------
FROM rust:1.92-alpine AS builder

RUN apk add --no-cache musl-dev pkgconfig

WORKDIR /src

# Cache des dépendances Cargo : si seul src/ change, crates.io n'est pas recompilé
COPY Cargo.toml Cargo.lock* ./
RUN mkdir -p src && \
    echo 'fn main() {}' > src/main.rs && \
    cargo build --release && \
    rm -rf src

# Compilation réelle de typst-renderd
COPY src ./src
RUN touch src/main.rs && \
    cargo build --release --locked 2>/dev/null || cargo build --release

# ----------------------------------------
# 2. Image runtime légère avec polices Alpine
# ----------------------------------------
FROM alpine:latest

RUN apk add --no-cache \
      ca-certificates \
      tzdata \
      libgcc \
      fontconfig \
      ttf-liberation \
      ttf-dejavu \
      ttf-opensans \
      font-noto && \
    fc-cache -f

# Même UID/GID (1000) que fx-server pour le partage du Unix Domain Socket
RUN addgroup -g 1000 fx && \
    adduser -D -u 1000 -G fx fx

COPY --from=builder /src/target/release/typst-renderd /usr/local/bin/typst-renderd
RUN chmod +x /usr/local/bin/typst-renderd && \
    mkdir -p /var/run/typst /fonts && \
    chown -R fx:fx /var/run/typst /fonts

USER fx

ENV TYPST_DAEMON_SOCKET=/var/run/typst/typst-renderd.sock \
    TYPST_DAEMON_LISTEN=0.0.0.0:9090 \
    TYPST_FONT_PATHS=/fonts

EXPOSE 9090

HEALTHCHECK --interval=5s --timeout=2s --start-period=5s --retries=10 \
  CMD wget -qO- http://127.0.0.1:9090/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/typst-renderd"]
