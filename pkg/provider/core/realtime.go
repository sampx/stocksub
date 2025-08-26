package core

import (
	"context"
	"stocksub/pkg/subscriber"
)

// RealtimeStockProvider 实时股票数据提供商接口
// 用于获取股票的实时行情数据
type RealtimeStockProvider interface {
	Provider

	// FetchStockData 获取股票实时数据
	// symbols: 股票代码列表，如 ["600000", "000001"]
	// 返回对应的股票数据列表
	FetchStockData(ctx context.Context, symbols []string) ([]subscriber.StockData, error)

	// FetchStockDataWithRaw 获取股票数据和原始响应
	// 返回处理后的数据和原始响应字符串，用于调试和分析
	FetchStockDataWithRaw(ctx context.Context, symbols []string) ([]subscriber.StockData, string, error)

	// IsSymbolSupported 检查是否支持该股票代码
	IsSymbolSupported(symbol string) bool
}

// RealtimeIndexProvider 实时指数数据提供商接口
// 用于获取指数的实时行情数据
type RealtimeIndexProvider interface {
	Provider

	// FetchIndexData 获取指数实时数据
	// indexSymbols: 指数代码列表，如 ["000001", "399001"]
	// 返回对应的指数数据列表
	FetchIndexData(ctx context.Context, indexSymbols []string) ([]IndexData, error)

	// IsIndexSupported 检查是否支持该指数代码
	IsIndexSupported(indexSymbol string) bool
}

// IndexData 指数数据结构
// 根据实际需求定义，当前提供基础结构
type IndexData struct {
	Symbol        string  `json:"symbol"`         // 指数代码
	Name          string  `json:"name"`           // 指数名称
	Value         float64 `json:"value"`          // 当前点数
	Change        float64 `json:"change"`         // 涨跌点数
	ChangePercent float64 `json:"change_percent"` // 涨跌幅(%)
	Volume        int64   `json:"volume"`         // 成交量
	Turnover      float64 `json:"turnover"`       // 成交额
}
