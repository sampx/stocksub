package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
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

// MarshalStockData a helper to marshal stock data to json
func MarshalStockData(sd StockData) ([]byte, error) {
	return json.Marshal(sd)
}

// UnmarshalStockData a helper to unmarshal stock data from json
func UnmarshalStockData(data []byte, sd *StockData) error {
	return json.Unmarshal(data, sd)
}

// FieldType 支持的字段类型
type FieldType int

const (
	FieldTypeString FieldType = iota
	FieldTypeInt
	FieldTypeFloat64
	FieldTypeBool
	FieldTypeTime
)

// String returns the string representation of FieldType
func (ft FieldType) String() string {
	switch ft {
	case FieldTypeString:
		return "string"
	case FieldTypeInt:
		return "int"
	case FieldTypeFloat64:
		return "float64"
	case FieldTypeBool:
		return "bool"
	case FieldTypeTime:
		return "time"
	default:
		return "unknown"
	}
}

// FieldDefinition 字段定义
type FieldDefinition struct {
	Name         string                  `json:"name"`          // 字段名（英文）
	Type         FieldType               `json:"type"`          // 字段类型
	Description  string                  `json:"description"`   // 中文描述
	Comment      string                  `json:"comment"`       // 中文字段备注（可选）
	Required     bool                    `json:"required"`      // 是否必填
	DefaultValue interface{}             `json:"default_value"` // 默认值
	Validator    func(interface{}) error `json:"-"`             // 验证函数（不序列化）
}

// DataSchema 数据模式定义
type DataSchema struct {
	Name        string                      `json:"name"`        // 模式名称
	Description string                      `json:"description"` // 模式描述
	Fields      map[string]*FieldDefinition `json:"fields"`      // 字段定义
	FieldOrder  []string                    `json:"field_order"` // 字段顺序（用于CSV输出）
}

// StructuredData 结构化数据，支持动态字段和元数据
type StructuredData struct {
	Schema    *DataSchema            `json:"schema"`    // 数据模式定义
	Values    map[string]interface{} `json:"values"`    // 字段值存储
	Timestamp time.Time              `json:"timestamp"` // 数据时间戳
}

// NewStructuredData 创建新的结构化数据实例
func NewStructuredData(schema *DataSchema) *StructuredData {
	return &StructuredData{
		Schema:    schema,
		Values:    make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// SetField 设置字段值（类型安全）
func (sd *StructuredData) SetField(fieldName string, value interface{}) error {
	fieldDef, exists := sd.Schema.Fields[fieldName]
	if !exists {
		return NewStructuredDataError(ErrFieldNotFound, fieldName, "field not found in schema")
	}

	// 类型验证
	if !isValidFieldType(value, fieldDef.Type) {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, "invalid field type")
	}

	// 自定义验证
	if fieldDef.Validator != nil {
		if err := fieldDef.Validator(value); err != nil {
			return NewStructuredDataError(ErrFieldValidationFailed, fieldName, err.Error())
		}
	}

	sd.Values[fieldName] = value
	return nil
}

// GetField 获取字段值（类型安全）
func (sd *StructuredData) GetField(fieldName string) (interface{}, error) {
	fieldDef, exists := sd.Schema.Fields[fieldName]
	if !exists {
		return nil, NewStructuredDataError(ErrFieldNotFound, fieldName, "field not found in schema")
	}

	value, exists := sd.Values[fieldName]
	if !exists {
		// 返回默认值
		if fieldDef.DefaultValue != nil {
			return fieldDef.DefaultValue, nil
		}
		if fieldDef.Required {
			return nil, NewStructuredDataError(ErrRequiredFieldMissing, fieldName, "required field missing")
		}
		return nil, nil
	}

	return value, nil
}

// ValidateData 验证数据完整性
func (sd *StructuredData) ValidateData() error {
	for fieldName, fieldDef := range sd.Schema.Fields {
		value, exists := sd.Values[fieldName]

		// 检查必填字段
		if fieldDef.Required && !exists {
			return NewStructuredDataError(ErrRequiredFieldMissing, fieldName, "required field missing")
		}

		// 类型验证
		if exists && !isValidFieldType(value, fieldDef.Type) {
			return NewStructuredDataError(ErrInvalidFieldType, fieldName, "invalid field type")
		}

		// 自定义验证
		if exists && fieldDef.Validator != nil {
			if err := fieldDef.Validator(value); err != nil {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, err.Error())
			}
		}
	}
	return nil
}

