package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stocksub/pkg/config"
	"stocksub/pkg/logger"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"

	"github.com/sirupsen/logrus"
)

// 全局日志记录器
var log *logrus.Entry

func main() {
	// 初始化配置
	cfg := config.Default()

	// 初始化日志系统
	loggerConfig := logger.Config{
		Level:  cfg.Logger.Level,
		Format: "text", // 使用文本格式
	}
	logger.Init(loggerConfig)

	// 初始化全局日志记录器
	log = logger.WithComponent("StockSub")

	log.Infof("StockSub starting...")

	// 创建 ProviderManager (新架构)
	providerManager := provider.NewProviderManager()

	// 创建腾讯数据提供商
	tencentProvider := tencent.NewProvider()

	// 注册提供商到管理器
	err := providerManager.RegisterProvider("tencent", tencentProvider)
	if err != nil {
		log.Errorf("Failed to register tencent provider: %v", err)
		os.Exit(1)
	}

	// 获取实时股票数据提供商 (通过新接口)
	stockProvider, err := providerManager.GetRealtimeStockProvider("tencent")
	if err != nil {
		log.Errorf("Failed to get stock provider: %v", err)
		// 回退到旧实现
		log.Warnf("Falling back to legacy provider...")
		stockProvider = tencentProvider
	}

	log.Infof("Using provider: %s (health: %v)", stockProvider.Name(), stockProvider.IsHealthy())

	// 创建订阅器 (兼容模式：使用原始腾讯提供商以保证兼容性)
	// 注意：NewSubscriber 返回 *DefaultSubscriber，而不是接口
	sub := subscriber.NewSubscriber(tencentProvider)

	// 创建管理器
	manager := subscriber.NewManager(sub)

	// 启动系统
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		log.Errorf("Failed to start manager: %v", err)
		os.Exit(1)
	}

	// 示例订阅
	symbols := []string{"600000", "000001", "00700", "AAPL"}

	for _, symbol := range symbols {
		err := manager.Subscribe(symbol, 6*time.Second, func(data subscriber.StockData) error {
			log.Infof("收到 %s (%s) 数据: 价格=%.2f, 涨跌=%+.2f (%.2f%%), 成交量=%d, 买一=%.2f(%d), 卖一=%.2f(%d), 时间=%s",
				data.Symbol, data.Name, data.Price, data.Change, data.ChangePercent,
				data.Volume, data.BidPrice1, data.BidVolume1, data.AskPrice1, data.AskVolume1,
				data.Timestamp.Format("15:04:05"))
			return nil
		})

		if err != nil {
			log.Errorf("Failed to subscribe to %s: %v", symbol, err)
		}
	}

	log.Infof("已订阅 %d 只股票，每6秒更新一次", len(symbols))

	// 启动统计输出
	go printStatistics(manager)

	// 等待退出信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Infof("收到退出信号，正在关闭...")
	manager.Stop()
	
	// 清理 ProviderManager
	if err := providerManager.Close(); err != nil {
		log.Warnf("Error closing provider manager: %v", err)
	}
	
	log.Infof("已退出")
}

// printStatistics 定期打印统计信息
func printStatistics(manager *subscriber.Manager) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := manager.GetStatistics()

		log.Infof("=== 统计信息 ===")
		log.Infof("订阅总数: %d, 活跃订阅: %d", stats.TotalSubscriptions, stats.ActiveSubscriptions)
		log.Infof("数据点总数: %d, 错误总数: %d", stats.TotalDataPoints, stats.TotalErrors)
		log.Infof("运行时间: %v", time.Since(stats.StartTime).Round(time.Second))

		// 打印各股票统计
		for symbol, subStats := range stats.SubscriptionStats {
			healthStatus := "健康"
			if !subStats.IsHealthy {
				healthStatus = "异常"
			}

			log.Infof("  %s: 数据点=%d, 错误=%d, 状态=%s, 最后更新=%s",
				symbol, subStats.DataPointCount, subStats.ErrorCount, healthStatus,
				subStats.LastDataTime.Format("15:04:05"))
		}
		log.Infof("===============")
	}
}
