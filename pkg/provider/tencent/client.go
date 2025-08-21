package tencent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"stocksub/pkg/logger"
	"stocksub/pkg/subscriber"

	"github.com/sirupsen/logrus"
)

// Provider 腾讯数据提供商
type Provider struct {
	httpClient  *http.Client
	lastRequest time.Time
	requestMu   sync.Mutex
	rateLimit   time.Duration
	maxRetries  int
	userAgent   string
	log         *logrus.Entry
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
		rateLimit:  200 * time.Millisecond,
		maxRetries: 3,
		userAgent:  "StockSub/1.0",
		log:        logger.WithComponent("TencentProvider"),
	}
}

// Name 返回提供商名称
func (p *Provider) Name() string {
	return "tencent"
}

// FetchData 获取股票数据
func (p *Provider) FetchData(ctx context.Context, symbols []string) ([]subscriber.StockData, error) {
	debugMode := os.Getenv("DEBUG") == "1"

	if len(symbols) == 0 {
		return []subscriber.StockData{}, nil
	}

	if debugMode {
		p.log.Debugf("Starting FetchData for symbols: %v", symbols)
	}

	// 限流控制
	if err := p.enforceRateLimit(); err != nil {
		return nil, err
	}

	url := p.buildURL(symbols)
	if debugMode {
		p.log.Debugf("Request URL: %s", url)
	}

	var lastErr error
	for i := 0; i < p.maxRetries; i++ {
		if i > 0 {
			if debugMode {
				p.log.Debugf("Retry attempt %d/%d", i+1, p.maxRetries)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(i) * time.Second):
			}
		}

		requestStart := time.Now()
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("create request failed: %w", err)
			if debugMode {
				p.log.Errorf("Request creation failed: %v", lastErr)
			}
			continue
		}

		req.Header.Set("User-Agent", p.userAgent)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			if debugMode {
				p.log.Errorf("HTTP request failed after %v: %v", time.Since(requestStart), lastErr)
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("read response failed: %w", err)
			if debugMode {
				p.log.Errorf("Response read failed: %v", lastErr)
			}
			continue
		}

		requestDuration := time.Since(requestStart)
		if debugMode {
			p.log.Debugf("HTTP request completed in %v, status: %d, body length: %d",
				requestDuration, resp.StatusCode, len(body))
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP status error: %d", resp.StatusCode)
			if debugMode {
				p.log.Errorf("HTTP status error: %d", resp.StatusCode)
			}
			continue
		}

		if len(body) == 0 {
			lastErr = fmt.Errorf("empty response")
			if debugMode {
				p.log.Warnf("Empty response received")
			}
			continue
		}

		if debugMode {
			p.log.Debugf("Parsing response data...")
		}
		parseStart := time.Now()
		result := parseTencentData(string(body))
		parseTime := time.Since(parseStart)

		if debugMode {
			p.log.Infof("Parsing completed in %v, parsed %d records", parseTime, len(result))
		}

		return result, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %v", p.maxRetries, lastErr)
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

// GetRateLimit 获取请求限制信息
func (p *Provider) GetRateLimit() time.Duration {
	return p.rateLimit
}

// SetRateLimit 设置请求频率限制
func (p *Provider) SetRateLimit(limit time.Duration) {
	p.rateLimit = limit
}

// SetMaxRetries 设置最大重试次数
func (p *Provider) SetMaxRetries(retries int) {
	p.maxRetries = retries
}

// SetTimeout 设置超时时间
func (p *Provider) SetTimeout(timeout time.Duration) {
	p.httpClient.Timeout = timeout
}

// enforceRateLimit 执行频率限制
func (p *Provider) enforceRateLimit() error {
	p.requestMu.Lock()
	defer p.requestMu.Unlock()

	elapsed := time.Since(p.lastRequest)
	if elapsed < p.rateLimit && !p.lastRequest.IsZero() {
		waitTime := p.rateLimit - elapsed
		time.Sleep(waitTime)
	}
	p.lastRequest = time.Now()

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