// ErrorCode 错误代码类型
type ErrorCode string

// 错误代码常量
const (
	ErrInvalidFieldType      ErrorCode = "INVALID_FIELD_TYPE"
	ErrRequiredFieldMissing  ErrorCode = "REQUIRED_FIELD_MISSING"
	ErrFieldValidationFailed ErrorCode = "FIELD_VALIDATION_FAILED"
	ErrSchemaNotFound        ErrorCode = "SCHEMA_NOT_FOUND"
	ErrCSVHeaderMismatch     ErrorCode = "CSV_HEADER_MISMATCH"
	ErrFieldNotFound         ErrorCode = "FIELD_NOT_FOUND"
)

// StructuredDataError 结构化数据相关错误
type StructuredDataError struct {
	Code    ErrorCode `json:"code"`
	Field   string    `json:"field"`
	Message string    `json:"message"`
	Cause   error     `json:"cause,omitempty"`
}

// Error 实现 error 接口
func (e *StructuredDataError) Error() string {
	if e.Field != "" {
		return string(e.Code) + ": " + e.Field + " - " + e.Message
	}
	return string(e.Code) + ": " + e.Message
}

// NewStructuredDataError 创建新的结构化数据错误
func NewStructuredDataError(code ErrorCode, field, message string) *StructuredDataError {
	return &StructuredDataError{
		Code:    code,
		Field:   field,
		Message: message,
	}
}

