// Package storage 
// 提供了 testkit 的存储层实现，包括CSV文件存储、内存存储和批量写入等功能。
package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stocksub/pkg/testkit/core"
)

// BatchWriter 封装了底层存储，提供批量写入功能以提高性能。
// 它会将写入操作缓存起来，直到达到预设的批次大小或刷新间隔，再一次性写入。
type BatchWriter struct {
	storage     core.Storage
	buffer      []interface{}
	bufferMu    sync.Mutex
	flushTicker *time.Ticker
	stopChan    chan struct{}
	config      BatchWriterConfig
	stats       BatchWriterStats
}

// BatchWriterConfig 定义了 BatchWriter 的配置选项。
type BatchWriterConfig struct {
	BatchSize     int           `yaml:"batch_size"`      // 触发批量写入的批次大小。
	FlushInterval time.Duration `yaml:"flush_interval"`  // 定期将缓冲区数据写入存储的时间间隔。
	MaxBufferSize int           `yaml:"max_buffer_size"` // 缓冲区中可容纳的最大记录数，防止内存无限增长。
	EnableAsync   bool          `yaml:"enable_async"`    // 是否启用异步写入。如果为true，批量写入将在独立的goroutine中执行。
}

// BatchWriterStats 包含了 BatchWriter 的运行统计信息。
type BatchWriterStats struct {
	TotalBatches    int64     `json:"total_batches"`    // 已成功写入的总批次数。
	TotalRecords    int64     `json:"total_records"`    // 已成功写入的总记录数。
	BufferSize      int       `json:"buffer_size"`      // 当前缓冲区中的记录数。
	LastFlush       time.Time `json:"last_flush"`       // 最后一次成功刷新的时间。
	FlushErrors     int64     `json:"flush_errors"`     // 刷新（写入）失败的次数。
	BufferOverflows int64     `json:"buffer_overflows"` // 因缓冲区满而导致强制刷新的次数。
}

// NewBatchWriter 创建一个新的 BatchWriter 实例。
func NewBatchWriter(storage core.Storage, config BatchWriterConfig) *BatchWriter {
	bw := &BatchWriter{
		storage:  storage,
		buffer:   make([]interface{}, 0, config.BatchSize),
		stopChan: make(chan struct{}),
		config:   config,
		stats:    BatchWriterStats{},
	}

	if config.FlushInterval > 0 {
		bw.flushTicker = time.NewTicker(config.FlushInterval)
		go bw.startPeriodicFlush()
	}

	return bw
}

// Write 将一条数据记录添加到写入缓冲区。
// 当缓冲区大小达到 BatchSize 时，它会触发一次批量写入操作。
func (bw *BatchWriter) Write(ctx context.Context, data interface{}) error {
	bw.bufferMu.Lock()
	defer bw.bufferMu.Unlock()

	if len(bw.buffer) >= bw.config.MaxBufferSize {
		bw.stats.BufferOverflows++
		if err := bw.flushBuffer(ctx); err != nil {
			return fmt.Errorf("强制刷新缓冲区失败: %w", err)
		}
	}

	bw.buffer = append(bw.buffer, data)

	if len(bw.buffer) >= bw.config.BatchSize {
		if bw.config.EnableAsync {
			go bw.asyncFlush(context.Background()) // 使用后台context
		} else {
			return bw.flushBuffer(ctx)
		}
	}

	return nil
}

// Flush 手动触发一次将缓冲区所有数据写入底层存储的操作。
func (bw *BatchWriter) Flush() error {
	bw.bufferMu.Lock()
	defer bw.bufferMu.Unlock()

	return bw.flushBuffer(context.Background())
}

// flushBuffer 是实际的刷新操作实现（需要在锁内调用）。
func (bw *BatchWriter) flushBuffer(ctx context.Context) error {
	if len(bw.buffer) == 0 {
		return nil
	}

	dataToFlush := make([]interface{}, len(bw.buffer))
	copy(dataToFlush, bw.buffer)
	bw.buffer = bw.buffer[:0]

	if batchSaver, ok := bw.storage.(interface{
		BatchSave(context.Context, []interface{}) error
	}); ok {
		if err := batchSaver.BatchSave(ctx, dataToFlush); err != nil {
			bw.stats.FlushErrors++
			return err
		}
	} else {
		for _, item := range dataToFlush {
			if err := bw.storage.Save(ctx, item); err != nil {
				bw.stats.FlushErrors++
				fmt.Printf("BatchWriter fallback save error: %v\n", err)
			}
		}
	}

	bw.stats.TotalBatches++
	bw.stats.TotalRecords += int64(len(dataToFlush))
	bw.stats.LastFlush = time.Now()

	return nil
}

// asyncFlush 异步执行刷新操作。
func (bw *BatchWriter) asyncFlush(ctx context.Context) {
	go func() {
		bw.bufferMu.Lock()
		defer bw.bufferMu.Unlock()
		bw.flushBuffer(ctx)
	}()
}

// startPeriodicFlush 启动一个 goroutine，按固定的时间间隔刷新缓冲区。
func (bw *BatchWriter) startPeriodicFlush() {
	for {
		select {
		case <-bw.flushTicker.C:
			bw.Flush()
		case <-bw.stopChan:
			return
		}
	}
}

// Close 优雅地关闭 BatchWriter，它会先刷新所有剩余在缓冲区中的数据，然后停止后台任务。
func (bw *BatchWriter) Close() error {
	if bw.flushTicker != nil {
		bw.flushTicker.Stop()
	}
	close(bw.stopChan)

	return bw.Flush()
}

// GetStats 返回当前的运行统计信息。
func (bw *BatchWriter) GetStats() BatchWriterStats {
	bw.bufferMu.Lock()
	defer bw.bufferMu.Unlock()

	stats := bw.stats
	stats.BufferSize = len(bw.buffer)
	return stats
}

// DefaultBatchWriterConfig 返回一个默认的 BatchWriter 配置实例。
func DefaultBatchWriterConfig() BatchWriterConfig {
	return BatchWriterConfig{
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
		MaxBufferSize: 1000,
		EnableAsync:   true,
	}
}
