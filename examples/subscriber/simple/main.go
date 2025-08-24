package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stocksub/pkg/logger"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
)

// 简单使用示例
func main() {
	fmt.Println("=== StockSub 简单示例 ===")

	// 0. 初始化日志
	logger.InitFromEnv()
	log := logger.WithComponent("SimpleExample")

	log.Debug("Debug mode enabled")

	// 1. 创建腾讯数据提供商
	provider := tencent.NewProvider()
	log.Debug("Created tencent provider")

	// 2. 创建订阅器
	sub := subscriber.NewSubscriber(provider)
	log.Debug("Created subscriber")

	// 3. 启动订阅器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Debug("Starting subscriber...")
	if err := sub.Start(ctx); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	log.Debug("Subscriber started successfully")

	// 4. 订阅股票
	symbols := []string{"600000", "000001"}
	log.Debugf("About to subscribe to symbols: %v", symbols)

	for _, symbol := range symbols {
		log.Debugf("Subscribing to symbol: %s", symbol)
		err := sub.Subscribe(symbol, 5*time.Second, func(data subscriber.StockData) error {
			fmt.Printf("[%s] %s (%s): %.2f %+.2f (%.2f%%) 量:%d 买一:%.2f(%d) 卖一:%.2f(%d)\n",
				data.Timestamp.Format("15:04:05"),
				data.Symbol,
				data.Name,
				data.Price,
				data.Change,
				data.ChangePercent,
				data.Volume,
				data.BidPrice1, data.BidVolume1,
				data.AskPrice1, data.AskVolume1)
			return nil
		})

		if err != nil {
			log.Errorf("订阅 %s 失败: %v", symbol, err)
		} else {
			log.Debugf("Successfully subscribed to %s", symbol)
		}
	}

	fmt.Printf("已订阅 %d 只股票，按 Ctrl+C 退出\n", len(symbols))
	log.Debug("Waiting for data updates...")

	// 5. 等待退出
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	fmt.Println("\n正在退出...")
	sub.Stop()
	fmt.Println("已退出")
}
