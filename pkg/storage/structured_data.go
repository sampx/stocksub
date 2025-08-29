package storage

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"stocksub/pkg/core"
)

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

// NewStructuredData 创建并返回一个新的 StructuredData 实例
//
// 参数:
//
//	schema (*DataSchema): 数据结构模式定义，用于规范数据的格式和类型
//
// 返回值:
//
//	*StructuredData: 新创建的 StructuredData 实例指针，包含以下字段：
//	  - Schema: 传入的数据结构模式
//	  - Values: 初始化的空 map，用于存储键值对数据
//	  - Timestamp: 当前时间戳，记录实例创建时间
func NewStructuredData(schema *DataSchema) *StructuredData {
	return &StructuredData{
		Schema:    schema,
		Values:    make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// SetField 设置结构化数据中指定字段的值（类型安全）
//
// 参数:
//   - fieldName: 要设置的字段名
//   - value: 要设置的值，可以是任意类型
//
// 返回值:
//   - error: 如果设置过程中出现错误，返回相应的错误信息。可能的错误包括：
//   - ErrFieldNotFound: 字段在模式中不存在
//   - ErrInvalidFieldType: 字段类型无效
//   - ErrFieldValidationFailed: 字段验证失败
//
// 功能说明:
// 1. 首先检查字段是否在模式中定义
// 2. 验证字段值的类型是否匹配模式定义
// 3. 如果字段定义了自定义验证器，执行验证
// 4. 所有验证通过后，将值存储到结构化数据中
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

// GetField 根据字段名从结构化数据中获取字段值(类型安全）
// 参数:
//   - fieldName: 要获取的字段名称
//
// 返回值:
//   - interface{}: 字段的值，如果字段不存在且不是必需字段则返回nil
//   - error: 错误信息，如果字段不存在或必需字段缺失则返回相应错误
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

// ValidateData 验证结构化数据是否符合其模式定义
//
// 该方法会遍历模式定义中的所有字段，对数据进行以下验证：
// 1. 检查必填字段是否存在
// 2. 验证字段值的类型是否正确
// 3. 执行字段的自定义验证函数（如果存在）
//
// 返回值：
// - error: 如果验证失败，返回对应的 StructuredDataError；验证通过则返回 nil
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

// StockDataToStructuredData 将股票数据转换为结构化数据: StockData -> StructuredData
//
// 参数:
//
//	stockData (StockData): 输入的股票数据，包含股票的各种信息
//
// 返回值:
//
//	*StructuredData: 转换后的结构化数据
//	error: 如果转换过程中出现错误则返回错误信息，否则返回nil
//
// 该函数将原始的股票数据映射到预定义的结构化数据格式中，
// 包括股票的基本信息、价格信息、买卖盘信息、财务指标等。
// 所有的字段都会被正确映射，如果设置字段值时发生错误会立即返回。
func StockDataToStructuredData(stockData core.StockData) (*StructuredData, error) {
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

// StructuredDataToStockData 将结构化数据转换为股票数据 StructuredData -> StockData
//
// 参数:
//
//	sd (*StructuredData): 输入的结构化数据指针
//
// 返回值:
//
//	(*core.StockData): 转换后的股票数据指针
//	(error): 错误信息，如果转换成功则为nil
//
// 功能说明:
//  1. 首先验证输入数据的schema名称是否为"stock_data"
//  2. 定义了四个辅助函数用于获取不同类型的字段值:
//     - getString: 获取字符串类型字段
//     - getFloat64: 获取float64类型字段
//     - getInt64: 获取int64类型字段(支持int64、int、int32的转换)
//     - getTime: 获取time.Time类型字段
//  3. 使用辅助函数填充StockData的所有字段，包括:
//     - 基本信息(股票代码、名称等)
//     - 价格信息(当前价、涨跌幅等)
//     - 交易信息(成交量、成交额等)
//     - 盘口信息(五档买卖价格和数量)
//     - 财务指标(市盈率、市净率等)
//     - 其他指标(换手率、振幅等)
func StructuredDataToStockData(sd *StructuredData) (*core.StockData, error) {
	if sd.Schema.Name != "stock_data" {
		return nil, NewStructuredDataError(ErrSchemaNotFound, "", "schema is not stock_data")
	}

	stockData := &core.StockData{}
	stockDataValue := reflect.ValueOf(stockData).Elem() // 获取可写的结构体引用

	// 获取字段值的辅助函数
	getString := func(fieldName string) *string {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			if str, ok := value.(string); ok {
				return &str
			}
		}
		return nil
	}

	getFloat64 := func(fieldName string) *float64 {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			if f, ok := value.(float64); ok {
				return &f
			}
		}
		return nil
	}

	getInt64 := func(fieldName string) *int64 {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			switch v := value.(type) {
			case int64:
				return &v
			case int:
				val := int64(v)
				return &val
			case int32:
				val := int64(v)
				return &val
			}
		}
		return nil
	}

	getTime := func(fieldName string) *time.Time {
		if value, err := sd.GetField(fieldName); err == nil && value != nil {
			if t, ok := value.(time.Time); ok {
				return &t
			}
		}
		return nil
	}

	// 填充 core.StockData 字段
	// 使用辅助函数处理指针类型，区分字段不存在和零值的情况
	assignString := func(dst *string, field string) {
		if val := getString(field); val != nil {
			*dst = *val
		}
	}

	assignFloat64 := func(dst *float64, field string) {
		if val := getFloat64(field); val != nil {
			*dst = *val
		}
	}

	assignInt64 := func(dst *int64, field string) {
		if val := getInt64(field); val != nil {
			*dst = *val
		}
	}

	assignTime := func(dst *time.Time, field string) {
		if val := getTime(field); val != nil {
			*dst = *val
		}
	}

	// 基本信息字段
	assignString(&stockData.Symbol, "symbol")
	assignString(&stockData.Name, "name")

	// 价格相关字段
	assignFloat64(&stockData.Price, "price")
	assignFloat64(&stockData.Change, "change")
	assignFloat64(&stockData.ChangePercent, "change_percent")
	assignFloat64(&stockData.Open, "open")
	assignFloat64(&stockData.High, "high")
	assignFloat64(&stockData.Low, "low")
	assignFloat64(&stockData.PrevClose, "prev_close")

	// 交易数据
	assignInt64(&stockData.MarketCode, "market_code")
	assignInt64(&stockData.Volume, "volume")
	assignFloat64(&stockData.Turnover, "turnover")

	// 买卖盘数据 - 使用反射动态赋值
	assignBidAskData := func() {
		for i := 1; i <= 5; i++ {
			// 动态生成字段名
			bidPriceFieldName := fmt.Sprintf("BidPrice%d", i)
			bidVolumeFieldName := fmt.Sprintf("BidVolume%d", i)
			askPriceFieldName := fmt.Sprintf("AskPrice%d", i)
			askVolumeFieldName := fmt.Sprintf("AskVolume%d", i)

			// 动态生成数据源字段名
			bidPriceDataField := fmt.Sprintf("bid_price%d", i)
			bidVolumeDataField := fmt.Sprintf("bid_volume%d", i)
			askPriceDataField := fmt.Sprintf("ask_price%d", i)
			askVolumeDataField := fmt.Sprintf("ask_volume%d", i)

			// 使用反射设置买盘价格
			if bidPriceValue := getFloat64(bidPriceDataField); bidPriceValue != nil {
				if field := stockDataValue.FieldByName(bidPriceFieldName); field.IsValid() && field.CanSet() {
					field.SetFloat(*bidPriceValue)
				}
			}

			// 使用反射设置买盘数量
			if bidVolumeValue := getInt64(bidVolumeDataField); bidVolumeValue != nil {
				if field := stockDataValue.FieldByName(bidVolumeFieldName); field.IsValid() && field.CanSet() {
					field.SetInt(*bidVolumeValue)
				}
			}

			// 使用反射设置卖盘价格
			if askPriceValue := getFloat64(askPriceDataField); askPriceValue != nil {
				if field := stockDataValue.FieldByName(askPriceFieldName); field.IsValid() && field.CanSet() {
					field.SetFloat(*askPriceValue)
				}
			}

			// 使用反射设置卖盘数量
			if askVolumeValue := getInt64(askVolumeDataField); askVolumeValue != nil {
				if field := stockDataValue.FieldByName(askVolumeFieldName); field.IsValid() && field.CanSet() {
					field.SetInt(*askVolumeValue)
				}
			}
		}
	}

	// 执行买卖盘数据赋值
	assignBidAskData()

	// 其他数据
	assignInt64(&stockData.InnerDisc, "inner_disc")
	assignInt64(&stockData.OuterDisc, "outer_disc")
	assignFloat64(&stockData.TurnoverRate, "turnover_rate")
	assignFloat64(&stockData.PE, "pe")
	assignFloat64(&stockData.PB, "pb")
	assignFloat64(&stockData.Amplitude, "amplitude")
	assignFloat64(&stockData.Circulation, "circulation")
	assignFloat64(&stockData.MarketValue, "market_value")
	assignFloat64(&stockData.LimitUp, "limit_up")
	assignFloat64(&stockData.LimitDown, "limit_down")
	assignTime(&stockData.Timestamp, "timestamp")

	return stockData, nil
}