// StockDataSchema 预定义的股票数据模式
var StockDataSchema = &DataSchema{
	Name:        "stock_data",
	Description: "股票行情数据",
	Fields: map[string]*FieldDefinition{
		// 基本信息
		"symbol": {
			Name:        "symbol",
			Type:        FieldTypeString,
			Description: "股票代码",
			Comment:     "如600000、000001等",
			Required:    true,
		},
		"name": {
			Name:        "name",
			Type:        FieldTypeString,
			Description: "股票名称",
			Comment:     "股票的中文名称",
			Required:    true,
		},
		"price": {
			Name:        "price",
			Type:        FieldTypeFloat64,
			Description: "当前价格",
			Comment:     "最新成交价格",
			Required:    true,
		},
		"change": {
			Name:        "change",
			Type:        FieldTypeFloat64,
			Description: "涨跌额",
			Comment:     "相对昨收价的涨跌金额",
		},
		"change_percent": {
			Name:        "change_percent",
			Type:        FieldTypeFloat64,
			Description: "涨跌幅(%)",
			Comment:     "涨跌幅百分比",
		},
		"market_code": {
			Name:        "market_code",
			Type:        FieldTypeInt,
			Description: "市场分类代码",
			Comment:     "交易所市场代码",
		},
		// 交易数据
		"volume": {
			Name:        "volume",
			Type:        FieldTypeInt,
			Description: "成交量",
			Comment:     "累计成交股数",
		},
		"turnover": {
			Name:        "turnover",
			Type:        FieldTypeFloat64,
			Description: "成交额(元)",
			Comment:     "累计成交金额",
		},
		"open": {
			Name:        "open",
			Type:        FieldTypeFloat64,
			Description: "开盘价",
			Comment:     "当日开盘价格",
		},
		"high": {
			Name:        "high",
			Type:        FieldTypeFloat64,
			Description: "最高价",
			Comment:     "当日最高成交价",
		},
		"low": {
			Name:        "low",
			Type:        FieldTypeFloat64,
			Description: "最低价",
			Comment:     "当日最低成交价",
		},
		"prev_close": {
			Name:        "prev_close",
			Type:        FieldTypeFloat64,
			Description: "昨收价",
			Comment:     "前一交易日收盘价",
		},
		// 5档买卖盘数据
		"bid_price1": {
			Name:        "bid_price1",
			Type:        FieldTypeFloat64,
			Description: "买一价",
			Comment:     "买盘第一档价格",
		},
		"bid_volume1": {
			Name:        "bid_volume1",
			Type:        FieldTypeInt,
			Description: "买一量",
			Comment:     "买盘第一档数量",
		},
		"bid_price2": {
			Name:        "bid_price2",
			Type:        FieldTypeFloat64,
			Description: "买二价",
			Comment:     "买盘第二档价格",
		},
		"bid_volume2": {
			Name:        "bid_volume2",
			Type:        FieldTypeInt,
			Description: "买二量",
			Comment:     "买盘第二档数量",
		},
		"bid_price3": {
			Name:        "bid_price3",
			Type:        FieldTypeFloat64,
			Description: "买三价",
			Comment:     "买盘第三档价格",
		},
		"bid_volume3": {
			Name:        "bid_volume3",
			Type:        FieldTypeInt,
			Description: "买三量",
			Comment:     "买盘第三档数量",
		},
		"bid_price4": {
			Name:        "bid_price4",
			Type:        FieldTypeFloat64,
			Description: "买四价",
			Comment:     "买盘第四档价格",
		},
		"bid_volume4": {
			Name:        "bid_volume4",
			Type:        FieldTypeInt,
			Description: "买四量",
			Comment:     "买盘第四档数量",
		},
		"bid_price5": {
			Name:        "bid_price5",
			Type:        FieldTypeFloat64,
			Description: "买五价",
			Comment:     "买盘第五档价格",
		},
		"bid_volume5": {
			Name:        "bid_volume5",
			Type:        FieldTypeInt,
			Description: "买五量",
			Comment:     "买盘第五档数量",
		},
		"ask_price1": {
			Name:        "ask_price1",
			Type:        FieldTypeFloat64,
			Description: "卖一价",
			Comment:     "卖盘第一档价格",
		},
		"ask_volume1": {
			Name:        "ask_volume1",
			Type:        FieldTypeInt,
			Description: "卖一量",
			Comment:     "卖盘第一档数量",
		},
		"ask_price2": {
			Name:        "ask_price2",
			Type:        FieldTypeFloat64,
			Description: "卖二价",
			Comment:     "卖盘第二档价格",
		},
		"ask_volume2": {
			Name:        "ask_volume2",
			Type:        FieldTypeInt,
			Description: "卖二量",
			Comment:     "卖盘第二档数量",
		},
		"ask_price3": {
			Name:        "ask_price3",
			Type:        FieldTypeFloat64,
			Description: "卖三价",
			Comment:     "卖盘第三档价格",
		},
		"ask_volume3": {
			Name:        "ask_volume3",
			Type:        FieldTypeInt,
			Description: "卖三量",
			Comment:     "卖盘第三档数量",
		},
		"ask_price4": {
			Name:        "ask_price4",
			Type:        FieldTypeFloat64,
			Description: "卖四价",
			Comment:     "卖盘第四档价格",
		},
		"ask_volume4": {
			Name:        "ask_volume4",
			Type:        FieldTypeInt,
			Description: "卖四量",
			Comment:     "卖盘第四档数量",
		},
		"ask_price5": {
			Name:        "ask_price5",
			Type:        FieldTypeFloat64,
			Description: "卖五价",
			Comment:     "卖盘第五档价格",
		},
		"ask_volume5": {
			Name:        "ask_volume5",
			Type:        FieldTypeInt,
			Description: "卖五量",
			Comment:     "卖盘第五档数量",
		},
		// 内外盘数据
		"inner_disc": {
			Name:        "inner_disc",
			Type:        FieldTypeInt,
			Description: "内盘",
			Comment:     "主动卖出成交量",
		},
		"outer_disc": {
			Name:        "outer_disc",
			Type:        FieldTypeInt,
			Description: "外盘",
			Comment:     "主动买入成交量",
		},
		// 财务指标
		"turnover_rate": {
			Name:        "turnover_rate",
			Type:        FieldTypeFloat64,
			Description: "换手率",
			Comment:     "成交量占流通股本的比例",
		},
		"pe": {
			Name:        "pe",
			Type:        FieldTypeFloat64,
			Description: "市盈率",
			Comment:     "股价与每股收益的比率",
		},
		"pb": {
			Name:        "pb",
			Type:        FieldTypeFloat64,
			Description: "市净率",
			Comment:     "股价与每股净资产的比率",
		},
		"amplitude": {
			Name:        "amplitude",
			Type:        FieldTypeFloat64,
			Description: "振幅",
			Comment:     "最高价与最低价的差值占昨收价的比例",
		},
		"circulation": {
			Name:        "circulation",
			Type:        FieldTypeFloat64,
			Description: "流通市值(亿元)",
			Comment:     "流通股本的市场价值",
		},
		"market_value": {
			Name:        "market_value",
			Type:        FieldTypeFloat64,
			Description: "总市值(亿元)",
			Comment:     "总股本的市场价值",
		},
		"limit_up": {
			Name:        "limit_up",
			Type:        FieldTypeFloat64,
			Description: "涨停价",
			Comment:     "当日涨停价格",
		},
		"limit_down": {
			Name:        "limit_down",
			Type:        FieldTypeFloat64,
			Description: "跌停价",
			Comment:     "当日跌停价格",
		},
		// 时间信息
		"timestamp": {
			Name:        "timestamp",
			Type:        FieldTypeTime,
			Description: "数据时间",
			Comment:     "数据获取时间戳",
			Required:    true,
		},
	},
	FieldOrder: []string{
		"symbol", "name", "price", "change", "change_percent", "market_code",
		"volume", "turnover", "open", "high", "low", "prev_close",
		"bid_price1", "bid_volume1", "bid_price2", "bid_volume2", "bid_price3", "bid_volume3",
		"bid_price4", "bid_volume4", "bid_price5", "bid_volume5",
		"ask_price1", "ask_volume1", "ask_price2", "ask_volume2", "ask_price3", "ask_volume3",
		"ask_price4", "ask_volume4", "ask_price5", "ask_volume5",
		"inner_disc", "outer_disc",
		"turnover_rate", "pe", "pb", "amplitude", "circulation", "market_value",
		"limit_up", "limit_down", "timestamp",
	},
}

