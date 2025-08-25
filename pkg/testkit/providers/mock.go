package providers

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
)

// MockProvider 智能Mock Provider实现
type MockProvider struct {
	mu           sync.RWMutex
	scenarios    map[string]*core.MockScenario
	currentScene string
	recorder     *CallRecorder
	mockData     map[string][]subscriber.StockData
	enabled      bool
	config       MockProviderConfig
	stats        MockProviderStats
	generator    *DataGenerator
}

// MockProviderConfig Mock Provider配置
type MockProviderConfig struct {
	EnableRecording bool          `yaml:"enable_recording"` // 是否启用调用记录
	EnablePlayback  bool          `yaml:"enable_playback"`  // 是否启用回放
	DefaultDelay    time.Duration `yaml:"default_delay"`    // 默认延迟
	RandomDelay     bool          `yaml:"random_delay"`     // 是否使用随机延迟
	MaxRandomDelay  time.Duration `yaml:"max_random_delay"` // 最大随机延迟
	EnableDataGen   bool          `yaml:"enable_data_gen"`  // 是否启用数据生成
	DataGenConfig   DataGenConfig `yaml:"data_gen_config"`  // 数据生成配置
}

// MockProviderStats Mock Provider统计
type MockProviderStats struct {
	TotalCalls       int64         `json:"total_calls"`
	SuccessfulCalls  int64         `json:"successful_calls"`
	FailedCalls      int64         `json:"failed_calls"`
	AverageDelay     time.Duration `json:"average_delay"`
	LastCall         time.Time     `json:"last_call"`
	CurrentScenario  string        `json:"current_scenario"`
	RecordingEnabled bool          `json:"recording_enabled"`
	PlaybackEnabled  bool          `json:"playback_enabled"`
}

// CallRecorder 调用记录器
type CallRecorder struct {
	mu      sync.RWMutex
	calls   []CallRecord
	enabled bool
	maxSize int
}

// CallRecord 调用记录
type CallRecord struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Symbols   []string               `json:"symbols"`
	Response  []subscriber.StockData `json:"response"`
	Error     error                  `json:"error"`
	Duration  time.Duration          `json:"duration"`
	Scenario  string                 `json:"scenario"`
}

// NewMockProvider 创建Mock Provider
func NewMockProvider(config MockProviderConfig) *MockProvider {
	mp := &MockProvider{
		scenarios: make(map[string]*core.MockScenario),
		mockData:  make(map[string][]subscriber.StockData),
		enabled:   true,
		config:    config,
		stats:     MockProviderStats{},
		generator: NewDataGenerator(config.DataGenConfig),
	}

	if config.EnableRecording {
		mp.recorder = NewCallRecorder(1000) // 默认记录1000条调用
	}

	// 加载默认场景
	mp.loadDefaultScenarios()

	return mp
}

// NewCallRecorder 创建调用记录器
func NewCallRecorder(maxSize int) *CallRecorder {
	return &CallRecorder{
		calls:   make([]CallRecord, 0, maxSize),
		enabled: true,
		maxSize: maxSize,
	}
}

// Name returns the name of the provider.
func (mp *MockProvider) Name() string {
	return "mock"
}

// GetRateLimit returns the rate limit of the provider.
func (mp *MockProvider) GetRateLimit() time.Duration {
	return 200 * time.Millisecond // A sensible default
}

// IsSymbolSupported checks if a symbol is supported by the mock provider.
func (mp *MockProvider) IsSymbolSupported(symbol string) bool {
	// In mock provider, we can assume all symbols are supported.
	return true
}

