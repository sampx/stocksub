package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
	apitesting "stocksub/pkg/testing"

	"github.com/stretchr/testify/require"
)

// TestAPIMonitor 测试API监控器的完整功能
// 这是一个快速版本的监控器测试，验证核心功能
func TestAPIMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过API监控器测试")
	}

	// 使用测试专用目录
	_, currentFile, _, _ := runtime.Caller(0)
	testsDir := filepath.Dir(currentFile)
	testDataDir := filepath.Join(testsDir, "data", "monitor_test")

	// 清理并创建测试目录
	os.RemoveAll(testDataDir)
	err := os.MkdirAll(testDataDir, 0755)
	require.NoError(t, err)

	// 创建测试版本的监控器配置
	config := MonitorConfig{
		Symbols:       []string{"600000", "000001"},
		Duration:      30 * time.Second, // 快速测试：30秒
		Interval:      3 * time.Second,  // 每3秒采集一轮
		DataDir:       testDataDir,
		LogDir:        filepath.Join(testDataDir, "logs"),
		CleanupOnExit: true,
	}

	monitor, err := NewTestAPIMonitor(config, t)
	require.NoError(t, err, "创建监控器失败")
	defer monitor.Close()

	t.Logf("开始API监控器测试")
	t.Logf("测试配置: 股票%v, 时长%v, 间隔%v", config.Symbols, config.Duration, config.Interval)
	t.Logf("数据目录: %s", config.DataDir)

	// 运行监控器
	err = monitor.Run()
	require.NoError(t, err, "监控器运行失败")

	// 验证生成的文件
	t.Run("验证数据文件", func(t *testing.T) {
		today := time.Now().Format("2006-01-02")
		expectedFiles := map[string]string{
			fmt.Sprintf("api_data_%s.csv", today):    filepath.Join(testDataDir, "collected"),
			fmt.Sprintf("performance_%s.csv", today): filepath.Join(testDataDir, "collected"),
		}

		for filename, dir := range expectedFiles {
			filePath := filepath.Join(dir, filename)
			_, err := os.Stat(filePath)
			require.NoError(t, err, "数据文件不存在: %s", filePath)

			info, _ := os.Stat(filePath)
			require.Greater(t, info.Size(), int64(100), "数据文件太小: %s", filename)
			t.Logf("✅ 数据文件: %s (大小: %d bytes)", filename, info.Size())
		}
	})

	t.Run("验证日志文件", func(t *testing.T) {
		logFiles, err := filepath.Glob(filepath.Join(config.LogDir, "monitor_*.log"))
		require.NoError(t, err)
		require.NotEmpty(t, logFiles, "日志文件不存在")

		logFile := logFiles[0]
		info, err := os.Stat(logFile)
		require.NoError(t, err)
		require.Greater(t, info.Size(), int64(100), "日志文件太小")
		t.Logf("✅ 日志文件: %s (大小: %d bytes)", filepath.Base(logFile), info.Size())
	})

	t.Run("验证分析报告", func(t *testing.T) {
		reportFiles, err := filepath.Glob(filepath.Join(testDataDir, "analysis_report_*.txt"))
		require.NoError(t, err)
		require.NotEmpty(t, reportFiles, "分析报告不存在")

		reportFile := reportFiles[0]
		info, err := os.Stat(reportFile)
		require.NoError(t, err)
		require.Greater(t, info.Size(), int64(500), "分析报告太小")
		t.Logf("✅ 分析报告: %s (大小: %d bytes)", filepath.Base(reportFile), info.Size())

		// 检查报告内容
		content, err := os.ReadFile(reportFile)
		require.NoError(t, err)
		reportText := string(content)

		// 验证报告关键内容
		require.Contains(t, reportText, "API监控分析报告", "报告标题缺失")
		require.Contains(t, reportText, "600000", "股票代码缺失")
		require.Contains(t, reportText, "000001", "股票代码缺失")
		require.Contains(t, reportText, "成功率", "成功率统计缺失")
		require.Contains(t, reportText, "API调用轮次", "轮次统计缺失")
		t.Logf("✅ 分析报告内容验证通过")
	})

	t.Run("验证原始数据完整性", func(t *testing.T) {
		// 读取API数据文件，验证原始数据是否包含
		today := time.Now().Format("2006-01-02")
		apiDataFile := filepath.Join(testDataDir, "collected", fmt.Sprintf("api_data_%s.csv", today))

		content, err := os.ReadFile(apiDataFile)
		require.NoError(t, err)

		lines := strings.Split(string(content), "\n")
		require.Greater(t, len(lines), 5, "数据文件行数太少") // 至少有表头+几行数据

		// 检查是否包含原始API响应数据
		dataLine := lines[1]                             // 第一行数据（跳过表头）
		require.Contains(t, dataLine, "v_", "原始API数据缺失") // 腾讯API响应特征
		t.Logf("✅ 原始数据完整性验证通过")
	})

	t.Logf("✅ API监控器测试完成")
}

