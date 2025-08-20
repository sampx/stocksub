package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stocksub/pkg/config"
	"stocksub/pkg/logger"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
)

func main() {
	// 初始化配置
	cfg := config.Default()

	// 初始化日志系统
	loggerConfig := logger.Config{
		Level:      cfg.Logger.Level,
		Output:     cfg.Logger.Output,
		Filename:   cfg.Logger.Filename,
		MaxSize:    cfg.Logger.MaxSize,
		MaxBackups: cfg.Logger.MaxBackups,
		MaxAge:     cfg.Logger.MaxAge,
	}
	if err := logger.Init(loggerConfig); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("StockSub starting...")

	// 创建数据提供商
	provider := tencent.NewProvider()

	// 创建订阅器
	sub := subscriber.NewSubscriber(provider)

	// 创建管理器
	manager := subscriber.NewManager(sub)

	// 启动系统
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		logger.Error("Failed to start manager: %v", err)
		os.Exit(1)
	}

	// 示例订阅
	symbols := []string{"600000", "000001", "00700", "AAPL"}

	for _, symbol := range symbols {
		err := manager.Subscribe(symbol, 6*time.Second, func(data subscriber.StockData) error {
			logger.Info("收到 %s (%s) 数据: 价格=%.2f, 涨跌=%+.2f (%.2f%%), 成交量=%d, 买一=%.2f(%d), 卖一=%.2f(%d), 时间=%s",
				data.Symbol, data.Name, data.Price, data.Change, data.ChangePercent,
				data.Volume, data.BidPrice1, data.BidVolume1, data.AskPrice1, data.AskVolume1,
				data.Timestamp.Format("15:04:05"))
			return nil
		})

		if err != nil {
			logger.Error("Failed to subscribe to %s: %v", symbol, err)
		}
	}

	logger.Info("已订阅 %d 只股票，每6秒更新一次", len(symbols))

	// 启动统计输出
	go printStatistics(manager)

	// 等待退出信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	logger.Info("收到退出信号，正在关闭...")
	manager.Stop()
	logger.Info("已退出")
}

// printStatistics 定期打印统计信息
func printStatistics(manager *subscriber.Manager) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := manager.GetStatistics()

		logger.Info("=== 统计信息 ===")
		logger.Info("订阅总数: %d, 活跃订阅: %d", stats.TotalSubscriptions, stats.ActiveSubscriptions)
		logger.Info("数据点总数: %d, 错误总数: %d", stats.TotalDataPoints, stats.TotalErrors)
		logger.Info("运行时间: %v", time.Since(stats.StartTime).Round(time.Second))

		// 打印各股票统计
		for symbol, subStats := range stats.SubscriptionStats {
			healthStatus := "健康"
			if !subStats.IsHealthy {
				healthStatus = "异常"
			}

			logger.Info("  %s: 数据点=%d, 错误=%d, 状态=%s, 最后更新=%s",
				symbol, subStats.DataPointCount, subStats.ErrorCount, healthStatus,
				subStats.LastDataTime.Format("15:04:05"))
		}
		logger.Info("===============")
	}
}
