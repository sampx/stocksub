//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"stocksub/pkg/core"
	"stocksub/pkg/testkit/config"
	"stocksub/pkg/testkit/manager"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TimeFieldAnalysis 时间字段分析结果
type TimeFieldAnalysis struct {
	Symbol    string    `json:"symbol"`
	Market    string    `json:"market"`
	TimeField string    `json:"time_field"`
	Pattern   string    `json:"pattern"`
	Length    int       `json:"length"`
	Timestamp time.Time `json:"timestamp"`
}

// TestSystem_TimeFieldConsistencyLongRun 长时间运行的时间字段格式收集测试
func TestSystem_TimeFieldConsistencyLongRun(t *testing.T) {
	if os.Getenv("LONG_RUN") != "1" {
		t.Skip("跳过长时间运行测试，设置 LONG_RUN=1 启用")
	}

	// 使用新的 TestDataManager
	cfg := &config.Config{
		Cache:   config.CacheConfig{Type: "memory"},
		Storage: config.StorageConfig{Type: "csv", Directory: t.TempDir()},
	}
	manager := manager.NewTestDataManager(cfg)
	defer manager.Close()

	// 运行参数配置
	runDuration := 4 * time.Hour // 默认运行4小时
	if envDuration := os.Getenv("RUN_DURATION"); envDuration != "" {
		if d, err := time.ParseDuration(envDuration); err == nil {
			runDuration = d
		}
	}

	collectInterval := 10 * time.Minute // 每10分钟收集一次
	if envInterval := os.Getenv("COLLECT_INTERVAL"); envInterval != "" {
		if d, err := time.ParseDuration(envInterval); err == nil {
			collectInterval = d
		}
	}

	t.Logf("🚀 启动长时间运行模式: 持续 %v，每 %v 收集一次数据", runDuration, collectInterval)

	extendedSamples := map[string][]string{
		"上海主板": {"600000", "600036", "600519", "600887", "601318", "601166"},
		"深圳主板": {"000001", "000002", "000858", "000166", "000725", "002415"},
		"创业板":  {"300001", "300750", "300059", "300015", "300124", "300347"},
		"科创板":  {"688001", "688036", "688041", "688111", "688169", "688223"},
		"北交所":  {"835174", "832000", "430047", "831865", "833533", "872925"},
	}

	var allSymbols []string
	for _, symbols := range extendedSamples {
		allSymbols = append(allSymbols, symbols...)
	}

	startTime := time.Now()
	endTime := startTime.Add(runDuration)
	collectionCount := 0
	allAnalysis := make(map[string][]TimeFieldAnalysis)

	t.Logf("📊 将收集 %d 个股票在 %v 时间段内的时间字段数据", len(allSymbols), runDuration)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	interrupted := false

	go func() {
		<-sigChan
		t.Logf("⚠️ 接收到停止信号，将在下一次数据收集后停止并生成报告...")
		interrupted = true
	}()

	for time.Now().Before(endTime) && !interrupted {
		collectionCount++
		currentTime := time.Now()
		t.Logf("\n🔄 第 %d 次数据收集 (%s)", collectionCount, currentTime.Format("15:04:05"))

		// 强制刷新缓存以获取真实API数据
		manager.EnableCache(false)
		results, err := manager.GetStockData(context.Background(), allSymbols)
		manager.EnableCache(true) // 立即恢复缓存

		if err != nil {
			t.Logf("❌ 第 %d 次收集失败: %v", collectionCount, err)
		} else {
			t.Logf("✅ 成功获取 %d 个股票的实时数据", len(results))
			// 分析时间字段格式
			timeFieldAnalysis := analyzeTimeFields(results)

			// 保存本次分析结果
			timestamp := currentTime.Format("150405")
			allAnalysis[timestamp] = make([]TimeFieldAnalysis, 0, len(timeFieldAnalysis))
			for _, analysis := range timeFieldAnalysis {
				allAnalysis[timestamp] = append(allAnalysis[timestamp], analysis)
			}

			// 实时统计格式分布
			formatStats := make(map[string]int)
			for _, analysis := range timeFieldAnalysis {
				formatStats[analysis.Pattern]++
			}

			t.Logf("📈 本次发现格式:")
			for pattern, count := range formatStats {
				t.Logf("  %s: %d个样本", pattern, count)
			}

			// 保存单次分析结果
			saveSingleAnalysis(t, timeFieldAnalysis, collectionCount, currentTime)
		}

		// 等待下一次收集
		if time.Now().Before(endTime.Add(-collectInterval)) && !interrupted {
			t.Logf("⏰ 等待 %v 后进行下次收集...", collectInterval)

			// 分段睡眠以便及时响应信号
			sleepStart := time.Now()
			for time.Since(sleepStart) < collectInterval && !interrupted {
				time.Sleep(1 * time.Second)
			}
		} else {
			break
		}
	}

	if interrupted {
		t.Logf("🛑 测试被手动停止，正在生成报告...")
	}

	generateLongRunSummaryReport(t, allAnalysis, startTime, time.Now(), collectionCount)

	t.Logf("✅ 长时间运行完成，共收集 %d 次数据，运行时长 %v",
		collectionCount, time.Since(startTime).Round(time.Second))
}

// saveSingleAnalysis 保存单次分析结果
func saveSingleAnalysis(t *testing.T, analysis map[string]TimeFieldAnalysis, collectionNum int, timestamp time.Time) {
	outputDir := "data/long_run"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Logf("⚠️ 创建输出目录失败: %v", err)
		return
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("collection_%03d_%s.json",
		collectionNum, timestamp.Format("20060102_150405")))

	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		t.Logf("⚠️ 序列化第 %d 次分析结果失败: %v", collectionNum, err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		t.Logf("⚠️ 保存第 %d 次分析结果失败: %v", collectionNum, err)
	}
}

