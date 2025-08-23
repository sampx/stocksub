//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"stocksub/pkg/testkit"
	"stocksub/pkg/testkit/config"
)

// TestTimeFieldAPIIntegration 集成测试：验证API返回数据的时间字段可被正确解析
func TestTimeFieldAPIIntegration(t *testing.T) {
	// 使用新的 testkit 管理器
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Type: "memory",
		},
		Storage: config.StorageConfig{
			Type:      "csv",
			Directory: t.TempDir(), // 使用临时目录进行存储
		},
	}
	manager := testkit.NewTestDataManager(cfg)
	defer manager.Close()

	// 选择代表性样本（每个市场1个）
	testSymbols := []string{
		"600000", // 上海主板
		"000001", // 深圳主板
		"300750", // 创业板
		"688036", // 科创板
		"835174", // 北交所
	}

	// 注意：此测试会真实调用外部API
	results, err := manager.GetStockData(context.Background(), testSymbols)
	assert.NoError(t, err, "API数据获取失败")
	assert.Equal(t, len(testSymbols), len(results), "返回数据数量不匹配")

	for _, result := range results {
		// 验证时间字段不为零值
		assert.False(t, result.Timestamp.IsZero(),
			"股票 %s 的时间字段为零值", result.Symbol)

		// 验证时间在合理范围内（不太过旧或过新）
		now := time.Now()
		age := now.Sub(result.Timestamp)
		assert.True(t, age >= 0 && age <= 24*time.Hour,
			"股票 %s 的时间戳 %s 不在合理范围内（与当前时间相差 %v）",
			result.Symbol, result.Timestamp.Format("2006-01-02 15:04:05"), age)

		t.Logf("✅ %s: 时间戳 %s, 距现在 %v",
			result.Symbol, result.Timestamp.Format("15:04:05"), age.Round(time.Second))
	}

	// 验证批量数据的时间一致性（同一次API调用获取的数据时间应该接近）
	if len(results) > 1 {
		var timestamps []time.Time
		for _, result := range results {
			timestamps = append(timestamps, result.Timestamp)
		}

		// 计算时间范围
		minTime, maxTime := timestamps[0], timestamps[0]
		for _, ts := range timestamps {
			if ts.Before(minTime) {
				minTime = ts
			}
			if ts.After(maxTime) {
				maxTime = ts
			}
		}

		timeRange := maxTime.Sub(minTime)
		// 批量获取的数据时间差异应该在合理范围内
		assert.LessOrEqual(t, timeRange, 60*time.Second,
			"批量获取的股票数据时间范围过大: %v", timeRange)

		t.Logf("📊 批量数据时间一致性: 范围 %v (从 %s 到 %s)",
			timeRange, minTime.Format("15:04:05"), maxTime.Format("15:04:05"))
	}
}