# 多阶段构建 - 优化镜像大小
FROM golang:1.24.4-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o go-ai-insight ./cmd

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/go-ai-insight .
COPY --from=builder /app/config/config.json .
EXPOSE 8080
CMD ["./go-ai-insight"]
