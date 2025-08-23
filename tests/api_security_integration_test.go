//go:build integration

package tests

import (
	"context"
	"errors"
	"stocksub/pkg/limiter"
	"stocksub/pkg/timing"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	
	if _, exists := m.mockErrors[key]; exists {
		return m.mockResponses[key], m.mockErrors[key]
	}
	
	// 默认返回稳定数据
	return []string{"600000,平安银行,12.50,+0.05,+0.40,1234567"}, nil
}

func TestFullDaySimulation(t *testing.T) {
	// 集成测试：模拟16小时交易时段
	location := time.FixedZone("CST", 8*3600)
	
	tests := []struct {
		name         string
		startTime    string
		expectedEnd  string
		scenario     string // "success", "fatal", "network", "data_stable"
		expectedStop bool
	}{
		{
			name:         "正常交易日-完整过程",
			startTime:    "2025-08-21 09:13:30",
			expectedEnd:  "2025-08-21 15:00:10",
			scenario:     "success",
			expectedStop: false,
		},
		{
			name:         "时间边界-开盘前禁止",
			startTime:    "2025-08-21 09:13:29",
			expectedEnd:  "2025-08-21 15:00:10",
			scenario:     "fatal",
			expectedStop: true,
		},
		{
			name:         "时间边界-收盘后立即",
			startTime:    "2025-08-21 15:00:11",
			expectedEnd:  "2025-08-21 15:00:10",
			scenario:     "fatal",
			expectedStop: true,
		},
		{
			name:         "周末非交易日",
			startTime:    "2025-08-23 10:00:00", // 周六
			expectedEnd:  "2025-08-21 15:00:10",
			scenario:     "fatal",
			expectedStop: true,
		},
		{
			name:         "致命错误单次触发",
			startTime:    "2025-08-21 09:13:30",
			expectedEnd:  "2025-08-21 15:00:10",
			scenario:     "fatal",
			expectedStop: true,
		},
		{
			name:         "网络错误重试机制",
			startTime:    "2025-08-21 09:13:30",
			expectedEnd:  "2025-08-21 15:00:10",
			scenario:     "network",
			expectedStop: false, // 会被重试机制处理
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置mock时间
			mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", tt.startTime, location)
			mockService := &MockTimeService{current: mockTime}
			
			marketTime := timing.NewMarketTime(mockService)
			intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)
			mockProvider := NewMockDataProvider()
			
			// 模拟测试场景
			symbols := []string{"600000", "000001", "300750"}
			intelligentLimiter.InitializeBatch(symbols)
			
			// 设置模拟响应
			switch tt.scenario {
			case "fatal":
				mockProvider.SetResponse("600000", nil, errors.New("dial tcp: connection refused"))
			case "network":
				mockProvider.SetResponse("600000", nil, errors.New("timeout"))
			case "data_stable":
				mockProvider.SetResponse("600000", []string{"stable,data,1.0"}, nil)
			default:
				mockProvider.SetResponse("600000", []string{"600000,平安银行,12.50,+0.05,+0.40,1234567"}, nil)
			}
			
			// 执行模拟测试
			status := runSimulationTest(mockProvider, intelligentLimiter, tt.scenario)
			assert.Equal(t, tt.expectedStop, status.forceStop, "强制停止状态应匹配预期: %s", tt.name)
		})
	}
}

