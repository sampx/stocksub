package tencent

import (
	"io"
	"strconv"
	"strings"
	"time"

	"stocksub/pkg/subscriber"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

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
func parseTencentData(data string) []subscriber.StockData {
	if data == "" {
		return []subscriber.StockData{}
	}

	data = strings.TrimSpace(data)
	stocks := strings.Split(data, ";")
	results := make([]subscriber.StockData, 0, len(stocks))

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

		if len(fields) < 50 {
			continue
		}

		stockData := subscriber.StockData{
			// 基本信息
			Symbol:        extractSymbol(fields[2]),
			Name:          gbkToUtf8(fields[1]),
			Price:         parseFloat(fields[3]),
			Change:        parseFloat(fields[31]),
			ChangePercent: parseFloat(fields[32]),
			MarketCode:    parseInt(fields[0]), // 市场分类代码

			// 交易数据
			Volume:    parseInt(fields[6]),       // 成交量(手)
			Turnover:  parseTurnover(fields[35]), // 从最新价/成交量(手)/成交额(元)格式中提取成交额
			Open:      parseFloat(fields[5]),
			High:      parseFloat(fields[33]),
			Low:       parseFloat(fields[34]),
			PrevClose: parseFloat(fields[4]),

			// 5档买卖盘数据
			BidPrice1:  parseFloat(fields[9]),
			BidVolume1: parseInt(fields[10]), // 买一量(手)
			BidPrice2:  parseFloat(fields[11]),
			BidVolume2: parseInt(fields[12]), // 买二量(手)
			BidPrice3:  parseFloat(fields[13]),
			BidVolume3: parseInt(fields[14]), // 买三量(手)
			BidPrice4:  parseFloat(fields[15]),
			BidVolume4: parseInt(fields[16]), // 买四量(手)
			BidPrice5:  parseFloat(fields[17]),
			BidVolume5: parseInt(fields[18]), // 买五量(手)
			AskPrice1:  parseFloat(fields[19]),
			AskVolume1: parseInt(fields[20]), // 卖一量(手)
			AskPrice2:  parseFloat(fields[21]),
			AskVolume2: parseInt(fields[22]), // 卖二量(手)
			AskPrice3:  parseFloat(fields[23]),
			AskVolume3: parseInt(fields[24]), // 卖三量(手)
			AskPrice4:  parseFloat(fields[25]),
			AskVolume4: parseInt(fields[26]), // 卖四量(手)
			AskPrice5:  parseFloat(fields[27]),
			AskVolume5: parseInt(fields[28]), // 卖五量(手)

			// 内外盘数据
			OuterDisc: parseInt(fields[7]), // 外盘(手)
			InnerDisc: parseInt(fields[8]), // 内盘(手)

			// 财务指标
			TurnoverRate: parseFloat(fields[38]), // 换手率
			PE:           parseFloat(fields[39]), // 市盈率
			PB:           parseFloat(fields[46]), // 市净率
			Amplitude:    parseFloat(fields[43]), // 振幅
			Circulation:  parseFloat(fields[44]), // 流通市值(亿)
			MarketValue:  parseFloat(fields[45]), // 总市值(亿)
			LimitUp:      parseFloat(fields[47]), // 涨停价
			LimitDown:    parseFloat(fields[48]), // 跌停价

			// 时间信息
			Timestamp: parseTime(fields[30]),
		}

		// 单位转换（A股数据从手转换为股）
		// fields[0]: 1-科创板+上海主板, 51-创业板+深圳主板, 62-北交所
		// 所有A股市场：成交量和买卖盘口数据都需要从手转换为股
		stockData.Volume *= 100
		stockData.BidVolume1 *= 100
		stockData.BidVolume2 *= 100
		stockData.BidVolume3 *= 100
		stockData.BidVolume4 *= 100
		stockData.BidVolume5 *= 100
		stockData.AskVolume1 *= 100
		stockData.AskVolume2 *= 100
		stockData.AskVolume3 *= 100
		stockData.AskVolume4 *= 100
		stockData.AskVolume5 *= 100
		stockData.InnerDisc *= 100
		stockData.OuterDisc *= 100

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

// parseFloat 安全解析浮点数
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

// parseInt 安全解析整数
func parseInt(s string) int64 {
	if s == "" {
		return 0
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// parseTime 解析时间戳
func parseTime(timeStr string) time.Time {
	if len(timeStr) < 12 {
		return time.Now()
	}

	var layout string
	if len(timeStr) == 14 {
		layout = "20060102150405"
	} else if len(timeStr) == 12 {
		layout = "200601021504"
	} else {
		return time.Now()
	}

	t, err := time.ParseInLocation(layout, timeStr, time.Local)
	if err != nil {
		return time.Now()
	}

	return t
}

// parseTurnover 从复合字段中提取成交额
func parseTurnover(s string) float64 {
	if s == "" {
		return 0
	}

	parts := strings.Split(s, "/")
	if len(parts) >= 3 {
		val, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0
		}
		return val
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}
