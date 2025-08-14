# FakeMCServer Makefile

APP_NAME := fake-mc-server
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# 默认目标
.PHONY: all
all: build

# 构建
.PHONY: build
build:
	@echo "🔨 构建 $(APP_NAME) $(VERSION)..."
	go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME) cmd/server/main.go

# 运行
.PHONY: run
run: build
	@echo "🚀 启动应用..."
	./$(APP_NAME) -config config/config.yml

# 测试
.PHONY: test
test:
	@echo "🧪 运行测试..."
	go test -v ./...

# 清理
.PHONY: clean
clean:
	@echo "🧹 清理..."
	rm -f $(APP_NAME) $(APP_NAME).exe
	rm -f $(APP_NAME)-*

# 构建所有平台
.PHONY: build-all
build-all:
	@echo "🔨 构建所有平台版本..."
	@echo "构建 Windows AMD64..."
	GOOS=windows GOARCH=amd64 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-windows-amd64.exe cmd/server/main.go
	@echo "构建 Windows ARM64..."
	GOOS=windows GOARCH=arm64 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-windows-arm64.exe cmd/server/main.go
	@echo "构建 Linux AMD64..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-linux-amd64 cmd/server/main.go
	@echo "构建 Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-linux-arm64 cmd/server/main.go
	@echo "构建 Linux ARM..."
	GOOS=linux GOARCH=arm go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-linux-arm cmd/server/main.go
	@echo "构建 macOS AMD64..."
	GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-darwin-amd64 cmd/server/main.go
	@echo "构建 macOS ARM64..."
	GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-darwin-arm64 cmd/server/main.go
	@echo "构建 FreeBSD AMD64..."
	GOOS=freebsd GOARCH=amd64 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(APP_NAME)-$(VERSION)-freebsd-amd64 cmd/server/main.go
	@echo "✅ 所有平台构建完成!"

# 创建发布包
.PHONY: release
release:
	@echo "🚀 创建发布包..."
ifeq ($(OS),Windows_NT)
	scripts\release.bat $(VERSION)
else
	bash scripts/release.sh $(VERSION)
endif

# Docker构建
.PHONY: docker
docker:
	@echo "🐳 构建Docker镜像..."
	docker build -t $(APP_NAME):$(VERSION) .

# 显示帮助
.PHONY: help
help:
	@echo "可用命令:"
	@echo "  build     - 构建应用"
	@echo "  build-all - 构建所有平台版本"
	@echo "  release   - 创建发布包（使用脚本）"
	@echo "  run       - 运行应用"
	@echo "  test      - 运行测试"
	@echo "  clean     - 清理文件"
	@echo "  docker    - 构建Docker镜像"