// FetchData 获取股票数据
func (mp *MockProvider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	startTime := time.Now()
	atomic.AddInt64(&mp.stats.TotalCalls, 1)
	mp.stats.LastCall = startTime

	// 应用延迟
	if err := mp.applyDelay(); err != nil {
		atomic.AddInt64(&mp.stats.FailedCalls, 1)
		return nil, err
	}

	mp.mu.RLock()
	currentScene := mp.currentScene
	scenarios := mp.scenarios
	mp.mu.RUnlock()

	var result []subscriber.StockData
	var err error

	// 优先检查SetMockData提供的数据
	result, err = mp.getMockData(symbols)
	if err == nil && len(result) > 0 {
		// 如果getMockData成功返回了所有请求的symbol的数据，则直接返回
		if len(result) == len(symbols) {
			goto end
		}
	}

	// 如果mockData不完整或不存在，则检查场景
	if currentScene != "" && scenarios[currentScene] != nil {
		result, err = mp.executeScenario(symbols, scenarios[currentScene])
	} else if mp.config.EnableDataGen {
		result, err = mp.generator.GenerateStockData(symbols)
	} else {
		// 如果mockData为空，且没有场景，且没有数据生成器，则返回错误
		if len(mp.mockData) == 0 {
			err = fmt.Errorf("未配置Mock数据、场景或数据生成器")
		}
	}

end:
	duration := time.Since(startTime)

	// 记录调用
	if mp.recorder != nil && mp.recorder.enabled {
		mp.recorder.RecordCall(CallRecord{
			ID:        fmt.Sprintf("call_%d", time.Now().UnixNano()),
			Timestamp: startTime,
			Symbols:   symbols,
			Response:  result,
			Error:     err,
			Duration:  duration,
			Scenario:  currentScene,
		})
	}

	if err != nil {
		atomic.AddInt64(&mp.stats.FailedCalls, 1)
		return nil, err
	}

	atomic.AddInt64(&mp.stats.SuccessfulCalls, 1)
	mp.updateAverageDelay(duration)

	return result, nil
}

// SetMockMode 设置Mock模式
func (mp *MockProvider) SetMockMode(enabled bool) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.enabled = enabled
}

// SetMockData 设置Mock数据
func (mp *MockProvider) SetMockData(symbols []string, data []subscriber.StockData) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	for i, symbol := range symbols {
		if i < len(data) {
			mp.mockData[symbol] = []subscriber.StockData{data[i]}
		}
	}
}

// SetScenario 设置当前场景
func (mp *MockProvider) SetScenario(scenarioName string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if _, exists := mp.scenarios[scenarioName]; !exists {
		return fmt.Errorf("场景 %s 不存在", scenarioName)
	}

	mp.currentScene = scenarioName
	mp.stats.CurrentScenario = scenarioName
	return nil
}

// AddScenario 添加场景
func (mp *MockProvider) AddScenario(scenario *core.MockScenario) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.scenarios[scenario.Name] = scenario
}

// RemoveScenario 移除场景
func (mp *MockProvider) RemoveScenario(scenarioName string) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	delete(mp.scenarios, scenarioName)
	if mp.currentScene == scenarioName {
		mp.currentScene = ""
	}
}

// GetScenarios 获取所有场景
func (mp *MockProvider) GetScenarios() map[string]*core.MockScenario {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	result := make(map[string]*core.MockScenario)
	for name, scenario := range mp.scenarios {
		result[name] = scenario
	}
	return result
}

// Close 关闭Provider
func (mp *MockProvider) Close() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// 检查是否已经关闭
	if !mp.enabled {
		return fmt.Errorf("provider already closed")
	}

	mp.enabled = false
	mp.scenarios = make(map[string]*core.MockScenario)
	mp.mockData = make(map[string][]subscriber.StockData)

	return nil
}

// GetStats 获取统计信息
func (mp *MockProvider) GetStats() MockProviderStats {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	stats := mp.stats
	stats.RecordingEnabled = mp.recorder != nil && mp.recorder.enabled
	stats.PlaybackEnabled = mp.config.EnablePlayback

	return stats
}

// executeScenario 执行场景
func (mp *MockProvider) executeScenario(symbols []string, scenario *core.MockScenario) ([]subscriber.StockData, error) {
	// 检查场景是否有通配符错误定义
	if err, exists := scenario.Errors["*"]; exists && err != nil {
		return nil, err
	}

	// 检查场景是否有特定错误定义
	for _, symbol := range symbols {
		if err, exists := scenario.Errors[symbol]; exists && err != nil {
			return nil, err
		}
	}

	// 应用场景延迟（支持通配符）
	if delay, exists := scenario.Delays["*"]; exists && delay > 0 {
		time.Sleep(delay)
	} else {
		for _, symbol := range symbols {
			if delay, exists := scenario.Delays[symbol]; exists && delay > 0 {
				time.Sleep(delay)
			}
		}
	}

	// 获取响应数据
	result := make([]subscriber.StockData, 0, len(symbols))
	for _, symbol := range symbols {
		if response, exists := scenario.Responses[symbol]; exists {
			result = append(result, response.Data...)
		} else {
			// 如果场景中没有该股票的数据，生成默认数据
			if mp.config.EnableDataGen {
				data, err := mp.generator.GenerateStockData([]string{symbol})
				if err == nil && len(data) > 0 {
					result = append(result, data[0])
				}
			}
		}
	}

	return result, nil
}

