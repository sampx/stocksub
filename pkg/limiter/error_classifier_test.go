package limiter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorLevel
	}{
		// 致命级错误测试
		{"连接拒绝", errors.New("dial tcp: connection refused"), LevelFatal},
		{"连接重置", errors.New("read: connection reset by peer"), LevelFatal},
		{"主机未找到", errors.New("dial tcp: lookup host: Nosuchhost"), LevelFatal},
		{"403禁止", errors.New("HTTP/1.1 403 Forbidden"), LevelFatal},

		// 网络错误测试
		{"超时", errors.New("i/o timeout"), LevelNetwork},
		{"网络不可达", errors.New("network is unreachable"), LevelNetwork},
		{"暂时失败", errors.New("temporary failure in name resolution"), LevelNetwork},
		{"读TCP失败", errors.New("read tcp: connection reset by peer"), LevelNetwork},
		{"写TCP失败", errors.New("write tcp: broken pipe"), LevelNetwork},

		// 无效参数测试
		{"无效参数", errors.New("invalid argument"), LevelInvalid},
		{"请求错误", errors.New("HTTP/1.1 400 Bad Request"), LevelInvalid},
		{"未找到", errors.New("HTTP/1.1 404 Not Found"), LevelInvalid},

		// 未知错误测试
		{"nil错误", nil, LevelUnknown},
		{"其他错误", errors.New("some other error"), LevelUnknown},
	}

	classifier := NewErrorClassifier()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := classifier.Classify(tt.err)
			assert.Equal(t, tt.expected, actual, "错误分类应匹配预期: %s", tt.name)
		})
	}
}

func TestRetryStrategy(t *testing.T) {
	tests := []struct {
		name                string
		level               ErrorLevel
		attempt             int
		expectedShouldRetry bool
		expectedWait        time.Duration
		expectedMessage     string
	}{
		// 致命级错误 - 不通知重试
		{"致命错误-0次", LevelFatal, 0, false, 0, "致命错误，立即终止操作"},
		{"致命错误-1次", LevelFatal, 1, false, 0, "致命错误，立即终止操作"},

		// 网络错误的可重试次数测试
		{"网络错误-0次", LevelNetwork, 0, true, RetryBase1, "网络错误，等待重试..."},
		{"网络错误-1次", LevelNetwork, 1, true, RetryBase2, "网络错误，等待重试..."},
		{"网络错误-2次", LevelNetwork, 2, true, RetryBase3, "网络错误，等待重试..."},
		{"网络错误-3次", LevelNetwork, 3, false, 0, "网络错误已达到最大重试次数，终止此次操作"},

		// 无效参数 - 不通知重试
		{"无效参数-0次", LevelInvalid, 0, false, 0, "参数无效，跳过重试"},
		{"无效参数-1次", LevelInvalid, 1, false, 0, "参数无效，跳过重试"},

		// 未知错误 - 不通知重试
		{"未知错误-0次", LevelUnknown, 0, false, 0, "未知错误，跳过重试"},
		{"未知错误-1次", LevelUnknown, 1, false, 0, "未知错误，跳过重试"},
	}

	classifier := NewErrorClassifier()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRetry, waitDuration := classifier.GetRetryStrategy(tt.level, tt.attempt)
			message := classifier.GetRetryMessage(tt.level, tt.attempt, waitDuration)

			assert.Equal(t, tt.expectedShouldRetry, shouldRetry, "%s 的重试判定应匹配预期", tt.name)
			assert.Equal(t, tt.expectedWait, waitDuration, "%s 的重试等待时间应匹配预期", tt.name)
			assert.Equal(t, tt.expectedMessage, message, "%s 的重试消息应匹配预期", tt.name)
		})
	}
}

func TestTimeBoundaryChecking(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	classifier := NewErrorClassifier()

	tests := []struct {
		name       string
		nextRetry  string
		tradingEnd time.Time
		expected   bool
	}{
		// 重试时间在交易结束前30秒之内 - 允许
		{"允许重试-14:30:00", "14:30:00", time.Date(2025, 8, 21, 15, 0, 10, 0, location), true},
		{"允许重试-14:59:20", "14:59:20", time.Date(2025, 8, 21, 15, 0, 10, 0, location), true},

		// 重试时间在交易结束前30秒之内 - 禁止
		{"禁止重试-14:59:40", "14:59:40", time.Date(2025, 8, 21, 15, 0, 10, 0, location), false},
		{"禁止重试-15:00:00", "15:00:00", time.Date(2025, 8, 21, 15, 0, 10, 0, location), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextRetryTime, _ := time.ParseInLocation("15:04:05", tt.nextRetry, location)
			fullNextRetryTime := time.Date(2025, 8, 21, nextRetryTime.Hour(), nextRetryTime.Minute(), nextRetryTime.Second(), 0, location)

			actual := classifier.IsRetryAllowedInTime(fullNextRetryTime, tt.tradingEnd)
			assert.Equal(t, tt.expected, actual, "时间边界检查应匹配预期")
		})
	}
}

func TestSimplifiedTimeBoundaryChecking(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	classifier := NewErrorClassifier()

	tradingEnd := time.Date(2025, 8, 21, 15, 0, 10, 0, location)

	// 边界测试
	justAllowed := time.Date(2025, 8, 21, 14, 29, 40, 0, location)
	actual := classifier.IsRetryAllowedInTime(justAllowed, tradingEnd)
	assert.True(t, actual, "提前30秒应该允许重试")

	// 超过边界
	forbidden := time.Date(2025, 8, 21, 14, 59, 40, 0, location)
	actual = classifier.IsRetryAllowedInTime(forbidden, tradingEnd)
	assert.False(t, actual, "30秒缓冲区内应该禁止重试")
}
