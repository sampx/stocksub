package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"stocksub/pkg/limiter"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/testkit/storage"
	"stocksub/pkg/timing"
)

// MonitorConfig 监控配置
type MonitorConfig struct {
	Symbols       []string      `json:"symbols"`
	Duration      time.Duration `json:"duration"`
	Interval      time.Duration `json:"interval"`
	DataDir       string        `json:"data_dir"`
	LogDir        string        `json:"log_dir"`
	CleanupOnExit bool          `json:"cleanup_on_exit"`
}

// PerformanceMetric 定义了用于此监控器的性能指标结构
type PerformanceMetric struct {
	Timestamp         time.Time `json:"timestamp"`
	Symbol            string    `json:"symbol"`
	RequestDurationMs int64     `json:"request_duration_ms"`
	ResponseSizeBytes int64     `json:"response_size_bytes"`
	ErrorOccurred     bool      `json:"error_occurred"`
	ErrorMessage      string    `json:"error_message"`
}

// APIMonitor API监控器
type APIMonitor struct {
	config   MonitorConfig
	provider *tencent.Provider
	storage  *storage.CSVStorage
	logger   *log.Logger
	logFile  *os.File
	cancel   context.CancelFunc

	// 安全组件
	marketTime         *timing.MarketTime
	intelligentLimiter *limiter.IntelligentLimiter
}

func main() {
	// 解析命令行参数
	var (
		symbols  = flag.String("symbols", "600000,000001", "股票代码列表，逗号分隔")
		duration = flag.Duration("duration", 5*time.Minute, "监控持续时间")
		interval = flag.Duration("interval", 3*time.Second, "采集间隔")
		dataDir  = flag.String("data-dir", "", "数据保存目录（默认：tests/data/collected）")
		cleanup  = flag.Bool("cleanup", true, "开始前清理旧数据")
	)
	flag.Parse()

	// 获取默认数据目录
	if *dataDir == "" {
		_, currentFile, _, _ := runtime.Caller(0)
		projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
		*dataDir = filepath.Join(projectRoot, "tests", "data")
	}

	// 解析股票代码
	symbolList := strings.Split(*symbols, ",")
	for i, symbol := range symbolList {
		symbolList[i] = strings.TrimSpace(symbol)
	}

	config := MonitorConfig{
		Symbols:       symbolList,
		Duration:      *duration,
		Interval:      *interval,
		DataDir:       *dataDir,
		LogDir:        filepath.Join(*dataDir, "logs"),
		CleanupOnExit: *cleanup,
	}

	// 创建监控器
	monitor, err := NewAPIMonitor(config)
	if err != nil {
		log.Fatalf("创建监控器失败: %v", err)
	}
	defer monitor.Close()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动监控
	fmt.Printf("开始API监控...\n")
	fmt.Printf("股票代码: %v\n", config.Symbols)
	fmt.Printf("运行时长: %v\n", config.Duration)
	fmt.Printf("采集间隔: %v\n", config.Interval)
	fmt.Printf("数据目录: %s\n", config.DataDir)
	fmt.Printf("日志目录: %s\n", config.LogDir)
	fmt.Printf("按 Ctrl+C 停止监控\n")

	// 创建可取消的context
	ctx, cancel := context.WithCancel(context.Background())
	monitor.cancel = cancel

	// 在goroutine中运行监控
	done := make(chan error, 1)
	go func() {
		done <- monitor.Run(ctx)
	}()

	// 等待完成或中断信号
	select {
	case err := <-done:
		if err != nil {
			log.Printf("监控运行错误: %v", err)
		}
	case sig := <-sigChan:
		fmt.Printf("\n收到信号 %v，正在停止监控...\n", sig)
		cancel() // 触发取消
		<-done   // 等待监控完全停止
	}

	fmt.Println("监控已停止")
}