// SetFieldSafe 安全地设置结构化数据中的字段值
//
// 该方法会验证字段是否存在以及字段值是否符合schema定义，验证通过后才会设置字段值。
//
// 参数:
//
//	fieldName (string): 要设置的字段名称
//	value (interface{}): 要设置的值
//
// 返回值:
//
//	error: 如果字段不存在或值验证失败，返回相应的错误；否则返回nil
//
// 错误类型:
//   - ErrFieldNotFound: 当指定字段在schema中不存在时返回
//   - 其他验证错误: 参见: @ValidateFieldValue
//   - 所有字段设置完成后, 你应该使用: ValidateDataComplete 来验证整个数据的合法性
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

// ValidateDataComplete 验证结构化数据的完整性和正确性,比 ValidateData 更严格）
// 该方法会依次验证：
// 1. 数据的模式(Schema)是否有效
// 2. 所有字段值是否符合其定义
// 3. 是否存在未在Schema中定义的多余字段
//
// 返回值：
// - error: 如果验证通过返回nil，否则返回相应的错误信息
//
// 可能的错误类型：
// - 模式验证错误, 参见: @ValidateSchema
// - 字段值验证错误, 参见: @ValidateFieldValue
// - 未定义字段错误
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

// GetFieldSafe 从结构化数据中安全地获取指定字段的值
//
// 参数:
//   - fieldName: 要获取的字段名称
//   - targetType: 期望的字段类型
//
// 返回值:
//   - interface{}: 字段的值
//   - error: 错误信息，可能为nil
//
// 功能说明:
//  1. 首先检查字段是否在schema中定义，如果不存在则返回错误
//  2. 检查字段是否有值，如果没有值：
//     - 如果有默认值则返回默认值
//     - 如果是必填字段则返回错误
//     - 否则返回nil
//  3. 检查请求的类型是否与字段定义的类型匹配，不匹配则返回错误
//  4. 所有检查通过后返回字段值
//
// 错误处理:
//   - ErrFieldNotFound: 字段未在schema中定义
//   - ErrRequiredFieldMissing: 必填字段缺失
//   - ErrInvalidFieldType: 字段类型不匹配
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

