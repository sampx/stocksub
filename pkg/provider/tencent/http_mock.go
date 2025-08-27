package tencent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// TencentHTTPMock 腾讯API HTTP模拟服务器
type TencentHTTPMock struct {
	server    *httptest.Server
	generator *TencentDataGenerator
	responses map[string]string // 自定义响应
}

// NewTencentHTTPMock 创建腾讯HTTP模拟服务器
func NewTencentHTTPMock(generator *TencentDataGenerator) *TencentHTTPMock {
	mock := &TencentHTTPMock{
		generator: generator,
		responses: make(map[string]string),
	}

	// 创建测试服务器
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))

	return mock
}

// GetURL 获取模拟服务器的URL
func (m *TencentHTTPMock) GetURL() string {
	return m.server.URL
}

// Close 关闭模拟服务器
func (m *TencentHTTPMock) Close() {
	if m.server != nil {
		m.server.Close()
	}
}

// SetCustomResponse 设置自定义响应
func (m *TencentHTTPMock) SetCustomResponse(symbols []string, response string) {
	key := strings.Join(symbols, ",")
	m.responses[key] = response
}

// SetCustomResponseForSymbol 为单个股票设置自定义响应
func (m *TencentHTTPMock) SetCustomResponseForSymbol(symbol string, response string) {
	m.responses[symbol] = response
}

// handleRequest 处理HTTP请求
func (m *TencentHTTPMock) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	queryParams, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}

	// 获取股票代码参数
	qParam := queryParams.Get("q")
	if qParam == "" {
		http.Error(w, "Missing 'q' parameter", http.StatusBadRequest)
		return
	}

	// 解析股票代码
	symbols := m.parseSymbols(qParam)
	if len(symbols) == 0 {
		http.Error(w, "No valid symbols found", http.StatusBadRequest)
		return
	}

	// 检查是否有自定义响应
	key := strings.Join(symbols, ",")
	if customResponse, exists := m.responses[key]; exists {
		w.Header().Set("Content-Type", "text/javascript; charset=GBK")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(customResponse))
		return
	}

	// 检查单个股票的自定义响应
	for _, symbol := range symbols {
		if customResponse, exists := m.responses[symbol]; exists {
			w.Header().Set("Content-Type", "text/javascript; charset=GBK")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(customResponse))
			return
		}
	}

	// 使用数据生成器生成响应
	response := m.generator.GenerateTencentResponse(symbols)

	// 设置响应头
	w.Header().Set("Content-Type", "text/javascript; charset=GBK")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

// parseSymbols 从查询参数中解析股票代码
func (m *TencentHTTPMock) parseSymbols(qParam string) []string {
	var symbols []string

	// 腾讯API格式：sh600000,sz000001
	parts := strings.Split(qParam, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 移除市场前缀，只保留股票代码
		symbol := m.extractSymbol(part)
		if symbol != "" {
			symbols = append(symbols, symbol)
		}
	}

	return symbols
}

// extractSymbol 从带前缀的股票代码中提取纯代码
func (m *TencentHTTPMock) extractSymbol(code string) string {
	// 移除市场前缀
	code = strings.TrimPrefix(code, "sh")
	code = strings.TrimPrefix(code, "sz")
	code = strings.TrimPrefix(code, "bj")

	// 移除可能的后缀
	if dotIndex := strings.Index(code, "."); dotIndex != -1 {
		code = code[:dotIndex]
	}

	// 验证股票代码格式（6位数字）
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

// SimulateError 模拟错误响应
func (m *TencentHTTPMock) SimulateError(symbols []string, statusCode int, errorMessage string) {
	key := strings.Join(symbols, ",")
	// 使用特殊标记来识别错误响应
	m.responses[key] = fmt.Sprintf("ERROR:%d:%s", statusCode, errorMessage)
}

// SimulateTimeout 模拟超时（通过返回空响应）
func (m *TencentHTTPMock) SimulateTimeout(symbols []string) {
	key := strings.Join(symbols, ",")
	m.responses[key] = "TIMEOUT"
}

// SimulateEmptyResponse 模拟空响应
func (m *TencentHTTPMock) SimulateEmptyResponse(symbols []string) {
	key := strings.Join(symbols, ",")
	m.responses[key] = ""
}

// SimulateInvalidData 模拟无效数据响应
func (m *TencentHTTPMock) SimulateInvalidData(symbols []string) {
	key := strings.Join(symbols, ",")
	m.responses[key] = "invalid_data_format"
}

// ClearCustomResponses 清除所有自定义响应
func (m *TencentHTTPMock) ClearCustomResponses() {
	m.responses = make(map[string]string)
}

// GetRequestCount 获取请求计数（简单实现）
func (m *TencentHTTPMock) GetRequestCount() int {
	// 这里可以添加更复杂的统计逻辑
	return len(m.responses)
}