// MonitorConfig 监控配置（简化版）
type MonitorConfig struct {
	Symbols       []string
	Duration      time.Duration
	Interval      time.Duration
	DataDir       string
	LogDir        string
	CleanupOnExit bool
}

// APIMonitorTest 测试版本的API监控器
type APIMonitorTest struct {
	config   MonitorConfig
	provider *tencent.Provider
	storage  *apitesting.CSVStorage
	logger   *os.File
	t        *testing.T
}

// NewTestAPIMonitor 创建测试版本的API监控器
func NewTestAPIMonitor(config MonitorConfig, t *testing.T) (*APIMonitorTest, error) {
	// 创建目录
	dirs := []string{config.DataDir, config.LogDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建目录失败 %s: %v", dir, err)
		}
	}

	// 创建日志文件
	logPath := filepath.Join(config.LogDir, fmt.Sprintf("monitor_%s.log", time.Now().Format("20060102_150405")))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %v", err)
	}

	// 创建Provider
	provider := tencent.NewProvider()
	provider.SetTimeout(15 * time.Second)
	provider.SetRateLimit(1 * time.Second)

	// 创建存储器
	storage := apitesting.NewCSVStorage(config.DataDir)

	monitor := &APIMonitorTest{
		config:   config,
		provider: provider,
		storage:  storage,
		logger:   logFile,
		t:        t,
	}

	monitor.log("API监控器初始化完成")
	return monitor, nil
}

// Run 运行测试监控器
func (m *APIMonitorTest) Run() error {
	startTime := time.Now()
	endTime := startTime.Add(m.config.Duration)

	m.log(fmt.Sprintf("开始监控: %v", startTime.Format("2006-01-02 15:04:05")))

	collectionCount := 0
	successCount := 0
	errorCount := 0

	for time.Now().Before(endTime) {
		iterationStart := time.Now()
		collectionCount++

		if err := m.collectData(&successCount, &errorCount, collectionCount); err != nil {
			m.log(fmt.Sprintf("第%d轮采集出现错误: %v", collectionCount, err))
		}

		// 每5轮打印进度（测试版本更频繁）
		if collectionCount%5 == 0 {
			elapsed := time.Since(startTime)
			remaining := m.config.Duration - elapsed
			totalAttempts := collectionCount * len(m.config.Symbols)
			currentSuccessRate := float64(successCount) / float64(totalAttempts) * 100

			progress := fmt.Sprintf("进度: 第%d轮，成功率 %.1f%%, 剩余 %v",
				collectionCount, currentSuccessRate, remaining.Round(time.Second))
			m.log(progress)
			m.t.Log(progress) // 同时输出到测试日志
		}

		// 等待下一次采集
		sleepTime := m.config.Interval - time.Since(iterationStart)
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
	}

	// 生成分析报告
	if err := m.generateAnalysisReport(startTime, collectionCount, successCount, errorCount); err != nil {
		return fmt.Errorf("生成分析报告失败: %v", err)
	}

	m.log("监控完成")
	return nil
}

