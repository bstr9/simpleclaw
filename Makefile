# ===========================================
# SimpleClaw Makefile
# ===========================================

.PHONY: all build test coverage clean lint sonar help

# 默认目标
all: build

# 编译项目
build:
	go build -o simpleclaw ./cmd/simpleclaw

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

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build          - 编译项目"
	@echo "  make test           - 运行测试"
	@echo "  make coverage       - 生成覆盖率报告"
	@echo "  make coverage-stats - 显示覆盖率统计"
	@echo "  make lint           - 运行代码检查"
	@echo "  make sonar          - 运行 SonarQube 扫描"
	@echo "  make clean          - 清理构建产物"
