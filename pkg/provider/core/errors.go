package core

import "errors"

// 定义核心错误
var (
	// ErrProviderNotFound 提供商未找到错误
	ErrProviderNotFound = errors.New("provider not found")
	
	// ErrProviderNotSupported 不支持的提供商类型错误
	ErrProviderNotSupported = errors.New("provider type not supported")
	
	// ErrProviderNotHealthy 提供商不健康错误
	ErrProviderNotHealthy = errors.New("provider is not healthy")
	
	// ErrInvalidSymbol 无效的股票代码错误
	ErrInvalidSymbol = errors.New("invalid symbol")
	
	// ErrEmptySymbols 股票代码列表为空错误
	ErrEmptySymbols = errors.New("symbols list is empty")
	
	// ErrContextCanceled 上下文被取消错误
	ErrContextCanceled = errors.New("context canceled")
	
	// ErrTimeout 请求超时错误
	ErrTimeout = errors.New("request timeout")
	
	// ErrRateLimitExceeded 频率限制超出错误
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	
	// ErrProviderClosed 提供商已关闭错误
	ErrProviderClosed = errors.New("provider is closed")
)