// getMockData 获取Mock数据
func (mp *MockProvider) getMockData(symbols []string) ([]subscriber.StockData, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	// 如果没有mock数据，直接返回空切片
	if len(mp.mockData) == 0 {
		return []subscriber.StockData{}, nil
	}

	result := make([]subscriber.StockData, 0, len(symbols))

	for _, symbol := range symbols {
		if data, exists := mp.mockData[symbol]; exists && len(data) > 0 {
			result = append(result, data[0])
		}
	}

	// 只有当所有请求的symbols都在mockData中找到时，才认为成功
	if len(result) == len(symbols) {
		return result, nil
	}

	// 如果mockData不完整，返回一个空切片，让上层逻辑继续处理
	return []subscriber.StockData{}, nil
}

// applyDelay 应用延迟
func (mp *MockProvider) applyDelay() error {
	delay := mp.config.DefaultDelay

	if mp.config.RandomDelay && mp.config.MaxRandomDelay > 0 {
		randomDelay := time.Duration(rand.Int63n(int64(mp.config.MaxRandomDelay)))
		delay += randomDelay
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	return nil
}

// updateAverageDelay 更新平均延迟
func (mp *MockProvider) updateAverageDelay(duration time.Duration) {
	// 简单的移动平均计算
	if mp.stats.AverageDelay == 0 {
		mp.stats.AverageDelay = duration
	} else {
		mp.stats.AverageDelay = (mp.stats.AverageDelay + duration) / 2
	}
}

// loadDefaultScenarios 加载默认场景
func (mp *MockProvider) loadDefaultScenarios() {
	// 正常场景
	normalScenario := &core.MockScenario{
		Name:        "normal",
		Description: "正常数据返回场景",
		Responses:   make(map[string]core.MockResponse),
		Delays:      make(map[string]time.Duration),
		Errors:      make(map[string]error),
	}

	// 错误场景
	errorScenario := &core.MockScenario{
		Name:        "error",
		Description: "API错误场景",
		Responses:   make(map[string]core.MockResponse),
		Delays:      make(map[string]time.Duration),
		Errors: map[string]error{
			"*": fmt.Errorf("API服务暂时不可用"),
		},
	}

	// 延迟场景
	slowScenario := &core.MockScenario{
		Name:        "slow",
		Description: "高延迟场景",
		Responses:   make(map[string]core.MockResponse),
		Delays: map[string]time.Duration{
			"*": 2 * time.Second,
		},
		Errors: make(map[string]error),
	}

	mp.scenarios["normal"] = normalScenario
	mp.scenarios["error"] = errorScenario
	mp.scenarios["slow"] = slowScenario

	// 设置默认场景
	mp.currentScene = "normal"
}

// RecordCall 记录调用
func (cr *CallRecorder) RecordCall(record CallRecord) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if !cr.enabled {
		return
	}

	// 如果达到最大容量，移除最旧的记录
	if len(cr.calls) >= cr.maxSize {
		cr.calls = cr.calls[1:]
	}

	cr.calls = append(cr.calls, record)
}

// GetCalls 获取所有调用记录
func (cr *CallRecorder) GetCalls() []CallRecord {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	result := make([]CallRecord, len(cr.calls))
	copy(result, cr.calls)
	return result
}

// GetCallsBySymbol 获取指定股票的调用记录
func (cr *CallRecorder) GetCallsBySymbol(symbol string) []CallRecord {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	result := make([]CallRecord, 0)
	for _, call := range cr.calls {
		for _, s := range call.Symbols {
			if s == symbol {
				result = append(result, call)
				break
			}
		}
	}
	return result
}

// Clear 清空记录
func (cr *CallRecorder) Clear() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.calls = cr.calls[:0]
}

// Enable 启用记录
func (cr *CallRecorder) Enable() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.enabled = true
}

// Disable 禁用记录
func (cr *CallRecorder) Disable() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.enabled = false
}

// DataGenerator 数据生成器
type DataGenerator struct {
	config DataGenConfig
	rand   *rand.Rand
}

// DataGenConfig 数据生成配置
type DataGenConfig struct {
	PriceRange      PriceRange  `yaml:"price_range"`
	VolumnRange     VolumeRange `yaml:"volume_range"`
	ChangeRange     ChangeRange `yaml:"change_range"`
	RandomSeed      int64       `yaml:"random_seed"`
	EnableRealistic bool        `yaml:"enable_realistic"` // 是否生成现实的数据
	MarketHours     bool        `yaml:"market_hours"`     // 是否模拟市场时间
}

