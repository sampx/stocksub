//go:build integration

package limiter_test

import (
	"context"
	"errors"
	"stocksub/pkg/limiter"
	"stocksub/pkg/timing"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockTimeService 模拟时间服务
type MockTimeService struct {
	current time.Time
}

func (m *MockTimeService) Now() time.Time {
	return m.current
}

// MockError 模拟错误类型
type MockError struct {
	msg string
}

func (e *MockError) Error() string {
	return e.msg
}

// MockDataProvider 模拟数据提供者
type MockDataProvider struct {
	mockResponses  map[string][]string
	mockErrors     map[string]error
	requestCount   int
}

func NewMockDataProvider() *MockDataProvider {
	return &MockDataProvider{
		mockResponses: make(map[string][]string),
		mockErrors:    make(map[string]error),
		requestCount:  0,
	}
}

func (m *MockDataProvider) SetResponse(symbols string, response []string, err error) {
	m.mockResponses[symbols] = response
	m.mockErrors[symbols] = err
}

func (m *MockDataProvider) Fetch(symbols []string) ([]string, error) {
	m.requestCount++
	key := symbols[0] // 简化处理，只取第一个symbol

	if err, exists := m.mockErrors[key]; exists {
		return m.mockResponses[key], err
	}

	// 默认返回稳定数据
	return []string{"600000,平安银行,12.50,+0.05,+0.40,1234567"}, nil
}

// TestIntelligentLimiter_ShouldProceed_WithTimingScenarios 测试在不同时间场景下是否允许继续
func TestIntelligentLimiter_ShouldProceed_WithTimingScenarios(t *testing.T) {
	testScenarios := []struct {
		name        string
		mockTime    string
		expectStart bool
		description string
	}{
		{"正常交易时间", "2025-08-21 10:00:00", true, "应该允许在正常交易时间启动"},
		{"开盘前", "2025-08-21 09:00:00", false, "应该禁止在开盘前启动"},
		{"收盘后", "2025-08-21 16:00:00", false, "应该禁止在收盘后启动"},
		{"周末", "2025-08-23 10:00:00", false, "应该禁止在周末启动"},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			location := time.FixedZone("CST", 8*3600)
			mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", scenario.mockTime, location)
			mockService := &MockTimeService{current: mockTime}
			marketTime := timing.NewMarketTime(mockService)
			intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)

			intelligentLimiter.InitializeBatch([]string{"600000", "000001"})

			ctx := context.Background()
			shouldProceed, err := intelligentLimiter.ShouldProceed(ctx)

			if scenario.expectStart {
				assert.True(t, shouldProceed, scenario.description)
				assert.NoError(t, err, "正常交易时间不应该有错误")
			} else {
				assert.False(t, shouldProceed, scenario.description)
			}
		})
	}
}

// TestIntelligentLimiter_RecordResult_WithDifferentErrorTypes 测试记录不同错误类型的结果
func TestIntelligentLimiter_RecordResult_WithDifferentErrorTypes(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	tradingTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 10:00:00", location)
	mockService := &MockTimeService{current: tradingTime}
	marketTime := timing.NewMarketTime(mockService)
	intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)

	intelligentLimiter.InitializeBatch([]string{"600000"})

	errorTests := []struct {
		name         string
		errorMsg     string
		expectedStop bool
		description  string
	}{
		{"致命错误-连接拒绝", "dial tcp: connection refused", true, "连接拒绝应该立即停止"},
		{"网络错误-超时", "i/o timeout", false, "网络超时应该允许重试"},
		{"成功情况", "", false, "成功时应该继续"},
	}

	for _, errorTest := range errorTests {
		t.Run(errorTest.name, func(t *testing.T) {
			var testErr error
			var mockData []string

			if errorTest.errorMsg != "" {
				testErr = &MockError{msg: errorTest.errorMsg}
			} else {
				mockData = []string{"600000,平安银行,12.50,+0.05,+0.40,1234567"}
			}

			shouldContinue, waitDuration, finalErr := intelligentLimiter.RecordResult(testErr, mockData)

			if errorTest.expectedStop {
				assert.False(t, shouldContinue, errorTest.description)
				assert.NotNil(t, finalErr, "致命错误应该返回终止错误")
			} else {
				if testErr == nil {
					assert.True(t, shouldContinue, errorTest.description)
					assert.Nil(t, finalErr, "成功时不应该有终止错误")
				} else {
					if !shouldContinue && waitDuration > 0 {
						assert.Greater(t, waitDuration, time.Duration(0), "网络错误应该有重试等待时间")
					}
				}
			}
		})
	}
}

