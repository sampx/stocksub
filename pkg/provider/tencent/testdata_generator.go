package tencent

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"stocksub/pkg/core"
)

// TencentDataGenerator 腾讯API数据生成器
type TencentDataGenerator struct {
	config TencentDataConfig
}

// TencentDataConfig 腾讯数据生成配置
type TencentDataConfig struct {
	BasePrice     float64 // 基础价格
	PriceRange    float64 // 价格波动范围
	VolumeRange   int64   // 成交量范围
	EnableRandom  bool    // 是否启用随机数据
	TimestampMode string  // 时间戳模式: "current", "fixed", "random"
	FixedTime     time.Time
}

// DefaultTencentDataConfig 默认配置
func DefaultTencentDataConfig() TencentDataConfig {
	return TencentDataConfig{
		BasePrice:     100.0,
		PriceRange:    20.0,
		VolumeRange:   1000000,
		EnableRandom:  true,
		TimestampMode: "current",
	}
}

// NewTencentDataGenerator 创建腾讯数据生成器
func NewTencentDataGenerator(config TencentDataConfig) *TencentDataGenerator {
	return &TencentDataGenerator{
		config: config,
	}
}

// GenerateTencentResponse 生成符合腾讯API格式的响应字符串
func (g *TencentDataGenerator) GenerateTencentResponse(symbols []string) string {
	var responses []string

	for _, symbol := range symbols {
		response := g.generateSingleStockResponse(symbol)
		responses = append(responses, response)
	}

	return strings.Join(responses, ";") + ";"
}

