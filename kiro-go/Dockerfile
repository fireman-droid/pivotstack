# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web-vue
COPY web-vue/package.json ./
RUN npm install
COPY web-vue/ .
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o kiro-api-proxy .

# Stage 3: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates mailcap
WORKDIR /app
COPY --from=builder /app/kiro-api-proxy .
COPY --from=frontend /app/web-vue/dist ./web-vue/dist

EXPOSE 8080
VOLUME /app/data

CMD ["./kiro-api-proxy"]
