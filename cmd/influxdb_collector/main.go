package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"stocksub/pkg/message"
)

var (
	logLevel  = flag.String("log-level", "info", "日志级别 (debug, info, warn, error)")
	logFormat = flag.String("log-format", "json", "日志格式 (json or text)")
)

type InfluxDBCollector struct {
	redisClient     *redis.Client
	influxClient    influxdb2.Client
	writeAPI        api.WriteAPI
	consumerGroup   string
	consumerName    string
	streams         []string
	logger          *logrus.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	processedMsgIDs map[string]bool // 用于幂等处理
}

type Config struct {
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`

	InfluxDB struct {
		URL    string `mapstructure:"url"`
		Token  string `mapstructure:"token"`
		Org    string `mapstructure:"org"`
		Bucket string `mapstructure:"bucket"`
	} `mapstructure:"influxdb"`

	Consumer struct {
		Group   string   `mapstructure:"group"`
		Name    string   `mapstructure:"name"`
		Streams []string `mapstructure:"streams"`
	} `mapstructure:"consumer"`
}

func main() {
	flag.Parse()

	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("无效的日志级别")
	}
	logger.SetLevel(level)

	switch *logFormat {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{})
	default:
		logger.Fatal("无效的日志格式")
	}

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Create collector
	collector, err := NewInfluxDBCollector(config, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create InfluxDB collector")
	}
	defer collector.Close()

	// Start collector
	if err := collector.Start(); err != nil {
		logger.WithError(err).Fatal("Failed to start collector")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down InfluxDB collector...")
	collector.Stop()
}

func loadConfig() (*Config, error) {
	viper.SetConfigName("influxdb_collector")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("influxdb.url", "http://localhost:8086")
	viper.SetDefault("influxdb.token", "")
	viper.SetDefault("influxdb.org", "stocksub")
	viper.SetDefault("influxdb.bucket", "stock_data")
	viper.SetDefault("consumer.group", "influxdb_collectors")
	viper.SetDefault("consumer.name", "influxdb_collector_1")
	viper.SetDefault("consumer.streams", []string{
		"stream:stock:realtime",
		"stream:index:realtime",
	})

	// Environment variable overrides
	viper.SetEnvPrefix("INFLUXDB_COLLECTOR")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func NewInfluxDBCollector(config *Config, logger *logrus.Logger) (*InfluxDBCollector, error) {
	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.Redis.Addr,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create InfluxDB client
	influxClient := influxdb2.NewClient(config.InfluxDB.URL, config.InfluxDB.Token)

	// Test InfluxDB connection
	health, err := influxClient.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to InfluxDB: %w", err)
	}
	if health.Status != "pass" {
		return nil, fmt.Errorf("InfluxDB health check failed: %s", health.Status)
	}

	// Create write API
	writeAPI := influxClient.WriteAPI(config.InfluxDB.Org, config.InfluxDB.Bucket)

	ctx, cancel = context.WithCancel(context.Background())

	return &InfluxDBCollector{
		redisClient:     redisClient,
		influxClient:    influxClient,
		writeAPI:        writeAPI,
		consumerGroup:   config.Consumer.Group,
		consumerName:    config.Consumer.Name,
		streams:         config.Consumer.Streams,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		processedMsgIDs: make(map[string]bool),
	}, nil
}

func (c *InfluxDBCollector) Start() error {
	c.logger.Info("Starting InfluxDB collector...")

	// Create consumer groups for all streams
	for _, stream := range c.streams {
		err := c.redisClient.XGroupCreateMkStream(c.ctx, stream, c.consumerGroup, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("failed to create consumer group for stream %s: %w", stream, err)
		}
	}

	// Start consuming messages
	go c.consumeMessages()

	// Start error handling for write API
	go c.handleWriteErrors()

	c.logger.WithFields(logrus.Fields{
		"consumer_group": c.consumerGroup,
		"consumer_name":  c.consumerName,
		"streams":        c.streams,
	}).Info("InfluxDB collector started successfully")

	return nil
}

func (c *InfluxDBCollector) Stop() {
	c.logger.Info("Stopping InfluxDB collector...")
	c.cancel()

	// Flush any remaining writes
	c.writeAPI.Flush()

	c.logger.Info("InfluxDB collector stopped")
}

func (c *InfluxDBCollector) Close() {
	if c.redisClient != nil {
		c.redisClient.Close()
	}
	if c.influxClient != nil {
		c.influxClient.Close()
	}
}

func (c *InfluxDBCollector) consumeMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Read messages from all streams
			streams := make([]string, 0, len(c.streams)*2)
			for _, stream := range c.streams {
				streams = append(streams, stream, ">")
			}

			result, err := c.redisClient.XReadGroup(c.ctx, &redis.XReadGroupArgs{
				Group:    c.consumerGroup,
				Consumer: c.consumerName,
				Streams:  streams,
				Count:    10,
				Block:    time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue // No messages available
				}
				c.logger.WithError(err).Error("Failed to read messages from Redis streams")
				time.Sleep(time.Second)
				continue
			}

			// Process messages
			for _, stream := range result {
				for _, msg := range stream.Messages {
					if err := c.processMessage(stream.Stream, msg); err != nil {
						c.logger.WithError(err).WithFields(logrus.Fields{
							"stream":     stream.Stream,
							"message_id": msg.ID,
						}).Error("Failed to process message")
						continue
					}

					// Acknowledge message
					if err := c.redisClient.XAck(c.ctx, stream.Stream, c.consumerGroup, msg.ID).Err(); err != nil {
						c.logger.WithError(err).WithFields(logrus.Fields{
							"stream":     stream.Stream,
							"message_id": msg.ID,
						}).Error("Failed to acknowledge message")
					}
				}
			}
		}
	}
}

func (c *InfluxDBCollector) processMessage(streamName string, msg redis.XMessage) error {
	// 幂等处理：检查消息是否已处理过
	if c.processedMsgIDs[msg.ID] {
		c.logger.WithFields(logrus.Fields{
			"stream":     streamName,
			"message_id": msg.ID,
		}).Debug("Message already processed, skipping")
		return nil
	}

	// Extract message data
	data, ok := msg.Values["data"].(string)
	if !ok {
		return fmt.Errorf("message data is not a string")
	}

	// Parse message format
	var msgFormat message.MessageFormat
	if err := json.Unmarshal([]byte(data), &msgFormat); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Verify checksum
	if err := msgFormat.Validate(); err != nil {
		return fmt.Errorf("message checksum verification failed: %w", err)
	}

	// Process based on data type
	var processErr error
	switch msgFormat.Metadata.DataType {
	case "stock_realtime":
		processErr = c.processStockData(&msgFormat)
	case "index_realtime":
		processErr = c.processIndexData(&msgFormat)
	default:
		c.logger.WithField("data_type", msgFormat.Metadata.DataType).Warn("Unknown data type, skipping")
		return nil
	}

	// 如果处理成功，标记消息为已处理
	if processErr == nil {
		c.processedMsgIDs[msg.ID] = true

		// 定期清理已处理消息记录（防止内存泄漏）
		if len(c.processedMsgIDs) > 10000 {
			go c.cleanupProcessedMessages()
		}
	}

	return processErr
}

func (c *InfluxDBCollector) processStockData(msgFormat *message.MessageFormat) error {
	// First convert payload to JSON bytes
	payloadBytes, err := json.Marshal(msgFormat.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	var stockData []message.StockData
	if err := json.Unmarshal(payloadBytes, &stockData); err != nil {
		return fmt.Errorf("failed to unmarshal stock data: %w", err)
	}

	// Convert to InfluxDB points
	for _, stock := range stockData {
		// Parse timestamp string to time.Time
		timestamp, err := time.Parse(time.RFC3339, stock.Timestamp)
		if err != nil {
			c.logger.WithError(err).WithField("timestamp", stock.Timestamp).Warn("Failed to parse timestamp, using current time")
			timestamp = time.Now()
		}

		point := influxdb2.NewPointWithMeasurement("stock_realtime").
			AddTag("symbol", stock.Symbol).
			AddTag("name", stock.Name).
			AddTag("provider", msgFormat.Metadata.Provider).
			AddTag("market", msgFormat.Metadata.Market).
			AddField("price", stock.Price).
			AddField("change", stock.Change).
			AddField("change_percent", stock.ChangePercent).
			AddField("volume", stock.Volume).
			SetTime(timestamp)

		c.writeAPI.WritePoint(point)
	}

	c.logger.WithFields(logrus.Fields{
		"count":    len(stockData),
		"provider": msgFormat.Metadata.Provider,
	}).Debug("Processed stock data points")

	return nil
}

func (c *InfluxDBCollector) processIndexData(msgFormat *message.MessageFormat) error {
	// First convert payload to JSON bytes
	payloadBytes, err := json.Marshal(msgFormat.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	var indexData []message.IndexData
	if err := json.Unmarshal(payloadBytes, &indexData); err != nil {
		return fmt.Errorf("failed to unmarshal index data: %w", err)
	}

	// Convert to InfluxDB points
	for _, index := range indexData {
		// Parse timestamp string to time.Time
		timestamp, err := time.Parse(time.RFC3339, index.Timestamp)
		if err != nil {
			c.logger.WithError(err).WithField("timestamp", index.Timestamp).Warn("Failed to parse timestamp, using current time")
			timestamp = time.Now()
		}

		point := influxdb2.NewPointWithMeasurement("index_realtime").
			AddTag("symbol", index.Symbol).
			AddTag("name", index.Name).
			AddTag("provider", msgFormat.Metadata.Provider).
			AddTag("market", msgFormat.Metadata.Market).
			AddField("value", index.Value).
			AddField("change", index.Change).
			AddField("change_percent", index.ChangePercent).
			SetTime(timestamp)

		c.writeAPI.WritePoint(point)
	}

	c.logger.WithFields(logrus.Fields{
		"count":    len(indexData),
		"provider": msgFormat.Metadata.Provider,
	}).Debug("Processed index data points")

	return nil
}

func (c *InfluxDBCollector) handleWriteErrors() {
	errorsCh := c.writeAPI.Errors()
	for {
		select {
		case <-c.ctx.Done():
			return
		case err := <-errorsCh:
			c.logger.WithError(err).Error("InfluxDB write error")
		}
	}
}

// cleanupProcessedMessages 定期清理已处理消息记录，防止内存泄漏
func (c *InfluxDBCollector) cleanupProcessedMessages() {
	c.logger.Debug("Cleaning up processed messages cache")

	// 保留最近的 5000 条记录，删除其余的
	if len(c.processedMsgIDs) > 5000 {
		// 简单的清理策略：清空一半
		newMap := make(map[string]bool)
		count := 0
		for msgID, processed := range c.processedMsgIDs {
			if count < 5000 {
				newMap[msgID] = processed
				count++
			}
		}
		c.processedMsgIDs = newMap

		c.logger.WithFields(logrus.Fields{
			"remaining_count": len(c.processedMsgIDs),
			"cleaned_count":   len(c.processedMsgIDs) - 5000,
		}).Info("Processed messages cache cleaned up")
	}
}
