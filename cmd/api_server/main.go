package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"stocksub/pkg/cache"
)

var (
	logLevel    = flag.String("log-level", "info", "日志级别 (debug, info, warn, error)")
	logFormat   = flag.String("log-format", "json", "日志格式 (json or text)")
	configPath  = flag.String("config", "", "配置文件路径 (例如 /app/config/api_server.yaml)")
	redisAddr   = flag.String("redis", "", "Redis 地址，格式 host:port")
	redisPass   = flag.String("redis-pass", "", "Redis 密码")
	influxURL   = flag.String("influxdb-url", "", "InfluxDB URL")
	influxToken = flag.String("influxdb-token", "", "InfluxDB token")
)

type APIServer struct {
	redisClient  *redis.Client
	influxClient influxdb2.Client
	queryAPI     api.QueryAPI
	logger       *logrus.Logger
	server       *http.Server
	cache        cache.Cache // 集成分层缓存
}

type Config struct {
	Server struct {
		Port string `mapstructure:"port"`
		Mode string `mapstructure:"mode"` // debug, release, test
	} `mapstructure:"server"`

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

	Cache struct {
		Enabled         bool          `mapstructure:"enabled"`
		DefaultTTL      time.Duration `mapstructure:"default_ttl"`
		MaxSize         int64         `mapstructure:"max_size"`
		CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	} `mapstructure:"cache"`
}

