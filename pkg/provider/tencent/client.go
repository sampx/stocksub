package tencent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/logger"
)

// Client 腾讯股票数据提供商 - 简化版
// 专注于核心数据获取功能，频率控制等横切关注点通过装饰器处理
type Client struct {
	httpClient *http.Client
	userAgent  string
	log        *logger.Entry
}

// NewClient 创建腾讯数据提供商
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
				DisableKeepAlives:   false,
				MaxConnsPerHost:     10,
			},
			Timeout: 15 * time.Second,
		},
		userAgent: "StockSub/1.0",
		log:       logger.WithComponent("TencentProvider"),
	}
}

// Name 返回提供商名称
func (p *Client) Name() string {
	return "tencent"
}

// GetRateLimit 获取请求频率限制 (为了接口兼容性提供默认值)
func (p *Client) GetRateLimit() time.Duration {
	return 200 * time.Millisecond // 默认频率限制，实际由装饰器控制
}

// IsHealthy 检查提供商健康状态
func (p *Client) IsHealthy() bool {
	// 简单的健康检查：检查HTTP客户端是否可用
	return p.httpClient != nil
}

// FetchStockData 获取股票数据 (实现 core.RealtimeStockProvider 接口)
func (p *Client) FetchStockData(ctx context.Context, symbols []string) ([]core.StockData, error) {
	result, _, err := p.FetchStockDataWithRaw(ctx, symbols)
	return result, err
}

// FetchStockDataWithRaw 获取股票数据和原始响应 (实现 core.RealtimeStockProvider 接口)
func (p *Client) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]core.StockData, string, error) {
	debugMode := os.Getenv("DEBUG") == "1"

	if len(symbols) == 0 {
		return []core.StockData{}, "", nil
	}

	if debugMode {
		p.log.Debugf("Starting FetchStockDataWithRaw for symbols: %v", symbols)
	}

	url := p.buildURL(symbols)
	if debugMode {
		p.log.Debugf("Request URL: %s", url)
	}

	requestStart := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response failed: %w", err)
	}

	requestDuration := time.Since(requestStart)
	if debugMode {
		p.log.Debugf("HTTP request completed in %v, status: %d, body length: %d",
			requestDuration, resp.StatusCode, len(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP status error: %d", resp.StatusCode)
	}

	if len(body) == 0 {
		return nil, "", fmt.Errorf("empty response")
	}

	rawData := string(body)

	if debugMode {
		p.log.Debugf("Parsing response data...")
	}
	parseStart := time.Now()
	result := parseTencentData(rawData)
	parseTime := time.Since(parseStart)

	if debugMode {
		p.log.Infof("Parsing completed in %v, parsed %d records", parseTime, len(result))
	}

	return result, rawData, nil
}

// IsSymbolSupported 检查是否支持该股票代码
func (p *Client) IsSymbolSupported(symbol string) bool {
	if symbol == "" {
		return false
	}

	// A股上证
	if len(symbol) == 6 && (strings.HasPrefix(symbol, "6")) {
		return true
	}

	// A股深证
	if len(symbol) == 6 && (strings.HasPrefix(symbol, "0") ||
		strings.HasPrefix(symbol, "300")) {
		return true
	}

	// A股北交所
	if len(symbol) == 6 && (strings.HasPrefix(symbol, "43") ||
		strings.HasPrefix(symbol, "82") || strings.HasPrefix(symbol, "83") ||
		strings.HasPrefix(symbol, "87") || strings.HasPrefix(symbol, "920")) {
		return true
	}

	return false
}

// Close 关闭提供商，清理资源
func (p *Client) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}

// buildURL 构建腾讯行情URL
func (p *Client) buildURL(symbols []string) string {
	var parts []string
	for _, symbol := range symbols {
		prefix := p.getMarketPrefix(symbol)
		parts = append(parts, prefix+symbol)
	}

	return "http://qt.gtimg.cn/q=" + strings.Join(parts, ",")
}

// getMarketPrefix 根据股票代码获取市场前缀
func (p *Client) getMarketPrefix(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "6") || strings.HasPrefix(symbol, "5"):
		return "sh"
	case strings.HasPrefix(symbol, "0") || strings.HasPrefix(symbol, "3"):
		return "sz"
	case strings.HasPrefix(symbol, "4") || strings.HasPrefix(symbol, "8"):
		return "bj"
	default:
		return "sh" // 默认使用上海市场前缀
	}
}