// StockDataToStructuredData 将 StockData 转换为 StructuredData
func StockDataToStructuredData(stockData StockData) (*StructuredData, error) {
	sd := NewStructuredData(StockDataSchema)
	sd.Timestamp = stockData.Timestamp

	// 设置所有字段值
	fieldMappings := map[string]interface{}{
		"symbol":         stockData.Symbol,
		"name":           stockData.Name,
		"price":          stockData.Price,
		"change":         stockData.Change,
		"change_percent": stockData.ChangePercent,
		"market_code":    stockData.MarketCode,
		"volume":         stockData.Volume,
		"turnover":       stockData.Turnover,
		"open":           stockData.Open,
		"high":           stockData.High,
		"low":            stockData.Low,
		"prev_close":     stockData.PrevClose,
		"bid_price1":     stockData.BidPrice1,
		"bid_volume1":    stockData.BidVolume1,
		"bid_price2":     stockData.BidPrice2,
		"bid_volume2":    stockData.BidVolume2,
		"bid_price3":     stockData.BidPrice3,
		"bid_volume3":    stockData.BidVolume3,
		"bid_price4":     stockData.BidPrice4,
		"bid_volume4":    stockData.BidVolume4,
		"bid_price5":     stockData.BidPrice5,
		"bid_volume5":    stockData.BidVolume5,
		"ask_price1":     stockData.AskPrice1,
		"ask_volume1":    stockData.AskVolume1,
		"ask_price2":     stockData.AskPrice2,
		"ask_volume2":    stockData.AskVolume2,
		"ask_price3":     stockData.AskPrice3,
		"ask_volume3":    stockData.AskVolume3,
		"ask_price4":     stockData.AskPrice4,
		"ask_volume4":    stockData.AskVolume4,
		"ask_price5":     stockData.AskPrice5,
		"ask_volume5":    stockData.AskVolume5,
		"inner_disc":     stockData.InnerDisc,
		"outer_disc":     stockData.OuterDisc,
		"turnover_rate":  stockData.TurnoverRate,
		"pe":             stockData.PE,
		"pb":             stockData.PB,
		"amplitude":      stockData.Amplitude,
		"circulation":    stockData.Circulation,
		"market_value":   stockData.MarketValue,
		"limit_up":       stockData.LimitUp,
		"limit_down":     stockData.LimitDown,
		"timestamp":      stockData.Timestamp,
	}

	// 设置字段值
	for fieldName, value := range fieldMappings {
		if err := sd.SetField(fieldName, value); err != nil {
			return nil, err
		}
	}

	return sd, nil
}