// NewAPIMonitor 创建新的API监控器
func NewAPIMonitor(config MonitorConfig) (*APIMonitor, error) {
	// 确保目录存在
	dirs := []string{config.DataDir, config.LogDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建目录失败 %s: %v", dir, err)
		}
	}

	// 清理旧数据
	if config.CleanupOnExit {
		if err := cleanupOldData(config.DataDir); err != nil {
			return nil, fmt.Errorf("清理旧数据失败: %v", err)
		}
	}

	// 创建日志文件
	logPath := filepath.Join(config.LogDir, fmt.Sprintf("monitor_%s.log", time.Now().Format("20060102_150405")))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %v", err)
	}

	// 创建logger
	logger := log.New(logFile, "[API-MONITOR] ", log.LstdFlags|log.Lmicroseconds)

	// 创建Provider
	provider := tencent.NewProvider()
	provider.SetTimeout(30 * time.Second)
	provider.SetRateLimit(1 * time.Second)

	// 创建存储器
	storageCfg := storage.DefaultCSVStorageConfig()
	storageCfg.Directory = config.DataDir
	csvStorage, err := storage.NewCSVStorage(storageCfg)
	if err != nil {
		return nil, fmt.Errorf("创建CSVStorage失败: %w", err)
	}

	// 创建安全组件
	marketTime := timing.DefaultMarketTime()
	intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)

	monitor := &APIMonitor{
		config:             config,
		provider:           provider,
		storage:            csvStorage,
		logger:             logger,
		logFile:            logFile,
		marketTime:         marketTime,
		intelligentLimiter: intelligentLimiter,
	}

	logger.Printf("API监控器初始化完成: 股票%v, 时长%v, 间隔%v",
		config.Symbols, config.Duration, config.Interval)

	return monitor, nil
}

// Run 运行监控
func (m *APIMonitor) Run(ctx context.Context) error {
	startTime := time.Now()

	// 初始化智能限制器
	m.intelligentLimiter.InitializeBatch(m.config.Symbols)

	m.logger.Printf("开始监控: %v", startTime.Format("2006-01-02 15:04:05"))
	m.logger.Printf("交易时间检查: %t", m.marketTime.IsTradingTime())

	collectionCount := 0
	successCount := 0
	errorCount := 0

	// 主循环：使用智能限制器的安全逻辑
	for {
		select {
		case <-ctx.Done():
			m.logger.Printf("收到取消信号，正在安全关闭...")
			return nil
		default:
			// 继续执行
		}

		// 检查是否可以继续进行API调用
		shouldProceed, err := m.intelligentLimiter.ShouldProceed(ctx)
		if err != nil {
			// 检查是否是市场时间相关的错误
			if strings.Contains(err.Error(), "交易时段") || strings.Contains(err.Error(), "交易时间") {
				m.logger.Printf("非交易时间，等待交易开始: %v", err)
				fmt.Printf("当前非交易时间，等待交易开始...\n")

				// 等待到下一个交易时间开始
				if waitErr := m.waitForTradingTime(ctx); waitErr != nil {
					m.logger.Printf("等待交易时间期间收到取消信号: %v", waitErr)
					return waitErr
				}

				// 交易时间开始，重新初始化智能限制器
				m.logger.Printf("交易时间开始，恢复监控")
				fmt.Printf("交易时间开始，恢复监控...\n")
				m.intelligentLimiter.InitializeBatch(m.config.Symbols)
				continue
			} else {
				// 其他致命错误，终止监控
				m.logger.Printf("智能限制器致命错误，终止: %v", err)
				break
			}
		}

		if !shouldProceed {
			m.logger.Printf("智能限制器指示暂停，等待后重试")
			// 等待一段时间后重试，而不是直接退出
			select {
			case <-time.After(30 * time.Second):
				// 等待30秒后重新检查
				continue
			case <-ctx.Done():
				m.logger.Printf("等待期间收到取消信号")
				return nil
			}
		}

		iterationStart := time.Now()
		collectionCount++

		// 执行数据采集并记录结果
		shouldContinue, waitDuration, finalErr := m.collectDataWithLimiter(ctx, &successCount, &errorCount, collectionCount)

		if finalErr != nil {
			m.logger.Printf("致命错误，终止监控: %v", finalErr)
			break
		}

		if !shouldContinue {
			if waitDuration > 0 {
				// 需要等待重试
				m.logger.Printf("等待 %v 后重试...", waitDuration)
				select {
				case <-time.After(waitDuration):
					// 继续下一轮
					continue
				case <-ctx.Done():
					m.logger.Printf("等待期间收到取消信号")
					return nil
				}
			} else {
				// 需要终止
				m.logger.Printf("智能限制器指示终止采集")
				break
			}
		}

		// 每10轮打印进度
		if collectionCount%10 == 0 {
			elapsed := time.Since(startTime)
			totalAttempts := collectionCount * len(m.config.Symbols)
			currentSuccessRate := float64(successCount) / float64(totalAttempts) * 100

			m.logger.Printf("进度: 已运行 %v，第%d轮，数据点成功率 %.1f%%",
				elapsed.Round(time.Second),
				collectionCount,
				currentSuccessRate)

			// 同时输出到控制台
			fmt.Printf("进度: 第%d轮，成功率 %.1f%%, 已运行 %v\n",
				collectionCount, currentSuccessRate, elapsed.Round(time.Second))
		}

		// 等待下一次采集（但不超过配置的间隔）
		sleepTime := m.config.Interval - time.Since(iterationStart)
		if sleepTime > 0 {
			select {
			case <-time.After(sleepTime):
				// 正常继续
			case <-ctx.Done():
				m.logger.Printf("等待期间收到取消信号")
				return nil
			}
		}
	}

	// 完成统计和分析
	if err := m.finishAndAnalyze(startTime, collectionCount, successCount, errorCount); err != nil {
		return fmt.Errorf("完成分析失败: %v", err)
	}

	return nil
}

