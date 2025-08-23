// Package config 定义了 testkit 框架的所有配置选项。
// 通过这些结构体，用户可以灵活地配置缓存、存储、数据提供者和Mock等行为。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config 是 testkit 的主配置结构，聚合了所有子模块的配置。
type Config struct {
	Storage     StorageConfig     `json:"storage" yaml:"storage"`
	Cache       CacheConfig       `json:"cache" yaml:"cache"`
	Provider    ProviderConfig    `json:"provider" yaml:"provider"`
	Performance PerformanceConfig `json:"performance" yaml:"performance"`
	Logging     LoggingConfig     `json:"logging" yaml:"logging"`
	Mock        MockConfig        `json:"mock" yaml:"mock"`
}

// StorageConfig 定义了持久化存储的配置。
type StorageConfig struct {
	Type         string        `json:"type" yaml:"type"`                   // 存储类型，如 "csv", "memory", "json"。
	Directory    string        `json:"directory" yaml:"directory"`         // 存储目录的路径。
	MaxFileSize  int64         `json:"max_file_size" yaml:"max_file_size"` // 单个文件的最大大小（字节）。
	MaxFiles     int           `json:"max_files" yaml:"max_files"`         // 最多保留的文件数量。
	Compression  bool          `json:"compression" yaml:"compression"`     // 是否对旧的存储文件启用压缩。
	AutoCleanup  bool          `json:"auto_cleanup" yaml:"auto_cleanup"`   // 是否自动清理过期的文件。
	CleanupAge   time.Duration `json:"cleanup_age" yaml:"cleanup_age"`     // 文件的最大保留时间。
	BufferSize   int           `json:"buffer_size" yaml:"buffer_size"`     // 写入文件时的缓冲区大小。
	SyncInterval time.Duration `json:"sync_interval" yaml:"sync_interval"` // 将缓冲区数据同步到磁盘的间隔。
}

// CacheConfig 定义了缓存的配置。
type CacheConfig struct {
	Type            string        `json:"type" yaml:"type"`                         // 缓存类型，如 "memory", "layered"。
	MaxSize         int64         `json:"max_size" yaml:"max_size"`                 // 缓存中最多保留的条目数。
	MaxMemory       int64         `json:"max_memory" yaml:"max_memory"`             // 缓存占用的最大内存（字节）。
	TTL             time.Duration `json:"ttl" yaml:"ttl"`                           // 缓存条目的默认生存时间。
	EvictionPolicy  string        `json:"eviction_policy" yaml:"eviction_policy"`   // 缓存淘汰策略，如 "lru", "lfu", "fifo"。
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"` // 清理过期缓存条目的时间间隔。
	Layers          []LayerConfig `json:"layers" yaml:"layers"`                     // 当 Type 为 "layered" 时，定义各缓存层的配置。
}

// LayerConfig 定义了分层缓存中每一层的具体配置。
type LayerConfig struct {
	Name           string        `json:"name" yaml:"name"`                       // 缓存层的名称，如 "L1", "L2"。
	Type           string        `json:"type" yaml:"type"`                       // 缓存层的类型，如 "memory", "disk"。
	MaxSize        int64         `json:"max_size" yaml:"max_size"`               // 该层的最大条目数。
	TTL            time.Duration `json:"ttl" yaml:"ttl"`                         // 该层的默认生存时间。
	EvictionPolicy string        `json:"eviction_policy" yaml:"eviction_policy"` // 该层的淘汰策略。
}

// ProviderConfig 定义了数据提供者的配置。
type ProviderConfig struct {
	Type            string            `json:"type" yaml:"type"`                         // 提供者类型，如 "tencent", "mock", "hybrid"。
	Timeout         time.Duration     `json:"timeout" yaml:"timeout"`                   // API请求的超时时间。
	RetryAttempts   int               `json:"retry_attempts" yaml:"retry_attempts"`     // 请求失败时的最大重试次数。
	RetryDelay      time.Duration     `json:"retry_delay" yaml:"retry_delay"`           // 每次重试之间的延迟。
	ConcurrentLimit int               `json:"concurrent_limit" yaml:"concurrent_limit"` // 最大并发请求数。
	RateLimitQPS    int               `json:"rate_limit_qps" yaml:"rate_limit_qps"`     // 每秒请求速率限制 (QPS)。
	UserAgent       string            `json:"user_agent" yaml:"user_agent"`             // 发起HTTP请求时使用的User-Agent。
	Headers         map[string]string `json:"headers" yaml:"headers"`                   // 附加到HTTP请求中的自定义头部。
}

