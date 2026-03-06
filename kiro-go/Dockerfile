# Stage 1: Build Vue frontend
FROM node:20-alpine AS frontend
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY web-vue/package.json web-vue/pnpm-lock.yaml* ./
RUN pnpm install --no-frozen-lockfile
COPY web-vue/ .
RUN pnpm run build

# Stage 2: Build Go backend
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o kiro-api-proxy .

# Stage 3: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/kiro-api-proxy .
COPY --from=builder /app/web ./web
COPY --from=frontend /app/dist ./web/dist

EXPOSE 8080
VOLUME /app/data

CMD ["./kiro-api-proxy"]
