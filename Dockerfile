# ----------------------------------------
# 1. Build du frontend Angular
# ----------------------------------------
FROM node:22-alpine AS ng-builder
WORKDIR /app
COPY ng_web/package*.json ./
RUN npm install
COPY ng_web/ ./
RUN npm run build -- --configuration production

# ----------------------------------------
# 2. Build du backend Go
# ----------------------------------------
FROM golang:alpine AS builder

# Installer les dépendances de build
RUN apk add --no-cache git make ca-certificates wget tar xz

WORKDIR /app

# Copier les fichiers de dépendances
COPY go.mod go.sum ./
RUN go mod download

# Copier le code source
COPY . .

# Builder le serveur
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/fx-server ./cmd/server

# ----------------------------------------
# 3. Image finale légère
# ----------------------------------------
FROM alpine:latest

# Installer les dépendances runtime et quelques polices standard
RUN apk add --no-cache ca-certificates tzdata fontconfig ttf-liberation ttf-dejavu ttf-opensans

# Installer Typst
ARG TYPST_VERSION=0.15.0
RUN wget https://github.com/typst/typst/releases/download/v${TYPST_VERSION}/typst-x86_64-unknown-linux-musl.tar.xz \
    && tar xf typst-x86_64-unknown-linux-musl.tar.xz \
    && mv typst-x86_64-unknown-linux-musl/typst /usr/local/bin/ \
    && rm -rf typst-* \
    && chmod +x /usr/local/bin/typst

# Créer un utilisateur non-root
RUN addgroup -g 1000 fx && \
    adduser -D -u 1000 -G fx fx

# Copier le binaire depuis le builder
COPY --from=builder /bin/fx-server /usr/local/bin/fx-server

# Copier les templates embarqués (si nécessaire)
COPY --from=builder /app/pkg/pdf/templates /templates

# Copier le frontend Angular compilé
COPY --from=ng-builder /app/dist/ng_web/browser /web/ng

# Créer les répertoires nécessaires
RUN mkdir -p /storage /templates-custom /web/ng && \
    chown -R fx:fx /storage /templates-custom /web/ng

# Changer vers l'utilisateur non-root
USER fx

# Variables d'environnement par défaut
ENV PORT=8080 \
    HOST=0.0.0.0 \
    STORAGE_TYPE=local \
    STORAGE_LOCAL_PATH=/storage \
    WEB_UI_ENABLED=true \
    WEB_UI_PATH=/web/ng \
    READ_TIMEOUT=30 \
    WRITE_TIMEOUT=30 \
    SHUTDOWN_TIMEOUT=5 \
    TYPST_ROOT=/

# Exposer le port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Démarrer le serveur
CMD ["fx-server"]
