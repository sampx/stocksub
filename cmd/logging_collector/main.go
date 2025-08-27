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

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

var (
	redisAddr     = flag.String("redis", "localhost:6379", "Redis 服务器地址")
	redisPass     = flag.String("redis-pass", "", "Redis 密码")
	consumerID    = flag.String("consumer-id", "", "消费者ID（默认自动生成）")
	consumerGroup = flag.String("consumer-group", "logging-collectors", "消费者组名称")
	streams       = flag.String("streams", "stream:stock:realtime,stream:index:realtime", "要监听的流名称（逗号分隔）")
	logLevel      = flag.String("log-level", "info", "日志级别")
)

type LoggingCollector struct {
	redisClient   *redis.Client
	consumerID    string
	consumerGroup string
	streamNames   []string
	logger        *logrus.Logger
	ctx           context.Context
	cancel        context.CancelFunc
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

	// 生成消费者ID
	if *consumerID == "" {
		*consumerID = fmt.Sprintf("logging-collector-%d", time.Now().Unix())
	}

	logger.WithField("consumerID", *consumerID).Info("启动 Logging Collector")

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

	// 解析流名称
	streamNames := parseStreams(*streams)
	logger.Infof("监听流: %v", streamNames)

	// 创建收集器
	ctx, cancel = context.WithCancel(context.Background())
	collector := &LoggingCollector{
		redisClient:   redisClient,
		consumerID:    *consumerID,
		consumerGroup: *consumerGroup,
		streamNames:   streamNames,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}

	// 初始化消费者组
	if err := collector.initConsumerGroups(); err != nil {
		logger.WithError(err).Fatal("初始化消费者组失败")
	}

	// 启动消费者
	go collector.startConsuming()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Logging Collector 运行中，按 Ctrl+C 停止...")
	<-sigChan

	logger.Info("正在停止 Logging Collector...")
	cancel()

	// 关闭 Redis 连接
	if err := redisClient.Close(); err != nil {
		logger.WithError(err).Error("关闭 Redis 连接失败")
	}

	logger.Info("Logging Collector 已停止")
}

// initConsumerGroups 初始化消费者组
func (c *LoggingCollector) initConsumerGroups() error {
	for _, streamName := range c.streamNames {
		// 尝试创建消费者组，如果已存在则忽略错误
		err := c.redisClient.XGroupCreate(c.ctx, streamName, c.consumerGroup, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			c.logger.WithError(err).Errorf("创建消费者组失败: %s", streamName)
			return err
		}
		c.logger.Infof("消费者组已就绪: %s -> %s", streamName, c.consumerGroup)
	}
	return nil
}

// startConsuming 开始消费消息
func (c *LoggingCollector) startConsuming() {
	c.logger.Info("开始消费消息...")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("收到停止信号，退出消费循环")
			return
		default:
			// 构建流参数
			streams := make([]string, len(c.streamNames)*2)
			for i, streamName := range c.streamNames {
				streams[i*2] = streamName
				streams[i*2+1] = ">" // 只读取新消息
			}

			// 从消费者组读取消息
			result := c.redisClient.XReadGroup(c.ctx, &redis.XReadGroupArgs{
				Group:    c.consumerGroup,
				Consumer: c.consumerID,
				Streams:  streams,
				Count:    10,
				Block:    time.Second * 5,
			})

			if err := result.Err(); err != nil {
				if err == redis.Nil {
					// 没有新消息，继续等待
					continue
				}
				c.logger.WithError(err).Error("读取消息失败")
				time.Sleep(time.Second)
				continue
			}

			// 处理消息
			for _, stream := range result.Val() {
				for _, msg := range stream.Messages {
					c.processMessage(stream.Stream, msg)
				}
			}
		}
	}
}

// processMessage 处理单个消息
func (c *LoggingCollector) processMessage(streamName string, msg redis.XMessage) {
	logger := c.logger.WithFields(logrus.Fields{
		"stream":    streamName,
		"messageID": msg.ID,
	})

	// 获取消息数据
	dataStr, exists := msg.Values["data"]
	if !exists {
		logger.Error("消息中缺少 data 字段")
		c.ackMessage(streamName, msg.ID)
		return
	}

	dataJSON, ok := dataStr.(string)
	if !ok {
		logger.Error("data 字段不是字符串类型")
		c.ackMessage(streamName, msg.ID)
		return
	}

	// 解析消息
	messageFormat, err := message.FromJSON(dataJSON)
	if err != nil {
		logger.WithError(err).Error("解析消息失败")
		c.ackMessage(streamName, msg.ID)
		return
	}

	// 验证消息完整性
	if err := messageFormat.Validate(); err != nil {
		logger.WithError(err).Error("消息校验失败")
		c.ackMessage(streamName, msg.ID)
		return
	}

	// 打印消息信息
	logger.WithFields(logrus.Fields{
		"producer":  messageFormat.Header.Producer,
		"provider":  messageFormat.Metadata.Provider,
		"dataType":  messageFormat.Metadata.DataType,
		"batchSize": messageFormat.Metadata.BatchSize,
		"market":    messageFormat.Metadata.Market,
		"session":   messageFormat.Metadata.TradingSession,
		"timestamp": time.Unix(messageFormat.Header.Timestamp, 0).Format(time.RFC3339),
	}).Info("收到消息")

	// 打印股票数据详情
	if stockDataList, ok := messageFormat.Payload.([]interface{}); ok {
		for i, stockDataInterface := range stockDataList {
			if stockDataMap, ok := stockDataInterface.(map[string]interface{}); ok {
				symbol, _ := stockDataMap["symbol"].(string)
				name, _ := stockDataMap["name"].(string)
				price, _ := stockDataMap["price"].(float64)
				change, _ := stockDataMap["change"].(float64)
				changePercent, _ := stockDataMap["changePercent"].(float64)
				volume, _ := stockDataMap["volume"].(float64)

				logger.WithFields(logrus.Fields{
					"index":         i + 1,
					"symbol":        symbol,
					"name":          name,
					"price":         price,
					"change":        change,
					"changePercent": changePercent,
					"volume":        int64(volume),
				}).Info("股票数据")
			}
		}
	}

	// 确认消息处理完成
	c.ackMessage(streamName, msg.ID)
}

// ackMessage 确认消息处理完成
func (c *LoggingCollector) ackMessage(streamName, messageID string) {
	err := c.redisClient.XAck(c.ctx, streamName, c.consumerGroup, messageID).Err()
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"stream":    streamName,
			"messageID": messageID,
		}).Error("确认消息失败")
	}
}

// parseStreams 解析流名称字符串
func parseStreams(streamsStr string) []string {
	if streamsStr == "" {
		return []string{"stream:stock:realtime"}
	}

	var streams []string
	for _, stream := range splitString(streamsStr, ",") {
		if trimmed := trimSpace(stream); trimmed != "" {
			streams = append(streams, trimmed)
		}
	}

	if len(streams) == 0 {
		return []string{"stream:stock:realtime"}
	}

	return streams
}

// splitString 分割字符串
func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}

	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace 去除字符串两端空格
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
