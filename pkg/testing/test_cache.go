package testing

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"strings"
	"time"

	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/subscriber"
)

type TestDataCache struct {
	storage     *CSVStorage
	memCache    map[string][]subscriber.StockData // L1内存缓存
	cacheExpiry time.Duration                     // 缓存过期时间
	dataDir     string                            // 数据目录
	sessionID   string                            // 测试会话ID
}

func NewTestDataCache(dataDir string) *TestDataCache {
	// 生成测试会话ID
	sessionID := fmt.Sprintf("test_%d", time.Now().Unix())

	return &TestDataCache{
		storage:     NewCSVStorage(dataDir),
		memCache:    make(map[string][]subscriber.StockData),
		cacheExpiry: 24 * time.Hour, // 缓存24小时
		dataDir:     dataDir,
		sessionID:   sessionID,
	}
}

// GetStockDataBatch 智能获取股票数据（三层缓存策略）
func (tdc *TestDataCache) GetStockDataBatch(symbols []string) ([]subscriber.StockData, error) {
	cacheKey := tdc.generateCacheKey(symbols)

	// L1: 检查内存缓存
	if cached, exists := tdc.memCache[cacheKey]; exists {
		fmt.Printf("🎯 使用L1内存缓存，股票: %v\n", symbols)
		return cached, nil
	}

	// L2: 检查CSV缓存
	if cached, err := tdc.loadFromCSVCache(symbols); err == nil && len(cached) == len(symbols) {
		fmt.Printf("📁 使用L2 CSV缓存，股票: %v\n", symbols)
		tdc.memCache[cacheKey] = cached // 提升到L1
		return cached, nil
	}

	// L3: API调用（仅在必要时）
	fmt.Printf("🌐 执行API调用，股票: %v\n", symbols)

	// 检查是否强制使用缓存（CI/CD环境）
	if os.Getenv("TEST_FORCE_CACHE") == "1" {
		return nil, fmt.Errorf("强制缓存模式，但未找到有效缓存数据")
	}

	provider := tencent.NewProvider()
	// 测试环境下稍微宽松的限流
	provider.SetRateLimit(500 * time.Millisecond)
	provider.SetTimeout(30 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.FetchData(ctx, symbols)
	if err != nil {
		return nil, fmt.Errorf("API调用失败: %w", err)
	}

	// 保存到CSV缓存
	if err := tdc.saveToCSVCache(results); err != nil {
		fmt.Printf("⚠️ 保存缓存失败: %v\n", err)
	}

	// 存入L1缓存
	tdc.memCache[cacheKey] = results

	return results, nil
}

// ForceRefreshCache 强制刷新缓存（用于需要最新数据的测试）
func (tdc *TestDataCache) ForceRefreshCache(symbols []string) ([]subscriber.StockData, error) {
	cacheKey := tdc.generateCacheKey(symbols)
	delete(tdc.memCache, cacheKey) // 清除L1缓存
	// 清除L2缓存的逻辑可以根据需要实现
	return tdc.GetStockDataBatch(symbols)
}

// generateCacheKey 生成缓存键
func (tdc *TestDataCache) generateCacheKey(symbols []string) string {
	joined := strings.Join(symbols, ",")
	hash := md5.Sum([]byte(joined))
	return fmt.Sprintf("%x", hash[:8]) // 使用8字节hash作为键
}

// loadFromCSVCache 从CSV缓存加载数据
func (tdc *TestDataCache) loadFromCSVCache(symbols []string) ([]subscriber.StockData, error) {
	today := time.Now()
	yesterday := today.AddDate(0, 0, -1)

	// 尝试加载最近2天的缓存数据
	dataPoints, err := tdc.storage.ReadDataPoints(yesterday, today)
	if err != nil {
		return nil, err
	}

	// 转换为StockData并筛选所需股票
	symbolSet := make(map[string]bool)
	for _, symbol := range symbols {
		symbolSet[symbol] = true
	}

	var results []subscriber.StockData
	for _, dp := range dataPoints {
		if symbolSet[dp.Symbol] {
			// 检查缓存是否过期
			if time.Since(dp.Timestamp) > tdc.cacheExpiry {
				continue
			}

			// 转换DataPoint为StockData
			var stockData subscriber.StockData
			// 使用json unmarshal来从AllFields恢复完整的StockData
			// 这是一个假设，假设AllFields就是StockData的json序列化
			// 如果不是，我们需要更复杂的转换逻辑
			if len(dp.AllFields) > 0 {
				// 假设AllFields的第一个元素是json数据
				if err := subscriber.UnmarshalStockData([]byte(dp.AllFields[0]), &stockData); err == nil {
					results = append(results, stockData)
					continue
				}
			}

			// 如果上面的方法失败，回退到手动映射
			stockData = subscriber.StockData{
				Symbol:    dp.Symbol,
				Price:     dp.Price,
				Timestamp: dp.Timestamp,
				// ... 其他字段需要从dp.AllFields中解析
			}
			results = append(results, stockData)
		}
	}

	// 检查是否所有股票都有缓存
	if len(results) < len(symbols) {
		return nil, fmt.Errorf("缓存数据不完整")
	}

	return results, nil
}

// saveToCSVCache 保存数据到CSV缓存
func (tdc *TestDataCache) saveToCSVCache(results []subscriber.StockData) error {
	var dataPoints []DataPoint

	now := time.Now()
	for _, result := range results {
		// 将完整的StockData序列化为json并存储
		jsonData, err := subscriber.MarshalStockData(result)
		if err != nil {
			// 记录错误但继续
			fmt.Printf("序列化StockData失败: %v\n", err)
			continue
		}

		dp := DataPoint{
			Timestamp:    now,
			Symbol:       result.Symbol,
			QueryTime:    now,
			ResponseTime: now,
			Price:        result.Price,
			Field30:      result.Timestamp.Format("20060102150405"),
			AllFields:    []string{string(jsonData)},
		}
		dataPoints = append(dataPoints, dp)
	}

	return tdc.storage.BatchSaveDataPoints(dataPoints)
}

// GetCacheStats 获取缓存统计信息
func (tdc *TestDataCache) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"session_id":     tdc.sessionID,
		"l1_cache_size":  len(tdc.memCache),
		"cache_expiry":   tdc.cacheExpiry,
		"data_directory": tdc.dataDir,
	}
}

// Close 关闭缓存管理器
func (tdc *TestDataCache) Close() error {
	return tdc.storage.Close()
}
