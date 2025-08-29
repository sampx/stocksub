package subscriber

import (
	"context"
	"encoding/json"
	"time"

	"stocksub/pkg/core"
	"stocksub/pkg/provider"
)

// Subscription 订阅信息
type Subscription struct {
	Symbol   string        // 股票代码
	Interval time.Duration // 订阅间隔
	Callback CallbackFunc  // 回调函数
	Active   bool          // 是否激活
}

// CallbackFunc 数据回调函数类型
type CallbackFunc func(data core.StockData) error

// UpdateEvent 更新事件
type UpdateEvent struct {
	Type   EventType       `json:"type"`
	Symbol string          `json:"symbol"`
	Data   *core.StockData `json:"data,omitempty"`
	Error  error           `json:"error,omitempty"`
	Time   time.Time       `json:"timestamp"`
}

// EventType 事件类型
type EventType int

const (
	EventTypeData         EventType = iota // 数据更新
	EventTypeError                         // 错误事件
	EventTypeSubscribed                    // 订阅成功
	EventTypeUnsubscribed                  // 取消订阅
)

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
	SetProvider(provider provider.Provider)
}

// MarshalStockData a helper to marshal stock data to json
func MarshalStockData(sd core.StockData) ([]byte, error) {
	return json.Marshal(sd)
}

// UnmarshalStockData a helper to unmarshal stock data from json
func UnmarshalStockData(data []byte, sd *core.StockData) error {
	return json.Unmarshal(data, sd)
}
