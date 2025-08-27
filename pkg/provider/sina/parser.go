package sina

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

// parseSinaData 解析新浪返回的数据
func parseSinaData(data string) []subscriber.StockData {
	lines := strings.Split(data, ";")
	results := make([]subscriber.StockData, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "=") || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}

		varPart := parts[0]
		symbol := extractSymbol(varPart)

		dataPart := strings.Trim(parts[1], ` "`)
		fields := strings.Split(dataPart, ",")

		if len(fields) < 32 {
			continue
		}

		price := parseFloat(fields[3])
		prevClose := parseFloat(fields[2])
		change := price - prevClose
		var changePercent float64
		if prevClose != 0 {
			changePercent = (change / prevClose) * 100
		}

		stockData := subscriber.StockData{
			Symbol:        symbol,
			Name:          gbkToUtf8(fields[0]),
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
			Open:          parseFloat(fields[1]),
			High:          parseFloat(fields[4]),
			Low:           parseFloat(fields[5]),
			PrevClose:     prevClose,
			Volume:        parseInt(fields[8]) / 100, // 单位是股，转换为手
			Turnover:      parseFloat(fields[9]),
			Timestamp:     parseTime(fields[30], fields[31]),
		}

		results = append(results, stockData)
	}

	return results
}

// extractSymbol 从变量名中提取股票代码, e.g., hq_str_sh600000 -> 600000
func extractSymbol(rawVar string) string {
	parts := strings.Split(rawVar, "_")
	if len(parts) < 3 {
		return ""
	}
	code := parts[len(parts)-1]
	return strings.TrimPrefix(strings.TrimPrefix(code, "sh"), "sz")
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

// parseTime 解析日期和时间
func parseTime(dateStr, timeStr string) time.Time {
	layout := "2006-01-02 15:04:05"
	ts, err := time.ParseInLocation(layout, dateStr+" "+timeStr, time.Local)
	if err != nil {
		return time.Now()
	}
	return ts
}
