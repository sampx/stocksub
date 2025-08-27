//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default 默认任务：显示帮助信息
func Default() {
	fmt.Println("StockSub 构建系统")
	fmt.Println("================")
	fmt.Println("可用任务:")
	fmt.Println("  mage build       - 构建所有二进制文件")
	fmt.Println("  mage test        - 运行所有测试")
	fmt.Println("  mage testUnit    - 运行单元测试")
	fmt.Println("  mage testIntegration - 运行集成测试")
	fmt.Println("  mage benchmark   - 运行性能基准测试")
	fmt.Println("  mage docker:up  - 启动基础环境 (Redis + InfluxDB)")
	fmt.Println("  mage docker:upall - 启动所有服务")
	fmt.Println("  mage docker:upapps - 启动所有应用服务")
	fmt.Println("  mage docker:fetcher - 启动数据提供节点")
	fmt.Println("  mage docker:rediscollector - 启动 Redis 收集器")
	fmt.Println("  mage docker:influxcollector - 启动 InfluxDB 收集器")
	fmt.Println("  mage docker:apiserver - 启动 API 服务器")
	fmt.Println("  mage docker:down - 停止所有服务")
	fmt.Println("  mage clean       - 清理构建产物")
	fmt.Println("  mage lint        - 运行代码检查")
	fmt.Println("  mage coverage    - 生成测试覆盖率报告")
	fmt.Println("  mage deploy      - 部署到生产环境")
}

// Build 构建所有二进制文件
func Build() error {
	mg.Deps(Clean)

	targets := []struct {
		name string
		path string
	}{
		{"stocksub", "./cmd/stocksub"},
		{"fetcher", "./cmd/fetcher"},
		{"api_monitor", "./cmd/api_monitor"},
		{"api_server", "./cmd/api_server"},
		{"redis_collector", "./cmd/redis_collector"},
		{"influxdb_collector", "./cmd/influxdb_collector"},
	}

	fmt.Println("🚀 开始构建 StockSub 组件...")

	for _, target := range targets {
		fmt.Printf("📦 构建 %s...\n", target.name)
		output := filepath.Join("./dist", target.name)
		if runtime.GOOS == "windows" {
			output += ".exe"
		}

		cmd := exec.Command("go", "build", "-o", output, target.path)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("构建 %s 失败: %v\n输出: %s", target.name, err, string(output))
		}

		// 检查文件大小
		if info, err := os.Stat(output); err == nil {
			fmt.Printf("   ✅ %s: %d MB\n", target.name, info.Size()/1024/1024)
		}
	}

	fmt.Println("🎉 所有组件构建完成!")
	return nil
}

// Test 运行所有测试
func Test() error {
	mg.Deps(TestUnit, TestIntegration)
	return nil
}

// TestUnit 运行单元测试
func TestUnit() error {
	fmt.Println("🧪 运行单元测试...")

	// 运行所有包的单元测试（排除集成测试）
	cmd := exec.Command("go", "test", "./pkg/...", "-v", "-timeout=5m")
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		// 检查是否只是因为没有测试文件而失败
		if strings.Contains(string(output), "[no test files]") &&
			!strings.Contains(string(output), "FAIL") &&
			!strings.Contains(string(output), "build failed") {
			fmt.Println("✅ 单元测试通过! (部分包没有测试文件)")
			return nil
		}
		fmt.Printf("单元测试失败输出:\n%s\n", string(output))
		return fmt.Errorf("单元测试失败: %v", err)
	}

	fmt.Println("✅ 单元测试通过!")
	return nil
}

// TestIntegration 运行集成测试
func TestIntegration() error {
	fmt.Println("🔗 运行集成测试...")

	// 检查依赖服务是否运行
	if !isRedisRunning() {
		fmt.Println("⚠️  Redis 未运行，集成测试可能需要外部依赖")
	}

	// 运行集成测试（需要 integration tag）
	cmd := exec.Command("go", "test", "-v", "-tags=integration", "./pkg/...", "./tests/...", "-timeout=10m")
	cmd.Env = os.Environ()

	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("集成测试失败输出:\n%s\n", string(output))
		return fmt.Errorf("集成测试失败: %v", err)
	}

	fmt.Println("✅ 集成测试通过!")
	return nil
}

