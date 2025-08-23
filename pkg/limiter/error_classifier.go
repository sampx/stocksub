package limiter

import (
	"strings"
	"time"
)

// ErrorLevel 定义错误的严重级别
type ErrorLevel int

const (
	LevelFatal ErrorLevel = iota   // 致命级，立即终止
	LevelNetwork                   // 网络错误，可重试
	LevelInvalid                   // 无效参数，可忽略或特殊处理
	LevelUnknown                   // 未知错误
)

const (
	MaxRetries   = 3               // 最大重试次数
	RetryBase1   = 1 * time.Minute  // 第一次重试等待时间
	RetryBase2   = 3 * time.Minute  // 第二次重试等待时间
	RetryBase3   = 5 * time.Minute  // 第三次重试等待时间
)

// ErrorClassifier 负责根据错误类型进行分类
type ErrorClassifier struct {
	// 可以扩展添加自定义规则
}

// NewErrorClassifier 创建新的错误分类器
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{}
}

// Classify 根据错误内容分类错误级别
func (c *ErrorClassifier) Classify(err error) ErrorLevel {
	if err == nil {
		return LevelUnknown
	}
	
	msg := strings.ToLower(err.Error())
	
	// 致命级错误 - 立即终止
	switch {
	case strings.Contains(msg, "connection refused"):
		return LevelFatal
	case strings.Contains(msg, "connection reset") && (!strings.Contains(msg, "read tcp") && !strings.Contains(msg, "write tcp")):
		return LevelFatal  // 只有直接连接重置才是致命错误，TCP读写重置是网络错误
	case strings.Contains(msg, "nosuchhost"),
		 strings.Contains(msg, "dial tcp"),
		 strings.Contains(msg, "dial udp"):
		return LevelFatal
	case strings.Contains(msg, "forbidden") && 
		 strings.Contains(msg, "403"):
		return LevelFatal
	}
	
	// 网络错误 - 可重试
	switch {
	case strings.Contains(msg, "timeout"):
		return LevelNetwork
	case strings.Contains(msg, "network is unreachable"):
		return LevelNetwork
	case strings.Contains(msg, "temporary failure"):
		return LevelNetwork
	case strings.Contains(msg, "read tcp") && strings.Contains(msg, "connection reset"):
		return LevelNetwork  // 读TCP连接重置也算网络错误，而不是致命错误
	case strings.Contains(msg, "write tcp"):
		return LevelNetwork
	}
	
	// 无效参数 - 通常可忽略
	switch {
	case strings.Contains(msg, "invalid argument"):
		return LevelInvalid
	case strings.Contains(msg, "bad request"):
		return LevelInvalid
	case strings.Contains(msg, "not found") && strings.Contains(msg, "404"):
		return LevelInvalid
	}
	
	// 其他错误归类为未知
	return LevelUnknown
}

// GetRetryStrategy 根据错误级别提供重试策略
func (c *ErrorClassifier) GetRetryStrategy(level ErrorLevel, attempt int) (shouldRetry bool, waitDuration time.Duration) {
	switch level {
	case LevelFatal:
		// 致命级错误，不尝试重试
		return false, 0
		
	case LevelNetwork:
		if attempt >= MaxRetries {
			return false, 0
		}
		
		// 根据尝试次数返回递增的等待时间
		switch attempt {
		case 0:
			return true, RetryBase1
		case 1:
			return true, RetryBase2
		case 2:
			return true, RetryBase3
		default:
			return true, RetryBase3
		}
		
	case LevelInvalid, LevelUnknown:
		// 无效参数或未知错误，不重试
		return false, 0
		
	default:
		return false, 0
	}
}

// IsRetryAllowedInTime 检查重试是否在有效时间内
func (c *ErrorClassifier) IsRetryAllowedInTime(nextRetryTime time.Time, tradingEnd time.Time) bool {
	// 重试时间必须在交易结束前
	buffer := 30 * time.Second // 30秒缓冲时间
	return nextRetryTime.Before(tradingEnd.Add(-buffer))
}

// GetRetryMessage 获取重试提示信息
func (c *ErrorClassifier) GetRetryMessage(level ErrorLevel, attempt int, nextWait time.Duration) string {
	switch level {
	case LevelFatal:
		return "致命错误，立即终止操作"
	case LevelNetwork:
		if attempt >= MaxRetries {
			return "网络错误已达到最大重试次数，终止此次操作"
		}
		return "网络错误，等待重试..."
	case LevelInvalid:
		return "参数无效，跳过重试"
	case LevelUnknown:
		return "未知错误，跳过重试"
	default:
		return "错误处理中..."
	}
}