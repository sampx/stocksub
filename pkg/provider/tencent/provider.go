package tencent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"stocksub/pkg/logger"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/subscriber"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Provider 腾讯股票数据提供商
// 现在实现了新的 core.RealtimeStockProvider 接口
type Provider struct {
	httpClient  *http.Client
	lastRequest time.Time
	requestMu   sync.Mutex
	userAgent   string
	log         *logrus.Entry
	
	// 配置选项 - 为了向后兼容保留，但不在内部使用
	rateLimit  time.Duration
	maxRetries int
	timeout    time.Duration
}

// NewProvider 创建腾讯数据提供商
func NewProvider() *Provider {
	return &Provider{
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
		userAgent:  "StockSub/1.0",
		log:        logger.WithComponent("TencentProvider"),
		
		// 默认配置值，保持向后兼容
		rateLimit:  200 * time.Millisecond,
		maxRetries: 3,
		timeout:    15 * time.Second,
	}
}

// Name 返回提供商名称
func (p *Provider) Name() string {
	return "tencent"
}

// GetRateLimit 获取请求频率限制
func (p *Provider) GetRateLimit() time.Duration {
	return p.rateLimit
}

// IsHealthy 检查提供商健康状态
func (p *Provider) IsHealthy() bool {
	// 简单的健康检查：检查HTTP客户端是否可用
	return p.httpClient != nil
}

// SetRateLimit 设置请求频率限制
func (p *Provider) SetRateLimit(limit time.Duration) {
	p.rateLimit = limit
}

// SetTimeout 设置请求超时时间
func (p *Provider) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
	p.httpClient.Timeout = timeout
}

// SetMaxRetries 设置最大重试次数
func (p *Provider) SetMaxRetries(retries int) {
	p.maxRetries = retries
}

// FetchStockData 获取股票数据 (实现 core.RealtimeStockProvider 接口)
func (p *Provider) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	result, _, err := p.FetchStockDataWithRaw(ctx, symbols)
	return result, err
}

// FetchStockDataWithRaw 获取股票数据和原始响应 (实现 core.RealtimeStockProvider 接口)
func (p *Provider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	debugMode := os.Getenv("DEBUG") == "1"

	if len(symbols) == 0 {
		return []subscriber.StockData{}, "", nil
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
func (p *Provider) IsSymbolSupported(symbol string) bool {
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
func (p *Provider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}

// buildURL 构建腾讯行情URL
func (p *Provider) buildURL(symbols []string) string {
	var parts []string
	for _, symbol := range symbols {
		prefix := p.getMarketPrefix(symbol)
		parts = append(parts, prefix+symbol)
	}

	return "http://qt.gtimg.cn/q=" + strings.Join(parts, ",")
}

// getMarketPrefix 根据股票代码获取市场前缀
func (p *Provider) getMarketPrefix(symbol string) string {
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

// ===== 以下方法用于向后兼容 =====

// FetchData 获取股票数据 (兼容旧版 subscriber.Provider 接口)
func (p *Provider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	return p.FetchStockData(ctx, symbols)
}

// FetchDataWithRaw 获取股票数据和原始响应数据 (兼容旧版接口)
func (p *Provider) FetchDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	return p.FetchStockDataWithRaw(ctx, symbols)
}

// 确保 Provider 实现了所需的接口
var _ core.RealtimeStockProvider = (*Provider)(nil)
var _ core.Configurable = (*Provider)(nil)
var _ core.Closable = (*Provider)(nil)
var _ subscriber.Provider = (*Provider)(nil) // 向后兼容