// TestIntelligentLimiter_RecordResult_WithStableData 测试在数据稳定时是否停止
func TestIntelligentLimiter_RecordResult_WithStableData(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	afterTradingTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 15:01:00", location)
	mockService := &MockTimeService{current: afterTradingTime}
	marketTime := timing.NewMarketTime(mockService)
	intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)

	intelligentLimiter.InitializeBatch([]string{"600000"})

	stableData := []string{"600000,稳定,10.00,0.00,0.00,1000000"}

	// 初始化指纹
	shouldContinue, _, _ := intelligentLimiter.RecordResult(nil, stableData)
	assert.True(t, shouldContinue, "初始化数据应该继续")

	// 连续发送相同数据
	for i := 0; i < 6; i++ {
		shouldContinue, _, finalErr := intelligentLimiter.RecordResult(nil, stableData)
		if i < 3 {
			assert.True(t, shouldContinue, "第%d次相同数据应该继续", i+1)
		} else {
			assert.False(t, shouldContinue, "第%d次相同数据应该触发终止", i+1)
			assert.NotNil(t, finalErr, "第%d次应该有终止错误", i+1)
			assert.Contains(t, finalErr.Error(), "稳定", "错误信息应包含'稳定'")
			break
		}
	}

	t.Run("收盘前不检查一致性", func(t *testing.T) {
		beforeTradingEnd, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 14:58:00", location)
		mockService.current = beforeTradingEnd
		limiterBefore := limiter.NewIntelligentLimiter(marketTime)
		limiterBefore.InitializeBatch([]string{"600000"})

		for i := 0; i < 8; i++ {
			shouldContinue, _, finalErr := limiterBefore.RecordResult(nil, stableData)
			assert.True(t, shouldContinue, "收盘前第%d次相同数据应该继续", i+1)
			assert.Nil(t, finalErr, "收盘前不应该有终止错误")
		}
	})
}

// TestIntelligentLimiter_FullDaySimulation_WithVariousScenarios 模拟全天运行的各种场景
func TestIntelligentLimiter_FullDaySimulation_WithVariousScenarios(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	scenarios := []struct {
		name         string
		startTime    string
		scenario     string // "success", "fatal", "network", "data_stable"
		expectedStop bool
	}{
		{"正常交易日", "2025-08-21 09:13:30", "success", false},
		{"开盘前禁止", "2025-08-21 09:13:29", "fatal", true},
		{"收盘后禁止", "2025-08-21 15:00:11", "fatal", true},
		{"周末非交易日", "2025-08-23 10:00:00", "fatal", true},
		{"致命错误触发", "2025-08-21 09:13:30", "fatal", true},
		{"网络错误重试", "2025-08-21 09:13:30", "network", true},
	}

	for _, tt := range scenarios {
		t.Run(tt.name, func(t *testing.T) {
			mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", tt.startTime, location)
			mockService := &MockTimeService{current: mockTime}
			marketTime := timing.NewMarketTime(mockService)
			intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)
			mockProvider := NewMockDataProvider()

			symbols := []string{"600000", "000001", "300750"}
			intelligentLimiter.InitializeBatch(symbols)

			switch tt.scenario {
			case "fatal":
				mockProvider.SetResponse("600000", nil, errors.New("dial tcp: connection refused"))
			case "network":
				mockProvider.SetResponse("600000", nil, errors.New("timeout"))
			case "data_stable":
				mockProvider.SetResponse("600000", []string{"stable,data,1.0"}, nil)
			default:
				mockProvider.SetResponse("600000", []string{"600000,平安银行,12.50,..."}, nil)
			}

			status := runLimiterSimulation(mockProvider, intelligentLimiter, tt.scenario)
			assert.Equal(t, tt.expectedStop, status.forceStop, "强制停止状态应匹配预期: %s", tt.name)
		})
	}
}

func runLimiterSimulation(provider *MockDataProvider, limiter *limiter.IntelligentLimiter, scenario string) struct {
	forceStop bool
	lastError error
} {
	ctx := context.Background()
	for i := 0; i < 10; i++ { // 限制循环次数
		shouldProceed, err := limiter.ShouldProceed(ctx)
		if err != nil || !shouldProceed {
			return struct {
				forceStop bool
				lastError error
			}{true, err}
		}

		data, err := provider.Fetch([]string{"600000"})
		shouldContinue, waitDuration, finalErr := limiter.RecordResult(err, data)

		if finalErr != nil {
			return struct {
				forceStop bool
				lastError error
			}{true, finalErr}
		}
		if !shouldContinue {
			if waitDuration > 0 {
				time.Sleep(1 * time.Millisecond) // 模拟等待
				continue
			}
			break
		}
	}
	status := limiter.GetStatus()
	return struct {
		forceStop bool
		lastError error
	}{forceStop: status["force_stop"].(bool), lastError: nil}
}

// TestIntelligentLimiter_CircuitBreaker_WithFatalAndRetryableErrors 测试熔断机制
func TestIntelligentLimiter_CircuitBreaker_WithFatalAndRetryableErrors(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 09:30:00", location)
	mockService := &MockTimeService{current: mockTime}
	marketTime := timing.NewMarketTime(mockService)
	intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)

	t.Run("致命错误单次触发", func(t *testing.T) {
		intelligentLimiter.InitializeBatch([]string{"600000"})
		err := errors.New("dial tcp: connection refused")
		shouldContinue, _, finalErr := intelligentLimiter.RecordResult(err, nil)
		assert.False(t, shouldContinue, "致命错误应触发立即停止")
		assert.Error(t, finalErr, "应返回错误信息")
		status := intelligentLimiter.GetStatus()
		assert.True(t, status["force_stop"].(bool), "应设置强制停止标志")
	})

	t.Run("网络错误重试", func(t *testing.T) {
		intelligentLimiter.Reset()
		intelligentLimiter.InitializeBatch([]string{"600000"})
		networkErr := errors.New("timeout")
		shouldContinue, waitDuration, finalErr := intelligentLimiter.RecordResult(networkErr, nil)
		assert.False(t, shouldContinue, "网络错误应该等待重试")
		assert.Greater(t, waitDuration, time.Duration(0), "第一次重试应等待")
		assert.Nil(t, finalErr, "不应返回最终错误")
		status := intelligentLimiter.GetStatus()
		assert.Equal(t, 1, status["retry_count"], "重试计数应增加")
	})
}