// isValidFieldType 检查给定值是否符合预期的字段类型
//
// 参数:
//
//	value interface{} - 要检查的值，可以为nil
//	expectedType FieldType - 预期的字段类型
//
// 返回值:
//
//	bool - 如果值符合预期类型或为nil则返回true，否则返回false
//
// 说明:
//   - nil值总是被认为是有效的（用于可选字段）
//   - 支持的类型检查包括:
//   - 字符串 (FieldTypeString)
//   - 整数类型 (FieldTypeInt): int, int8, int16, int32, int64
//   - 浮点数类型 (FieldTypeFloat64): float32, float64
//   - 布尔值 (FieldTypeBool)
//   - 时间类型 (FieldTypeTime): time.Time
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

// ValidateSchema 验证数据结构的schema定义是否符合规范
//
// 参数:
//
//	schema (*DataSchema): 待验证的数据结构schema指针
//
// 返回值:
//
//	error: 如果验证通过返回nil，否则返回相应的错误信息，可能包含以下错误类型:
//	  - ErrSchemaNotFound: 当schema为nil、名称为空或没有字段时
//	  - ErrFieldNotFound: 当字段顺序中包含不存在的字段或字段未在顺序中指定时
//	  - 其他字段验证相关的错误
//
// 验证规则:
//  1. schema不能为nil
//  2. schema名称不能为空
//  3. schema必须至少包含一个字段
//  4. 如果指定了字段顺序(FieldOrder):
//     - 顺序中的所有字段都必须在schema.Fields中存在
//     - schema.Fields中的所有字段都必须在字段顺序中指定
//  5. 所有字段定义必须通过ValidateFieldDefinition验证
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

