# ==========================================
# Stage 1: Build Frontend (React + Vite)
# ==========================================
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web

COPY web/package.json ./
RUN npm config set registry https://registry.npmmirror.com && npm install

COPY web/ ./
RUN npm run build

# ==========================================
# Stage 2: Build Backend (Go Static Binary)
# ==========================================
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app

# Enable GOPROXY for fast dependency retrieval
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod go.sum ./
RUN go mod download

# Copy backend source
COPY internal/ ./internal/
COPY cmd/ ./cmd/

# Copy built frontend assets for Go embed
COPY --from=frontend-builder /app/web/dist ./web/dist
COPY web/embed.go ./web/

# Compile statically linked binary with optimizations
RUN go build -ldflags="-s -w" -o pikpak-manager ./cmd/server

# ==========================================
# Stage 3: Minimal Production Runtime
# ==========================================
FROM alpine:3.20

# Install ca-certificates and tzdata for TLS and timezones
RUN apk --no-cache add ca-certificates tzdata curl

ENV PORT=8080
ENV DATA_DIR=/data
ENV IN_DOCKER=true

WORKDIR /app

# Copy binary from backend builder
COPY --from=backend-builder /app/pikpak-manager /app/pikpak-manager

# Persistence volume for SQLite database and configs
VOLUME ["/data"]

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/pikpak-manager"]