// PerformanceConfig 定义了与性能相关的配置。
type PerformanceConfig struct {
	WorkerCount     int           `json:"worker_count" yaml:"worker_count"`         // 用于处理后台任务的工作协程数。
	BatchSize       int           `json:"batch_size" yaml:"batch_size"`             // 批处理操作的大小。
	MaxConcurrency  int           `json:"max_concurrency" yaml:"max_concurrency"`   // 框架内部允许的最大并发数。
	MemoryLimit     int64         `json:"memory_limit" yaml:"memory_limit"`         // 框架的总内存使用限制（字节）。
	GCInterval      time.Duration `json:"gc_interval" yaml:"gc_interval"`           // 强制进行垃圾回收（GC）的时间间隔。
	MetricsInterval time.Duration `json:"metrics_interval" yaml:"metrics_interval"` // 收集和报告性能指标的时间间隔。
	EnableProfiling bool          `json:"enable_profiling" yaml:"enable_profiling"` // 是否启用性能分析（如 pprof）。
	EnableMetrics   bool          `json:"enable_metrics" yaml:"enable_metrics"`     // 是否启用指标收集。
}

// LoggingConfig 定义了日志记录的配置。
type LoggingConfig struct {
	Level        string `json:"level" yaml:"level"`                 // 日志级别，如 "debug", "info", "warn", "error"。
	Format       string `json:"format" yaml:"format"`               // 日志格式，如 "json", "text"。
	Output       string `json:"output" yaml:"output"`               // 日志输出位置，如 "stdout", "file", "both"。
	File         string `json:"file" yaml:"file"`                   // 日志文件的路径。
	MaxSize      int    `json:"max_size" yaml:"max_size"`           // 日志文件的最大大小（MB）。
	MaxBackups   int    `json:"max_backups" yaml:"max_backups"`     // 保留的旧日志文件的最大数量。
	MaxAge       int    `json:"max_age" yaml:"max_age"`             // 日志文件的最大保留天数。
	Compress     bool   `json:"compress" yaml:"compress"`           // 是否压缩归档的日志文件。
	EnableCaller bool   `json:"enable_caller" yaml:"enable_caller"` // 是否在日志中记录调用者的文件和行号。
	EnableTrace  bool   `json:"enable_trace" yaml:"enable_trace"`   // 是否启用全链路跟踪日志。
}

// MockConfig 定义了Mock功能相关的配置。
type MockConfig struct {
	Enabled       bool                   `json:"enabled" yaml:"enabled"`               // 是否全局启用Mock模式。
	ScenarioFile  string                 `json:"scenario_file" yaml:"scenario_file"`   // 用于加载Mock场景的YAML文件路径。
	DefaultDelay  time.Duration          `json:"default_delay" yaml:"default_delay"`   // Mock响应的默认延迟。
	ErrorRate     float64                `json:"error_rate" yaml:"error_rate"`         // 随机返回错误的概率 (0.0 to 1.0)。
	DataGenerator DataGenConfig          `json:"data_generator" yaml:"data_generator"` // 自动数据生成器的配置。
	Scenarios     map[string]interface{} `json:"scenarios" yaml:"scenarios"`           // 直接在配置中以内联方式定义的Mock场景。
}

// DataGenConfig 定义了Mock数据自动生成器的配置。
type DataGenConfig struct {
	Enabled        bool          `json:"enabled" yaml:"enabled"`                 // 是否启用自动数据生成。
	StockCount     int           `json:"stock_count" yaml:"stock_count"`         // 生成的模拟股票数量。
	PriceRange     [2]float64    `json:"price_range" yaml:"price_range"`         // 生成价格的范围 [min, max]。
	VolumeRange    [2]int64      `json:"volume_range" yaml:"volume_range"`       // 生成成交量的范围 [min, max]。
	UpdateInterval time.Duration `json:"update_interval" yaml:"update_interval"` // 随机数据更新的时间间隔。
	Volatility     float64       `json:"volatility" yaml:"volatility"`           // 价格波动率。
}