// ValidateFieldDefinition 验证字段定义的有效性
//
// 参数:
//
//	fieldName string - 字段名称
//	fieldDef *FieldDefinition - 字段定义指针
//
// 返回值:
//
//	error - 如果验证通过返回nil，否则返回相应的错误信息
//
// 验证规则:
//  1. 字段定义不能为nil
//  2. 字段名称不能为空
//  3. 字段名称必须与输入的fieldName一致
//  4. 字段类型必须在有效范围内
//  5. 如果存在默认值，默认值类型必须与字段类型匹配
//
// 注意事项:
//   - 必填字段(Required=true)可以没有默认值，这样可以强制用户提供值
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
		// TODO: 打印警告日志
		// 注意：必填字段可以没有默认值，这样可以强制用户提供值
	}

	return nil
}

// ValidateFieldValue 验证字段值是否符合字段定义的要求
//
// 参数:
//   - fieldName: string 字段名称
//   - value: interface{} 待验证的字段值
//   - fieldDef: *FieldDefinition 字段定义指针，包含验证规则
//
// 返回值:
//   - error 如果验证通过返回nil，否则返回相应的错误信息
//
// 验证流程:
//  1. 检查字段定义是否存在
//  2. 检查必填字段
//  3. 如果值为nil且非必填，则验证通过
//  4. 验证字段值类型
//  5. 验证数值范围（如果是数值类型）
//  6. 执行自定义验证（如果定义了验证器）
//
// 错误类型:
//   - ErrFieldNotFound: 字段定义未找到
//   - ErrRequiredFieldMissing: 必填字段缺失
//   - ErrInvalidFieldType: 字段类型不匹配
//   - ErrFieldValidationFailed: 自定义验证失败
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

// validateValueRange 验证字段值是否在有效范围内
//
// 参数:
//
//	fieldName: string - 要验证的字段名称
//	value: interface{} - 要验证的字段值
//	fieldDef: *FieldDefinition - 字段定义，包含字段类型等信息
//
// 返回值:
//
//	error - 如果验证通过返回nil，否则返回相应的错误信息
//
// 功能说明:
//
//   - 对于float64类型：检查是否为NaN或无穷大，并对price字段进行非负验证
//   - 对于int类型：对volume相关字段进行非负验证
//   - 对于string类型：对symbol字段进行长度验证(2-10字符)，对name字段进行最大长度验证(50字符)
func validateValueRange(fieldName string, value interface{}, fieldDef *FieldDefinition) error {
	//	TODO: 扩展用于不同字段类型的范围验证
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
