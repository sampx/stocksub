package providers

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"stocksub/pkg/core"
)

// TencentDataGenerator 腾讯API专用数据生成器
type TencentDataGenerator struct {
	config TencentDataConfig
	rand   *rand.Rand
}

// TencentDataConfig 腾讯数据生成配置
type TencentDataConfig struct {
	PriceRange      PriceRange  `yaml:"price_range"`
	VolumeRange     VolumeRange `yaml:"volume_range"`
	ChangeRange     ChangeRange `yaml:"change_range"`
	RandomSeed      int64       `yaml:"random_seed"`
	EnableRealistic bool        `yaml:"enable_realistic"`
	MarketHours     bool        `yaml:"market_hours"`
}

// NewTencentDataGenerator 创建腾讯数据生成器
func NewTencentDataGenerator(config TencentDataConfig) *TencentDataGenerator {
	seed := config.RandomSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &TencentDataGenerator{
		config: config,
		rand:   rand.New(rand.NewSource(seed)),
	}
}

// GenerateTencentResponse 生成腾讯API格式的响应
func (g *TencentDataGenerator) GenerateTencentResponse(symbols []string) string {
	var parts []string
	for _, symbol := range symbols {
		response := g.generateSingleStockResponse(symbol)
		parts = append(parts, response)
	}
	return strings.Join(parts, "")
}

// generateSingleStockResponse 生成单个股票的腾讯API响应
func (g *TencentDataGenerator) generateSingleStockResponse(symbol string) string {
	marketCode := g.getMarketCode(symbol)
	marketPrefix := g.getMarketPrefix(symbol)

	// 生成基础数据
	name := g.generateStockName(symbol)
	price := g.generatePrice()
	prevClose := price + g.generateSmallChange()
	open := price + g.generateSmallChange()
	change := price - prevClose
	changePercent := (change / prevClose) * 100

	volume := g.generateVolume()
	high := price + g.generateSmallChange()
	low := price - g.generateSmallChange()

	// 生成五档买卖数据
	bidData := g.generateBidData(price)
	askData := g.generateAskData(price)

	// 生成其他字段
	timestamp := g.generateTimestamp()
	turnoverRate := g.generateTurnoverRate()
	pe := g.generatePE()
	pb := g.generatePB()
	circulation := g.generateCirculation()

	// 构建腾讯API格式的响应
	response := fmt.Sprintf(
		"v_%s%s=\"%s~%s~%s~%.2f~%.2f~%.2f~%d~%d~%d~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%.2f~%.0f~%s~%.2f~%.2f~%.2f~%.2f~%.2f~%d~%.2f~%.2f~%d~%.2f~%d~%d~%d~%.2f~%d\";\n",
		marketPrefix, symbol, // v_sh600000=
		marketCode,             // 1
		name,                   // 浦发银行
		symbol,                 // 600000
		price,                  // 当前价格
		prevClose,              // 昨收
		open,                   // 今开
		volume,                 // 成交量
		0,                      // 外盘
		0,                      // 内盘
		bidData[0], bidData[1], // 买一价格和数量
		bidData[2], bidData[3], // 买二价格和数量
		bidData[4], bidData[5], // 买三价格和数量
		bidData[6], bidData[7], // 买四价格和数量
		bidData[8], bidData[9], // 买五价格和数量
		askData[0], askData[1], // 卖一价格和数量
		askData[2], askData[3], // 卖二价格和数量
		askData[4], askData[5], // 卖三价格和数量
		askData[6], askData[7], // 卖四价格和数量
		askData[8], askData[9], // 卖五价格和数量
		timestamp,          // 时间戳
		change,             // 涨跌额
		changePercent,      // 涨跌幅
		high,               // 最高
		low,                // 最低
		price,              // 最新价
		int64(circulation), // 流通股本
		turnoverRate,       // 换手率
		pe,                 // 市盈率
		0,                  // 未知字段
		pb,                 // 市净率
		int64(60000000),    // 总股本
		0,                  // 未知字段
		0,                  // 未知字段
		0,                  // 未知字段
		12.45,              // 净资产
		0,                  // 未知字段
	)

	return response
}

// getMarketCode 获取市场代码
func (g *TencentDataGenerator) getMarketCode(symbol string) string {
	if len(symbol) != 6 {
		return "1" // 默认上海
	}

	switch symbol[0] {
	case '6':
		return "1" // 上海主板
	case '0', '3':
		return "0" // 深圳主板/创业板
	case '8':
		return "2" // 北交所
	default:
		return "1" // 默认上海
	}
}

// getMarketPrefix 获取市场前缀
func (g *TencentDataGenerator) getMarketPrefix(symbol string) string {
	if len(symbol) != 6 {
		return "sh" // 默认上海
	}

	switch symbol[0] {
	case '6':
		return "sh" // 上海
	case '0', '3':
		return "sz" // 深圳
	case '8':
		return "bj" // 北京
	default:
		return "sh" // 默认上海
	}
}

// addMarketPrefix 添加市场前缀
func (g *TencentDataGenerator) addMarketPrefix(symbol string) string {
	prefix := g.getMarketPrefix(symbol)
	return prefix + symbol
}

