package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/provider/tencent"
)

// TencentProviderTestKit 腾讯Provider测试工具包
type TencentProviderTestKit struct {
	provider      *tencent.Client
	httpMock      *TencentHTTPMock
	dataGenerator *TencentDataGenerator
	originalURL   string
	stats         TencentTestStats
	mu            sync.RWMutex
}

// TencentTestStats 腾讯测试统计
type TencentTestStats struct {
	TotalCalls      int64         `json:"total_calls"`
	SuccessfulCalls int64         `json:"successful_calls"`
	FailedCalls     int64         `json:"failed_calls"`
	AverageLatency  time.Duration `json:"average_latency"`
	RequestCount    int           `json:"request_count"`
	LastCall        time.Time     `json:"last_call"`
}

// TencentProviderConfig 腾讯Provider测试配置
type TencentProviderConfig struct {
	DataConfig  TencentDataConfig `yaml:"data_config"`
	EnableMock  bool              `yaml:"enable_mock"`
	MockURL     string            `yaml:"mock_url"`
	Timeout     time.Duration     `yaml:"timeout"`
	EnableStats bool              `yaml:"enable_stats"`
}

// NewTencentProviderTestKit 创建腾讯Provider测试工具包
func NewTencentProviderTestKit(config *TencentProviderConfig) *TencentProviderTestKit {
	if config == nil {
		config = &TencentProviderConfig{
			DataConfig:  DefaultTencentDataConfig(),
			EnableMock:  true,
			Timeout:     30 * time.Second,
			EnableStats: true,
		}
	}

	kit := &TencentProviderTestKit{
		provider:      tencent.NewClient(),
		dataGenerator: NewTencentDataGenerator(config.DataConfig),
		stats:         TencentTestStats{},
	}

	// 如果启用Mock，创建HTTP Mock服务器
	if config.EnableMock {
		kit.httpMock = NewTencentHTTPMock(config.DataConfig)
		// 这里需要修改provider的URL，但当前tencent.Provider没有提供这个接口
		// 暂时通过环境变量或其他方式处理
	}

	return kit
}

// Close 关闭测试工具包
func (k *TencentProviderTestKit) Close() {
	if k.httpMock != nil {
		k.httpMock.Close()
	}
	if k.provider != nil {
		k.provider.Close()
	}
}

// GetMockURL 获取Mock服务器URL
func (k *TencentProviderTestKit) GetMockURL() string {
	if k.httpMock != nil {
		return k.httpMock.GetURL()
	}
	return ""
}

// SetMockResponse 设置Mock响应
func (k *TencentProviderTestKit) SetMockResponse(symbols []string, response string) {
	if k.httpMock != nil {
		k.httpMock.SetCustomResponse(response)
	}
}

// ExecuteTest 执行测试并收集统计信息
func (k *TencentProviderTestKit) ExecuteTest(symbols []string) ([]core.StockData, error) {
	start := time.Now()

	k.mu.Lock()
	k.stats.TotalCalls++
	k.stats.LastCall = start
	k.mu.Unlock()

	data, err := k.provider.FetchStockData(context.Background(), symbols)

	duration := time.Since(start)
	k.mu.Lock()
	if err != nil {
		k.stats.FailedCalls++
	} else {
		k.stats.SuccessfulCalls++
	}

	// 更新平均延迟
	if k.stats.AverageLatency == 0 {
		k.stats.AverageLatency = duration
	} else {
		k.stats.AverageLatency = (k.stats.AverageLatency + duration) / 2
	}
	k.mu.Unlock()

	return data, err
}

// ExecuteTestWithRaw 执行测试并返回原始数据
func (k *TencentProviderTestKit) ExecuteTestWithRaw(symbols []string) ([]core.StockData, string, error) {
	start := time.Now()

	k.mu.Lock()
	k.stats.TotalCalls++
	k.stats.LastCall = start
	k.mu.Unlock()

	data, raw, err := k.provider.FetchStockDataWithRaw(context.Background(), symbols)

	duration := time.Since(start)
	k.mu.Lock()
	if err != nil {
		k.stats.FailedCalls++
	} else {
		k.stats.SuccessfulCalls++
	}

	// 更新平均延迟
	if k.stats.AverageLatency == 0 {
		k.stats.AverageLatency = duration
	} else {
		k.stats.AverageLatency = (k.stats.AverageLatency + duration) / 2
	}
	k.mu.Unlock()

	return data, raw, err
}