// collectDataWithLimiter 执行一轮数据采集（使用智能限制器）
func (m *APIMonitor) collectDataWithLimiter(ctx context.Context, successCount, errorCount *int, roundNum int) (
	shouldContinue bool, waitingDuration time.Duration, finalError error) {

	queryTime := time.Now()

	// 一次API调用获取所有股票数据
	result, rawData, err := m.provider.FetchDataWithRaw(ctx, m.config.Symbols)

	responseTime := time.Now()
	requestDuration := responseTime.Sub(queryTime)

	// 记录性能指标（每个请求一条记录）
	perfMetric := PerformanceMetric{
		Timestamp:         queryTime,
		Symbol:            strings.Join(m.config.Symbols, ","), // 多股票用逗号分隔
		RequestDurationMs: requestDuration.Milliseconds(),
		ResponseSizeBytes: int64(len(rawData)),
		ErrorOccurred:     err != nil,
		ErrorMessage:      "",
	}

	if err != nil {
		perfMetric.ErrorMessage = err.Error()
		m.logger.Printf("第%d轮采集失败: %v (耗时: %v)", roundNum, err, requestDuration)

		// 保存失败的性能指标
		if saveErr := m.storage.Save(ctx, perfMetric); saveErr != nil {
			m.logger.Printf("保存性能指标失败: %v", saveErr)
		}

		*errorCount++
	} else {
		// 成功情况
		m.logger.Printf("第%d轮采集成功: 获取%d只股票 (耗时: %v)", roundNum, len(result), requestDuration)

		// 保存成功的性能指标
		if saveErr := m.storage.Save(ctx, perfMetric); saveErr != nil {
			m.logger.Printf("保存性能指标失败: %v", saveErr)
		}

		// 为每只股票保存数据点
		*successCount += len(result)
		for _, stockData := range result {
			if saveErr := m.storage.Save(ctx, stockData); saveErr != nil {
				m.logger.Printf("保存数据点失败 [%s]: %v", stockData.Symbol, saveErr)
			}
		}
	}

	// 将结果提交给智能限制器判断
	var responseData []string
	if err == nil && len(result) > 0 {
		// 构建响应数据用于一致性检查
		responseData = make([]string, len(result))
		for i, stockData := range result {
			responseData[i] = fmt.Sprintf("%s,%.2f,%d", stockData.Symbol, stockData.Price, int(stockData.Volume))
		}
	}

	// 记录结果并获取下一步指示
	return m.intelligentLimiter.RecordResult(err, responseData)
}

