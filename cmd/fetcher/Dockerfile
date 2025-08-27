# ---- Build Stage ----
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源代码
COPY . .

# 构建 fetcher 应用
# -o /app/fetcher: 将输出的可执行文件命名为 fetcher 并放在 /app 目录下
# -ldflags="-w -s": 优化编译选项，减小可执行文件大小
# ./cmd/fetcher: 要编译的 main 包的路径
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/fetcher -ldflags="-w -s" ./cmd/fetcher

# ---- Final Stage ----
# 使用一个非常小的基础镜像
FROM alpine:3.19

# 设置工作目录
WORKDIR /app

# 从 builder 阶段复制编译好的二进制文件
# 复制构建好的可执行文件到运行时镜像
COPY --from=builder /app/fetcher .

# 暴露端口（如果需要）
# EXPOSE 8080

# 定义容器启动时执行的命令
# 配置文件将通过 docker-compose volume 挂载
CMD ["/app/fetcher", "--config", "/app/config/jobs.yaml"]