// Benchmark 运行性能基准测试
func Benchmark() error {
	fmt.Println("📊 运行性能基准测试...")

	// 确保报告目录存在
	if err := os.MkdirAll("./tests/reports", 0755); err != nil {
		return fmt.Errorf("创建报告目录失败: %v", err)
	}

	cmd := exec.Command("go", "test", "./pkg/testkit", "-bench=.", "-benchmem", "-run=^$", "-timeout=15m")
	cmd.Env = os.Environ()

	// 将输出重定向到文件以便分析
	outputFile, err := os.Create("./tests/reports/benchmark.txt")
	if err != nil {
		return fmt.Errorf("创建基准测试报告失败: %v", err)
	}
	defer outputFile.Close()

	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("基准测试失败: %v", err)
	}

	fmt.Println("✅ 基准测试完成! 报告保存到 ./tests/reports/benchmark.txt")
	return nil
}

type Docker mg.Namespace

// Build 构建所有在 docker-compose.dev.yml 中定义的服务镜像
func (Docker) Build() error {
	fmt.Println("🐳 构建所有 Docker 镜像...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "build")
}

// Up 启动基础环境服务 (redis, influxdb)
func (Docker) Env() error {
	fmt.Println("🚀 启动基础环境服务 (redis, influxdb)...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "up", "-d", "redis", "influxdb")
}

// UpAll 启动所有服务（基础环境 + 应用服务）
func (Docker) UpAll() error {
	fmt.Println("🚀 启动所有服务...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "up", "-d", "--build")
}

// ProviderNode 启动数据提供节点
func (Docker) Fetcher() error {
	fmt.Println("🚀 启动数据提供节点...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "up", "-d", "--build", "fetcher")
}

// UpRedisCollector 启动 Redis 收集器
func (Docker) RedisCollector() error {
	fmt.Println("🚀 启动 Redis 收集器...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "up", "-d", "--build", "redis-collector")
}

// UpInfluxCollector 启动 InfluxDB 收集器
func (Docker) InfluxCollector() error {
	fmt.Println("🚀 启动 InfluxDB 收集器...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "up", "-d", "--build", "influxdb-collector")
}

// UpApiServer 启动 API 服务器
func (Docker) ApiServer() error {
	fmt.Println("🚀 启动 API 服务器...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "up", "-d", "--build", "api-server")
}

// UpApps 启动所有应用服务（不包括基础环境）
func (Docker) UpApps() error {
	fmt.Println("🚀 启动所有应用服务...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "up", "-d", "--build", "fetcher", "redis-collector", "influxdb-collector", "api-server")
}

// Down 停止所有开发环境服务
func (Docker) Down() error {
	fmt.Println("🛑 停止所有开发环境服务...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "down")
}

// Status 查看所有服务的当前状态
func (Docker) Status() error {
	fmt.Println("📊 查看服务状态...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "ps")
}

// Logs 查看所有服务的日志
func (Docker) Logs() error {
	fmt.Println("📜 查看服务日志...")
	return sh.RunV("docker-compose", "-f", "docker-compose.dev.yml", "-p", "stocksub-dev", "logs", "-f", "--tail=100")
}

// Clean 清理构建产物
func Clean() error {
	fmt.Println("🧹 清理构建产物...")

	// 创建 dist 目录如果不存在
	if err := os.MkdirAll("./dist", 0755); err != nil {
		return fmt.Errorf("创建 dist 目录失败: %v", err)
	}

	// 清理二进制文件
	files, err := filepath.Glob("./dist/*")
	if err != nil {
		return fmt.Errorf("查找文件失败: %v", err)
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			fmt.Printf("警告: 无法删除文件 %s: %v\n", file, err)
		}
	}

	// 清理测试缓存
	if err := sh.Run("go", "clean", "-cache"); err != nil {
		fmt.Printf("警告: 清理缓存失败: %v\n", err)
	}

	// 清理测试数据
	if err := os.RemoveAll("./coverage.out"); err != nil && !os.IsNotExist(err) {
		fmt.Printf("警告: 清理覆盖率文件失败: %v\n", err)
	}

	fmt.Println("✅ 清理完成!")
	return nil
}

// Lint 运行代码检查并自动修复
func Lint() error {
	fmt.Println("🔍 运行代码检查...")

	// 首先检查格式问题
	cmd := exec.Command("gofmt", "-d", ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gofmt 检查失败: %v", err)
	}

	if len(output) > 0 {
		fmt.Printf("发现代码格式问题:\n%s\n", string(output))
		fmt.Println("🛠️  正在自动修复格式问题...")

		// 自动修复格式问题
		fixCmd := exec.Command("gofmt", "-w", ".")
		if fixOutput, fixErr := fixCmd.CombinedOutput(); fixErr != nil {
			return fmt.Errorf("自动修复失败: %v\n输出: %s", fixErr, string(fixOutput))
		}

		fmt.Println("✅ 代码格式已自动修复!")

		// 再次检查确认修复成功
		cmd = exec.Command("gofmt", "-d", ".")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("修复后检查失败: %v", err)
		}

		if len(output) > 0 {
			fmt.Printf("⚠️  修复后仍有格式问题:\n%s\n", string(output))
			return fmt.Errorf("代码格式修复不完整")
		}
	}

	fmt.Println("✅ 代码格式检查通过!")
	return nil
}

// Coverage 生成测试覆盖率报告
func Coverage() error {
	fmt.Println("📈 生成测试覆盖率报告...")

	// 确保报告目录存在
	if err := os.MkdirAll("./tests/reports", 0755); err != nil {
		return fmt.Errorf("创建报告目录失败: %v", err)
	}

	cmd := exec.Command("go", "test", "./pkg/...", "-coverprofile=./tests/reports/coverage.out", "-covermode=atomic")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("测试输出:\n%s\n", string(output))
		return fmt.Errorf("生成覆盖率失败: %v", err)
	}

	// 生成HTML报告
	if err := sh.Run("go", "tool", "cover", "-html=./tests/reports/coverage.out", "-o", "./tests/reports/coverage.html"); err != nil {
		return fmt.Errorf("生成HTML报告失败: %v", err)
	}

	// 显示覆盖率摘要
	if err := sh.Run("go", "tool", "cover", "-func=./tests/reports/coverage.out"); err != nil {
		return fmt.Errorf("显示覆盖率失败: %v", err)
	}

	fmt.Println("✅ 覆盖率报告生成完成!")
	fmt.Println("   详细报告: file://" + getAbsolutePath("./tests/reports/coverage.html"))
	return nil
}

// Deploy 部署到生产环境
func Deploy() error {
	mg.Deps(Build, Test)

	fmt.Println("🚀 部署到生产环境...")

	// 这里可以添加具体的部署逻辑
	// 例如：构建Docker镜像、推送到仓库、更新生产环境等

	fmt.Println("✅ 部署准备完成!")
	fmt.Println("运行以下命令进行部署:")
	fmt.Println("  docker-compose -f docker-compose.prod.yml up -d")

	return nil
}

// 辅助函数
func isRedisRunning() bool {
	// 给Redis更多时间启动
	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用docker exec在容器内执行命令，避免本地redis-cli依赖
	cmd := exec.CommandContext(ctx, "docker", "exec", "stocksub-redis-dev", "redis-cli", "-a", "dev_redis_pass", "ping")
	return cmd.Run() == nil
}

func getAbsolutePath(relativePath string) string {
	absPath, err := filepath.Abs(relativePath)
	if err != nil {
		return relativePath
	}
	return absPath
}

// 初始化函数
func init() {
	// 确保必要的目录存在
	os.MkdirAll("./dist", 0755)
	os.MkdirAll("./reports", 0755)
	os.MkdirAll("./config", 0755)
}
