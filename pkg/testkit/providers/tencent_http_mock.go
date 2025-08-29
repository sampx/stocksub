package providers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// TencentHTTPMock 腾讯API的HTTP模拟服务器
type TencentHTTPMock struct {
	server         *httptest.Server
	dataGenerator  *TencentDataGenerator
	customResponse map[string]string // symbol -> response
	requestCount   int
	mu             sync.RWMutex
	errorMode      bool
	timeoutMode    bool
	emptyMode      bool
	invalidMode    bool
}

// NewTencentHTTPMock 创建腾讯HTTP Mock服务器
func NewTencentHTTPMock(config TencentDataConfig) *TencentHTTPMock {
	mock := &TencentHTTPMock{
		dataGenerator:  NewTencentDataGenerator(config),
		customResponse: make(map[string]string),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// GetURL 获取Mock服务器URL
func (m *TencentHTTPMock) GetURL() string {
	return m.server.URL
}

// Close 关闭Mock服务器
func (m *TencentHTTPMock) Close() {
	if m.server != nil {
		m.server.Close()
	}
}

// SetCustomResponse 设置自定义响应
func (m *TencentHTTPMock) SetCustomResponse(response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customResponse["*"] = response
}

// SetCustomResponseForSymbol 为特定股票代码设置自定义响应
func (m *TencentHTTPMock) SetCustomResponseForSymbol(symbol, response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customResponse[symbol] = response
}

// handleRequest 处理HTTP请求
func (m *TencentHTTPMock) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.requestCount++
	m.mu.Unlock()

	// 解析股票代码
	symbols := m.parseSymbols(r.URL.Query().Get("q"))

	// 检查错误模式
	if m.errorMode {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 检查超时模式
	if m.timeoutMode {
		// 模拟超时，不响应
		select {}
	}

	// 检查空响应模式
	if m.emptyMode {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 检查无效数据模式
	if m.invalidMode {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid response data"))
		return
	}

	// 生成响应
	var response string
	m.mu.RLock()
	if customResp, exists := m.customResponse["*"]; exists {
		response = customResp
	} else {
		// 为每个股票生成响应
		var parts []string
		for _, symbol := range symbols {
			if customResp, exists := m.customResponse[symbol]; exists {
				parts = append(parts, customResp)
			} else {
				parts = append(parts, m.dataGenerator.generateSingleStockResponse(symbol))
			}
		}
		response = strings.Join(parts, "")
	}
	m.mu.RUnlock()

	// 设置响应头
	w.Header().Set("Content-Type", "text/plain; charset=GBK")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

// parseSymbols 从查询参数中解析股票代码
func (m *TencentHTTPMock) parseSymbols(q string) []string {
	if q == "" {
		return []string{}
	}

	// 腾讯API格式：sh600000,sz000001,bj835174
	codes := strings.Split(q, ",")
	var symbols []string

	for _, code := range codes {
		symbol := m.extractSymbol(code)
		if symbol != "" {
			symbols = append(symbols, symbol)
		}
	}

	return symbols
}

// extractSymbol 从带前缀的代码中提取股票代码
func (m *TencentHTTPMock) extractSymbol(code string) string {
	code = strings.TrimSpace(code)
	if len(code) < 6 {
		return ""
	}

	// 移除前缀（sh, sz, bj等）
	if len(code) > 6 {
		// 找到数字开始的位置
		for i, char := range code {
			if char >= '0' && char <= '9' {
				if len(code)-i == 6 {
					return code[i:]
				}
				break
			}
		}
	}

	// 如果已经是6位数字，直接返回
	if len(code) == 6 {
		for _, char := range code {
			if char < '0' || char > '9' {
				return ""
			}
		}
		return code
	}

	return ""
}

// SimulateError 模拟服务器错误
func (m *TencentHTTPMock) SimulateError(enable bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorMode = enable
}

// SimulateTimeout 模拟请求超时
func (m *TencentHTTPMock) SimulateTimeout(enable bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeoutMode = enable
}

// SimulateEmptyResponse 模拟空响应
func (m *TencentHTTPMock) SimulateEmptyResponse(enable bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emptyMode = enable
}

// SimulateInvalidData 模拟无效数据
func (m *TencentHTTPMock) SimulateInvalidData(enable bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invalidMode = enable
}

// ClearCustomResponses 清除所有自定义响应
func (m *TencentHTTPMock) ClearCustomResponses() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customResponse = make(map[string]string)
}

// GetRequestCount 获取请求次数
func (m *TencentHTTPMock) GetRequestCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestCount
}
