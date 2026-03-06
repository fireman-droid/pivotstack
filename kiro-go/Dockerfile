# Stage 1: Build Go backend
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o kiro-api-proxy .

# Stage 2: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/kiro-api-proxy .
COPY --from=builder /app/web ./web
# Vue build output (run `pnpm run build` in web-vue/ before docker build)
COPY web-vue/dist ./web/dist

EXPOSE 8080
VOLUME /app/data

CMD ["./kiro-api-proxy"]
