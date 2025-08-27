package tencent

import (
	"context"
	"stocksub/pkg/subscriber"
	"sync"
	"time"
)

// TencentProviderTestKit 腾讯Provider完整测试工具包
type TencentProviderTestKit struct {
	provider *Provider
	dataGen  *TencentDataGenerator
	httpMock *TencentHTTPMock
	stats    *TestStats
	mu       sync.RWMutex
}

// TestStats 测试统计信息
type TestStats struct {
	TotalCalls      int64         `json:"total_calls"`
	SuccessfulCalls int64         `json:"successful_calls"`
	FailedCalls     int64         `json:"failed_calls"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastCall        time.Time     `json:"last_call"`
	totalLatency    time.Duration
}

// TencentProviderConfig 腾讯Provider测试配置
type TencentProviderConfig struct {
	DataGenConfig TencentDataConfig `yaml:"data_gen_config"`
	EnableMock    bool              `yaml:"enable_mock"`
	MockURL       string            `yaml:"mock_url"`
}

// NewTencentProviderTestKit 创建腾讯Provider测试工具包
func NewTencentProviderTestKit(config *TencentProviderConfig) *TencentProviderTestKit {
	if config == nil {
		config = &TencentProviderConfig{
			DataGenConfig: DefaultTencentDataConfig(),
			EnableMock:    true,
		}
	}

	// 创建数据生成器
	dataGen := NewTencentDataGenerator(config.DataGenConfig)

	// 创建HTTP模拟服务器
	var httpMock *TencentHTTPMock
	if config.EnableMock {
		httpMock = NewTencentHTTPMock(dataGen)
	}

	// 创建Provider
	provider := NewProvider()

	// 如果有Mock服务器，修改Provider的HTTP客户端设置
	if httpMock != nil {
		// 这里我们需要修改Provider来支持自定义URL
		// 暂时直接使用，后续可以通过接口注入等方式改进
	}

	return &TencentProviderTestKit{
		provider: provider,
		dataGen:  dataGen,
		httpMock: httpMock,
		stats:    &TestStats{},
	}
}

// Close 关闭测试工具包
func (tk *TencentProviderTestKit) Close() {
	if tk.httpMock != nil {
		tk.httpMock.Close()
	}
	if tk.provider != nil {
		tk.provider.Close()
	}
}

// GetMockURL 获取Mock服务器URL
func (tk *TencentProviderTestKit) GetMockURL() string {
	if tk.httpMock != nil {
		return tk.httpMock.GetURL()
	}
	return ""
}

// SetMockResponse 设置Mock响应
func (tk *TencentProviderTestKit) SetMockResponse(symbols []string, response string) {
	if tk.httpMock != nil {
		tk.httpMock.SetCustomResponse(symbols, response)
	}
}

// ExecuteTest 执行测试
func (tk *TencentProviderTestKit) ExecuteTest(symbols []string) ([]subscriber.StockData, error) {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	start := time.Now()
	tk.stats.TotalCalls++
	tk.stats.LastCall = start

	data, err := tk.provider.FetchStockData(context.Background(), symbols)

	latency := time.Since(start)
	tk.stats.totalLatency += latency
	tk.stats.AverageLatency = tk.stats.totalLatency / time.Duration(tk.stats.TotalCalls)

	if err != nil {
		tk.stats.FailedCalls++
		return nil, err
	}

	tk.stats.SuccessfulCalls++
	return data, nil
}

// ExecuteTestWithRaw 执行测试并返回原始数据
func (tk *TencentProviderTestKit) ExecuteTestWithRaw(symbols []string) ([]subscriber.StockData, string, error) {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	start := time.Now()
	tk.stats.TotalCalls++
	tk.stats.LastCall = start

	data, raw, err := tk.provider.FetchStockDataWithRaw(context.Background(), symbols)

	latency := time.Since(start)
	tk.stats.totalLatency += latency
	tk.stats.AverageLatency = tk.stats.totalLatency / time.Duration(tk.stats.TotalCalls)

	if err != nil {
		tk.stats.FailedCalls++
		return nil, "", err
	}

	tk.stats.SuccessfulCalls++
	return data, raw, nil
}

// ValidateResult 验证测试结果
func (tk *TencentProviderTestKit) ValidateResult(data []subscriber.StockData, expected map[string]interface{}) error {
	// 将结果转换为map以便验证
	resultMap := make(map[string]subscriber.StockData)
	for _, stock := range data {
		resultMap[stock.Symbol] = stock
	}

	// 验证每个期望的股票数据
	for symbol, expectedData := range expected {
		stock, exists := resultMap[symbol]
		if !exists {
			return &ValidationError{
				Field:    "symbol",
				Expected: symbol,
				Actual:   "not found",
				Message:  "股票代码未找到",
			}
		}

		// 验证具体字段
		if expectedMap, ok := expectedData.(map[string]interface{}); ok {
			if err := tk.validateStockFields(stock, expectedMap); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateStockFields 验证股票字段
func (tk *TencentProviderTestKit) validateStockFields(stock subscriber.StockData, expected map[string]interface{}) error {
	// 验证名称
	if expectedName, exists := expected["name"]; exists {
		if stock.Name != expectedName {
			return &ValidationError{
				Field:    "name",
				Expected: expectedName,
				Actual:   stock.Name,
				Message:  "股票名称不匹配",
			}
		}
	}

	// 验证价格
	if expectedPrice, exists := expected["price"]; exists {
		if price, ok := expectedPrice.(float64); ok {
			if stock.Price != price {
				return &ValidationError{
					Field:    "price",
					Expected: price,
					Actual:   stock.Price,
					Message:  "股票价格不匹配",
				}
			}
		}
	}

	return nil
}

// GetStats 获取统计信息
func (tk *TencentProviderTestKit) GetStats() *TestStats {
	tk.mu.RLock()
	defer tk.mu.RUnlock()

	// 返回统计信息的副本
	return &TestStats{
		TotalCalls:      tk.stats.TotalCalls,
		SuccessfulCalls: tk.stats.SuccessfulCalls,
		FailedCalls:     tk.stats.FailedCalls,
		AverageLatency:  tk.stats.AverageLatency,
		LastCall:        tk.stats.LastCall,
	}
}

// ResetStats 重置统计信息
func (tk *TencentProviderTestKit) ResetStats() {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	tk.stats = &TestStats{}
}

// GenerateTestData 生成测试数据
func (tk *TencentProviderTestKit) GenerateTestData(symbols []string) []subscriber.StockData {
	return tk.dataGen.GenerateStockData(symbols)
}

// GenerateTestResponse 生成测试响应
func (tk *TencentProviderTestKit) GenerateTestResponse(symbols []string) string {
	return tk.dataGen.GenerateTencentResponse(symbols)
}

// ValidationError 验证错误
type ValidationError struct {
	Field    string      `json:"field"`
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
	Message  string      `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// TencentParserTestKit 解析器专用测试工具
type TencentParserTestKit struct {
	generator *TencentDataGenerator
}

// NewTencentParserTestKit 创建解析器测试工具
func NewTencentParserTestKit() *TencentParserTestKit {
	config := DefaultTencentDataConfig()
	config.EnableRandom = false // 解析器测试使用固定数据

	return &TencentParserTestKit{
		generator: NewTencentDataGenerator(config),
	}
}

// TestParseValid 测试有效数据解析
func (ptk *TencentParserTestKit) TestParseValid(symbols []string) ([]subscriber.StockData, error) {
	response := ptk.generator.GenerateTencentResponse(symbols)
	data := parseTencentData(response)

	if len(data) != len(symbols) {
		return nil, &ValidationError{
			Field:    "length",
			Expected: len(symbols),
			Actual:   len(data),
			Message:  "解析结果数量与输入不匹配",
		}
	}

	return data, nil
}

// TestParseEmpty 测试空数据解析
func (ptk *TencentParserTestKit) TestParseEmpty() []subscriber.StockData {
	return parseTencentData("")
}

// TestParseInvalid 测试无效数据解析
func (ptk *TencentParserTestKit) TestParseInvalid() []subscriber.StockData {
	return parseTencentData("invalid_data_format")
}

// TestParseIncomplete 测试不完整数据解析
func (ptk *TencentParserTestKit) TestParseIncomplete() []subscriber.StockData {
	incompleteData := "v_sh600000=\"1~浦发银行~sh600000~13.72\""
	return parseTencentData(incompleteData)
}