// DefaultConfig 返回一个包含所有模块默认值的完整配置实例。
// 此默认配置已针对测试环境进行优化
func DefaultConfig() *Config {
	return &Config{
		Storage: StorageConfig{
			Type:         "csv",
			Directory:    "./testdata",
			MaxFileSize:  10 * 1024 * 1024, // 测试环境：10MB（原100MB）
			MaxFiles:     100,              // 测试环境：100个文件（原1000）
			Compression:  false,
			AutoCleanup:  true,
			CleanupAge:   24 * time.Hour,
			BufferSize:   8192,
			SyncInterval: 5 * time.Second,
		},
		Cache: CacheConfig{
			Type:            "memory",
			MaxSize:         500,              // 测试环境：500条记录（原1000）
			MaxMemory:       50 * 1024 * 1024, // 测试环境：50MB（原100MB）
			TTL:             2 * time.Hour,    // 测试环境：2小时（原24小时）
			EvictionPolicy:  "lru",
			CleanupInterval: 2 * time.Minute, // 测试环境：2分钟（原5分钟）
			Layers: []LayerConfig{
				{
					Name:           "L1",
					Type:           "memory",
					MaxSize:        100,
					TTL:            1 * time.Hour,
					EvictionPolicy: "lru",
				},
				{
					Name:           "L2",
					Type:           "memory",
					MaxSize:        500,           // 测试环境：500条记录（原1000）
					TTL:            2 * time.Hour, // 测试环境：2小时（原24小时）
					EvictionPolicy: "lru",
				},
			},
		},
		Provider: ProviderConfig{
			Type:            "tencent",
			Timeout:         30 * time.Second,
			RetryAttempts:   3,
			RetryDelay:      1 * time.Second,
			ConcurrentLimit: 10,
			RateLimitQPS:    100,
			UserAgent:       "stocksub-testkit/1.0",
			Headers:         make(map[string]string),
		},
		Performance: PerformanceConfig{
			WorkerCount:     2,                 // 测试环境：2个工作协程（原4）
			BatchSize:       20,                // 测试环境：20条批处理（原50）
			MaxConcurrency:  50,                // 测试环境：50并发（原100）
			MemoryLimit:     100 * 1024 * 1024, // 测试环境：100MB（原500MB）
			GCInterval:      5 * time.Minute,   // 测试环境：5分钟（原10分钟）
			MetricsInterval: 30 * time.Second,  // 测试环境：30秒（原1分钟）
			EnableProfiling: false,
			EnableMetrics:   true,
		},
		Logging: LoggingConfig{
			Level:        "info",
			Format:       "text",
			Output:       "stdout",
			File:         "./logs/testkit.log",
			MaxSize:      100,
			MaxBackups:   3,
			MaxAge:       7,
			Compress:     true,
			EnableCaller: false,
			EnableTrace:  false,
		},
		Mock: MockConfig{
			Enabled:      false,
			ScenarioFile: "./scenarios.yaml",
			DefaultDelay: 100 * time.Millisecond,
			ErrorRate:    0.0,
			DataGenerator: DataGenConfig{
				Enabled:        false,
				StockCount:     100,
				PriceRange:     [2]float64{1.0, 1000.0},
				VolumeRange:    [2]int64{1000, 10000000},
				UpdateInterval: 1 * time.Second,
				Volatility:     0.02,
			},
			Scenarios: make(map[string]interface{}),
		},
	}
}

