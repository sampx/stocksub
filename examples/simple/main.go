package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
)

// 简单使用示例
func main() {
	fmt.Println("=== StockSub 简单示例 ===")

	// 0. 设置调试日志
	debugMode := os.Getenv("DEBUG") == "1"
	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		log.Println("[DEBUG] Debug mode enabled")
	}

	// 1. 创建腾讯数据提供商
	provider := tencent.NewProvider()
	if debugMode {
		log.Printf("[DEBUG] Created tencent provider")
	}

	// 2. 创建订阅器
	sub := subscriber.NewSubscriber(provider)
	if debugMode {
		log.Printf("[DEBUG] Created subscriber")
	}

	// 3. 启动订阅器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if debugMode {
		log.Printf("[DEBUG] Starting subscriber...")
	}
	if err := sub.Start(ctx); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	if debugMode {
		log.Printf("[DEBUG] Subscriber started successfully")
	}

	// 4. 订阅股票
	symbols := []string{"600000", "000001"}
	if debugMode {
		log.Printf("[DEBUG] About to subscribe to symbols: %v", symbols)
	}

	for _, symbol := range symbols {
		if debugMode {
			log.Printf("[DEBUG] Subscribing to symbol: %s", symbol)
		}
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
			log.Printf("订阅 %s 失败: %v", symbol, err)
		} else if debugMode {
			log.Printf("[DEBUG] Successfully subscribed to %s", symbol)
		}
	}

	fmt.Printf("已订阅 %d 只股票，按 Ctrl+C 退出\n", len(symbols))
	if debugMode {
		log.Printf("[DEBUG] Waiting for data updates...")
	}

	// 5. 等待退出
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	fmt.Println("\n正在退出...")
	sub.Stop()
	fmt.Println("已退出")
}