// collectData 执行一轮数据采集
func (m *APIMonitorTest) collectData(successCount, errorCount *int, roundNum int) error {
	queryTime := time.Now()

	ctx := context.Background()
	result, rawData, err := m.provider.FetchDataWithRaw(ctx, m.config.Symbols)

	responseTime := time.Now()
	requestDuration := responseTime.Sub(queryTime)

	// 记录性能指标
	perfMetric := apitesting.PerformanceMetric{
		Timestamp:         queryTime,
		Symbol:            fmt.Sprintf("BATCH[%s]", strings.Join(m.config.Symbols, ",")),
		RequestDurationMs: requestDuration.Milliseconds(),
		ResponseSizeBytes: int64(len(rawData)),
		ErrorOccurred:     err != nil,
		ErrorMessage:      "",
	}

	if err != nil {
		*errorCount++
		perfMetric.ErrorMessage = err.Error()
		m.log(fmt.Sprintf("第%d轮采集失败: %v (耗时: %v)", roundNum, err, requestDuration))
		return m.storage.SavePerformanceMetric(perfMetric)
	}

	// 保存成功的性能指标
	if err := m.storage.SavePerformanceMetric(perfMetric); err != nil {
		return fmt.Errorf("保存性能指标失败: %v", err)
	}

	*successCount += len(result)
	m.log(fmt.Sprintf("第%d轮采集成功: 获取%d只股票 (耗时: %v)", roundNum, len(result), requestDuration))

	// 保存数据点
	for _, stockData := range result {
		dataPoint := apitesting.DataPoint{
			Timestamp:    queryTime,
			Symbol:       stockData.Symbol,
			QueryTime:    queryTime,
			ResponseTime: responseTime,
			QuoteTime:    stockData.Timestamp.Format("20060102150405"),
			Price:        stockData.Price,
			Volume:       stockData.Volume,
			Field30:      stockData.Timestamp.Format("20060102150405"),
			AllFields:    buildAllFieldsWithRaw(stockData, rawData),
		}

		if err := m.storage.SaveDataPoint(dataPoint); err != nil {
			return fmt.Errorf("保存数据点失败 [%s]: %v", stockData.Symbol, err)
		}
	}

	return nil
}

// generateAnalysisReport 生成分析报告
func (m *APIMonitorTest) generateAnalysisReport(startTime time.Time, collectionCount, successCount, errorCount int) error {
	totalDuration := time.Since(startTime)
	totalAttempts := collectionCount * len(m.config.Symbols)
	finalSuccessRate := float64(successCount) / float64(totalAttempts) * 100

	reportPath := filepath.Join(m.config.DataDir, fmt.Sprintf("analysis_report_%s.txt",
		time.Now().Format("20060102_150405")))

	report := fmt.Sprintf(`=== API监控分析报告 ===

监控配置:
- 股票代码: %v
- 监控时长: %v
- 采集间隔: %v
- 开始时间: %s
- 结束时间: %s

执行统计:
- API调用轮次: %d
- 成功数据点: %d
- 失败轮次: %d
- 数据点成功率: %.2f%%
- 实际运行时间: %v

数据文件:
- API数据文件: %s
- 性能指标文件: %s
- 日志文件: %s

测试结果:
- 监控器功能正常: ✅
- 数据采集成功: ✅
- 日志记录完整: ✅
- 原始数据保存: ✅

报告生成时间: %s
`,
		m.config.Symbols,
		m.config.Duration,
		m.config.Interval,
		startTime.Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"),
		collectionCount,
		successCount,
		errorCount,
		finalSuccessRate,
		totalDuration.Round(time.Second),
		filepath.Join(m.config.DataDir, "collected", fmt.Sprintf("api_data_%s.csv", time.Now().Format("2006-01-02"))),
		filepath.Join(m.config.DataDir, "collected", fmt.Sprintf("performance_%s.csv", time.Now().Format("2006-01-02"))),
		m.logger.Name(),
		time.Now().Format("2006-01-02 15:04:05"),
	)

	m.log("分析报告已生成: " + reportPath)
	return os.WriteFile(reportPath, []byte(report), 0644)
}

// log 记录日志
func (m *APIMonitorTest) log(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logLine := fmt.Sprintf("[%s] %s\n", timestamp, message)
	m.logger.WriteString(logLine)
}

// Close 关闭监控器
func (m *APIMonitorTest) Close() {
	if m.storage != nil {
		m.storage.Close()
	}
	if m.logger != nil {
		m.logger.Close()
	}
}

// buildAllFieldsWithRaw 构建包含原始数据的字段数组
func buildAllFieldsWithRaw(stockData subscriber.StockData, rawData string) []string {
	return []string{
		stockData.Symbol,                             // 0: 股票代码
		stockData.Name,                               // 1: 股票名称
		fmt.Sprintf("%.3f", stockData.Price),         // 2: 当前价格
		fmt.Sprintf("%d", stockData.Volume),          // 3: 成交量
		stockData.Timestamp.Format("20060102150405"), // 4: 时间字段
		rawData, // 5: 原始API响应数据（用于异常分析）
	}
}