// generateStockName 生成股票名称
func (g *TencentDataGenerator) generateStockName(symbol string) string {
	names := map[string]string{
		"600000": "浦发银行",
		"000001": "平安银行",
		"300750": "宁德时代",
		"688041": "海光信息",
		"835174": "茂莱光学",
	}

	if name, ok := names[symbol]; ok {
		return name
	}

	// 生成通用名称
	prefixes := []string{"测试", "模拟", "样本", "示例", "虚拟"}
	suffixes := []string{"科技", "银行", "制造", "服务", "投资"}

	prefix := prefixes[g.rand.Intn(len(prefixes))]
	suffix := suffixes[g.rand.Intn(len(suffixes))]

	return prefix + suffix
}

// generatePrice 生成价格
func (g *TencentDataGenerator) generatePrice() float64 {
	min := g.config.PriceRange.Min
	max := g.config.PriceRange.Max
	if min >= max {
		min, max = 1.0, 100.0
	}
	return min + g.rand.Float64()*(max-min)
}

// generateChange 生成涨跌额
func (g *TencentDataGenerator) generateChange() float64 {
	min := g.config.ChangeRange.Min
	max := g.config.ChangeRange.Max
	if min >= max {
		min, max = -10.0, 10.0
	}
	return min + g.rand.Float64()*(max-min)
}

// generateSmallChange 生成小幅变动
func (g *TencentDataGenerator) generateSmallChange() float64 {
	return (g.rand.Float64() - 0.5) * 2.0 // -1.0 到 1.0
}

// generateVolume 生成成交量
func (g *TencentDataGenerator) generateVolume() int64 {
	min := g.config.VolumeRange.Min
	max := g.config.VolumeRange.Max
	if min >= max {
		min, max = 1000, 1000000
	}
	return min + int64(g.rand.Intn(int(max-min)))
}

// generateBidData 生成买盘数据（五档）
func (g *TencentDataGenerator) generateBidData(basePrice float64) []float64 {
	data := make([]float64, 10) // 5档价格+5档数量
	for i := 0; i < 5; i++ {
		// 买盘价格递减
		data[i*2] = basePrice - float64(i+1)*0.01
		// 买盘数量
		data[i*2+1] = float64(g.rand.Intn(10000) + 1000)
	}
	return data
}

// generateAskData 生成卖盘数据（五档）
func (g *TencentDataGenerator) generateAskData(basePrice float64) []float64 {
	data := make([]float64, 10) // 5档价格+5档数量
	for i := 0; i < 5; i++ {
		// 卖盘价格递增
		data[i*2] = basePrice + float64(i+1)*0.01
		// 卖盘数量
		data[i*2+1] = float64(g.rand.Intn(10000) + 1000)
	}
	return data
}

// generateTimestamp 生成时间戳
func (g *TencentDataGenerator) generateTimestamp() string {
	now := time.Now()
	if g.config.MarketHours {
		// 市场时间：9:30-11:30, 13:00-15:00
		hour := 9 + g.rand.Intn(6)
		minute := g.rand.Intn(60)
		second := g.rand.Intn(60)
		now = time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, now.Location())
	}
	return now.Format("20060102150405")
}

// generateTurnoverRate 生成换手率
func (g *TencentDataGenerator) generateTurnoverRate() float64 {
	return g.rand.Float64() * 10.0 // 0-10%
}

// generatePE 生成市盈率
func (g *TencentDataGenerator) generatePE() float64 {
	return 5.0 + g.rand.Float64()*50.0 // 5-55
}

// generatePB 生成市净率
func (g *TencentDataGenerator) generatePB() float64 {
	return 0.5 + g.rand.Float64()*10.0 // 0.5-10.5
}

// generateCirculation 生成流通股本
func (g *TencentDataGenerator) generateCirculation() int64 {
	return int64(100000000 + g.rand.Intn(9000000000)) // 1亿-100亿
}

// GenerateStockData 生成StockData格式的数据
func (g *TencentDataGenerator) GenerateStockData(symbols []string) []core.StockData {
	var data []core.StockData
	for _, symbol := range symbols {
		stock := core.StockData{
			Symbol:    symbol,
			Name:      g.generateStockName(symbol),
			Price:     g.generatePrice(),
			PrevClose: 0, // 会在下面计算
			Open:      0, // 会在下面计算
			High:      0, // 会在下面计算
			Low:       0, // 会在下面计算
			Volume:    g.generateVolume(),
			Turnover:  0, // 暂不实现
			Timestamp: time.Now(),
		}

		// 计算相关价格
		stock.PrevClose = stock.Price + g.generateSmallChange()
		stock.Open = stock.Price + g.generateSmallChange()
		stock.High = stock.Price + math.Abs(g.generateSmallChange())
		stock.Low = stock.Price - math.Abs(g.generateSmallChange())

		data = append(data, stock)
	}

	return data
}

// DefaultTencentDataConfig 默认腾讯数据配置
func DefaultTencentDataConfig() TencentDataConfig {
	return TencentDataConfig{
		PriceRange:      PriceRange{Min: 1.0, Max: 100.0},
		VolumeRange:     VolumeRange{Min: 1000, Max: 1000000},
		ChangeRange:     ChangeRange{Min: -10.0, Max: 10.0},
		RandomSeed:      0,
		EnableRealistic: true,
		MarketHours:     false,
	}
}