// LoadConfig 从指定的JSON文件加载配置。
// 如果文件不存在，它会使用默认配置创建一个新文件。
func LoadConfig(filename string) (*Config, error) {
	if filename == "" {
		return DefaultConfig(), nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// 文件不存在，使用默认配置并创建配置文件
		config := DefaultConfig()
		if err := SaveConfig(filename, config); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
		return config, nil
	}

	// 读取配置文件
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析配置
	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// SaveConfig 将给定的配置对象序列化为JSON并保存到文件。
func SaveConfig(filename string, config *Config) error {
	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 序列化配置
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate 检查配置中的关键字段是否有效。
func (c *Config) Validate() error {

	// 验证存储配置
	if c.Storage.Type == "" {
		return fmt.Errorf("storage type is required")
	}
	if !isValidStorageType(c.Storage.Type) {
		return fmt.Errorf("invalid storage type: %s, must be one of: csv, memory, json", c.Storage.Type)
	}
	if c.Storage.Directory == "" {
		return fmt.Errorf("storage directory is required")
	}
	if c.Storage.MaxFileSize <= 0 {
		return fmt.Errorf("storage max_file_size must be positive")
	}
	if c.Storage.MaxFiles <= 0 {
		return fmt.Errorf("storage max_files must be positive")
	}

	// 验证缓存配置
	if c.Cache.MaxSize <= 0 {
		return fmt.Errorf("cache max_size must be positive")
	}
	if c.Cache.TTL <= 0 {
		return fmt.Errorf("cache ttl must be positive")
	}
	if !isValidCacheType(c.Cache.Type) {
		return fmt.Errorf("invalid cache type: %s, must be one of: memory, layered", c.Cache.Type)
	}
	if !isValidEvictionPolicy(c.Cache.EvictionPolicy) {
		return fmt.Errorf("invalid eviction policy: %s, must be one of: lru, lfu, fifo", c.Cache.EvictionPolicy)
	}

	// 验证Provider配置
	if c.Provider.Type == "" {
		return fmt.Errorf("provider type is required")
	}
	if !isValidProviderType(c.Provider.Type) {
		return fmt.Errorf("invalid provider type: %s, must be one of: tencent, mock, hybrid", c.Provider.Type)
	}
	if c.Provider.Timeout <= 0 {
		return fmt.Errorf("provider timeout must be positive")
	}
	if c.Provider.RetryAttempts < 0 {
		return fmt.Errorf("provider retry_attempts cannot be negative")
	}
	if c.Provider.RateLimitQPS < 0 {
		return fmt.Errorf("provider rate_limit_qps cannot be negative")
	}

	// 验证性能配置
	if c.Performance.WorkerCount <= 0 {
		return fmt.Errorf("performance worker_count must be positive")
	}
	if c.Performance.BatchSize <= 0 {
		return fmt.Errorf("performance batch_size must be positive")
	}
	if c.Performance.MemoryLimit <= 0 {
		return fmt.Errorf("performance memory_limit must be positive")
	}

	// 验证日志配置
	if !isValidLogLevel(c.Logging.Level) {
		return fmt.Errorf("invalid log level: %s, must be one of: debug, info, warn, error", c.Logging.Level)
	}
	if !isValidLogFormat(c.Logging.Format) {
		return fmt.Errorf("invalid log format: %s, must be one of: json, text", c.Logging.Format)
	}
	if !isValidLogOutput(c.Logging.Output) {
		return fmt.Errorf("invalid log output: %s, must be one of: stdout, file, both", c.Logging.Output)
	}

	// 验证Mock配置
	if c.Mock.ErrorRate < 0 || c.Mock.ErrorRate > 1 {
		return fmt.Errorf("mock error_rate must be between 0.0 and 1.0")
	}
	if c.Mock.DataGenerator.Volatility < 0 {
		return fmt.Errorf("mock data_generator volatility cannot be negative")
	}

	return nil
}

// 辅助验证函数
func isValidStorageType(t string) bool {
	validTypes := map[string]bool{"csv": true, "memory": true, "json": true}
	return validTypes[t]
}

func isValidCacheType(t string) bool {
	validTypes := map[string]bool{"memory": true, "layered": true}
	return validTypes[t]
}

func isValidEvictionPolicy(policy string) bool {
	validPolicies := map[string]bool{"lru": true, "lfu": true, "fifo": true}
	return validPolicies[policy]
}

func isValidProviderType(t string) bool {
	validTypes := map[string]bool{"tencent": true, "mock": true, "hybrid": true}
	return validTypes[t]
}

func isValidLogLevel(level string) bool {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	return validLevels[level]
}

func isValidLogFormat(format string) bool {
	validFormats := map[string]bool{"json": true, "text": true}
	return validFormats[format]
}

func isValidLogOutput(output string) bool {
	validOutputs := map[string]bool{"stdout": true, "file": true, "both": true}
	return validOutputs[output]
}

// IsTestEnvironment 检查当前是否运行在测试环境中
func IsTestEnvironment() bool {
	// 检查常见的测试环境标识
	if os.Getenv("GO_ENV") == "test" || os.Getenv("ENV") == "test" {
		return true
	}
	// 检查是否在运行测试
	if len(os.Args) > 0 && (contains(os.Args, "-test.") || contains(os.Args, "test")) {
		return true
	}
	// 检查是否在IDE的测试模式下运行
	if os.Getenv("IDE_TEST") == "true" || os.Getenv("TEST_MODE") == "true" {
		return true
	}
	return false
}

// DefaultConfigForEnvironment 根据运行环境返回合适的默认配置
func DefaultConfigForEnvironment() *Config {
	if IsTestEnvironment() {
		// 测试环境使用优化配置
		return DefaultConfig()
	}

	// 生产环境使用更保守的配置
	config := DefaultConfig()

	// 生产环境调整配置
	config.Storage.MaxFileSize = 50 * 1024 * 1024           // 生产环境：50MB
	config.Storage.MaxFiles = 500                           // 生产环境：500个文件
	config.Cache.MaxSize = 2000                             // 生产环境：2000条记录
	config.Cache.MaxMemory = 200 * 1024 * 1024              // 生产环境：200MB
	config.Cache.TTL = 12 * time.Hour                       // 生产环境：12小时
	config.Performance.MemoryLimit = 1 * 1024 * 1024 * 1024 // 生产环境：1GB
	config.Performance.WorkerCount = 8                      // 生产环境：8个工作协程

	return config
}

// contains 检查字符串切片是否包含特定字符串
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if strings.Contains(item, s) {
			return true
		}
	}
	return false
}