// StructuredDataToStockData 将 StructuredData 转换为 StockData
func StructuredDataToStockData(sd *StructuredData) (*StockData, error) {
	if sd.Schema.Name != "stock_data" {
		return nil, NewStructuredDataError(ErrSchemaNotFound, "", "schema is not stock_data")
	}

	stockData := &StockData{}

	// 获取字段值的辅助函数
	getString := func(fieldName string) string {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			if str, ok := value.(string); ok {
				return str
			}
		}
		return ""
	}

	getFloat64 := func(fieldName string) float64 {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			if f, ok := value.(float64); ok {
				return f
			}
		}
		return 0
	}

	getInt64 := func(fieldName string) int64 {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			switch v := value.(type) {
			case int64:
				return v
			case int:
				return int64(v)
			case int32:
				return int64(v)
			}
		}
		return 0
	}

	getTime := func(fieldName string) time.Time {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			if t, ok := value.(time.Time); ok {
				return t
			}
		}
		return time.Time{}
	}

	// 填充 StockData 字段
	stockData.Symbol = getString("symbol")
	stockData.Name = getString("name")
	stockData.Price = getFloat64("price")
	stockData.Change = getFloat64("change")
	stockData.ChangePercent = getFloat64("change_percent")
	stockData.MarketCode = getInt64("market_code")
	stockData.Volume = getInt64("volume")
	stockData.Turnover = getFloat64("turnover")
	stockData.Open = getFloat64("open")
	stockData.High = getFloat64("high")
	stockData.Low = getFloat64("low")
	stockData.PrevClose = getFloat64("prev_close")
	stockData.BidPrice1 = getFloat64("bid_price1")
	stockData.BidVolume1 = getInt64("bid_volume1")
	stockData.BidPrice2 = getFloat64("bid_price2")
	stockData.BidVolume2 = getInt64("bid_volume2")
	stockData.BidPrice3 = getFloat64("bid_price3")
	stockData.BidVolume3 = getInt64("bid_volume3")
	stockData.BidPrice4 = getFloat64("bid_price4")
	stockData.BidVolume4 = getInt64("bid_volume4")
	stockData.BidPrice5 = getFloat64("bid_price5")
	stockData.BidVolume5 = getInt64("bid_volume5")
	stockData.AskPrice1 = getFloat64("ask_price1")
	stockData.AskVolume1 = getInt64("ask_volume1")
	stockData.AskPrice2 = getFloat64("ask_price2")
	stockData.AskVolume2 = getInt64("ask_volume2")
	stockData.AskPrice3 = getFloat64("ask_price3")
	stockData.AskVolume3 = getInt64("ask_volume3")
	stockData.AskPrice4 = getFloat64("ask_price4")
	stockData.AskVolume4 = getInt64("ask_volume4")
	stockData.AskPrice5 = getFloat64("ask_price5")
	stockData.AskVolume5 = getInt64("ask_volume5")
	stockData.InnerDisc = getInt64("inner_disc")
	stockData.OuterDisc = getInt64("outer_disc")
	stockData.TurnoverRate = getFloat64("turnover_rate")
	stockData.PE = getFloat64("pe")
	stockData.PB = getFloat64("pb")
	stockData.Amplitude = getFloat64("amplitude")
	stockData.Circulation = getFloat64("circulation")
	stockData.MarketValue = getFloat64("market_value")
	stockData.LimitUp = getFloat64("limit_up")
	stockData.LimitDown = getFloat64("limit_down")
	stockData.Timestamp = getTime("timestamp")

	return stockData, nil
}

