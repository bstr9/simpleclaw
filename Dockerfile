# ===========================================
# SimpleClaw Dockerfile - 多阶段构建
# ===========================================

# 阶段1: 构建环境
FROM golang:1.25-alpine AS builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制 go.mod 和 go.sum 并下载依赖（利用缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o simpleclaw ./cmd/simpleclaw

# 阶段2: 运行环境
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1000 -S uai && \
    adduser -u 1000 -S uai -G uai

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/simpleclaw .

# 创建工作空间目录
RUN mkdir -p /app/workspace && chown -R uai:uai /app

# 切换到非 root 用户
USER uai

# 暴露端口（Web 渠道使用）
EXPOSE 8080

# 设置默认配置文件路径
ENV CONFIG_PATH=/app/config.json

# 启动命令
ENTRYPOINT ["./simpleclaw"]
CMD ["-config", "${CONFIG_PATH}"]
