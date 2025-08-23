package limiter

import (
	"context"
	"errors"
	"stocksub/pkg/timing"
	"sync"
	"time"
)

// IntelligentLimiter 智能限制器 - 统筹管理整个API批量调用的熔断策略
type IntelligentLimiter struct {
	mu               sync.RWMutex
	classifier       *ErrorClassifier
	marketTime       *timing.MarketTime
	currentBatch     []string     // 当前批次处理的symbols
	retryCount       int          // 重试计数
	lastError        error        // 最后发生的错误
	consecutiveSame  int          // 连续相同数据计数
	lastData         string       // 最后获取的数据指纹
	isInitialized    bool         // 是否已初始化 batch
	
	// 统计信息
	totalRequests    int64
	totalErrors      int64
	lastRequestTime  time.Time
	
	// 安全开关
	forceStopFlag    bool         // 强制停止标志
}

// NewIntelligentLimiter 创建新的智能熔断器
func NewIntelligentLimiter(marketTime *timing.MarketTime) *IntelligentLimiter {
	return &IntelligentLimiter{
		classifier:    NewErrorClassifier(),
		marketTime:    marketTime,
		consecutiveSame: 0,
		lastData:      "",
		currentBatch:  []string{},
		isInitialized: false,
	}
}

// InitializeBatch 初始化批次信息
func (l *IntelligentLimiter) InitializeBatch(symbols []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.currentBatch = symbols
	l.retryCount = 0
	l.lastError = nil
	l.consecutiveSame = 0
	l.lastData = ""
	l.isInitialized = true
	l.forceStopFlag = false
	
	// 预检查时间有效性
	if !l.marketTime.IsTradingTime() {
		l.forceStopFlag = true
		l.lastError = errors.New("当前不在交易时段内，自动停止")
	}
}

// ShouldProceed 判断是否可以继续进行API调用
func (l *IntelligentLimiter) ShouldProceed(ctx context.Context) (bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if !l.isInitialized {
		return false, errors.New("熔断器尚未初始化（请先调用InitializeBatch）")
	}
	
	// 强制停止标志
	if l.forceStopFlag {
		return false, l.lastError
	}
	
	// 交易时间检查
	if !l.marketTime.IsTradingTime() {
		return false, errors.New("交易时段已结束，停止监控")
	}
	
	// 重试次数检查
	if l.retryCount >= MaxRetries && l.lastError != nil {
		return false, errors.New("已达到最大重试次数，停止此次操作")
	}
	
	return true, nil
}

// RecordResult 记录批处理结果，供熔断器判断下一步行动
func (l *IntelligentLimiter) RecordResult(err error, data []string) (
	shouldContinue bool, 
	waitingDuration time.Duration, 
	finalError error) {
	
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.lastRequestTime = time.Now()
	l.totalRequests++
	
	// 成功情况
	if err == nil {
		l.totalErrors = 0
		l.lastError = nil
		
		// 始终记录数据指纹，但只在收盘后才检查一致性
		if len(data) > 0 {
			dataFingerprint := l.generateDataFingerprint(data)
			
			// 收盘后检查数据一致性（避免数据延迟问题）
			if l.marketTime.IsAfterTradingEnd() {
				if dataFingerprint == l.lastData {
					l.consecutiveSame++
					if l.consecutiveSame >= 5 {
						// 数据已稳定5次，可以终止
						return false, 0, errors.New("收盘后数据已稳定，终止收集")
					}
				} else {
					l.consecutiveSame = 1
				}
			} else {
				// 交易时段内，只更新数据不检查一致性
				if dataFingerprint != l.lastData {
					l.consecutiveSame = 1
				} else {
					l.consecutiveSame++
				}
			}
			
			l.lastData = dataFingerprint
		}
		
		return true, 0, nil
	}
	
	// 错误情况处理
	l.totalErrors++
	l.lastError = err
	
	// 错误分级
	level := l.classifier.Classify(err)
	
	switch level {
	case LevelFatal: // 致命级错误
		l.forceStopFlag = true
		return false, 0, errors.New("致命错误: " + err.Error())
		
	case LevelNetwork: // 网络错误，进行重试
		shouldRetry, waitDuration := l.classifier.GetRetryStrategy(level, l.retryCount)
		
		if !shouldRetry {
			return false, 0, errors.New("网络错误重试次数耗尽: " + err.Error())
		}
		
		// 检查重试时间是否在有效范围内
		nextRetryTime := time.Now().Add(waitDuration)
		tradingEnd := l.marketTime.GetTradingEndTime()
		
		if !l.classifier.IsRetryAllowedInTime(nextRetryTime, tradingEnd) {
			return false, 0, errors.New("重试时间超出交易时段，终止操作")
		}
		
		l.retryCount++
		return false, waitDuration, nil
		
	case LevelInvalid, LevelUnknown:
		// 无效参数或未知错误，不重试
		return false, 0, errors.New("不可重试错误: " + err.Error())
		
	default:
		return false, 0, errors.New("未知错误类型: " + err.Error())
	}
}

// generateDataFingerprint 为数据生成简单的指纹标识
func (l *IntelligentLimiter) generateDataFingerprint(data []string) string {
	// 简单的数据指纹生成 - 基于字符串内容
	if len(data) == 0 {
		return "empty"
	}
	
	// 这里使用字符串拼接作为简单指纹
	// 在生产环境中可以改进为更复杂的校验
	fingerprint := ""
	for _, item := range data {
		if len(item) > 5 {
			fingerprint += item[:5] // 取前5个字符
		} else {
			fingerprint += item
		}
	}
	
	if len(fingerprint) > 50 {
		return fingerprint[:50]
	}
	return fingerprint
}

// GetStatus 获取熔断器当前状态
func (l *IntelligentLimiter) GetStatus() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	return map[string]interface{}{
		"is_initialized":      l.isInitialized,
		"current_batch":       len(l.currentBatch),
		"retry_count":         l.retryCount,
		"has_last_error":      l.lastError != nil,
		"consecutive_same":    l.consecutiveSame,
		"force_stop":          l.forceStopFlag,
		"last_request_time":   l.lastRequestTime,
		"in_trading_time":     l.marketTime.IsTradingTime(),
		"is_close_to_end":     l.marketTime.IsCloseToEnd(),
		"is_after_trading_end": l.marketTime.IsAfterTradingEnd(),
		"total_requests":      l.totalRequests,
		"estimated_end":       l.marketTime.GetTradingEndTime(),
	}
}

// IsSafeToContinue 检查是否安全继续
func (l *IntelligentLimiter) IsSafeToContinue() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if !l.isInitialized {
		return false
	}
	
	return !l.forceStopFlag && l.marketTime.IsTradingTime()
}

// Reset 重置熔断器状态（测试用）
func (l *IntelligentLimiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.retryCount = 0
	l.lastError = nil
	l.consecutiveSame = 0
	l.lastData = ""
	l.forceStopFlag = false
	l.totalRequests = 0
	l.totalErrors = 0
}