// generateSingleStockResponse 生成单个股票的响应数据
func (g *TencentDataGenerator) generateSingleStockResponse(symbol string) string {
	marketCode := g.getMarketCode(symbol)
	stockName := g.generateStockName(symbol)

	// 生成基础价格数据
	price := g.generatePrice()
	prevClose := price - g.generateChange()
	open := prevClose + g.generateSmallChange()
	high := price + g.generateSmallChange()
	low := price - g.generateSmallChange()

	// 生成成交数据
	volume := g.generateVolume()
	outerDisc := volume / 2
	innerDisc := volume - outerDisc

	// 生成买卖盘数据
	bidPrices, bidVolumes := g.generateBidData(price)
	askPrices, askVolumes := g.generateAskData(price)

	// 生成时间戳
	timestamp := g.generateTimestamp()

	// 计算涨跌数据
	change := price - prevClose
	changePercent := (change / prevClose) * 100

	// 生成财务数据
	turnover := float64(volume) * price
	turnoverRate := g.generateTurnoverRate()
	pe := g.generatePE()
	pb := g.generatePB()
	amplitude := ((high - low) / prevClose) * 100
	circulation := g.generateCirculation()
	marketValue := circulation * 1.2 // 总市值通常比流通市值大
	limitUp := prevClose * 1.1
	limitDown := prevClose * 0.9

	// 构建响应字符串（49个字段）
	fields := []string{
		fmt.Sprintf("%d", marketCode),                        // 0: 市场分类
		stockName,                                            // 1: 名字
		g.addMarketPrefix(symbol),                            // 2: 代码
		fmt.Sprintf("%.2f", price),                           // 3: 当前价格
		fmt.Sprintf("%.2f", prevClose),                       // 4: 昨收
		fmt.Sprintf("%.2f", open),                            // 5: 今开
		fmt.Sprintf("%d", volume),                            // 6: 成交量(手)
		fmt.Sprintf("%d", outerDisc),                         // 7: 外盘
		fmt.Sprintf("%d", innerDisc),                         // 8: 内盘
		fmt.Sprintf("%.2f", bidPrices[0]),                    // 9: 买一价
		fmt.Sprintf("%d", bidVolumes[0]),                     // 10: 买一量
		fmt.Sprintf("%.2f", bidPrices[1]),                    // 11: 买二价
		fmt.Sprintf("%d", bidVolumes[1]),                     // 12: 买二量
		fmt.Sprintf("%.2f", bidPrices[2]),                    // 13: 买三价
		fmt.Sprintf("%d", bidVolumes[2]),                     // 14: 买三量
		fmt.Sprintf("%.2f", bidPrices[3]),                    // 15: 买四价
		fmt.Sprintf("%d", bidVolumes[3]),                     // 16: 买四量
		fmt.Sprintf("%.2f", bidPrices[4]),                    // 17: 买五价
		fmt.Sprintf("%d", bidVolumes[4]),                     // 18: 买五量
		fmt.Sprintf("%.2f", askPrices[0]),                    // 19: 卖一价
		fmt.Sprintf("%d", askVolumes[0]),                     // 20: 卖一量
		fmt.Sprintf("%.2f", askPrices[1]),                    // 21: 卖二价
		fmt.Sprintf("%d", askVolumes[1]),                     // 22: 卖二量
		fmt.Sprintf("%.2f", askPrices[2]),                    // 23: 卖三价
		fmt.Sprintf("%d", askVolumes[2]),                     // 24: 卖三量
		fmt.Sprintf("%.2f", askPrices[3]),                    // 25: 卖四价
		fmt.Sprintf("%d", askVolumes[3]),                     // 26: 卖四量
		fmt.Sprintf("%.2f", askPrices[4]),                    // 27: 卖五价
		fmt.Sprintf("%d", askVolumes[4]),                     // 28: 卖五量
		"",                                                   // 29: 未知字段
		timestamp,                                            // 30: 时间戳
		fmt.Sprintf("%.2f", change),                          // 31: 涨跌额
		fmt.Sprintf("%.2f", changePercent),                   // 32: 涨跌幅
		fmt.Sprintf("%.2f", high),                            // 33: 最高价
		fmt.Sprintf("%.2f", low),                             // 34: 最低价
		fmt.Sprintf("%.2f/%d/%.0f", price, volume, turnover), // 35: 价格/成交量/成交额
		fmt.Sprintf("%d", volume),                            // 36: 成交量(重复)
		fmt.Sprintf("%.0f", turnover/10000),                  // 37: 成交额(万)
		fmt.Sprintf("%.2f", turnoverRate),                    // 38: 换手率
		fmt.Sprintf("%.2f", pe),                              // 39: 市盈率
		"",                                                   // 40: 空字段
		fmt.Sprintf("%.2f", high),                            // 41: 最高价(重复)
		fmt.Sprintf("%.2f", low),                             // 42: 最低价(重复)
		fmt.Sprintf("%.2f", amplitude),                       // 43: 振幅
		fmt.Sprintf("%.2f", circulation),                     // 44: 流通市值
		fmt.Sprintf("%.2f", marketValue),                     // 45: 总市值
		fmt.Sprintf("%.2f", pb),                              // 46: 市净率
		fmt.Sprintf("%.2f", limitUp),                         // 47: 涨停价
		fmt.Sprintf("%.2f", limitDown),                       // 48: 跌停价
	}

	return fmt.Sprintf("v_%s%s=\"%s\"", g.getMarketPrefix(symbol), symbol, strings.Join(fields, "~"))
}

// getMarketCode 获取市场代码
func (g *TencentDataGenerator) getMarketCode(symbol string) int {
	switch {
	case strings.HasPrefix(symbol, "6") || strings.HasPrefix(symbol, "688"):
		return 1 // 上海主板+科创板
	case strings.HasPrefix(symbol, "0") || strings.HasPrefix(symbol, "300"):
		return 51 // 深圳主板+创业板
	case strings.HasPrefix(symbol, "4") || strings.HasPrefix(symbol, "8"):
		return 62 // 北交所
	default:
		return 1
	}
}

// getMarketPrefix 获取市场前缀
func (g *TencentDataGenerator) getMarketPrefix(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "6") || strings.HasPrefix(symbol, "688"):
		return "sh"
	case strings.HasPrefix(symbol, "0") || strings.HasPrefix(symbol, "300"):
		return "sz"
	case strings.HasPrefix(symbol, "4") || strings.HasPrefix(symbol, "8"):
		return "bj"
	default:
		return "sh"
	}
}

// addMarketPrefix 添加市场前缀到股票代码
func (g *TencentDataGenerator) addMarketPrefix(symbol string) string {
	return g.getMarketPrefix(symbol) + symbol
}

