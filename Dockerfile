# Stage 1: Build frontend
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.23-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./cmd/kestrel/web/dist
RUN CGO_ENABLED=1 go build -o kestrel ./cmd/kestrel

# Stage 3: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=go-builder /app/kestrel /usr/local/bin/
COPY --from=go-builder /app/migrations /migrations
EXPOSE 8080
CMD ["kestrel"]
