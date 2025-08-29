package cache

import (
	"stocksub/pkg/error"
)

type CacheError struct {
	error.BaseError
}

const (
	// ErrCacheTimeout 表示缓存操作超时。
	ErrCacheTimeout error.ErrorCode = "CACHE_TIMEOUT"
	// ErrCacheMiss 表示在缓存中未找到请求的条目。
	ErrCacheMiss error.ErrorCode = "CACHE_MISS"
	// ErrCacheFull 表示缓存已满，无法添加新条目。
	ErrCacheFull error.ErrorCode = "CACHE_FULL"
	// ErrCacheCorrupted 表示缓存数据已损坏。
	ErrCacheCorrupted error.ErrorCode = "CACHE_CORRUPTED"
)

var (
	ErrCacheMissNotFound    = NewCacheError(ErrCacheMiss, "cache entry not found")
	ErrCacheTimeoutExceeded = NewCacheError(ErrCacheTimeout, "cache operation timeout")
)

func NewCacheError(code error.ErrorCode, message string) *CacheError {
	return &CacheError{
		BaseError: *error.NewError(code, message),
	}
}
