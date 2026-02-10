# 构建阶段
FROM --platform=$BUILDPLATFORM golang:1.24.5-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata

# 下载依赖
COPY go.mod go.sum ./
RUN RUN go env -w GO111MODULE=on && go env -w GOPROXY=https://goproxy.cn,direct && go mod download

# 编译应用
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o fake-mc-server cmd/server/main.go

# 运行阶段
FROM scratch
WORKDIR /app

# 添加 OCI 标签来连接仓库
LABEL org.opencontainers.image.source=https://github.com/NewNanCity/FakeMCServer
LABEL org.opencontainers.image.description="A fake Minecraft server for security and testing"
LABEL org.opencontainers.image.licenses=MIT
LABEL maintainer="NewNanCity Team"

# 复制CA证书用于HTTPS/TLS验证
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 复制时区数据
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=builder /app/fake-mc-server .
EXPOSE 25565
VOLUME [ "/app/config", "/app/logs" ]
ENTRYPOINT ["./fake-mc-server"]
CMD ["-config", "config/config.yml"]