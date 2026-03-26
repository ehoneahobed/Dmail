# Stage 1: Build Go daemon
FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o dmaild ./cmd/dmaild/

# Stage 2: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --frozen-lockfile || npm install
COPY frontend/ .
RUN npm run build

# Stage 3: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=go-builder /app/dmaild .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

EXPOSE 7777

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:7777/api/v1/status || exit 1

VOLUME ["/data"]

ENTRYPOINT ["./dmaild"]
CMD ["--multi-tenant", "--port", "7777", "--listen-addr", "0.0.0.0", "--data-dir", "/data", "--static-dir", "./frontend/dist"]