// isValidFieldType 验证字段类型是否匹配
func isValidFieldType(value interface{}, expectedType FieldType) bool {
	if value == nil {
		return true // nil 值总是有效的（可选字段）
	}

	switch expectedType {
	case FieldTypeString:
		_, ok := value.(string)
		return ok
	case FieldTypeInt:
		switch value.(type) {
		case int, int8, int16, int32, int64:
			return true
		default:
			return false
		}
	case FieldTypeFloat64:
		switch value.(type) {
		case float32, float64:
			return true
		default:
			return false
		}
	case FieldTypeBool:
		_, ok := value.(bool)
		return ok
	case FieldTypeTime:
		_, ok := value.(time.Time)
		return ok
	default:
		return false
	}
}

// ValidateSchema 验证数据模式定义的完整性
func ValidateSchema(schema *DataSchema) error {
	if schema == nil {
		return NewStructuredDataError(ErrSchemaNotFound, "", "schema cannot be nil")
	}

	if schema.Name == "" {
		return NewStructuredDataError(ErrSchemaNotFound, "", "schema name cannot be empty")
	}

	if len(schema.Fields) == 0 {
		return NewStructuredDataError(ErrSchemaNotFound, "", "schema must have at least one field")
	}

	// 验证字段顺序是否包含所有字段
	if len(schema.FieldOrder) > 0 {
		fieldOrderMap := make(map[string]bool)
		for _, fieldName := range schema.FieldOrder {
			fieldOrderMap[fieldName] = true
			if _, exists := schema.Fields[fieldName]; !exists {
				return NewStructuredDataError(ErrFieldNotFound, fieldName, "field in order not found in schema fields")
			}
		}

		// 检查是否有字段没有在顺序中
		for fieldName := range schema.Fields {
			if !fieldOrderMap[fieldName] {
				return NewStructuredDataError(ErrFieldNotFound, fieldName, "field not specified in field order")
			}
		}
	}

	// 验证每个字段定义
	for fieldName, fieldDef := range schema.Fields {
		if err := ValidateFieldDefinition(fieldName, fieldDef); err != nil {
			return err
		}
	}

	return nil
}

// ValidateFieldDefinition 验证字段定义的正确性
func ValidateFieldDefinition(fieldName string, fieldDef *FieldDefinition) error {
	if fieldDef == nil {
		return NewStructuredDataError(ErrFieldNotFound, fieldName, "field definition cannot be nil")
	}

	if fieldDef.Name == "" {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, "field name cannot be empty")
	}

	if fieldDef.Name != fieldName {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, "field name mismatch")
	}

	// 验证字段类型
	if fieldDef.Type < FieldTypeString || fieldDef.Type > FieldTypeTime {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, "invalid field type")
	}

	// 验证默认值类型
	if fieldDef.DefaultValue != nil {
		if !isValidFieldType(fieldDef.DefaultValue, fieldDef.Type) {
			return NewStructuredDataError(ErrInvalidFieldType, fieldName, "default value type mismatch")
		}
	}

	// 验证必填字段是否有默认值
	if fieldDef.Required && fieldDef.DefaultValue == nil {
		// 注意：必填字段可以没有默认值，这样可以强制用户提供值
	}

	return nil
}

// ValidateFieldValue 验证单个字段值
func ValidateFieldValue(fieldName string, value interface{}, fieldDef *FieldDefinition) error {
	if fieldDef == nil {
		return NewStructuredDataError(ErrFieldNotFound, fieldName, "field definition not found")
	}

	// 检查必填字段
	if fieldDef.Required && value == nil {
		return NewStructuredDataError(ErrRequiredFieldMissing, fieldName, "required field missing")
	}

	// 如果值为 nil 且不是必填字段，则有效
	if value == nil {
		return nil
	}

	// 类型验证
	if !isValidFieldType(value, fieldDef.Type) {
		return NewStructuredDataError(ErrInvalidFieldType, fieldName, fmt.Sprintf("expected %s, got %T", fieldDef.Type.String(), value))
	}

	// 范围验证（数值类型）
	if err := validateValueRange(fieldName, value, fieldDef); err != nil {
		return err
	}

	// 自定义验证
	if fieldDef.Validator != nil {
		if err := fieldDef.Validator(value); err != nil {
			return NewStructuredDataError(ErrFieldValidationFailed, fieldName, err.Error())
		}
	}

	return nil
}

