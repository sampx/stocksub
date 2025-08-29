package tencent

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"stocksub/pkg/core"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// 定义腾讯数据字段索引常量
const (
	FieldMarketCode          = iota // 0 - 市场分类代码
	FieldName                       // 1 - 股票名称
	FieldSymbol                     // 2 - 股票代码
	FieldPrice                      // 3 - 当前价格
	FieldPrevClose                  // 4 - 昨收价
	FieldOpen                       // 5 - 今开价
	FieldVolume                     // 6 - 成交量(手)
	FieldOuterDisc                  // 7 - 外盘(手)
	FieldInnerDisc                  // 8 - 内盘(手)
	FieldBidPrice1                  // 9 - 买一价
	FieldBidVolume1                 // 10 - 买一量(手)
	FieldBidPrice2                  // 11 - 买二价
	FieldBidVolume2                 // 12 - 买二量(手)
	FieldBidPrice3                  // 13 - 买三价
	FieldBidVolume3                 // 14 - 买三量(手)
	FieldBidPrice4                  // 15 - 买四价
	FieldBidVolume4                 // 16 - 买四量(手)
	FieldBidPrice5                  // 17 - 买五价
	FieldBidVolume5                 // 18 - 买五量(手)
	FieldAskPrice1                  // 19 - 卖一价
	FieldAskVolume1                 // 20 - 卖一量(手)
	FieldAskPrice2                  // 21 - 卖二价
	FieldAskVolume2                 // 22 - 卖二量(手)
	FieldAskPrice3                  // 23 - 卖三价
	FieldAskVolume3                 // 24 - 卖三量(手)
	FieldAskPrice4                  // 25 - 卖四价
	FieldAskVolume4                 // 26 - 卖四量(手)
	FieldAskPrice5                  // 27 - 卖五价
	FieldAskVolume5                 // 28 - 卖五量(手)
	FieldUnknown29                  // 29 - 未知字段
	FieldTimestamp                  // 30 - 时间戳
	FieldChange                     // 31 - 涨跌额
	FieldChangePercent              // 32 - 涨跌幅
	FieldHigh                       // 33 - 最高价
	FieldLow                        // 34 - 最低价
	FieldPriceVolumeTurnover        // 35 - 价格/成交量/成交额组合字段
	FieldVolumeRepeat               // 36 - 成交量(重复)
	FieldTurnoverRepeat             // 37 - 成交额(重复)
	FieldTurnoverRate               // 38 - 换手率
	FieldPE                         // 39 - 市盈率
	FieldUnknown40                  // 40 - 空字段
	FieldHighRepeat                 // 41 - 最高价(重复)
	FieldLowRepeat                  // 42 - 最低价(重复)
	FieldAmplitude                  // 43 - 振幅
	FieldCirculation                // 44 - 流通市值
	FieldMarketValue                // 45 - 总市值
	FieldPB                         // 46 - 市净率
	FieldLimitUp                    // 47 - 涨停价
	FieldLimitDown                  // 48 - 跌停价
)

// 最少需要的字段数量
const MinRequiredFields = 49

// gbkToUtf8 将GBK编码转换为UTF-8
func gbkToUtf8(gbkStr string) string {
	if gbkStr == "" {
		return ""
	}

	reader := transform.NewReader(strings.NewReader(gbkStr), simplifiedchinese.GBK.NewDecoder())
	data, err := io.ReadAll(reader)
	if err != nil {
		return gbkStr
	}

	return string(data)
}

