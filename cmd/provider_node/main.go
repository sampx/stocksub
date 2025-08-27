package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stocksub/pkg/message"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/scheduler"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "config/jobs.yaml", "任务配置文件路径")
	redisAddr  = flag.String("redis", "localhost:6379", "Redis 服务器地址")
	redisPass  = flag.String("redis-pass", "", "Redis 密码")
	nodeID     = flag.String("node-id", "", "节点ID（默认自动生成）")
	logLevel   = flag.String("log-level", "info", "日志级别")
)

type ProviderNodeExecutor struct {
	providerManager *provider.ProviderManager
	redisClient     *redis.Client
	nodeID          string
	logger          *logrus.Logger
}

func main() {
	flag.Parse()

	// 设置日志
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("无效的日志级别:", *logLevel)
	}
	logger.SetLevel(level)

	// 生成节点ID
	if *nodeID == "" {
		*nodeID = fmt.Sprintf("provider-node-%d", time.Now().Unix())
	}

	logger.WithField("nodeID", *nodeID).Info("启动 Provider Node")

	// 创建 Redis 客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: *redisPass,
		DB:       0,
	})

	// 测试 Redis 连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.WithError(err).Fatal("无法连接到 Redis")
	}
	logger.Info("Redis 连接成功")

	// 创建提供商管理器
	providerManager := provider.NewProviderManager()

	// 注册腾讯提供商
	tencentProvider := tencent.NewProvider()

	// 应用装饰器
	decoratedProvider, err := decorators.ApplyDefaultDecorators(tencentProvider)
	if err != nil {
		logger.WithError(err).Warn("应用装饰器失败，使用原始提供商")
		decoratedProvider = tencentProvider
	}

	if err := providerManager.RegisterRealtimeStockProvider("tencent", decoratedProvider); err != nil {
		logger.WithError(err).Fatal("注册腾讯提供商失败")
	}

	// 创建任务执行器
	executor := &ProviderNodeExecutor{
		providerManager: providerManager,
		redisClient:     redisClient,
		nodeID:          *nodeID,
		logger:          logger,
	}

	// 创建任务调度器
	jobScheduler := scheduler.NewJobScheduler()
	jobScheduler.SetExecutor(executor)

	// 加载配置
	if err := jobScheduler.LoadConfig(*configPath); err != nil {
		logger.WithError(err).Fatal("加载任务配置失败")
	}

	// 启动调度器
	if err := jobScheduler.Start(); err != nil {
		logger.WithError(err).Fatal("启动任务调度器失败")
	}

	// 打印任务状态
	jobs := jobScheduler.GetAllJobs()
	logger.Infof("已加载 %d 个任务:", len(jobs))
	for _, job := range jobs {
		status := "启用"
		if !job.Config.Enabled {
			status = "禁用"
		}
		logger.Infof("  - %s (%s): %s", job.Config.Name, status, job.Config.Schedule)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Provider Node 运行中，按 Ctrl+C 停止...")
	<-sigChan

	logger.Info("正在停止 Provider Node...")

	// 停止调度器
	if err := jobScheduler.Stop(); err != nil {
		logger.WithError(err).Error("停止任务调度器失败")
	}

	// 关闭 Redis 连接
	if err := redisClient.Close(); err != nil {
		logger.WithError(err).Error("关闭 Redis 连接失败")
	}

	logger.Info("Provider Node 已停止")
}

// Execute 实现 JobExecutor 接口
func (e *ProviderNodeExecutor) Execute(ctx context.Context, job *scheduler.Job) error {
	logger := e.logger.WithFields(logrus.Fields{
		"job":    job.Config.Name,
		"jobID":  job.ID,
		"nodeID": e.nodeID,
	})

	logger.Info("开始执行任务")

	// 根据提供商类型获取提供商
	var provider core.RealtimeStockProvider
	var err error

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

	logger.Infof("获取 %d 个股票的数据: %v", len(symbols), symbols)

	// 获取股票数据
	stockDataList, err := provider.FetchStockData(ctx, symbols)
	if err != nil {
		return fmt.Errorf("获取股票数据失败: %w", err)
	}

	if len(stockDataList) == 0 {
		logger.Warn("没有获取到股票数据")
		return nil
	}

	// 转换为消息格式的股票数据
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
	}

	// 创建标准消息格式
	msg := message.NewMessageFormat(
		e.nodeID,
		job.Config.Provider.Name,
		"stock_realtime",
		messageStockData,
	)

	// 设置市场信息
	msg.SetMarketInfo("A-share", e.getTradingSession())

	// 转换为 JSON
	jsonData, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 发布到 Redis Streams
	streamName := message.GetStreamName("stock_realtime")
	result := e.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data": jsonData,
		},
	})

	if err := result.Err(); err != nil {
		return fmt.Errorf("发布消息到 Redis Streams 失败: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"stream":    streamName,
		"messageID": result.Val(),
		"dataCount": len(messageStockData),
	}).Info("消息发布成功")

	return nil
}

// extractSymbols 从任务参数中提取股票符号
func (e *ProviderNodeExecutor) extractSymbols(params map[string]interface{}) ([]string, error) {
	symbolsParam, exists := params["symbols"]
	if !exists {
		return nil, fmt.Errorf("参数中缺少 symbols")
	}

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
		return symbols, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("symbols 参数格式无效")
	}
}

// getTradingSession 获取当前交易时段
func (e *ProviderNodeExecutor) getTradingSession() string {
	now := time.Now()
	hour := now.Hour()

	if hour >= 9 && hour < 12 {
		return "morning"
	} else if hour >= 13 && hour < 15 {
		return "afternoon"
	} else {
		return "closed"
	}
}