// generateStockName 生成股票名称
func (g *TencentDataGenerator) generateStockName(symbol string) string {
	names := map[string]string{
		"600000": "浦发银行",
		"000001": "平安银行",
		"000858": "五粮液",
		"300503": "昊志机电",
		"688041": "海光信息",
		"835174": "五新隧装",
	}

	if name, exists := names[symbol]; exists {
		return name
	}

	// 生成默认名称
	return fmt.Sprintf("测试股票%s", symbol)
}

// generatePrice 生成价格
func (g *TencentDataGenerator) generatePrice() float64 {
	if !g.config.EnableRandom {
		return g.config.BasePrice
	}

	variation := (rand.Float64() - 0.5) * g.config.PriceRange
	return g.config.BasePrice + variation
}

// generateChange 生成价格变化
func (g *TencentDataGenerator) generateChange() float64 {
	if !g.config.EnableRandom {
		return 1.0
	}

	return (rand.Float64() - 0.5) * 10.0
}

// generateSmallChange 生成小幅价格变化
func (g *TencentDataGenerator) generateSmallChange() float64 {
	if !g.config.EnableRandom {
		return 0.5
	}

	return (rand.Float64() - 0.5) * 2.0
}

// generateVolume 生成成交量
func (g *TencentDataGenerator) generateVolume() int64 {
	if !g.config.EnableRandom {
		return g.config.VolumeRange / 2
	}

	return rand.Int63n(g.config.VolumeRange) + 1000
}

// generateBidData 生成买盘数据
func (g *TencentDataGenerator) generateBidData(price float64) ([]float64, []int64) {
	prices := make([]float64, 5)
	volumes := make([]int64, 5)

	for i := 0; i < 5; i++ {
		prices[i] = price - float64(i+1)*0.01
		if g.config.EnableRandom {
			volumes[i] = rand.Int63n(10000) + 100
		} else {
			volumes[i] = int64(1000 - i*100)
		}
	}

	return prices, volumes
}

// generateAskData 生成卖盘数据
func (g *TencentDataGenerator) generateAskData(price float64) ([]float64, []int64) {
	prices := make([]float64, 5)
	volumes := make([]int64, 5)

	for i := 0; i < 5; i++ {
		prices[i] = price + float64(i+1)*0.01
		if g.config.EnableRandom {
			volumes[i] = rand.Int63n(10000) + 100
		} else {
			volumes[i] = int64(1000 - i*100)
		}
	}

	return prices, volumes
}

// generateTimestamp 生成时间戳
func (g *TencentDataGenerator) generateTimestamp() string {
	switch g.config.TimestampMode {
	case "fixed":
		if !g.config.FixedTime.IsZero() {
			return g.config.FixedTime.Format("20060102150405")
		}
		return "20250101120000"
	case "random":
		randomTime := time.Now().Add(-time.Duration(rand.Intn(86400)) * time.Second)
		return randomTime.Format("20060102150405")
	default: // "current"
		return time.Now().Format("20060102150405")
	}
}

// generateTurnoverRate 生成换手率
func (g *TencentDataGenerator) generateTurnoverRate() float64 {
	if !g.config.EnableRandom {
		return 2.5
	}
	return rand.Float64() * 10.0
}

// generatePE 生成市盈率
func (g *TencentDataGenerator) generatePE() float64 {
	if !g.config.EnableRandom {
		return 15.0
	}
	return rand.Float64()*50.0 + 5.0
}

// generatePB 生成市净率
func (g *TencentDataGenerator) generatePB() float64 {
	if !g.config.EnableRandom {
		return 2.0
	}
	return rand.Float64()*5.0 + 0.5
}

// generateCirculation 生成流通市值
func (g *TencentDataGenerator) generateCirculation() float64 {
	if !g.config.EnableRandom {
		return 1000.0
	}
	return rand.Float64()*10000.0 + 100.0
}

// GenerateStockData 生成 StockData 结构
func (g *TencentDataGenerator) GenerateStockData(symbols []string) []core.StockData {
	response := g.GenerateTencentResponse(symbols)
	return parseTencentData(response)
}