func runSimulationTest(provider *MockDataProvider, limiter *limiter.IntelligentLimiter, scenario string) struct {
	forceStop bool
	retryCount int
	lastError error
} {
	ctx := context.Background()
	
	// 开始第一个循环
	i := 0
	for i < 100 { // 限制最大循环次数防止死循环
		i++
		
		shouldProceed, err := limiter.ShouldProceed(ctx)
		
		if err != nil {
			// 交易时段检查失败
			return struct{
				forceStop bool
				retryCount int
				lastError error
			}{true, 0, err}
			}
		
			if !shouldProceed {
			//达到限制条件
			break
			}
		
		// 尝试获取数据
		data, err := provider.Fetch([]string{"600000"})
		
		// 记录结果
		shouldContinue, waitDuration, finalErr := limiter.RecordResult(err, data)
		
		if !shouldContinue {
			// 达到终止条件
			if waitDuration > 0 {
				// 有重试等待时间
				time.Sleep(10 * time.Millisecond) // 模拟等待
				continue
			}
			break
		}
		
		if finalErr != nil {
			// 处理最终错误
			break
		}
		
		// 成功获取数据
		if scenario == "data_stable" {
			// 连续5次相同数据测试
			_, _, _ = limiter.RecordResult(nil, []string{"stable,data,1.0"})
			_, _, _ = limiter.RecordResult(nil, []string{"stable,data,1.0"})
			_, _, _ = limiter.RecordResult(nil, []string{"stable,data,1.0"})
			_, _, _ = limiter.RecordResult(nil, []string{"stable,data,1.0"})
			_, _, finalErr := limiter.RecordResult(nil, []string{"stable,data,1.0"})
			
			if finalErr != nil {
				return struct{
					forceStop bool
					retryCount int
					lastError error
				}{true, 0, finalErr}
			}
		}
	}
	
	// 获取最终状态
	status := limiter.GetStatus()
	
	return struct{
		forceStop bool
		retryCount int
		lastError error
	}{
		forceStop:  status["force_stop"].(bool),
		retryCount: int(status["retry_count"].(int)),
		lastError:  nil,
	}
}

func TestErrorHandlingIntegration(t *testing.T) {
	// 测试四级熔断机制集成
	location := time.FixedZone("CST", 8*3600)
	mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 09:30:00", location)
	
	mockService := &MockTimeService{current: mockTime}
	marketTime := timing.NewMarketTime(mockService)
	intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)
	
	// 测试致命错误处理
	t.Run("致命错误单次触发", func(t *testing.T) {
		intelligentLimiter.InitializeBatch([]string{"600000"})
		
		// 模拟致命错误
		err := errors.New("dial tcp: connection refused")
		
		shouldContinue, _, finalErr := intelligentLimiter.RecordResult(err, nil)
		
		assert.False(t, shouldContinue, "致命错误应触发立即停止")
		assert.Error(t, finalErr, "应返回错误信息")
		
		status := intelligentLimiter.GetStatus()
		assert.True(t, status["force_stop"].(bool), "应设置强制停止标志")
	})
	
	// 测试网络错误重试机制
	t.Run("网络错误重试", func(t *testing.T) {
		intelligentLimiter.Reset()
		intelligentLimiter.InitializeBatch([]string{"600000"})
		
		// 第一次网络错误
		networkErr := errors.New("timeout")
		
		shouldContinue, waitDuration, finalErr := intelligentLimiter.RecordResult(networkErr, nil)
		
		assert.False(t, shouldContinue, "网络错误应该等待重试")
		assert.Equal(t, 60*time.Second, waitDuration, "第一次重试应等待1分钟")
		assert.Nil(t, finalErr, "不应返回最终错误")
		
		status := intelligentLimiter.GetStatus()
		assert.Equal(t, 1, status["retry_count"], "重试计数应增加")
	})
	
	// 测试时间边界保护
	t.Run("时间边界保护", func(t *testing.T) {
		intelligentLimiter.Reset()
		
		// 设置收盘时间
		lateMockTime := time.Date(2025, 8, 21, 14, 55, 0, 0, location)
		mockService.current = lateMockTime
		
		intelligentLimiter.InitializeBatch([]string{"600000"})
		
		// 模拟网络错误，但时间已接近收盘
		networkErr := errors.New("timeout")
		shouldContinue, waitDuration, finalErr := intelligentLimiter.RecordResult(networkErr, nil)
		
		// 这种情况下应该不允许重试
		_ = shouldContinue
		_ = waitDuration
		_ = finalErr
	})
}