// generateLongRunSummaryReport 生成长时间运行的汇总报告
func generateLongRunSummaryReport(t *testing.T, allAnalysis map[string][]TimeFieldAnalysis,
	startTime, endTime time.Time, totalCollections int) {

	outputDir := "data/long_run"
	reportFile := filepath.Join(outputDir, fmt.Sprintf("summary_report_%s.md",
		startTime.Format("20060102_150405")))

	var report strings.Builder
	report.WriteString("# 长时间运行时间字段格式分析报告\n\n")
	report.WriteString(fmt.Sprintf("**运行时间段**: %s 至 %s\n",
		startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05")))
	report.WriteString(fmt.Sprintf("**总运行时长**: %v\n",
		endTime.Sub(startTime).Round(time.Second)))
	report.WriteString(fmt.Sprintf("**数据收集次数**: %d 次\n\n", totalCollections))

	overallPatterns := make(map[string]int)
	timeDistribution := make(map[string]map[string]int)              // 时间段 -> 格式 -> 数量
	marketTimePatterns := make(map[string]map[string]map[string]int) // 市场 -> 时间 -> 格式 -> 数量

	for timestamp, analyses := range allAnalysis {
		timeDistribution[timestamp] = make(map[string]int)

		for _, analysis := range analyses {
			overallPatterns[analysis.Pattern]++
			timeDistribution[timestamp][analysis.Pattern]++

			if marketTimePatterns[analysis.Market] == nil {
				marketTimePatterns[analysis.Market] = make(map[string]map[string]int)
			}
			if marketTimePatterns[analysis.Market][timestamp] == nil {
				marketTimePatterns[analysis.Market][timestamp] = make(map[string]int)
			}
			marketTimePatterns[analysis.Market][timestamp][analysis.Pattern]++
		}
	}

	report.WriteString("## 发现的时间格式总览\n\n")
	for pattern, count := range overallPatterns {
		report.WriteString(fmt.Sprintf("- **%s**: %d 次出现\n", pattern, count))
	}

	report.WriteString("\n## 不同时间段的格式分布\n\n")
	for timestamp, patterns := range timeDistribution {
		report.WriteString(fmt.Sprintf("### %s 时间段\n", timestamp))
		for pattern, count := range patterns {
			report.WriteString(fmt.Sprintf("- %s: %d 个样本\n", pattern, count))
		}
		report.WriteString("\n")
	}

	report.WriteString("## 格式一致性分析\n\n")
	if len(overallPatterns) == 1 {
		report.WriteString("✅ **格式完全一致**: 所有时间段都使用相同的时间格式\n\n")
	} else {
		report.WriteString("⚠️ **发现格式变化**: 在不同时间段出现了不同的时间格式\n\n")
	}

	report.WriteString("## 建议\n\n")
	report.WriteString("基于本次长时间分析，建议parseTime函数支持以下格式:\n\n")
	for pattern := range overallPatterns {
		switch pattern {
		case "YYYYMMDDHHMMSS":
			report.WriteString("- 14位格式: `20060102150405`\n")
		case "YYYYMMDDHHMM":
			report.WriteString("- 12位格式: `200601021504`\n")
		case "YYYYMMDDHH":
			report.WriteString("- 10位格式: `2006010215`\n")
		case "YYYYMMDD":
			report.WriteString("- 8位格式: `20060102`\n")
		}
	}

	if err := os.WriteFile(reportFile, []byte(report.String()), 0644); err != nil {
		t.Logf("⚠️ 生成汇总报告失败: %v", err)
	} else {
		t.Logf("📝 汇总报告已生成: %s", reportFile)
	}
}

// analyzeTimeFields 分析股票数据中的时间字段格式
func analyzeTimeFields(stocksData []core.StockData) map[string]TimeFieldAnalysis {
	results := make(map[string]TimeFieldAnalysis)

	for _, stock := range stocksData {
		timeField := stock.Timestamp.Format("20060102150405")
		market := determineMarket(stock.Symbol)
		pattern := determineTimePattern(timeField)

		analysis := TimeFieldAnalysis{
			Symbol:    stock.Symbol,
			Market:    market,
			TimeField: timeField,
			Pattern:   pattern,
			Length:    len(timeField),
			Timestamp: time.Now(),
		}

		results[stock.Symbol] = analysis
	}

	return results
}

// determineMarket 根据股票代码确定市场
func determineMarket(symbol string) string {
	if len(symbol) != 6 {
		return "未知市场"
	}

	switch {
	case strings.HasPrefix(symbol, "6"):
		return "上海主板"
	case strings.HasPrefix(symbol, "688"):
		return "科创板"
	case strings.HasPrefix(symbol, "000") || strings.HasPrefix(symbol, "001"):
		return "深圳主板"
	case strings.HasPrefix(symbol, "300"):
		return "创业板"
	case strings.HasPrefix(symbol, "43") || strings.HasPrefix(symbol, "83") || strings.HasPrefix(symbol, "87") || strings.HasPrefix(symbol, "920"):
		return "北交所"
	default:
		return "其他市场"
	}
}

// determineTimePattern 确定时间格式模式
func determineTimePattern(timeField string) string {
	switch len(timeField) {
	case 14:
		return "YYYYMMDDHHMMSS"
	case 12:
		return "YYYYMMDDHHMM"
	case 10:
		return "YYYYMMDDHH"
	case 8:
		return "YYYYMMDD"
	default:
		return fmt.Sprintf("未知格式(长度:%d)", len(timeField))
	}
}