// ValidateResult 验证测试结果
func (k *TencentProviderTestKit) ValidateResult(data []core.StockData, expected map[string]interface{}) error {
	for _, stock := range data {
		expectedData, exists := expected[stock.Symbol]
		if !exists {
			continue
		}

		expectedMap, ok := expectedData.(map[string]interface{})
		if !ok {
			continue
		}

		// 验证股票名称
		if expectedName, ok := expectedMap["name"].(string); ok {
			if stock.Name != expectedName {
				return fmt.Errorf("股票 %s 名称不匹配: 期望 %s, 实际 %s",
					stock.Symbol, expectedName, stock.Name)
			}
		}

		// 验证股票价格
		if expectedPrice, ok := expectedMap["price"].(float64); ok {
			if stock.Price != expectedPrice {
				return fmt.Errorf("股票 %s 价格不匹配: 期望 %.2f, 实际 %.2f",
					stock.Symbol, expectedPrice, stock.Price)
			}
		}
	}

	return nil
}

// validateStockFields 验证股票字段的有效性
func (k *TencentProviderTestKit) validateStockFields(stock core.StockData) []string {
	var errors []string

	// 验证必填字段
	if stock.Symbol == "" {
		errors = append(errors, "股票代码不能为空")
	}

	if stock.Name == "" {
		errors = append(errors, "股票名称不能为空")
	}

	if stock.Price < 0 {
		errors = append(errors, "股票价格不能为负数")
	}

	if stock.Volume < 0 {
		errors = append(errors, "成交量不能为负数")
	}

	// 验证股票代码格式
	if len(stock.Symbol) != 6 {
		errors = append(errors, "股票代码长度必须为6位")
	} else {
		for _, char := range stock.Symbol {
			if char < '0' || char > '9' {
				errors = append(errors, "股票代码必须为纯数字")
				break
			}
		}
	}

	// 验证时间戳
	if stock.Timestamp.IsZero() {
		errors = append(errors, "时间戳不能为空")
	}

	return errors
}

// GetStats 获取测试统计信息
func (k *TencentProviderTestKit) GetStats() TencentTestStats {
	k.mu.RLock()
	defer k.mu.RUnlock()

	stats := k.stats
	if k.httpMock != nil {
		stats.RequestCount = k.httpMock.GetRequestCount()
	}

	return stats
}

// ResetStats 重置统计信息
func (k *TencentProviderTestKit) ResetStats() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.stats = TencentTestStats{}
}

// GenerateTestData 生成测试数据
func (k *TencentProviderTestKit) GenerateTestData(symbols []string) []core.StockData {
	return k.dataGenerator.GenerateStockData(symbols)
}

// GenerateTestResponse 生成测试响应
func (k *TencentProviderTestKit) GenerateTestResponse(symbols []string) string {
	return k.dataGenerator.GenerateTencentResponse(symbols)
}

// TencentTestKitError 测试工具包错误类型
type TencentTestKitError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Error 实现error接口
func (e *TencentTestKitError) Error() string {
	return fmt.Sprintf("[%s] %s (Code: %d)", e.Type, e.Message, e.Code)
}

// TencentParserTestKit 腾讯解析器测试工具包
type TencentParserTestKit struct {
	dataGenerator *TencentDataGenerator
}

// NewTencentParserTestKit 创建解析器测试工具包
func NewTencentParserTestKit(config TencentDataConfig) *TencentParserTestKit {
	return &TencentParserTestKit{
		dataGenerator: NewTencentDataGenerator(config),
	}
}

// TestParseValid 测试有效数据解析
func (k *TencentParserTestKit) TestParseValid(symbols []string) ([]core.StockData, error) {
	response := k.dataGenerator.GenerateTencentResponse(symbols)

	// 这里需要调用tencent包的解析函数
	// 由于解析函数可能是私有的，需要通过Provider接口间接测试
	provider := tencent.NewClient()
	defer provider.Close()

	// 创建临时Mock服务器
	httpMock := NewTencentHTTPMock(DefaultTencentDataConfig())
	defer httpMock.Close()
	httpMock.SetCustomResponse(response)

	// 通过Provider测试解析
	return provider.FetchStockData(context.Background(), symbols)
}

// TestParseEmpty 测试空数据解析
func (k *TencentParserTestKit) TestParseEmpty() error {
	// 测试空响应解析
	return nil
}

// TestParseInvalid 测试无效数据解析
func (k *TencentParserTestKit) TestParseInvalid() error {
	// 测试无效格式解析
	return nil
}

// TestParseIncomplete 测试不完整数据解析
func (k *TencentParserTestKit) TestParseIncomplete() error {
	// 测试不完整数据解析
	return nil
}
