package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	symbols    = flag.String("symbols", "600000,000001", "股票符号列表（逗号分隔）")
	duration   = flag.String("duration", "1h", "监控持续时间")
	interval   = flag.String("interval", "5s", "采集间隔")
	outputFile = flag.String("output", "config/migrated_jobs.yaml", "输出的 YAML 配置文件")
	jobName    = flag.String("job-name", "migrated-api-monitor", "任务名称")
)

// JobConfig 任务配置结构（与 scheduler 包中的结构相同）
type JobConfig struct {
	Name     string                 `yaml:"name"`
	Enabled  bool                   `yaml:"enabled"`
	Schedule string                 `yaml:"schedule"`
	Provider ProviderConfig         `yaml:"provider"`
	Params   map[string]interface{} `yaml:"params"`
	Output   *OutputConfig          `yaml:"output,omitempty"`
}

type ProviderConfig struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

type OutputConfig struct {
	Type   string `yaml:"type"`
	Stream string `yaml:"stream,omitempty"`
}

type JobsConfig struct {
	Jobs []JobConfig `yaml:"jobs"`
}

func main() {
	flag.Parse()

	fmt.Println("StockSub 配置迁移工具")
	fmt.Println("将 api_monitor 命令行参数转换为 jobs.yaml 配置")
	fmt.Println()

	// 解析参数
	symbolList := parseSymbols(*symbols)
	cronSchedule, err := convertIntervalToCron(*interval)
	if err != nil {
		fmt.Printf("错误: 无法转换间隔 '%s' 为 cron 表达式: %v\n", *interval, err)
		os.Exit(1)
	}

	// 创建任务配置
	jobConfig := JobConfig{
		Name:     *jobName,
		Enabled:  true,
		Schedule: cronSchedule,
		Provider: ProviderConfig{
			Name: "tencent",
			Type: "RealtimeStock",
		},
		Params: map[string]interface{}{
			"symbols": symbolList,
		},
		Output: &OutputConfig{
			Type:   "redis_stream",
			Stream: "stream:stock:realtime",
		},
	}

	// 创建完整配置
	config := JobsConfig{
		Jobs: []JobConfig{jobConfig},
	}

	// 生成 YAML
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		fmt.Printf("错误: 生成 YAML 失败: %v\n", err)
		os.Exit(1)
	}

	// 添加注释头部
	header := `# 从 api_monitor 迁移的配置
# 原始参数:
#   symbols: ` + *symbols + `
#   duration: ` + *duration + `
#   interval: ` + *interval + `
# 迁移时间: ` + time.Now().Format("2006-01-02 15:04:05") + `

`

	fullYAML := header + string(yamlData)

	// 写入文件
	err = os.WriteFile(*outputFile, []byte(fullYAML), 0644)
	if err != nil {
		fmt.Printf("错误: 写入文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 配置迁移成功!\n")
	fmt.Printf("   输出文件: %s\n", *outputFile)
	fmt.Printf("   任务名称: %s\n", *jobName)
	fmt.Printf("   调度表达式: %s\n", cronSchedule)
	fmt.Printf("   股票符号: %v\n", symbolList)
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Printf("   go run ./cmd/provider_node -config=%s\n", *outputFile)
	fmt.Println()
	fmt.Println("注意: 请根据需要调整生成的配置文件")
}

// parseSymbols 解析股票符号列表
func parseSymbols(symbolsStr string) []string {
	if symbolsStr == "" {
		return []string{}
	}

	symbols := strings.Split(symbolsStr, ",")
	result := make([]string, 0, len(symbols))

	for _, symbol := range symbols {
		trimmed := strings.TrimSpace(symbol)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// convertIntervalToCron 将时间间隔转换为 cron 表达式
func convertIntervalToCron(intervalStr string) (string, error) {
	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		return "", fmt.Errorf("无效的时间间隔格式: %w", err)
	}

	seconds := int(duration.Seconds())

	switch {
	case seconds < 60:
		// 秒级调度: */N * * * * *
		return fmt.Sprintf("*/%d * * * * *", seconds), nil
	case seconds < 3600:
		// 分钟级调度: 0 */N * * * *
		minutes := seconds / 60
		return fmt.Sprintf("0 */%d * * * *", minutes), nil
	case seconds < 86400:
		// 小时级调度: 0 0 */N * * *
		hours := seconds / 3600
		return fmt.Sprintf("0 0 */%d * * *", hours), nil
	default:
		// 天级调度: 0 0 0 */N * *
		days := seconds / 86400
		return fmt.Sprintf("0 0 0 */%d * *", days), nil
	}
}

// 示例用法函数
func printExamples() {
	fmt.Println("示例用法:")
	fmt.Println()
	fmt.Println("1. 迁移基本的 api_monitor 配置:")
	fmt.Println("   go run ./cmd/config_migrator -symbols=600000,000001 -interval=5s")
	fmt.Println()
	fmt.Println("2. 迁移长期监控配置:")
	fmt.Println("   go run ./cmd/config_migrator -symbols=600000 -duration=24h -interval=3s -job-name=long-term-monitor")
	fmt.Println()
	fmt.Println("3. 迁移多股票高频监控:")
	fmt.Println("   go run ./cmd/config_migrator -symbols=600000,000001,600036,600519 -interval=2s -job-name=high-freq-monitor")
	fmt.Println()
}