// finishAndAnalyze 完成监控并分析数据
func (m *APIMonitor) finishAndAnalyze(startTime time.Time, collectionCount, successCount, errorCount int) error {
	totalDuration := time.Since(startTime)
	totalAttempts := collectionCount * len(m.config.Symbols)
	finalSuccessRate := float64(successCount) / float64(totalAttempts) * 100

	// 记录完成统计
	m.logger.Printf("=== 监控完成统计 ===")
	m.logger.Printf("总运行时间: %v", totalDuration.Round(time.Second))
	m.logger.Printf("API调用轮次: %d", collectionCount)
	m.logger.Printf("总数据点尝试: %d (%d轮 × %d股票)", totalAttempts, collectionCount, len(m.config.Symbols))
	m.logger.Printf("成功数据点: %d", successCount)
	m.logger.Printf("失败轮次: %d", errorCount)
	m.logger.Printf("数据点成功率: %.2f%%", finalSuccessRate)
	m.logger.Printf("数据保存位置: %s", m.config.DataDir)

	// 输出到控制台
	fmt.Printf("\n=== 监控完成 ===\n")
	fmt.Printf("总运行时间: %v\n", totalDuration.Round(time.Second))
	fmt.Printf("API调用轮次: %d\n", collectionCount)
	fmt.Printf("数据点成功率: %.2f%%\n", finalSuccessRate)
	fmt.Printf("数据保存位置: %s\n", m.config.DataDir)

	// 生成分析报告
	return m.generateAnalysisReport(startTime, totalDuration, collectionCount, successCount, finalSuccessRate)
}

// generateAnalysisReport 生成数据分析报告
func (m *APIMonitor) generateAnalysisReport(startTime time.Time, duration time.Duration,
	collections, successPoints int, successRate float64) error {

	reportPath := filepath.Join(m.config.DataDir, fmt.Sprintf("analysis_report_%s.txt",
		time.Now().Format("20060102_150405")))

	reportFile, err := os.Create(reportPath)
	if err != nil {
		return err
	}
	defer reportFile.Close()

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
- 数据点成功率: %.2f%%
- 平均轮次间隔: %.1f秒
- 实际运行时间: %v

数据质量:
- CSV数据文件: %s
- 性能指标文件: %s
- 日志文件: %s

分析建议:
- 建议使用Excel或数据分析工具进一步分析CSV文件
- 重点关注时间字段(Field30)的变化模式
- 对比不同股票的价格和成交量变化
- 分析API响应时间的分布特征

报告生成时间: %s
`,
		m.config.Symbols,
		m.config.Duration,
		m.config.Interval,
		startTime.Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"),
		collections,
		successPoints,
		successRate,
		m.config.Interval.Seconds(),
		duration.Round(time.Second),
		filepath.Join(m.config.DataDir, "api_data.csv"),
		filepath.Join(m.config.DataDir, "performance.csv"),
		m.logFile.Name(),
		time.Now().Format("2006-01-02 15:04:05"),
	)

	_, err = reportFile.WriteString(report)
	if err == nil {
		m.logger.Printf("分析报告已生成: %s", reportPath)
		fmt.Printf("分析报告已生成: %s\n", reportPath)
	}

	return err
}

// waitForTradingTime 等待直到交易时间开始
func (m *APIMonitor) waitForTradingTime(ctx context.Context) error {
	// 每分钟检查一次是否到了交易时间
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if m.marketTime.IsTradingTime() {
				return nil // 交易时间开始，可以返回
			}
			// 输出等待状态
			currentTime := time.Now().Format("15:04:05")
			fmt.Printf("[%s] 等待交易时间开始...\n", currentTime)
			m.logger.Printf("等待交易时间开始，当前时间: %s", currentTime)
		}
	}
}

// Stop 停止监控
func (m *APIMonitor) Stop() {
	m.logger.Printf("收到停止信号，正在安全关闭...")
}

// Close 关闭监控器
func (m *APIMonitor) Close() {
	if m.storage != nil {
		m.storage.Close()
	}
	if m.logFile != nil {
		m.logFile.Close()
	}
}

// cleanupOldData 清理旧数据
func cleanupOldData(dataDir string) error {
	patterns := []string{"*.csv", "*.txt", "logs/*"}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dataDir, pattern))
		if err != nil {
			continue
		}

		for _, match := range matches {
			if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("删除文件失败 %s: %v", match, err)
			}
		}
	}

	return nil
}
