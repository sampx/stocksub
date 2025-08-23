package tests

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"stocksub/pkg/limiter"
	"stocksub/pkg/timing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SafeAPIMonitorConfig 安全测试版本的监控配置
type SafeAPIMonitorConfig struct {
	Symbols  []string
	DataDir  string
	LogDir   string
}

// SafeAPIMonitor 安全测试版本的API监控器
type SafeAPIMonitor struct {
	config             SafeAPIMonitorConfig
	marketTime         *timing.MarketTime
	intelligentLimiter *limiter.IntelligentLimiter
}

// NewSafeAPIMonitor 创建安全测试版本的API监控器
func NewSafeAPIMonitor(config SafeAPIMonitorConfig, mockTime time.Time) (*SafeAPIMonitor, error) {
	// 确保目录存在
	dirs := []string{config.DataDir, config.LogDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// 创建带有Mock时间的市场时间服务
	mockTimeService := &MockTimeService{current: mockTime}
	marketTime := timing.NewMarketTime(mockTimeService)
	intelligentLimiter := limiter.NewIntelligentLimiter(marketTime)

	return &SafeAPIMonitor{
		config:             config,
		marketTime:         marketTime,
		intelligentLimiter: intelligentLimiter,
	}, nil
}

// TestAPIMonitorSafeIntegration 测试API监控器的安全集成
func TestAPIMonitorSafeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 使用测试专用目录
	_, currentFile, _, _ := runtime.Caller(0)
	testsDir := filepath.Dir(currentFile)
	testDataDir := filepath.Join(testsDir, "data", "safe_monitor_test")

	// 清理并创建测试目录
	os.RemoveAll(testDataDir)
	err := os.MkdirAll(testDataDir, 0755)
	require.NoError(t, err)

	testScenarios := []struct {
		name         string
		mockTime     string
		expectStart  bool
		description  string
	}{
		{
			name:         "正常交易时间",
			mockTime:     "2025-08-21 10:00:00",
			expectStart:  true,
			description:  "应该允许在正常交易时间启动",
		},
		{
			name:         "开盘前",
			mockTime:     "2025-08-21 09:00:00",
			expectStart:  false,
			description:  "应该禁止在开盘前启动",
		},
		{
			name:         "收盘后",
			mockTime:     "2025-08-21 16:00:00",
			expectStart:  false,
			description:  "应该禁止在收盘后启动",
		},
		{
			name:         "周末",
			mockTime:     "2025-08-23 10:00:00", // 周六
			expectStart:  false,
			description:  "应该禁止在周末启动",
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			location := time.FixedZone("CST", 8*3600)
			mockTime, _ := time.ParseInLocation("2006-01-02 15:04:05", scenario.mockTime, location)

			config := SafeAPIMonitorConfig{
				Symbols: []string{"600000", "000001"},
				DataDir: filepath.Join(testDataDir, scenario.name),
				LogDir:  filepath.Join(testDataDir, scenario.name, "logs"),
			}

			monitor, err := NewSafeAPIMonitor(config, mockTime)
			require.NoError(t, err, "创建监控器失败")

			// 初始化批次
			monitor.intelligentLimiter.InitializeBatch(config.Symbols)

			// 测试是否可以开始
			ctx := context.Background()
			shouldProceed, err := monitor.intelligentLimiter.ShouldProceed(ctx)

			if scenario.expectStart {
				assert.True(t, shouldProceed, scenario.description)
				assert.NoError(t, err, "正常交易时间不应该有错误")
			} else {
				if err != nil {
					// 有错误时应该无法继续
					assert.False(t, shouldProceed, scenario.description)
				} else {
					// 即使没有错误，非交易时间也可能不允许继续
					assert.False(t, shouldProceed, scenario.description)
				}
			}

			t.Logf("✅ %s: 期望启动=%t, 实际可继续=%t, 错误=%v", 
				scenario.description, scenario.expectStart, shouldProceed, err)
		})
	}
}

