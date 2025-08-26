package core

import (
	"context"
	"time"
)

// HistoricalProvider 历史数据提供商接口
// 用于获取股票的历史K线数据
type HistoricalProvider interface {
	Provider

	// FetchHistoricalData 获取历史数据
	// symbol: 股票代码
	// start: 开始时间
	// end: 结束时间
	// period: 时间周期，如 "1d", "1h", "5m" 等
	FetchHistoricalData(ctx context.Context, symbol string, start, end time.Time, period string) ([]HistoricalDataPoint, error)

	// GetSupportedPeriods 获取支持的时间周期列表
	GetSupportedPeriods() []string

	// IsSymbolSupported 检查是否支持该股票代码
	IsSymbolSupported(symbol string) bool
}

// HistoricalDataPoint 历史数据点
// 表示一个时间周期内的K线数据
type HistoricalDataPoint struct {
	Symbol    string    `json:"symbol"`    // 股票代码
	Timestamp time.Time `json:"timestamp"` // 时间戳
	Open      float64   `json:"open"`      // 开盘价
	High      float64   `json:"high"`      // 最高价
	Low       float64   `json:"low"`       // 最低价
	Close     float64   `json:"close"`     // 收盘价
	Volume    int64     `json:"volume"`    // 成交量
	Turnover  float64   `json:"turnover"`  // 成交额
	Period    string    `json:"period"`    // 时间周期
}

// HistoricalBatchProvider 批量历史数据提供商接口
// 支持批量获取多个股票的历史数据
type HistoricalBatchProvider interface {
	HistoricalProvider

	// FetchHistoricalDataBatch 批量获取历史数据
	// symbols: 股票代码列表
	// start: 开始时间
	// end: 结束时间
	// period: 时间周期
	FetchHistoricalDataBatch(ctx context.Context, symbols []string, start, end time.Time, period string) (map[string][]HistoricalDataPoint, error)
}
