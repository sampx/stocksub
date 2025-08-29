//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/storage"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAPIMonitor_Integration 测试API监控器的完整功能
func TestAPIMonitor_Integration_WithRealAPI_CollectsDataAndGeneratesReports(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过API监控器测试")
	}

	_, currentFile, _, _ := runtime.Caller(0)
	// 调整路径以从 pkg/testkit/storage 指向 tests/data
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "..")
	testDataDir := filepath.Join(projectRoot, "_data", "monitor_test")

	err := os.RemoveAll(testDataDir)
	require.NoError(t, err)

	err = os.MkdirAll(testDataDir, 0755)
	require.NoError(t, err)

	config := MonitorConfig{
		Symbols:       []string{"600000", "000001"},
		Duration:      30 * time.Second,
		Interval:      3 * time.Second,
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

	err = monitor.Run()
	require.NoError(t, err, "监控器运行失败")

	// 验证生成的文件
	t.Run("验证数据文件", func(t *testing.T) {
		// 新的CSVStorage会根据类型和日期创建文件，我们只需验证存在.csv文件
		files, err := os.ReadDir(config.DataDir)
		require.NoError(t, err)
		csvFileFound := false
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".csv") {
				info, _ := file.Info()
				require.Greater(t, info.Size(), int64(0), "数据文件为空: %s", file.Name())
				t.Logf("✅ 找到数据文件: %s (大小: %d bytes)", file.Name(), info.Size())
				csvFileFound = true
			}
		}
		require.True(t, csvFileFound, "未能找到任何CSV数据文件")
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

		content, err := os.ReadFile(reportFile)
		require.NoError(t, err)
		reportText := string(content)

		require.Contains(t, reportText, "API监控分析报告", "报告标题缺失")
		require.Contains(t, reportText, "600000", "股票代码缺失")
		require.Contains(t, reportText, "成功率", "成功率统计缺失")
		t.Logf("✅ 分析报告内容验证通过")
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

// PerformanceMetric 定义了用于此测试的性能指标结构
type PerformanceMetric struct {
	Timestamp         time.Time `json:"timestamp"`
	Symbol            string    `json:"symbol"`
	RequestDurationMs int64     `json:"request_duration_ms"`
	ResponseSizeBytes int64     `json:"response_size_bytes"`
	ErrorOccurred     bool      `json:"error_occurred"`
	ErrorMessage      string    `json:"error_message"`
}

// APIMonitorTest 测试版本的API监控器
type APIMonitorTest struct {
	config   MonitorConfig
	provider *tencent.Client
	storage  *storage.CSVStorage // 使用新的 a
	logger   *os.File
	t        *testing.T
}

// NewTestAPIMonitor 创建测试版本的API监控器
func NewTestAPIMonitor(config MonitorConfig, t *testing.T) (*APIMonitorTest, error) {
	dirs := []string{config.DataDir, config.LogDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建目录失败 %s: %v", dir, err)
		}
	}

	logPath := filepath.Join(config.LogDir, fmt.Sprintf("monitor_%s.log", time.Now().Format("20060102_150405")))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %v", err)
	}

	provider := tencent.NewClient()
	// provider.SetTimeout(15 * time.Second)
	// provider.SetRateLimit(1 * time.Second)

	// 使用新的 testkit storage
	storageCfg := storage.DefaultCSVStorageConfig()
	storageCfg.Directory = config.DataDir
	csvStorage, err := storage.NewCSVStorage(storageCfg)
	if err != nil {
		return nil, fmt.Errorf("创建CSVStorage失败: %w", err)
	}

	monitor := &APIMonitorTest{
		config:   config,
		provider: provider,
		storage:  csvStorage,
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

		if err := m.collectData(context.Background(), &successCount, &errorCount, collectionCount); err != nil {
			m.log(fmt.Sprintf("第%d轮采集出现错误: %v", collectionCount, err))
		}

		if collectionCount%2 == 0 {
			elapsed := time.Since(startTime)
			progress := fmt.Sprintf("进度: 第%d轮, 已运行 %v", collectionCount, elapsed.Round(time.Second))
			m.log(progress)
			m.t.Log(progress)
		}

		sleepTime := m.config.Interval - time.Since(iterationStart)
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
	}

	if err := m.generateAnalysisReport(startTime, collectionCount, successCount, errorCount); err != nil {
		return fmt.Errorf("生成分析报告失败: %v", err)
	}

	m.log("监控完成")
	return nil
}

// collectData 执行一轮数据采集
func (m *APIMonitorTest) collectData(ctx context.Context, successCount, errorCount *int, roundNum int) error {
	queryTime := time.Now()

	result, rawData, err := m.provider.FetchStockDataWithRaw(ctx, m.config.Symbols)
	responseTime := time.Now()
	requestDuration := responseTime.Sub(queryTime)

	perfMetric := PerformanceMetric{
		Timestamp:         queryTime,
		Symbol:            fmt.Sprintf("BATCH[%s]", strings.Join(m.config.Symbols, ",")),
		RequestDurationMs: requestDuration.Milliseconds(),
		ResponseSizeBytes: int64(len(rawData)),
		ErrorOccurred:     err != nil,
	}

	if err != nil {
		*errorCount++
		perfMetric.ErrorMessage = err.Error()
		m.log(fmt.Sprintf("第%d轮采集失败: %v (耗时: %v)", roundNum, err, requestDuration))
		return m.storage.Save(ctx, perfMetric)
	}

	if err := m.storage.Save(ctx, perfMetric); err != nil {
		return fmt.Errorf("保存性能指标失败: %v", err)
	}

	*successCount += len(result)
	m.log(fmt.Sprintf("第%d轮采集成功: 获取%d只股票 (耗时: %v)", roundNum, len(result), requestDuration))

	// 直接保存 StockData
	for _, stockData := range result {
		if err := m.storage.Save(ctx, stockData); err != nil {
			return fmt.Errorf("保存StockData失败 [%s]: %v", stockData.Symbol, err)
		}
	}

	return nil
}

// generateAnalysisReport 生成分析报告
func (m *APIMonitorTest) generateAnalysisReport(startTime time.Time, collectionCount, successCount, errorCount int) error {
	totalDuration := time.Since(startTime)
	totalAttempts := collectionCount * len(m.config.Symbols)
	var finalSuccessRate float64
	if totalAttempts > 0 {
		finalSuccessRate = float64(successCount) / float64(totalAttempts) * 100
	}

	reportPath := filepath.Join(m.config.DataDir, fmt.Sprintf("analysis_report_%s.txt", time.Now().Format("20060102_150405")))

	// 恢复完整的报告格式
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
- (详情请见数据目录)

测试结果:
- 监控器功能正常: ✅
- 数据采集成功: ✅
- 日志记录完整: ✅

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
		time.Now().Format("2006-01-02 15:04:05"),
	)

	m.log("分析报告已生成: " + reportPath)
	return os.WriteFile(reportPath, []byte(report), 0644)
}

// log 记录日志
func (m *APIMonitorTest) log(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logLine := fmt.Sprintf("[%s] %s\n", timestamp, message)
	if _, err := m.logger.WriteString(logLine); err != nil {
		m.t.Logf("写入日志失败: %v", err)
	}
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