// parseTencentData 解析腾讯返回的数据
func parseTencentData(data string) []core.StockData {
	if data == "" {
		return []core.StockData{}
	}

	data = strings.TrimSpace(data)
	stocks := strings.Split(data, ";")
	results := make([]core.StockData, 0, len(stocks))

	for _, stock := range stocks {
		stock = strings.TrimSpace(stock)
		if stock == "" {
			continue
		}

		equalIndex := strings.Index(stock, "=")
		if equalIndex == -1 || equalIndex+1 >= len(stock) {
			continue
		}

		dataPart := stock[equalIndex+1:]
		dataPart = strings.Trim(dataPart, "\"")
		fields := strings.Split(dataPart, "~")

		if len(fields) < MinRequiredFields {
			continue
		}

		stockData := core.StockData{
			// 基本信息
			Symbol:        extractSymbol(fields[FieldSymbol]),
			Name:          gbkToUtf8(fields[FieldName]),
			Price:         parseFloatWithDefault(fields[FieldPrice]),
			Change:        parseFloatWithDefault(fields[FieldChange]),
			ChangePercent: parseFloatWithDefault(fields[FieldChangePercent]),
			MarketCode:    parseIntWithDefault(fields[FieldMarketCode]), // 市场分类代码

			// 交易数据
			Volume:    parseIntWithDefault(fields[FieldVolume]),        // 成交量(手)
			Turnover:  parseTurnover(fields[FieldPriceVolumeTurnover]), // 从最新价/成交量(手)/成交额(元)格式中提取成交额
			Open:      parseFloatWithDefault(fields[FieldOpen]),
			High:      parseFloatWithDefault(fields[FieldHigh]),
			Low:       parseFloatWithDefault(fields[FieldLow]),
			PrevClose: parseFloatWithDefault(fields[FieldPrevClose]),

			// 5档买卖盘数据
			BidPrice1:  parseFloatWithDefault(fields[FieldBidPrice1]),
			BidVolume1: parseIntWithDefault(fields[FieldBidVolume1]), // 买一量(手)
			BidPrice2:  parseFloatWithDefault(fields[FieldBidPrice2]),
			BidVolume2: parseIntWithDefault(fields[FieldBidVolume2]), // 买二量(手)
			BidPrice3:  parseFloatWithDefault(fields[FieldBidPrice3]),
			BidVolume3: parseIntWithDefault(fields[FieldBidVolume3]), // 买三量(手)
			BidPrice4:  parseFloatWithDefault(fields[FieldBidPrice4]),
			BidVolume4: parseIntWithDefault(fields[FieldBidVolume4]), // 买四量(手)
			BidPrice5:  parseFloatWithDefault(fields[FieldBidPrice5]),
			BidVolume5: parseIntWithDefault(fields[FieldBidVolume5]), // 买五量(手)
			AskPrice1:  parseFloatWithDefault(fields[FieldAskPrice1]),
			AskVolume1: parseIntWithDefault(fields[FieldAskVolume1]), // 卖一量(手)
			AskPrice2:  parseFloatWithDefault(fields[FieldAskPrice2]),
			AskVolume2: parseIntWithDefault(fields[FieldAskVolume2]), // 卖二量(手)
			AskPrice3:  parseFloatWithDefault(fields[FieldAskPrice3]),
			AskVolume3: parseIntWithDefault(fields[FieldAskVolume3]), // 卖三量(手)
			AskPrice4:  parseFloatWithDefault(fields[FieldAskPrice4]),
			AskVolume4: parseIntWithDefault(fields[FieldAskVolume4]), // 卖四量(手)
			AskPrice5:  parseFloatWithDefault(fields[FieldAskPrice5]),
			AskVolume5: parseIntWithDefault(fields[FieldAskVolume5]), // 卖五量(手)

			// 内外盘数据
			OuterDisc: parseIntWithDefault(fields[FieldOuterDisc]), // 外盘(手)
			InnerDisc: parseIntWithDefault(fields[FieldInnerDisc]), // 内盘(手)

			// 财务指标
			TurnoverRate: parseFloatWithDefault(fields[FieldTurnoverRate]), // 换手率
			PE:           parseFloatWithDefault(fields[FieldPE]),           // 市盈率
			PB:           parseFloatWithDefault(fields[FieldPB]),           // 市净率
			Amplitude:    parseFloatWithDefault(fields[FieldAmplitude]),    // 振幅
			Circulation:  parseFloatWithDefault(fields[FieldCirculation]),  // 流通市值(亿)
			MarketValue:  parseFloatWithDefault(fields[FieldMarketValue]),  // 总市值(亿)
			LimitUp:      parseFloatWithDefault(fields[FieldLimitUp]),      // 涨停价
			LimitDown:    parseFloatWithDefault(fields[FieldLimitDown]),    // 跌停价

			// 时间信息
			Timestamp: parseTime(fields[FieldTimestamp]),
		}

		// 单位转换（A股数据从手转换为股）
		// fields[0]: 1-科创板+上海主板, 51-创业板+深圳主板, 62-北交所
		// 所有A股市场：成交量和买卖盘口数据都需要从手转换为股
		// stockData.Volume *= 100
		// stockData.BidVolume1 *= 100
		// stockData.BidVolume2 *= 100
		// stockData.BidVolume3 *= 100
		// stockData.BidVolume4 *= 100
		// stockData.BidVolume5 *= 100
		// stockData.AskVolume1 *= 100
		// stockData.AskVolume2 *= 100
		// stockData.AskVolume3 *= 100
		// stockData.AskVolume4 *= 100
		// stockData.AskVolume5 *= 100
		// stockData.InnerDisc *= 100
		// stockData.OuterDisc *= 100

		results = append(results, stockData)
	}

	return results
}