// PriceRange 价格范围
type PriceRange struct {
	Min float64 `yaml:"min"`
	Max float64 `yaml:"max"`
}

// VolumeRange 成交量范围
type VolumeRange struct {
	Min int64 `yaml:"min"`
	Max int64 `yaml:"max"`
}

// ChangeRange 涨跌幅范围
type ChangeRange struct {
	Min float64 `yaml:"min"`
	Max float64 `yaml:"max"`
}

// NewDataGenerator 创建数据生成器
func NewDataGenerator(config DataGenConfig) *DataGenerator {
	seed := config.RandomSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &DataGenerator{
		config: config,
		rand:   rand.New(rand.NewSource(seed)),
	}
}

// GenerateStockData 生成股票数据
func (dg *DataGenerator) GenerateStockData(symbols []string) ([]subscriber.StockData, error) {
	result := make([]subscriber.StockData, 0, len(symbols))

	for _, symbol := range symbols {
		data := dg.generateSingleStock(symbol)
		result = append(result, data)
	}

	return result, nil
}

// generateSingleStock 生成单个股票数据
func (dg *DataGenerator) generateSingleStock(symbol string) subscriber.StockData {
	// 生成基础价格
	price := dg.generatePrice()
	change := dg.generateChange()
	changePercent := (change / price) * 100

	// 生成其他字段
	volume := dg.generateVolume()

	return subscriber.StockData{
		Symbol:        symbol,
		Name:          dg.generateStockName(symbol),
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		High:          price + dg.rand.Float64()*2,
		Low:           price - dg.rand.Float64()*2,
		Open:          price + (dg.rand.Float64()-0.5)*1,
		PrevClose:     price - change,
		Timestamp:     time.Now(),
	}
}

// generatePrice 生成价格
func (dg *DataGenerator) generatePrice() float64 {
	min := dg.config.PriceRange.Min
	max := dg.config.PriceRange.Max

	if min == 0 && max == 0 {
		min = 1.0
		max = 100.0
	}

	return min + dg.rand.Float64()*(max-min)
}

// generateChange 生成涨跌额
func (dg *DataGenerator) generateChange() float64 {
	min := dg.config.ChangeRange.Min
	max := dg.config.ChangeRange.Max

	if min == 0 && max == 0 {
		min = -10.0
		max = 10.0
	}

	return min + dg.rand.Float64()*(max-min)
}

// generateVolume 生成成交量
func (dg *DataGenerator) generateVolume() int64 {
	min := dg.config.VolumnRange.Min
	max := dg.config.VolumnRange.Max

	if min == 0 && max == 0 {
		min = 1000
		max = 1000000
	}

	return min + dg.rand.Int63n(max-min)
}

// generateStockName 生成股票名称
func (dg *DataGenerator) generateStockName(symbol string) string {
	// 简单的名称生成逻辑
	prefixes := []string{"测试", "模拟", "样本", "示例"}
	suffixes := []string{"科技", "控股", "实业", "集团", "股份"}

	prefix := prefixes[dg.rand.Intn(len(prefixes))]
	suffix := suffixes[dg.rand.Intn(len(suffixes))]

	return fmt.Sprintf("%s%s%s", prefix, symbol, suffix)
}

// DefaultMockProviderConfig 默认Mock Provider配置
func DefaultMockProviderConfig() MockProviderConfig {
	return MockProviderConfig{
		EnableRecording: true,
		EnablePlayback:  false,
		DefaultDelay:    100 * time.Millisecond,
		RandomDelay:     true,
		MaxRandomDelay:  500 * time.Millisecond,
		EnableDataGen:   true,
		DataGenConfig: DataGenConfig{
			PriceRange:      PriceRange{Min: 1.0, Max: 100.0},
			VolumnRange:     VolumeRange{Min: 1000, Max: 1000000},
			ChangeRange:     ChangeRange{Min: -10.0, Max: 10.0},
			RandomSeed:      0,
			EnableRealistic: true,
			MarketHours:     false,
		},
	}
}

// DefaultDataGenConfig 默认数据生成配置
func DefaultDataGenConfig() DataGenConfig {
	return DataGenConfig{
		PriceRange:      PriceRange{Min: 1.0, Max: 100.0},
		VolumnRange:     VolumeRange{Min: 1000, Max: 1000000},
		ChangeRange:     ChangeRange{Min: -10.0, Max: 10.0},
		RandomSeed:      0,
		EnableRealistic: true,
		MarketHours:     false,
	}
}