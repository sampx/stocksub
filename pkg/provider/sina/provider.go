package sina

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"stocksub/pkg/logger"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/subscriber"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Provider 新浪股票数据提供商
type Provider struct {
	httpClient *http.Client
	userAgent  string
	log        *logrus.Entry
	baseURL    string
	rateLimit  time.Duration
}

// NewProvider 创建新浪数据提供商
func NewProvider() *Provider {
	return &Provider{
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
			Timeout: 15 * time.Second,
		},
		userAgent: "StockSub/1.0",
		log:       logger.WithComponent("SinaProvider"),
		baseURL:   "http://hq.sinajs.cn/list=",
		rateLimit: 200 * time.Millisecond, // 默认速率限制
	}
}

// Name 返回提供商名称
func (p *Provider) Name() string {
	return "sina"
}

// GetRateLimit 获取请求频率限制
func (p *Provider) GetRateLimit() time.Duration {
	return p.rateLimit
}

// IsHealthy 检查提供商健康状态
func (p *Provider) IsHealthy() bool {
	return p.httpClient != nil
}

// SetRateLimit 设置请求频率限制
func (p *Provider) SetRateLimit(limit time.Duration) {
	p.rateLimit = limit
}

// SetTimeout 设置请求超时时间
func (p *Provider) SetTimeout(timeout time.Duration) {
	p.httpClient.Timeout = timeout
}

// SetMaxRetries (空实现，为了接口兼容性)
func (p *Provider) SetMaxRetries(retries int) {
	// 新浪 provider 暂不支持重试逻辑
}

// Close 关闭提供商，清理资源
func (p *Provider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}

// IsSymbolSupported 检查是否支持该股票代码
func (p *Provider) IsSymbolSupported(symbol string) bool {
	if len(symbol) != 6 {
		return false
	}
	return strings.HasPrefix(symbol, "6") || strings.HasPrefix(symbol, "0") || strings.HasPrefix(symbol, "3") || strings.HasPrefix(symbol, "8") || strings.HasPrefix(symbol, "4")
}

// FetchStockData 获取股票数据
func (p *Provider) FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	result, _, err := p.FetchStockDataWithRaw(ctx, symbols)
	return result, err
}

// FetchStockDataWithRaw 获取股票数据和原始响应
func (p *Provider) FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error) {
	if len(symbols) == 0 {
		return []subscriber.StockData{}, "", nil
	}

	url := p.buildURL(symbols)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP status error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response failed: %w", err)
	}

	rawData := string(body)
	result := parseSinaData(rawData)

	return result, rawData, nil
}

// buildURL 构建新浪行情URL
func (p *Provider) buildURL(symbols []string) string {
	var parts []string
	for _, symbol := range symbols {
		prefix := p.getMarketPrefix(symbol)
		parts = append(parts, prefix+symbol)
	}
	return p.baseURL + strings.Join(parts, ",")
}

// getMarketPrefix 根据股票代码获取市场前缀
func (p *Provider) getMarketPrefix(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "6"):
		return "sh"
	case strings.HasPrefix(symbol, "0"), strings.HasPrefix(symbol, "3"):
		return "sz"
	case strings.HasPrefix(symbol, "8"), strings.HasPrefix(symbol, "4"):
		return "bj"
	default:
		return "sh" // 默认
	}
}

// 确保 Provider 实现了所需的接口
var _ core.RealtimeStockProvider = (*Provider)(nil)
var _ core.Configurable = (*Provider)(nil)
var _ core.Closable = (*Provider)(nil)