// TestErrorHandlingSafety 测试错误处理的安全性
func TestErrorHandlingSafety(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	tradingTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 10:00:00", location)

	_, currentFile, _, _ := runtime.Caller(0)
	testsDir := filepath.Dir(currentFile)
	testDataDir := filepath.Join(testsDir, "data", "error_safety_test")

	os.RemoveAll(testDataDir)
	err := os.MkdirAll(testDataDir, 0755)
	require.NoError(t, err)

	config := SafeAPIMonitorConfig{
		Symbols: []string{"600000"},
		DataDir: testDataDir,
		LogDir:  filepath.Join(testDataDir, "logs"),
	}

	monitor, err := NewSafeAPIMonitor(config, tradingTime)
	require.NoError(t, err)

	monitor.intelligentLimiter.InitializeBatch(config.Symbols)

	errorTests := []struct {
		name          string
		errorMsg      string
		expectedStop  bool
		description   string
	}{
		{
			name:         "致命错误-连接拒绝",
			errorMsg:     "dial tcp: connection refused",
			expectedStop: true,
			description:  "连接拒绝应该立即停止",
		},
		{
			name:         "网络错误-超时",
			errorMsg:     "i/o timeout",
			expectedStop: false,
			description:  "网络超时应该允许重试",
		},
		{
			name:         "成功情况",
			errorMsg:     "",
			expectedStop: false,
			description:  "成功时应该继续",
		},
	}

	for _, errorTest := range errorTests {
		t.Run(errorTest.name, func(t *testing.T) {
			var testErr error
			var mockData []string

			if errorTest.errorMsg != "" {
				testErr = &MockError{msg: errorTest.errorMsg}
			} else {
				// 模拟成功数据
				mockData = []string{"600000,平安银行,12.50,+0.05,+0.40,1234567"}
			}

			shouldContinue, waitDuration, finalErr := monitor.intelligentLimiter.RecordResult(testErr, mockData)

			if errorTest.expectedStop {
				assert.False(t, shouldContinue, errorTest.description)
				assert.NotNil(t, finalErr, "致命错误应该返回终止错误")
			} else {
				if testErr == nil {
					// 成功情况
					assert.True(t, shouldContinue, errorTest.description)
					assert.Nil(t, finalErr, "成功时不应该有终止错误")
				} else {
					// 网络错误，可能需要等待重试
					if !shouldContinue && waitDuration > 0 {
						assert.Greater(t, waitDuration, time.Duration(0), "网络错误应该有重试等待时间")
					}
				}
			}

			t.Logf("✅ %s: 继续=%t, 等待=%v, 终止错误=%v", 
				errorTest.description, shouldContinue, waitDuration, finalErr)
		})
	}
}

// MockError 模拟错误类型
type MockError struct {
	msg string
}

func (e *MockError) Error() string {
	return e.msg
}

// TestDataConsistencyDetection 测试数据一致性检测
func TestDataConsistencyDetection(t *testing.T) {
	// 模拟收盘后的时间
	location := time.FixedZone("CST", 8*3600)
	afterTradingTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 15:01:00", location)

	_, currentFile, _, _ := runtime.Caller(0)
	testsDir := filepath.Dir(currentFile)
	testDataDir := filepath.Join(testsDir, "data", "consistency_test")

	os.RemoveAll(testDataDir)
	err := os.MkdirAll(testDataDir, 0755)
	require.NoError(t, err)

	config := SafeAPIMonitorConfig{
		Symbols: []string{"600000"},
		DataDir: testDataDir,
		LogDir:  filepath.Join(testDataDir, "logs"),
	}

	monitor, err := NewSafeAPIMonitor(config, afterTradingTime)
	require.NoError(t, err)

	monitor.intelligentLimiter.InitializeBatch(config.Symbols)

	// 模拟连续5次相同数据
	stableData := []string{"600000,稳定,10.00,0.00,0.00,1000000"}

	// 先发送一次数据初始化指纹
	shouldContinue, waitDuration, finalErr := monitor.intelligentLimiter.RecordResult(nil, stableData)
	assert.True(t, shouldContinue, "初始化数据应该继续")
	assert.Nil(t, finalErr, "初始化数据不应该有错误")
	t.Logf("初始化数据: 继续=%t, 等待=%v, 错误=%v", shouldContinue, waitDuration, finalErr)

	// 然后连续发送相同数据，在收盘后应该第5次触发终止
	for i := 0; i < 6; i++ {
		shouldContinue, waitDuration, finalErr = monitor.intelligentLimiter.RecordResult(nil, stableData)
		
		if i < 3 {
			// 前3次应该继续（因为加上初始化数据，总共需要5次才触发）
			assert.True(t, shouldContinue, "第%d次相同数据应该继续", i+1)
			assert.Nil(t, finalErr, "第%d次不应该有终止错误", i+1)
		} else {
			// 第4次（加上初始化的就是第5次）应该触发稳定数据检测
			assert.False(t, shouldContinue, "第%d次相同数据应该触发终止", i+1)
			assert.NotNil(t, finalErr, "第%d次应该有终止错误", i+1)
			assert.Contains(t, finalErr.Error(), "稳定", "第%d次相同数据应该触发稳定检测", i+1)
			break // 检测到稳定，结束循环
		}
		
		t.Logf("第%d次数据: 继续=%t, 等待=%v, 错误=%v", i+1, shouldContinue, waitDuration, finalErr)
	}

	// 测试收盘前不应该触发一致性检查
	t.Run("收盘前不检查一致性", func(t *testing.T) {
		// 模拟收盘前时间
		beforeTradingEnd, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-21 14:58:00", location)
		monitorBefore, err := NewSafeAPIMonitor(config, beforeTradingEnd)
		require.NoError(t, err)
		monitorBefore.intelligentLimiter.InitializeBatch(config.Symbols)

		// 连续多次相同数据，在收盘前不应该触发终止
		for i := 0; i < 8; i++ {
			shouldContinue, _, finalErr := monitorBefore.intelligentLimiter.RecordResult(nil, stableData)
			assert.True(t, shouldContinue, "收盘前第%d次相同数据应该继续（不检查一致性）", i+1)
			assert.Nil(t, finalErr, "收盘前第%d次不应该有终止错误", i+1)
		}
	})
}