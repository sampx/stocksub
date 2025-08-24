# TestKit Cache 示例运行工具
#
# 使用方法:
#   make run        - 交互式运行主程序
#   make basic      - 直接运行基础缓存示例
#   make policy     - 直接运行策略演示
#   make layered    - 直接运行分层缓存演示
#   make advanced   - 直接运行高级功能演示
#   make all        - 运行所有示例
#   make clean      - 清理临时文件

.PHONY: run basic policy layered advanced all clean help

# 默认目标
help:
	@echo "TestKit Cache 模块示例"
	@echo ""
	@echo "可用命令:"
	@echo "  make run        - 交互式运行主程序"
	@echo "  make basic      - 直接运行基础缓存示例"
	@echo "  make policy     - 直接运行缓存策略演示"
	@echo "  make layered    - 直接运行分层缓存演示"
	@echo "  make advanced   - 直接运行高级功能演示"
	@echo "  make all        - 运行所有示例(非交互)"
	@echo "  make test       - 运行cache模块的单元测试"
	@echo "  make clean      - 清理临时文件"
	@echo ""

# 交互式运行主程序
run:
	@echo "=== 交互式运行 Cache 示例 ==="
	@go run .
	@echo ""

# 基础缓存示例(通过环境变量控制)
basic:
	@echo "=== 运行基础缓存操作示例 ==="
	@echo "1" | go run .
	@echo ""

# 策略演示
policy:
	@echo "=== 运行缓存策略演示 ==="
	@echo "2" | go run .
	@echo ""

# 分层缓存演示
layered:
	@echo "=== 运行分层缓存演示 ==="
	@echo "3" | go run .
	@echo ""

# 高级功能演示
advanced:
	@echo "=== 运行高级功能演示 ==="
	@echo "4" | go run .
	@echo ""

# 运行所有示例
all:
	@echo "=== 运行所有示例 ==="
	@echo "5" | go run .
	@echo "=== 所有示例运行完成 ==="

# 运行cache模块测试
test:
	@echo "=== 运行 Cache 模块单元测试 ==="
	@cd ../../.. && go test -v ./pkg/testkit/cache/...
	@echo ""

# 性能基准测试
benchmark:
	@echo "=== 运行 Cache 模块性能基准测试 ==="
	@cd ../../.. && go test -bench=. -benchmem ./pkg/testkit/cache/...
	@echo ""

# 清理临时文件
clean:
	@echo "=== 清理临时文件 ==="
	@find . -name "*.log" -delete 2>/dev/null || true
	@find . -name "*.tmp" -delete 2>/dev/null || true
	@find . -name "*.cache" -delete 2>/dev/null || true
	@rm -rf bin/ 2>/dev/null || true
	@echo "清理完成"

# 检查代码格式
fmt:
	@echo "=== 格式化代码 ==="
	@go fmt .
	@echo "代码格式化完成"

# 代码检查
lint:
	@echo "=== 代码静态检查 ==="
	@go vet .
	@echo "代码检查完成"

# 构建示例
build: bin
	@echo "=== 构建 Cache 示例 ==="
	@go build -o bin/cache_examples .
	@echo "构建完成，可执行文件在 bin/ 目录"

# 创建bin目录
bin:
	@mkdir -p bin