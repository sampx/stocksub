package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stocksub/pkg/config"
	"stocksub/pkg/logger"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"

	"github.com/sirupsen/logrus"
)

// 全局日志记录器
var appLog *logrus.Entry

// 高级使用示例 - 展示所有功能
func main() {
	fmt.Println("=== StockSub 高级示例 ===")

	// 1. 创建自定义配置
	cfg := config.Default()
	cfg.SetRateLimit(1 * time.Second).
		SetDefaultInterval(3 * time.Second).
		SetMaxSubscriptions(20).
		SetLogLevel("debug")

	// 2. 初始化日志
	loggerConfig := logger.Config{
		Level:  cfg.Logger.Level,
		Format: "text", // 使用文本格式
	}
	logger.Init(loggerConfig)

	// 初始化全局日志记录器
	appLog = logger.WithComponent("AdvancedExample")

	appLog.Infof("开始高级示例演示...")

	// 3. 创建提供商并配置
	provider := tencent.NewProvider()
	provider.SetRateLimit(500 * time.Millisecond)
	provider.SetMaxRetries(5)
	provider.SetTimeout(10 * time.Second)

	// 4. 创建订阅器和管理器
	sub := subscriber.NewSubscriber(provider)
	sub.SetMaxSubscriptions(cfg.Subscriber.MaxSubscriptions)
	sub.SetIntervalLimits(cfg.Subscriber.MinInterval, cfg.Subscriber.MaxInterval)

	manager := subscriber.NewManager(sub)

	// 5. 启动系统
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	// 6. 批量订阅A股股票
	requests := []subscriber.SubscribeRequest{
		{Symbol: "600000", Interval: 2 * time.Second, Callback: createStockCallback("A股上证")},
		{Symbol: "000001", Interval: 3 * time.Second, Callback: createStockCallback("A股深证")},
		{Symbol: "300750", Interval: 4 * time.Second, Callback: createStockCallback("A股创业板")},
		{Symbol: "835174", Interval: 5 * time.Second, Callback: createStockCallback("A股北交所")},
		{Symbol: "688036", Interval: 6 * time.Second, Callback: createStockCallback("A股科创板")},
	}

	if err := manager.SubscribeBatch(requests); err != nil {
		appLog.Errorf("批量订阅失败: %v", err)
	}

	// 7. 启动监控和演示
	var wg sync.WaitGroup

	// 统计监控
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorStatistics(manager, ctx)
	}()

	// 动态订阅管理演示
	wg.Add(1)
	go func() {
		defer wg.Done()
		demonstrateDynamicManagement(manager, ctx)
	}()

	// 事件监控
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorEvents(sub, ctx)
	}()

	appLog.Infof("系统已启动，所有监控和演示任务正在运行...")

	// 8. 等待退出信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	appLog.Infof("收到退出信号，正在关闭...")
	cancel()

	// 等待所有goroutine结束
	wg.Wait()

	manager.Stop()
	appLog.Infof("系统已完全关闭")
}

// createStockCallback 创建股票数据回调
func createStockCallback(marketType string) subscriber.CallbackFunc {
	return func(data subscriber.StockData) error {
		appLog.Infof("[%s] %s (%s): ¥%.2f %+.2f (%.2f%%) 成交量:%d 时间:%s",
			marketType,
			data.Symbol,
			data.Name,
			data.Price,
			data.Change,
			data.ChangePercent,
			data.Volume,
			data.Timestamp.Format("15:04:05"))
		return nil
	}
}

// monitorStatistics 监控统计信息
func monitorStatistics(manager *subscriber.Manager, ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := manager.GetStatistics()

			appLog.Infof("=== 系统统计 ===")
			appLog.Infof("总订阅: %d, 活跃: %d, 数据点: %d, 错误: %d",
				stats.TotalSubscriptions, stats.ActiveSubscriptions,
				stats.TotalDataPoints, stats.TotalErrors)

			// 详细统计
			for symbol, subStats := range stats.SubscriptionStats {
				status := "正常"
				if !subStats.IsHealthy {
					status = "异常"
				}

				appLog.Infof("  %s: 数据=%d 错误=%d 状态=%s 最后更新=%v前",
					symbol, subStats.DataPointCount, subStats.ErrorCount, status,
					time.Since(subStats.LastDataTime).Round(time.Second))
			}

			// 输出JSON格式统计用于监控
			if statsJSON, err := json.MarshalIndent(stats, "", "  "); err == nil {
				fmt.Printf("\n=== JSON统计 ===\n%s\n", string(statsJSON))
			}
		}
	}
}

// demonstrateDynamicManagement 演示动态管理
func demonstrateDynamicManagement(manager *subscriber.Manager, ctx context.Context) {
	time.Sleep(10 * time.Second) // 等待初始数据

	// 演示添加新订阅
	appLog.Infof("=== 动态管理演示 ===")
	appLog.Infof("添加新的股票订阅...")

	newSymbols := []string{"002415", "600519"}
	for _, symbol := range newSymbols {
		err := manager.Subscribe(symbol, 7*time.Second, createStockCallback("动态添加"))
		if err != nil {
			appLog.Errorf("动态添加 %s 失败: %v", symbol, err)
		} else {
			appLog.Infof("动态添加 %s 成功", symbol)
		}
	}

	time.Sleep(20 * time.Second)

	// 演示删除订阅
	appLog.Infof("删除部分订阅...")
	for _, symbol := range []string{"688036", "002415"} {
		if err := manager.Unsubscribe(symbol); err != nil {
			appLog.Errorf("取消订阅 %s 失败: %v", symbol, err)
		} else {
			appLog.Infof("取消订阅 %s 成功", symbol)
		}
	}

	// 持续监控直到退出
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			subs := manager.GetSubscriptions()
			appLog.Infof("当前活跃订阅数: %d", len(subs))
		}
	}
}

// monitorEvents 监控系统事件
func monitorEvents(sub *subscriber.DefaultSubscriber, ctx context.Context) {
	eventChan := sub.GetEventChannel()

	appLog.Infof("开始监控系统事件...")

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}

			switch event.Type {
			case subscriber.EventTypeData:
				// 数据事件太频繁，只记录到debug级别
				appLog.Debugf("数据事件: %s", event.Symbol)
			case subscriber.EventTypeError:
				appLog.Warnf("错误事件: %s - %v", event.Symbol, event.Error)
			case subscriber.EventTypeSubscribed:
				appLog.Infof("订阅事件: %s 已成功订阅", event.Symbol)
			case subscriber.EventTypeUnsubscribed:
				appLog.Infof("取消订阅事件: %s 已取消订阅", event.Symbol)
			}
		}
	}
}
