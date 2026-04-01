# ===========================================
# SimpleClaw Makefile
# ===========================================

.PHONY: all build build-web test coverage clean lint sonar help

# 默认目标
all: build

# 编译项目
build: build-web
	go build -o simpleclaw ./cmd/simpleclaw

# 编译前端
build-web:
	@if [ -d "web" ]; then \
		cd web && npm install && npm run build && mkdir -p ../pkg/admin/static && cp -r dist/* ../pkg/admin/static/; \
		echo "前端编译完成，已输出到 pkg/admin/static/"; \
	else \
		echo "web 目录不存在，跳过前端编译"; \
	fi

# 开发模式运行前端
dev-web:
	cd web && npm install && npm run dev

# 运行测试
test:
	go test ./... -v

# 生成覆盖率报告
coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.out 和 coverage.html"

# 查看覆盖率统计
coverage-stats:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -n 1

# 代码检查
lint:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "golangci-lint 未安装，跳过"

# 运行 SonarQube 扫描
sonar: coverage
	sonar-scanner

# 清理
clean:
	rm -f simpleclaw
	rm -f coverage.out coverage.html
	rm -rf .scannerwork
	rm -rf web/dist
	rm -rf web/node_modules
	rm -rf pkg/admin/static

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build          - 编译项目（包含前端）"
	@echo "  make build-web      - 仅编译前端"
	@echo "  make dev-web        - 开发模式运行前端"
	@echo "  make test           - 运行测试"
	@echo "  make coverage       - 生成覆盖率报告"
	@echo "  make coverage-stats - 显示覆盖率统计"
	@echo "  make lint           - 运行代码检查"
	@echo "  make sonar          - 运行 SonarQube 扫描"
	@echo "  make clean          - 清理构建产物"
