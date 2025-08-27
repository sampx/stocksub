# ---- Build Stage ----
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源代码
COPY . .

# 构建 api_server 应用
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api_server -ldflags="-w -s" ./cmd/api_server

# ---- Final Stage ----
FROM alpine:3.19

# 设置工作目录
WORKDIR /app

# 从 builder 阶段复制编译好的二进制文件
COPY --from=builder /app/api_server .



# 暴露 API Server 的端口
EXPOSE 8080

# 定义容器启动时执行的命令
CMD ["/app/api_server", "--config", "/app/config/api_server.yaml"]