package subscriber

import (
	"context"
	"time"
)

// StockData 股票数据结构
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

// Subscription 订阅信息
type Subscription struct {
	Symbol   string        // 股票代码
	Interval time.Duration // 订阅间隔
	Callback CallbackFunc  // 回调函数
	Active   bool          // 是否激活
}

// CallbackFunc 数据回调函数类型
type CallbackFunc func(data StockData) error

// UpdateEvent 更新事件
type UpdateEvent struct {
	Type   EventType  `json:"type"`
	Symbol string     `json:"symbol"`
	Data   *StockData `json:"data,omitempty"`
	Error  error      `json:"error,omitempty"`
	Time   time.Time  `json:"timestamp"`
}

// EventType 事件类型
type EventType int

const (
	EventTypeData         EventType = iota // 数据更新
	EventTypeError                         // 错误事件
	EventTypeSubscribed                    // 订阅成功
	EventTypeUnsubscribed                  // 取消订阅
)

// Provider 数据提供商接口
type Provider interface {
	// Name 提供商名称
	Name() string

	// FetchData 获取股票数据
	FetchData(ctx context.Context, symbols []string) ([]StockData, error)

	// IsSymbolSupported 检查是否支持该股票代码
	IsSymbolSupported(symbol string) bool

	// GetRateLimit 获取请求限制信息
	GetRateLimit() time.Duration
}

// Subscriber 订阅器接口
type Subscriber interface {
	// Subscribe 订阅股票
	Subscribe(symbol string, interval time.Duration, callback CallbackFunc) error

	// Unsubscribe 取消订阅
	Unsubscribe(symbol string) error

	// Start 启动订阅器
	Start(ctx context.Context) error

	// Stop 停止订阅器
	Stop() error

	// GetSubscriptions 获取当前订阅列表
	GetSubscriptions() []Subscription

	// SetProvider 设置数据提供商
	SetProvider(provider Provider)
}
