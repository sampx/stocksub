package tests

import (
	"stocksub/pkg/timing"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockTimeService 提供可控制的测试时间
// 用于测试中精确模拟不同的交易时间点
type MockTimeService struct {
	current time.Time
}

func (m *MockTimeService) Now() time.Time {
	return m.current
}

// SetTime 设置当前模拟时间
func (m *MockTimeService) SetTime(t time.Time) {
	m.current = t
}

// AddDuration 增加模拟时间
func (m *MockTimeService) AddDuration(d time.Duration) {
	m.current = m.current.Add(d)
}

// CreateMarketTimeWithMock 创建带有mock时间的MarketTime实例
// 用于测试环境中的时间控制
func CreateMarketTimeWithMock(mockTime time.Time) *timing.MarketTime {
	mockService := &MockTimeService{current: mockTime}
	return timing.NewMarketTime(mockService)
}

// AdvanceTimeForTesting 用于测试中推进时间
// 主要用于完整的交易日模拟
func AdvanceTimeForTesting(start time.Time, duration time.Duration, callback func(current time.Time)) {
	current := start
	end := start.Add(duration)
	
	stepDuration := 30 * time.Second
	
	for current.Before(end) {
		callback(current)
		current = current.Add(stepDuration)
	}
}

// MarketTimeTestHelper 市场时间测试辅助结构
type MarketTimeTestHelper struct {
	TimeService *MockTimeService
	MarketTime  *timing.MarketTime
}

// NewMarketTimeTestHelper 创建新的测试辅助器
func NewMarketTimeTestHelper(initialTime time.Time) *MarketTimeTestHelper {
	mockService := &MockTimeService{current: initialTime}
	marketTime := timing.NewMarketTime(mockService)
	
	return &MarketTimeTestHelper{
		TimeService: mockService,
		MarketTime:  marketTime,
	}
}

// AdvanceMarketTime 推进市场时间到指定时间点
func (h *MarketTimeTestHelper) AdvanceMarketTime(newTime time.Time) {
	h.TimeService.SetTime(newTime)
}

// IsInTradingWindow 当前是否在现易时间窗口内（近似检查）
func (h *MarketTimeTestHelper) IsInTradingWindow() bool {
	return h.MarketTime.IsTradingTime()
}

// TestMarketTimeScenarios 标准的市场时间测试场景套件
func TestMarketTimeScenarios(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	
	testScenarios := []struct {
		description    string
		testTime       string
		expectedResult bool
	}{
		{
			description:    "开盘前30秒 - 应该禁止",
			testTime:       "2025-08-21 09:13:00",
			expectedResult: false,
		},
		{
			description:    "开盘后30秒 - 应该允许",
			testTime:       "2025-08-21 09:14:00",
			expectedResult: true,
		},
		{
			description:    "上午收盘前30秒 - 应该允许",
			testTime:       "2025-08-21 11:29:30",
			expectedResult: true,
		},
		{
			description:    "午休时间 - 应该禁止",
			testTime:       "2025-08-21 11:35:00",
			expectedResult: false,
		},
		{
			description:    "下午开盘后30秒 - 应该允许",
			testTime:       "2025-08-21 12:58:00",
			expectedResult: true,
		},
		{
			description:    "收盘前30分钟 - 应该允许",
			testTime:       "2025-08-21 14:30:30",
			expectedResult: true,
		},
		{
			description:    "收盘后30秒 - 应该禁止",
			testTime:       "2025-08-21 15:00:30",
			expectedResult: false,
		},
		{
			description:    "周末 - 应该禁止",
			testTime:       "2025-08-23 10:00:00", // Saturday
			expectedResult: false,
		},
	}
	
	for _, scenario := range testScenarios {
		t.Run(scenario.description, func(t *testing.T) {
			testTime, _ := time.ParseInLocation("2006-01-02 15:04:05", scenario.testTime, location)
			
			helper := NewMarketTimeTestHelper(testTime)
			result := helper.IsInTradingWindow()
			
			assert.Equal(t, scenario.expectedResult, result, scenario.description)
		})
	}
}

// SimulateFullTradingDay 模拟完整的交易日程
func SimulateFullTradingDay(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	_ = time.Date(2025, 8, 21, 8, 30, 0, 0, location) // 上午8:30开始
	
	timeline := []struct {
		timeString string
		description string
		shouldBeTrading bool
	}{
		{"08:30:00", "开盘前准备 - 市场尚未开始", false},
		{"09:13:30", "正式开始监控 - 市场开盘", true},
		{"10:15:45", "正常交易时段 - 应该运行", true},
		{"11:29:50", "上午收盘阶段 - 应该继续", true},
		{"11:30:11", "午休时间 - 应该暂停", false},
		{"12:57:30", "下午重启交易 - 重新开启", true},
		{"14:55:00", "收盘前稳定数据检查期", true},
		{"15:00:10", "交易结束 - 停止API调用", false},
		{"15:01:00", "盘后清理期 - 完全停止", false},
	}
	
	for _, event := range timeline {
		t.Run(event.description, func(t *testing.T) {
			testTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 "+event.timeString, location)
			
			helper := NewMarketTimeTestHelper(testTime)
			trading := helper.IsInTradingWindow()
			
			assert.Equal(t, event.shouldBeTrading, trading, event.description)
		})
	}
}