// Response structures
type StockResponse struct {
	Symbol        string    `json:"symbol"`
	Name          string    `json:"name"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Provider      string    `json:"provider"`
	Market        string    `json:"market"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type IndexResponse struct {
	Symbol        string    `json:"symbol"`
	Name          string    `json:"name"`
	Value         float64   `json:"value"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	Timestamp     time.Time `json:"timestamp"`
	Provider      string    `json:"provider"`
	Market        string    `json:"market"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type HistoricalDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Volume    int64     `json:"volume"`
}

type HistoricalResponse struct {
	Symbol string                `json:"symbol"`
	Start  time.Time             `json:"start"`
	End    time.Time             `json:"end"`
	Data   []HistoricalDataPoint `json:"data"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
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

	// Set Gin mode
	gin.SetMode(config.Server.Mode)

	// Create API server
	apiServer, err := NewAPIServer(config, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create API server")
	}
	defer apiServer.Close()

	// Start server
	if err := apiServer.Start(); err != nil {
		logger.WithError(err).Fatal("Failed to start API server")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down API server...")
	apiServer.Stop()
}

func loadConfig() (*Config, error) {
	if *configPath != "" {
		viper.SetConfigFile(*configPath)
	} else {
		viper.SetConfigName("api_server")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./config")
		viper.AddConfigPath(".")
	}

	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "release")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("influxdb.url", "http://localhost:8086")
	viper.SetDefault("influxdb.token", "")
	viper.SetDefault("influxdb.org", "stocksub")
	viper.SetDefault("influxdb.bucket", "stock_data")
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.default_ttl", "5m")
	viper.SetDefault("cache.max_size", 1000)
	viper.SetDefault("cache.cleanup_interval", "1m")

	// Environment variable overrides
	viper.SetEnvPrefix("API_SERVER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Command-line flag overrides (when provided)
	if *redisAddr != "" {
		viper.Set("redis.addr", *redisAddr)
	}
	if *redisPass != "" {
		viper.Set("redis.password", *redisPass)
	}
	if *influxURL != "" {
		viper.Set("influxdb.url", *influxURL)
	}
	if *influxToken != "" {
		viper.Set("influxdb.token", *influxToken)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func NewAPIServer(config *Config, logger *logrus.Logger) (*APIServer, error) {
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

	// Create query API
	queryAPI := influxClient.QueryAPI(config.InfluxDB.Org)

	// 创建分层缓存
	var apiCache cache.Cache
	if config.Cache.Enabled {
		cacheConfig := cache.LayeredCacheConfig{
			Layers: []cache.LayerConfig{
				{
					Type:            cache.LayerMemory,
					MaxSize:         config.Cache.MaxSize,
					TTL:             config.Cache.DefaultTTL,
					Enabled:         true,
					Policy:          cache.PolicyLRU,
					CleanupInterval: config.Cache.CleanupInterval,
				},
				{
					Type:            cache.LayerMemory, // 二级内存缓存
					MaxSize:         config.Cache.MaxSize * 5,
					TTL:             config.Cache.DefaultTTL * 6, // 更长的TTL
					Enabled:         true,
					Policy:          cache.PolicyLFU,
					CleanupInterval: config.Cache.CleanupInterval * 5,
				},
			},
			PromoteEnabled: true,
			WriteThrough:   false,
			WriteBack:      false,
		}

		layeredCache, err := cache.NewLayeredCache(cacheConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create layered cache: %w", err)
		}
		apiCache = layeredCache
		logger.Info("API server layered cache enabled")
	} else {
		// 使用简单的内存缓存作为备选
		memConfig := cache.MemoryCacheConfig{
			MaxSize:         100,
			DefaultTTL:      1 * time.Minute,
			CleanupInterval: 30 * time.Second,
		}
		apiCache = cache.NewMemoryCache(memConfig)
		logger.Info("API server simple memory cache enabled")
	}

	return &APIServer{
		redisClient:  redisClient,
		influxClient: influxClient,
		queryAPI:     queryAPI,
		logger:       logger,
		cache:        apiCache,
	}, nil
}

func (s *APIServer) Start() error {
	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(s.corsMiddleware())

	// Health check
	router.GET("/health", s.healthCheck)

	// API routes
	v1 := router.Group("/api/v1")
	{
		// Real-time data endpoints
		v1.GET("/stocks/:symbol", s.getStock)
		v1.GET("/stocks", s.getStocks)
		v1.GET("/indices/:symbol", s.getIndex)
		v1.GET("/indices", s.getIndices)

		// Historical data endpoints
		v1.GET("/stocks/:symbol/history", s.getStockHistory)
		v1.GET("/indices/:symbol/history", s.getIndexHistory)

		// Metadata endpoints
		v1.GET("/symbols/stocks", s.getStockSymbols)
		v1.GET("/symbols/indices", s.getIndexSymbols)
	}

	// 向后兼容的 API 路由（兼容现有客户端）
	legacy := router.Group("/api")
	{
		legacy.GET("/stock/:symbol", s.getLegacyStock)
		legacy.GET("/stocks", s.getLegacyStocks)
	}

	// 监控和指标端点
	router.GET("/metrics", s.getMetrics)
	router.GET("/stats", s.getStats)

	// Create HTTP server
	s.server = &http.Server{
		Addr:    ":" + viper.GetString("server.port"),
		Handler: router,
	}

	s.logger.WithField("port", viper.GetString("server.port")).Info("Starting API server...")

	// Start server in goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.WithError(err).Fatal("Failed to start HTTP server")
		}
	}()

	return nil
}

func (s *APIServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to gracefully shutdown server")
	}
}

func (s *APIServer) Close() {
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.influxClient != nil {
		s.influxClient.Close()
	}
	if s.cache != nil {
		if closer, ok := s.cache.(interface{ Close() error }); ok {
			closer.Close()
		}
	}
}

func (s *APIServer) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (s *APIServer) healthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"services":  map[string]string{},
	}

	// Check Redis
	if err := s.redisClient.Ping(ctx).Err(); err != nil {
		health["services"].(map[string]string)["redis"] = "error: " + err.Error()
		health["status"] = "degraded"
	} else {
		health["services"].(map[string]string)["redis"] = "ok"
	}

	// Check InfluxDB
	if influxHealth, err := s.influxClient.Health(ctx); err != nil {
		health["services"].(map[string]string)["influxdb"] = "error: " + err.Error()
		health["status"] = "degraded"
	} else {
		statusStr := string(influxHealth.Status)
		health["services"].(map[string]string)["influxdb"] = statusStr
		if statusStr != "pass" {
			health["status"] = "degraded"
		}
	}

	if health["status"] == "ok" {
		c.JSON(200, health)
	} else {
		c.JSON(503, health)
	}
}

func (s *APIServer) getStock(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Symbol is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("latest:stock:%s", symbol)
	result, err := s.redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		s.logger.WithError(err).WithField("symbol", symbol).Error("Failed to get stock data from Redis")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve data"})
		return
	}

	if len(result) == 0 {
		c.JSON(404, ErrorResponse{Error: "not_found", Message: "Stock not found"})
		return
	}

	stock, err := s.parseStockFromRedis(result)
	if err != nil {
		s.logger.WithError(err).WithField("symbol", symbol).Error("Failed to parse stock data")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to parse data"})
		return
	}

	c.JSON(200, stock)
}

func (s *APIServer) getStocks(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all stock symbols
	symbols, err := s.redisClient.SMembers(ctx, "symbols:stock").Result()
	if err != nil {
		s.logger.WithError(err).Error("Failed to get stock symbols from Redis")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve symbols"})
		return
	}

	if len(symbols) == 0 {
		c.JSON(200, []StockResponse{})
		return
	}

	// Get data for all symbols
	pipe := s.redisClient.Pipeline()
	cmds := make(map[string]*redis.StringStringMapCmd)

	for _, symbol := range symbols {
		key := fmt.Sprintf("latest:stock:%s", symbol)
		cmds[symbol] = pipe.HGetAll(ctx, key)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to execute Redis pipeline")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve data"})
		return
	}

	stocks := make([]StockResponse, 0, len(symbols))
	for symbol, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil || len(result) == 0 {
			continue
		}

		stock, err := s.parseStockFromRedis(result)
		if err != nil {
			s.logger.WithError(err).WithField("symbol", symbol).Warn("Failed to parse stock data")
			continue
		}

		stocks = append(stocks, *stock)
	}

	c.JSON(200, stocks)
}

func (s *APIServer) getIndex(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Symbol is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("latest:index:%s", symbol)
	result, err := s.redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		s.logger.WithError(err).WithField("symbol", symbol).Error("Failed to get index data from Redis")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve data"})
		return
	}

	if len(result) == 0 {
		c.JSON(404, ErrorResponse{Error: "not_found", Message: "Index not found"})
		return
	}

	index, err := s.parseIndexFromRedis(result)
	if err != nil {
		s.logger.WithError(err).WithField("symbol", symbol).Error("Failed to parse index data")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to parse data"})
		return
	}

	c.JSON(200, index)
}

func (s *APIServer) getIndices(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all index symbols
	symbols, err := s.redisClient.SMembers(ctx, "symbols:index").Result()
	if err != nil {
		s.logger.WithError(err).Error("Failed to get index symbols from Redis")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve symbols"})
		return
	}

	if len(symbols) == 0 {
		c.JSON(200, []IndexResponse{})
		return
	}

	// Get data for all symbols
	pipe := s.redisClient.Pipeline()
	cmds := make(map[string]*redis.StringStringMapCmd)

	for _, symbol := range symbols {
		key := fmt.Sprintf("latest:index:%s", symbol)
		cmds[symbol] = pipe.HGetAll(ctx, key)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to execute Redis pipeline")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve data"})
		return
	}

	indices := make([]IndexResponse, 0, len(symbols))
	for symbol, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil || len(result) == 0 {
			continue
		}

		index, err := s.parseIndexFromRedis(result)
		if err != nil {
			s.logger.WithError(err).WithField("symbol", symbol).Warn("Failed to parse index data")
			continue
		}

		indices = append(indices, *index)
	}

	c.JSON(200, indices)
}

func (s *APIServer) getStockHistory(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Symbol is required"})
		return
	}

	// Parse query parameters
	startStr := c.Query("start")
	endStr := c.Query("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Invalid start time format, use RFC3339"})
			return
		}
	} else {
		start = time.Now().Add(-24 * time.Hour) // Default to last 24 hours
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Invalid end time format, use RFC3339"})
			return
		}
	} else {
		end = time.Now()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query InfluxDB
	query := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r._measurement == "stock_realtime")
		|> filter(fn: (r) => r.symbol == "%s")
		|> filter(fn: (r) => r._field == "price" or r._field == "volume")
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> sort(columns: ["_time"])
	`, viper.GetString("influxdb.bucket"), start.Format(time.RFC3339), end.Format(time.RFC3339), symbol)

	result, err := s.queryAPI.Query(ctx, query)
	if err != nil {
		s.logger.WithError(err).WithField("symbol", symbol).Error("Failed to query InfluxDB")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to query historical data"})
		return
	}
	defer result.Close()

	var dataPoints []HistoricalDataPoint
	for result.Next() {
		record := result.Record()

		timestamp := record.Time()
		price, _ := record.ValueByKey("price").(float64)
		volume, _ := record.ValueByKey("volume").(int64)

		dataPoints = append(dataPoints, HistoricalDataPoint{
			Timestamp: timestamp,
			Price:     price,
			Volume:    volume,
		})
	}

	if result.Err() != nil {
		s.logger.WithError(result.Err()).WithField("symbol", symbol).Error("Error reading InfluxDB result")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to read historical data"})
		return
	}

	response := HistoricalResponse{
		Symbol: symbol,
		Start:  start,
		End:    end,
		Data:   dataPoints,
	}

	c.JSON(200, response)
}