// extractSymbol 从股票代码中提取纯符号
func extractSymbol(rawSymbol string) string {
	rawSymbol = strings.TrimPrefix(rawSymbol, "sh")
	rawSymbol = strings.TrimPrefix(rawSymbol, "sz")
	rawSymbol = strings.TrimPrefix(rawSymbol, "bj")

	if dotIndex := strings.Index(rawSymbol, "."); dotIndex != -1 {
		rawSymbol = rawSymbol[:dotIndex]
	}

	return rawSymbol
}

// parseFloat 安全解析浮点数，返回错误信息
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("解析浮点数失败: %s, 错误: %w", s, err)
	}
	return val, nil
}

// parseFloatWithDefault 安全解析浮点数，解析失败时返回默认值
func parseFloatWithDefault(s string) float64 {
	val, _ := parseFloat(s)
	return val
}

// parseInt 安全解析整数，返回错误信息
func parseInt(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析整数失败: %s, 错误: %w", s, err)
	}
	return val, nil
}

// parseIntWithDefault 安全解析整数，解析失败时返回默认值
func parseIntWithDefault(s string) int64 {
	val, _ := parseInt(s)
	return val
}

// parseTime 解析时间戳，支持更多格式和错误处理
func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Now()
	}

	// 支持的时间格式
	layouts := []string{
		"20060102150405", // 14位：YYYYMMDDHHMMSS
		"200601021504",   // 12位：YYYYMMDDHHMM
		"20060102",       // 8位：YYYYMMDD
	}

	for _, layout := range layouts {
		if len(timeStr) == len(layout) {
			t, err := time.ParseInLocation(layout, timeStr, time.Local)
			if err == nil {
				return t
			}
		}
	}

	// 如果所有格式都解析失败，返回当前时间
	return time.Now()
}

// parseTurnover 从复合字段中提取成交额，支持更好的错误处理
func parseTurnover(s string) float64 {
	if s == "" {
		return 0
	}

	// 腾讯API格式：价格/成交量(手)/成交额(元)
	// 例如：125.80/399865/5027135558
	parts := strings.Split(s, "/")
	if len(parts) >= 3 {
		// 取第三部分作为成交额
		turnoverStr := strings.TrimSpace(parts[2])
		if val, err := strconv.ParseFloat(turnoverStr, 64); err == nil {
			return val
		}
	}

	// 如果解析复合字段失败，尝试直接解析整个字符串
	if val, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return val
	}

	return 0
}
