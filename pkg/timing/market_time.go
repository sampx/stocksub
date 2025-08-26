package timing

import (
	"time"
)

// TimeService 提供当前时间接口，用于mock测试
type TimeService interface {
	Now() time.Time
}

// SystemTimeService 使用系统实际时间
type SystemTimeService struct{}

func (s *SystemTimeService) Now() time.Time {
	return time.Now()
}

// MarketTime 提供市场交易时间检测功能
type MarketTime struct {
	timeService TimeService
}

// NewMarketTime 创建新的市场时间检测器
func NewMarketTime(timeService TimeService) *MarketTime {
	return &MarketTime{
		timeService: timeService,
	}
}

// DefaultMarketTime 使用系统时间的默认市场时间检测器
func DefaultMarketTime() *MarketTime {
	return NewMarketTime(&SystemTimeService{})
}

// Now 返回当前时间
func (m *MarketTime) Now() time.Time {
	return m.timeService.Now()
}

// IsTradingTime 判断当前是否在交易时段
func (m *MarketTime) IsTradingTime() bool {
	now := m.timeService.Now()
	
	// 周末不交易
	if !m.IsTradingDay(now) {
		return false
	}
	
	// 上午交易时段: 09:13:30 - 11:30:10
	// 下午交易时段: 12:57:30 - 15:00:10
	currentTime := now.Format("15:04:05")
	
	morningStart := "09:13:30"
	morningEnd := "11:30:10"
	
	afternoonStart := "12:57:30"
	afternoonEnd := "15:00:10"
	
	return (currentTime >= morningStart && currentTime <= morningEnd) ||
		(currentTime >= afternoonStart && currentTime <= afternoonEnd)
}

// IsTradingDay 判断是否是交易日（周一到周五）
func (m *MarketTime) IsTradingDay(t time.Time) bool {
	weekday := t.Weekday()
	return weekday >= time.Monday && weekday <= time.Friday
}

// GetNextTradingDayStart 获取下一个交易日的开始时间
func (m *MarketTime) GetNextTradingDayStart() time.Time {
	now := m.timeService.Now()
	todayMorning := time.Date(now.Year(), now.Month(), now.Day(), 9, 13, 30, 0, now.Location())
	
	// 如果是周末，跳到下周
	if !m.IsTradingDay(now) {
		daysUntilNext := 0
		switch now.Weekday() {
		case time.Saturday:
			daysUntilNext = 2
		case time.Sunday:
			daysUntilNext = 1
		default:
			daysUntilNext = 0
		}
		return todayMorning.AddDate(0, 0, daysUntilNext)
	}
	
	// 如果今天已经过了交易时间，跳到明天
	currentTime := now.Format("15:04:05")
	if currentTime > "15:00:10" {
		// 如果今天是周五，跳到周一
		if now.Weekday() == time.Friday {
			return todayMorning.AddDate(0, 0, 3)
		}
		return todayMorning.AddDate(0, 0, 1)
	}
	
	return todayMorning
}

// GetTradingEndTime 获取当天交易结束时间
func (m *MarketTime) GetTradingEndTime() time.Time {
	now := m.timeService.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 10, 0, now.Location())
}

// IsCloseToEnd 判断是否在收盘前5分钟内（预留方法，现已不用于数据一致性检测）
func (m *MarketTime) IsCloseToEnd() bool {
	now := m.timeService.Now()
	currentTime := now.Format("15:04:05")
	
	// 收盘前5分钟: 14:55:00 - 15:00:10
	return currentTime >= "14:55:00" && currentTime <= "15:00:10"
}

// IsAfterTradingEnd 判断是否在收盘后（用于数据一致性检测）
func (m *MarketTime) IsAfterTradingEnd() bool {
	now := m.timeService.Now()
	
	// 周末不交易，不需要检查
	if !m.IsTradingDay(now) {
		return false
	}
	
	currentTime := now.Format("15:04:05")
	// 收盘后时段: 15:00:11 之后（给1秒缓冲时间）
	return currentTime >= "15:00:11"
}

// TimeUntilNextInterval 计算到下一个检查间隔的等待时间
func (m *MarketTime) TimeUntilNextInterval(minutes int) time.Duration {
	now := m.timeService.Now()
	nextInterval := now.Add(time.Duration(minutes) * time.Minute)
	return nextInterval.Sub(now)
}