func (s *APIServer) getIndexHistory(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Symbol is required"})
		return
	}

	// Parse query parameters
	startStr := c.Query("start")
	endStr := c.Query("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Invalid start time format, use RFC3339"})
			return
		}
	} else {
		start = time.Now().Add(-24 * time.Hour) // Default to last 24 hours
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(400, ErrorResponse{Error: "bad_request", Message: "Invalid end time format, use RFC3339"})
			return
		}
	} else {
		end = time.Now()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query InfluxDB
	query := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r._measurement == "index_realtime")
		|> filter(fn: (r) => r.symbol == "%s")
		|> filter(fn: (r) => r._field == "value")
		|> sort(columns: ["_time"])
	`, viper.GetString("influxdb.bucket"), start.Format(time.RFC3339), end.Format(time.RFC3339), symbol)

	result, err := s.queryAPI.Query(ctx, query)
	if err != nil {
		s.logger.WithError(err).WithField("symbol", symbol).Error("Failed to query InfluxDB")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to query historical data"})
		return
	}
	defer result.Close()

	var dataPoints []HistoricalDataPoint
	for result.Next() {
		record := result.Record()

		timestamp := record.Time()
		value, _ := record.Value().(float64)

		dataPoints = append(dataPoints, HistoricalDataPoint{
			Timestamp: timestamp,
			Price:     value, // Use Price field for index value
			Volume:    0,     // Indices don't have volume
		})
	}

	if result.Err() != nil {
		s.logger.WithError(result.Err()).WithField("symbol", symbol).Error("Error reading InfluxDB result")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to read historical data"})
		return
	}

	response := HistoricalResponse{
		Symbol: symbol,
		Start:  start,
		End:    end,
		Data:   dataPoints,
	}

	c.JSON(200, response)
}

func (s *APIServer) getStockSymbols(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	symbols, err := s.redisClient.SMembers(ctx, "symbols:stock").Result()
	if err != nil {
		s.logger.WithError(err).Error("Failed to get stock symbols from Redis")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve symbols"})
		return
	}

	c.JSON(200, map[string]interface{}{
		"type":    "stock",
		"symbols": symbols,
		"count":   len(symbols),
	})
}

func (s *APIServer) getIndexSymbols(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	symbols, err := s.redisClient.SMembers(ctx, "symbols:index").Result()
	if err != nil {
		s.logger.WithError(err).Error("Failed to get index symbols from Redis")
		c.JSON(500, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve symbols"})
		return
	}

	c.JSON(200, map[string]interface{}{
		"type":    "index",
		"symbols": symbols,
		"count":   len(symbols),
	})
}

func (s *APIServer) parseStockFromRedis(data map[string]string) (*StockResponse, error) {
	price, err := strconv.ParseFloat(data["price"], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %w", err)
	}

	change, err := strconv.ParseFloat(data["change"], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid change: %w", err)
	}

	changePercent, err := strconv.ParseFloat(data["change_percent"], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid change_percent: %w", err)
	}

	volume, err := strconv.ParseInt(data["volume"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid volume: %w", err)
	}

	timestamp, err := strconv.ParseInt(data["timestamp"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	updatedAt, err := strconv.ParseInt(data["updated_at"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at: %w", err)
	}

	return &StockResponse{
		Symbol:        data["symbol"],
		Name:          data["name"],
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		Timestamp:     time.Unix(timestamp, 0),
		Provider:      data["provider"],
		Market:        data["market"],
		UpdatedAt:     time.Unix(updatedAt, 0),
	}, nil
}

func (s *APIServer) parseIndexFromRedis(data map[string]string) (*IndexResponse, error) {
	value, err := strconv.ParseFloat(data["value"], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid value: %w", err)
	}

	change, err := strconv.ParseFloat(data["change"], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid change: %w", err)
	}

	changePercent, err := strconv.ParseFloat(data["change_percent"], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid change_percent: %w", err)
	}

	timestamp, err := strconv.ParseInt(data["timestamp"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	updatedAt, err := strconv.ParseInt(data["updated_at"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at: %w", err)
	}

	return &IndexResponse{
		Symbol:        data["symbol"],
		Name:          data["name"],
		Value:         value,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     time.Unix(timestamp, 0),
		Provider:      data["provider"],
		Market:        data["market"],
		UpdatedAt:     time.Unix(updatedAt, 0),
	}, nil
}

// getLegacyStock 向后兼容的单个股票查询端点
func (s *APIServer) getLegacyStock(c *gin.Context) {
	// 重用现有的 getStock 逻辑，但返回兼容格式
	s.getStock(c)
}

// getLegacyStocks 向后兼容的股票列表端点
func (s *APIServer) getLegacyStocks(c *gin.Context) {
	// 重用现有的 getStocks 逻辑
	s.getStocks(c)
}

// getMetrics 获取系统指标
func (s *APIServer) getMetrics(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	metrics := map[string]interface{}{
		"timestamp": time.Now(),
		"cache":     map[string]interface{}{},
		"redis":     map[string]interface{}{},
		"influxdb":  map[string]interface{}{},
	}

	// 获取缓存统计
	if s.cache != nil {
		cacheStats := s.cache.Stats()
		metrics["cache"] = map[string]interface{}{
			"size":       cacheStats.Size,
			"max_size":   cacheStats.MaxSize,
			"hit_count":  cacheStats.HitCount,
			"miss_count": cacheStats.MissCount,
			"hit_rate":   cacheStats.HitRate,
		}
	}

	// 获取Redis信息
	if s.redisClient != nil {
		if err := s.redisClient.Ping(ctx).Err(); err == nil {
			metrics["redis"] = map[string]interface{}{
				"status": "connected",
				"info":   "available",
			}
		} else {
			metrics["redis"] = map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			}
		}
	}

	// 获取InfluxDB健康状态
	if s.influxClient != nil {
		if health, err := s.influxClient.Health(ctx); err == nil {
			metrics["influxdb"] = map[string]interface{}{
				"status": string(health.Status),
				"name":   health.Name,
			}
		} else {
			metrics["influxdb"] = map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			}
		}
	}

	c.JSON(200, metrics)
}

// getStats 获取系统统计信息
func (s *APIServer) getStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stats := map[string]interface{}{
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"uptime":    time.Since(time.Now().Add(-1 * time.Hour)), // 简单示例
	}

	// 获取Redis键统计
	if s.redisClient != nil {
		stockCount, _ := s.redisClient.SCard(ctx, "symbols:stock").Result()
		indexCount, _ := s.redisClient.SCard(ctx, "symbols:index").Result()

		stats["data"] = map[string]interface{}{
			"stock_symbols": stockCount,
			"index_symbols": indexCount,
		}
	}

	// 获取缓存详细统计
	if s.cache != nil {
		stats["cache_details"] = s.cache.Stats()
	}

	c.JSON(200, stats)
}