// Merge 将另一个配置对象合并到当前配置，返回一个新的配置实例。
// other中的非零/非空值将覆盖当前配置的值。
func (c *Config) Merge(other *Config) *Config {
	// 创建新配置，避免修改原配置
	merged := *c

	if other == nil {
		return &merged
	}

	// 合并存储配置
	if other.Storage.Type != "" {
		merged.Storage.Type = other.Storage.Type
	}
	if other.Storage.Directory != "" {
		merged.Storage.Directory = other.Storage.Directory
	}

	// 合并缓存配置
	if other.Cache.MaxSize > 0 {
		merged.Cache.MaxSize = other.Cache.MaxSize
	}
	if other.Cache.TTL > 0 {
		merged.Cache.TTL = other.Cache.TTL
	}

	// 其他配置合并...

	return &merged
}

// Clone 创建并返回当前配置对象的一个深拷贝。
func (c *Config) Clone() *Config {
	clone := *c

	// 深拷贝切片和映射
	clone.Cache.Layers = make([]LayerConfig, len(c.Cache.Layers))
	copy(clone.Cache.Layers, c.Cache.Layers)

	clone.Provider.Headers = make(map[string]string)
	for k, v := range c.Provider.Headers {
		clone.Provider.Headers[k] = v
	}

	clone.Mock.Scenarios = make(map[string]interface{})
	for k, v := range c.Mock.Scenarios {
		clone.Mock.Scenarios[k] = v
	}

	return &clone
}

// SimpleConfig 是 Config 的简化版本，专为测试环境设计
// 包含最常用的配置选项，减少配置复杂性
type SimpleConfig struct {
	StorageType  string        `json:"storage_type" yaml:"storage_type"`   // 存储类型
	StorageDir   string        `json:"storage_dir" yaml:"storage_dir"`     // 存储目录
	CacheType    string        `json:"cache_type" yaml:"cache_type"`       // 缓存类型
	CacheSize    int64         `json:"cache_size" yaml:"cache_size"`       // 缓存大小
	CacheTTL     time.Duration `json:"cache_ttl" yaml:"cache_ttl"`         // 缓存生存时间
	ProviderType string        `json:"provider_type" yaml:"provider_type"` // Provider类型
	MockEnabled  bool          `json:"mock_enabled" yaml:"mock_enabled"`   // 是否启用Mock
	LogLevel     string        `json:"log_level" yaml:"log_level"`         // 日志级别
}

// ToSimple 将完整配置转换为简化配置
func (c *Config) ToSimple() *SimpleConfig {
	return &SimpleConfig{
		StorageType:  c.Storage.Type,
		StorageDir:   c.Storage.Directory,
		CacheType:    c.Cache.Type,
		CacheSize:    c.Cache.MaxSize,
		CacheTTL:     c.Cache.TTL,
		ProviderType: c.Provider.Type,
		MockEnabled:  c.Mock.Enabled,
		LogLevel:     c.Logging.Level,
	}
}

// ToFull 将简化配置转换为完整配置
func (s *SimpleConfig) ToFull() *Config {
	full := DefaultConfig()

	// 只覆盖简化配置中指定的字段
	if s.StorageType != "" {
		full.Storage.Type = s.StorageType
	}
	if s.StorageDir != "" {
		full.Storage.Directory = s.StorageDir
	}
	if s.CacheType != "" {
		full.Cache.Type = s.CacheType
	}
	if s.CacheSize > 0 {
		full.Cache.MaxSize = s.CacheSize
	}
	if s.CacheTTL > 0 {
		full.Cache.TTL = s.CacheTTL
	}
	if s.ProviderType != "" {
		full.Provider.Type = s.ProviderType
	}
	full.Mock.Enabled = s.MockEnabled
	if s.LogLevel != "" {
		full.Logging.Level = s.LogLevel
	}

	return full
}

// DefaultSimpleConfig 返回适合测试环境的简化默认配置
func DefaultSimpleConfig() *SimpleConfig {
	return &SimpleConfig{
		StorageType:  "csv",
		StorageDir:   "./testdata",
		CacheType:    "memory",
		CacheSize:    100,           // 测试环境：100条记录
		CacheTTL:     1 * time.Hour, // 测试环境：1小时
		ProviderType: "tencent",
		MockEnabled:  false,
		LogLevel:     "info",
	}
}
