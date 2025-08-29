package main

import (
	"context"
	"fmt"
	"time"

	"stocksub/pkg/logger"
	"stocksub/pkg/message"
	"stocksub/pkg/provider"
	"stocksub/pkg/scheduler"

	"github.com/go-redis/redis/v8"
)

// FetcherExecutor 任务执行器，负责获取股票数据并发布到 Redis
type FetcherExecutor struct {
	providerManager *provider.ProviderManager
	redisClient     *redis.Client
	nodeID          string
	log             *logger.Entry
}

// NewFetcherExecutor 创建新的 FetcherExecutor 实例
func NewFetcherExecutor(providerManager *provider.ProviderManager, redisClient *redis.Client, nodeID string, baseLog *logger.Entry) *FetcherExecutor {
	return &FetcherExecutor{
		providerManager: providerManager,
		redisClient:     redisClient,
		nodeID:          nodeID,
		log:             baseLog.WithField("executor", "fetcher"),
	}
}

// Execute 实现 JobExecutor 接口，执行具体的股票数据获取任务
func (e *FetcherExecutor) Execute(ctx context.Context, job *scheduler.Job) error {
	e.log = e.log.WithFields(map[string]interface{}{
		"job":    job.Config.Name,
		"jobID":  job.ID,
		"nodeID": e.nodeID,
	})

	e.log.Info("开始执行任务")
	e.log.Debugf("任务参数: %+v", job.Config.Params)

	// 根据提供商类型获取提供商
	var provider provider.RealtimeStockProvider
	var err error

	e.log.Debugf("获取提供商: type=%s, name=%s", job.Config.Provider.Type, job.Config.Provider.Name)
	switch job.Config.Provider.Type {
	case "RealtimeStock":
		provider, err = e.providerManager.GetRealtimeStockProvider(job.Config.Provider.Name)
		if err != nil {
			return fmt.Errorf("获取实时股票提供商失败: %w", err)
		}
	default:
		return fmt.Errorf("不支持的提供商类型: %s", job.Config.Provider.Type)
	}

	// 获取股票符号列表
	symbols, err := e.extractSymbols(job.Config.Params)
	if err != nil {
		return fmt.Errorf("提取股票符号失败: %w", err)
	}

	if len(symbols) == 0 {
		return fmt.Errorf("没有找到股票符号")
	}

	e.log.Debugf("准备获取 %d 个股票的数据: %v", len(symbols), symbols)

	// 获取股票数据
	start := time.Now()
	stockDataList, err := provider.FetchStockData(ctx, symbols)
	if err != nil {
		return fmt.Errorf("获取股票数据失败: %w", err)
	}

	duration := time.Since(start)
	e.log.Debugf("数据获取耗时: %v", duration)

	if len(stockDataList) == 0 {
		e.log.Warn("没有获取到股票数据")
		return nil
	}

	e.log.Debugf("成功获取 %d 个股票数据", len(stockDataList))

	// 转换为消息格式的股票数据
	e.log.Debug("转换股票数据为消息格式")
	messageStockData := make([]message.StockData, len(stockDataList))
	for i, stock := range stockDataList {
		messageStockData[i] = message.StockData{
			Symbol:        stock.Symbol,
			Name:          stock.Name,
			Price:         stock.Price,
			Change:        stock.Change,
			ChangePercent: stock.ChangePercent,
			Volume:        stock.Volume,
			Timestamp:     stock.Timestamp.Format(time.RFC3339),
		}
		e.log.Debugf("股票数据: %s - 价格:%.2f, 涨跌:%.2f(%.2f%%)",
			stock.Symbol, stock.Price, stock.Change, stock.ChangePercent)
	}

	// 创建标准消息格式
	e.log.Debug("创建标准消息格式")
	msg := message.NewMessageFormat(
		e.nodeID,
		job.Config.Provider.Name,
		"stock_realtime",
		messageStockData,
	)

	// 设置市场信息
	tradingSession := e.getTradingSession()
	msg.SetMarketInfo("A-share", tradingSession)
	e.log.Debugf("设置市场信息: 交易时段=%s", tradingSession)

	// 转换为 JSON
	jsonData, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 发布到 Redis Streams
	streamName := message.GetStreamName("stock_realtime")
	e.log.Debugf("发布消息到 Redis Stream: %s", streamName)

	result := e.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data": jsonData,
		},
	})

	if err := result.Err(); err != nil {
		return fmt.Errorf("发布消息到 Redis Streams 失败: %w", err)
	}

	e.log.WithFields(map[string]interface{}{
		"stream":    streamName,
		"messageID": result.Val(),
		"dataCount": len(messageStockData),
	}).Info("消息发布成功")

	e.log.Debugf("消息内容大小: %d bytes", len(jsonData))
	return nil
}

// extractSymbols 从任务参数中提取股票符号
func (e *FetcherExecutor) extractSymbols(params map[string]interface{}) ([]string, error) {
	symbolsParam, exists := params["symbols"]
	if !exists {
		return nil, fmt.Errorf("参数中缺少 symbols")
	}

	e.log.Debugf("提取股票符号参数: %+v", symbolsParam)

	switch v := symbolsParam.(type) {
	case []interface{}:
		symbols := make([]string, len(v))
		for i, symbol := range v {
			if str, ok := symbol.(string); ok {
				symbols[i] = str
			} else {
				return nil, fmt.Errorf("股票符号必须是字符串")
			}
		}
		e.log.Debugf("提取的股票符号: %v", symbols)
		return symbols, nil
	case []string:
		e.log.Debugf("提取的股票符号: %v", v)
		return v, nil
	default:
		return nil, fmt.Errorf("symbols 参数格式无效")
	}
}

// getTradingSession 获取当前交易时段
func (e *FetcherExecutor) getTradingSession() string {
	now := time.Now()
	hour := now.Hour()

	e.log.Debugf("当前时间: %v, 小时: %d", now.Format("15:04:05"), hour)

	if hour >= 9 && hour < 12 {
		return "morning"
	} else if hour >= 13 && hour < 15 {
		return "afternoon"
	} else {
		return "closed"
	}
}
