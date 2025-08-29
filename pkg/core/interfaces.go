package core

import (
	"context"
	"time"
)

// CoreStockData 股票数据结构 (核心包中的定义)
type StockData struct {
	// 基本信息
	Symbol        string  `json:"symbol"`         // 股票代码
	Name          string  `json:"name"`           // 股票名称
	Price         float64 `json:"price"`          // 当前价格
	Change        float64 `json:"change"`         // 涨跌额
	ChangePercent float64 `json:"change_percent"` // 涨跌幅
	MarketCode    int64   `json:"market_code"`    // 市场分类代码

	// 交易数据
	Volume    int64   `json:"volume"`     // 成交量
	Turnover  float64 `json:"turnover"`   // 成交额(元)
	Open      float64 `json:"open"`       // 开盘价
	High      float64 `json:"high"`       // 最高价
	Low       float64 `json:"low"`        // 最低价
	PrevClose float64 `json:"prev_close"` // 昨收价

	// 5档买卖盘数据
	BidPrice1  float64 `json:"bid_price1"`  // 买一价
	BidVolume1 int64   `json:"bid_volume1"` // 买一量
	BidPrice2  float64 `json:"bid_price2"`  // 买二价
	BidVolume2 int64   `json:"bid_volume2"` // 买二量
	BidPrice3  float64 `json:"bid_price3"`  // 买三价
	BidVolume3 int64   `json:"bid_volume3"` // 买三量
	BidPrice4  float64 `json:"bid_price4"`  // 买四价
	BidVolume4 int64   `json:"bid_volume4"` // 买四量
	BidPrice5  float64 `json:"bid_price5"`  // 买五价
	BidVolume5 int64   `json:"bid_volume5"` // 买五量
	AskPrice1  float64 `json:"ask_price1"`  // 卖一价
	AskVolume1 int64   `json:"ask_volume1"` // 卖一量
	AskPrice2  float64 `json:"ask_price2"`  // 卖二价
	AskVolume2 int64   `json:"ask_volume2"` // 卖二量
	AskPrice3  float64 `json:"ask_price3"`  // 卖三价
	AskVolume3 int64   `json:"ask_volume3"` // 卖三量
	AskPrice4  float64 `json:"ask_price4"`  // 卖四价
	AskVolume4 int64   `json:"ask_volume4"` // 卖四量
	AskPrice5  float64 `json:"ask_price5"`  // 卖五价
	AskVolume5 int64   `json:"ask_volume5"` // 卖五量

	// 内外盘数据
	InnerDisc int64 `json:"inner_disc"` // 内盘
	OuterDisc int64 `json:"outer_disc"` // 外盘

	// 财务指标
	TurnoverRate float64 `json:"turnover_rate"` // 换手率
	PE           float64 `json:"pe"`            // 市盈率
	PB           float64 `json:"pb"`            // 市净率
	Amplitude    float64 `json:"amplitude"`     // 振幅
	Circulation  float64 `json:"circulation"`   // 流通市值(亿元)
	MarketValue  float64 `json:"market_value"`  // 总市值(亿元)
	LimitUp      float64 `json:"limit_up"`      // 涨停价
	LimitDown    float64 `json:"limit_down"`    // 跌停价

	// 时间信息
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// IndexData 指数数据结构
type IndexData struct {
	Symbol        string  `json:"symbol"`         // 指数代码
	Name          string  `json:"name"`           // 指数名称
	Value         float64 `json:"value"`          // 当前点数
	Change        float64 `json:"change"`         // 涨跌点数
	ChangePercent float64 `json:"change_percent"` // 涨跌幅(%)
	Volume        int64   `json:"volume"`         // 成交量
	Turnover      float64 `json:"turnover"`       // 成交额
}

// HistoricalData 历史数据点
// 表示一个时间周期内的K线数据
type HistoricalData struct {
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

// Query 定义了在存储层进行数据查询的条件。
type Query struct {
	Symbols   []string  `json:"symbols"`    // 目标股票代码
	StartTime time.Time `json:"start_time"` // 查询的开始时间
	EndTime   time.Time `json:"end_time"`   // 查询的结束时间
	Fields    []string  `json:"fields"`     // 需要返回的字段
	Limit     int       `json:"limit"`      // 返回记录的最大数量
	Offset    int       `json:"offset"`     // 返回记录的偏移量
}

// Record 代表一条通用的、可被存储的数据记录。
type Record struct {
	Type      string      `json:"type"`      // 数据类型 (e.g., "stock_data", "performance_metric")
	Symbol    string      `json:"symbol"`    // 关联的股票代码
	Timestamp time.Time   `json:"timestamp"` // 记录生成的时间戳
	Date      string      `json:"date"`      // 记录生成的日期 (YYYY-MM-DD)
	Fields    []string    `json:"fields"`    // 用于CSV存储的字段数组
	Data      interface{} `json:"data"`      // 原始数据对象
}

// BatchGetter 批量获取接口
type BatchGetter interface {
	BatchGet(ctx context.Context, keys []string) (map[string]any, error)
}

// BatchSetter 批量设置接口
type BatchSetter interface {
	BatchSet(ctx context.Context, items map[string]any, ttl time.Duration) error
}
