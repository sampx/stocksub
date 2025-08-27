package main

import (
	"context"
	"fmt"
	"log"
	"stocksub/pkg/provider/core"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/provider/tencent"
	"time"
)

// 这个文件演示了如何在 api_monitor 中使用新的装饰器架构
// 保持现有功能不变的同时，获得装饰器的所有优势

// EnhancedAPIMonitor 增强版API监控器，使用装饰器架构
type EnhancedAPIMonitor struct {
	config            MonitorConfig
	decoratedProvider core.RealtimeStockProvider // 使用装饰器封装的提供商
	originalProvider  *tencent.Provider          // 保留原始提供商引用（用于兼容性）
	logger            *log.Logger
}

// NewEnhancedAPIMonitor 创建使用装饰器的增强版监控器
func NewEnhancedAPIMonitor(config MonitorConfig) (*EnhancedAPIMonitor, error) {
	// 创建基础的腾讯提供商
	baseProvider := tencent.NewProvider()
	baseProvider.SetTimeout(30 * time.Second)

	// 根据监控场景选择合适的装饰器配置
	decoratorConfig := decorators.MonitoringDecoratorConfig()

	// 使用装饰器工厂创建装饰后的提供商
	decoratedProvider, err := decorators.CreateDecoratedProvider(baseProvider, decoratorConfig)
	if err != nil {
		return nil, fmt.Errorf("创建装饰提供商失败: %w", err)
	}

	monitor := &EnhancedAPIMonitor{
		config:            config,
		decoratedProvider: decoratedProvider,
		originalProvider:  baseProvider,
		logger:            log.New(log.Writer(), "[ENHANCED-MONITOR] ", log.LstdFlags),
	}

	// 输出装饰器链信息
	if chain, ok := decoratedProvider.(*decorators.CircuitBreakerProvider); ok {
		status := chain.GetStatus()
		monitor.logger.Printf("使用装饰器: %v", status["decorator_type"])

		// 检查是否还有更深层的装饰器
		if freqControl, ok := chain.GetBaseProvider().(*decorators.FrequencyControlProvider); ok {
			freqStatus := freqControl.GetStatus()
			monitor.logger.Printf("频率控制装饰器: 间隔=%v, 重试=%v",
				freqStatus["min_interval"], freqStatus["max_retries"])
		}
	}

	return monitor, nil
}

// RunWithDecorators 使用装饰器架构运行监控
func (m *EnhancedAPIMonitor) RunWithDecorators(ctx context.Context) error {
	m.logger.Printf("开始增强版监控，使用装饰器架构")

	successCount := 0
	errorCount := 0

	for i := 0; i < 10; i++ { // 演示10次调用
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		start := time.Now()

		// 直接使用装饰后的提供商，所有智能限流、熔断等逻辑自动处理
		data, err := m.decoratedProvider.FetchStockData(ctx, m.config.Symbols)

		duration := time.Since(start)

		if err != nil {
			errorCount++
			m.logger.Printf("第%d轮采集失败: %v (耗时: %v)", i+1, err, duration)
		} else {
			successCount += len(data)
			m.logger.Printf("第%d轮采集成功: %d股票 (耗时: %v)", i+1, len(data), duration)
		}

		// 获取装饰器状态（用于监控和调试）
		if statusProvider, ok := m.decoratedProvider.(interface{ GetStatus() map[string]interface{} }); ok {
			status := statusProvider.GetStatus()
			m.logger.Printf("装饰器状态: 健康=%v", status["enabled"])
		}

		// 等待间隔
		time.Sleep(m.config.Interval)
	}

	successRate := float64(successCount) / float64(10*len(m.config.Symbols)) * 100
	m.logger.Printf("监控完成: 成功率 %.1f%%", successRate)

	return nil
}

// DemonstrateCompatibility 演示新架构与旧代码的兼容性
func DemonstrateCompatibility() {
	fmt.Println("=== api_monitor 装饰器兼容性演示 ===")

	// 创建测试配置
	config := MonitorConfig{
		Symbols:  []string{"600000", "000001"},
		Duration: 30 * time.Second,
		Interval: 3 * time.Second,
	}

	// 创建增强版监控器
	monitor, err := NewEnhancedAPIMonitor(config)
	if err != nil {
		fmt.Printf("创建增强版监控器失败: %v\n", err)
		return
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 运行演示
	if err := monitor.RunWithDecorators(ctx); err != nil {
		fmt.Printf("运行演示失败: %v\n", err)
		return
	}

	fmt.Println("=== 兼容性验证完成 ===")
}

// 注意: MonitoringDecoratorConfig 现在定义在 pkg/provider/decorators/configurable_chain.go 中