// validateValueRange 验证数值范围（可扩展用于不同字段类型的范围验证）
func validateValueRange(fieldName string, value interface{}, fieldDef *FieldDefinition) error {
	switch fieldDef.Type {
	case FieldTypeFloat64:
		if f, ok := value.(float64); ok {
			// 检查是否为 NaN 或无穷大
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return NewStructuredDataError(ErrInvalidFieldType, fieldName, "value cannot be NaN or Infinity")
			}
			// 可以添加更多的范围检查
			if fieldName == "price" && f < 0 {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "price cannot be negative")
			}
		}
	case FieldTypeInt:
		// 可以添加整数范围验证
		if fieldName == "volume" || strings.Contains(fieldName, "volume") {
			var intVal int64
			switch v := value.(type) {
			case int:
				intVal = int64(v)
			case int64:
				intVal = v
			case int32:
				intVal = int64(v)
			}
			if intVal < 0 {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "volume cannot be negative")
			}
		}
	case FieldTypeString:
		if str, ok := value.(string); ok {
			// 字符串长度验证
			if fieldName == "symbol" && (len(str) < 2 || len(str) > 10) {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "symbol length must be between 2 and 10 characters")
			}
			if fieldName == "name" && len(str) > 50 {
				return NewStructuredDataError(ErrFieldValidationFailed, fieldName, "name cannot exceed 50 characters")
			}
		}
	}

	return nil
}

// SetFieldSafe 安全地设置字段值，包含完整的验证
func (sd *StructuredData) SetFieldSafe(fieldName string, value interface{}) error {
	fieldDef, exists := sd.Schema.Fields[fieldName]
	if !exists {
		return NewStructuredDataError(ErrFieldNotFound, fieldName, "field not found in schema")
	}

	// 完整的字段值验证
	if err := ValidateFieldValue(fieldName, value, fieldDef); err != nil {
		return err
	}

	sd.Values[fieldName] = value
	return nil
}

// ValidateDataComplete 完整的数据验证（比 ValidateData 更严格）
func (sd *StructuredData) ValidateDataComplete() error {
	// 首先验证模式
	if err := ValidateSchema(sd.Schema); err != nil {
		return err
	}

	// 验证所有字段值
	for fieldName, fieldDef := range sd.Schema.Fields {
		value := sd.Values[fieldName]
		if err := ValidateFieldValue(fieldName, value, fieldDef); err != nil {
			return err
		}
	}

	// 检查是否有多余的字段
	for fieldName := range sd.Values {
		if _, exists := sd.Schema.Fields[fieldName]; !exists {
			return NewStructuredDataError(ErrFieldNotFound, fieldName, "unknown field in data")
		}
	}

	return nil
}

// GetFieldSafe 安全地获取字段值，包含类型转换
func (sd *StructuredData) GetFieldSafe(fieldName string, targetType FieldType) (interface{}, error) {
	fieldDef, exists := sd.Schema.Fields[fieldName]
	if !exists {
		return nil, NewStructuredDataError(ErrFieldNotFound, fieldName, "field not found in schema")
	}

	value, exists := sd.Values[fieldName]
	if !exists {
		// 返回默认值
		if fieldDef.DefaultValue != nil {
			return fieldDef.DefaultValue, nil
		}
		if fieldDef.Required {
			return nil, NewStructuredDataError(ErrRequiredFieldMissing, fieldName, "required field missing")
		}
		return nil, nil
	}

	// 类型匹配检查
	if targetType != fieldDef.Type {
		return nil, NewStructuredDataError(ErrInvalidFieldType, fieldName, fmt.Sprintf("field type is %s, requested %s", fieldDef.Type.String(), targetType.String()))
	}

	return value, nil
}
