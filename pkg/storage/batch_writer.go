package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BatchWriter 封装了底层存储，提供批量写入功能以提高性能。
// 它会将写入操作缓存起来，直到达到预设的批次大小或刷新间隔，再一次性写入。
type BatchWriter struct {
	storage     Storage
	buffer      []interface{}
	bufferMu    sync.Mutex
	flushTicker *time.Ticker
	stopChan    chan struct{}
	config      BatchWriterConfig
	stats       BatchWriterStats
	// StructuredData 优化相关字段
	structuredDataBuffer map[string][]*StructuredData // 按 schema 名称分组的缓存
	lastSchemaFlush      map[string]time.Time         // 每个 schema 的上次刷新时间
}

// BatchWriterConfig 定义了 BatchWriter 的配置选项。
type BatchWriterConfig struct {
	BatchSize                 int           `yaml:"batch_size"`                   // 触发批量写入的批次大小。
	FlushInterval             time.Duration `yaml:"flush_interval"`               // 定期将缓冲区数据写入存储的时间间隔。
	MaxBufferSize             int           `yaml:"max_buffer_size"`              // 缓冲区中可容纳的最大记录数，防止内存无限增长。
	EnableAsync               bool          `yaml:"enable_async"`                 // 是否启用异步写入。如果为true，批量写入将在独立的goroutine中执行。
	EnableStructuredDataOptim bool          `yaml:"enable_structured_data_optim"` // 是否启用 StructuredData 优化
	StructuredDataBatchSize   int           `yaml:"structured_data_batch_size"`   // StructuredData 的特别批次大小
	StructuredDataFlushDelay  time.Duration `yaml:"structured_data_flush_delay"`  // StructuredData 刷新延迟（用于合并同类型数据）
}

// BatchWriterStats 包含了 BatchWriter 的运行统计信息。
type BatchWriterStats struct {
	TotalBatches             int64     `json:"total_batches"`               // 已成功写入的总批次数。
	TotalRecords             int64     `json:"total_records"`               // 已成功写入的总记录数。
	BufferSize               int       `json:"buffer_size"`                 // 当前缓冲区中的记录数。
	LastFlush                time.Time `json:"last_flush"`                  // 最后一次成功刷新的时间。
	FlushErrors              int64     `json:"flush_errors"`                // 刷新（写入）失败的次数。
	BufferOverflows          int64     `json:"buffer_overflows"`            // 因缓冲区满而导致强制刷新的次数。
	StructuredDataBatches    int64     `json:"structured_data_batches"`     // StructuredData 的批次数
	StructuredDataRecords    int64     `json:"structured_data_records"`     // StructuredData 的记录数
	StructuredDataBufferSize int       `json:"structured_data_buffer_size"` // StructuredData 缓冲区大小
	StructuredDataFlushes    int64     `json:"structured_data_flushes"`     // StructuredData 专用刷新次数
}

// NewBatchWriter 创建一个新的 BatchWriter 实例。
func NewBatchWriter(storage Storage, config BatchWriterConfig) *BatchWriter {
	bw := &BatchWriter{
		storage:              storage,
		buffer:               make([]interface{}, 0, config.BatchSize),
		stopChan:             make(chan struct{}),
		config:               config,
		stats:                BatchWriterStats{},
		structuredDataBuffer: make(map[string][]*StructuredData),
		lastSchemaFlush:      make(map[string]time.Time),
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

	// 对 StructuredData 进行特殊处理
	if bw.config.EnableStructuredDataOptim {
		if structData, ok := data.(*StructuredData); ok {
			return bw.writeStructuredData(ctx, structData)
		}
	}

	// 常规数据处理
	return bw.writeRegularData(ctx, data)
}

// writeStructuredData 将 StructuredData 添加到专用缓冲区
func (bw *BatchWriter) writeStructuredData(ctx context.Context, data *StructuredData) error {
	if data.Schema == nil {
		return fmt.Errorf("StructuredData 缺少 schema 定义")
	}

	schemaName := data.Schema.Name
	if _, exists := bw.structuredDataBuffer[schemaName]; !exists {
		bw.structuredDataBuffer[schemaName] = make([]*StructuredData, 0)
		bw.lastSchemaFlush[schemaName] = time.Now()
	}

	bw.structuredDataBuffer[schemaName] = append(bw.structuredDataBuffer[schemaName], data)

	// 检查是否需要刷新该 schema 的数据
	schemaBuffer := bw.structuredDataBuffer[schemaName]
	if len(schemaBuffer) >= bw.config.StructuredDataBatchSize {
		return bw.flushStructuredDataSchema(ctx, schemaName)
	}

	// 检查延迟刷新
	if time.Since(bw.lastSchemaFlush[schemaName]) > bw.config.StructuredDataFlushDelay {
		return bw.flushStructuredDataSchema(ctx, schemaName)
	}

	return nil
}

// writeRegularData 处理常规数据
func (bw *BatchWriter) writeRegularData(ctx context.Context, data interface{}) error {
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

	// 刷新常规数据
	if err := bw.flushBuffer(context.Background()); err != nil {
		return err
	}

	// 刷新所有 StructuredData 数据
	if bw.config.EnableStructuredDataOptim {
		for schemaName := range bw.structuredDataBuffer {
			if err := bw.flushStructuredDataSchema(context.Background(), schemaName); err != nil {
				return err
			}
		}
	}

	return nil
}

// flushStructuredDataSchema 刷新指定 schema 的 StructuredData
func (bw *BatchWriter) flushStructuredDataSchema(ctx context.Context, schemaName string) error {
	schemaBuffer, exists := bw.structuredDataBuffer[schemaName]
	if !exists || len(schemaBuffer) == 0 {
		return nil
	}

	// 复制数据并清空缓冲区
	dataToFlush := make([]*StructuredData, len(schemaBuffer))
	copy(dataToFlush, schemaBuffer)
	bw.structuredDataBuffer[schemaName] = bw.structuredDataBuffer[schemaName][:0]
	bw.lastSchemaFlush[schemaName] = time.Now()

	// 转换为 interface{} 类型的切片
	interfaceData := make([]interface{}, len(dataToFlush))
	for i, data := range dataToFlush {
		interfaceData[i] = data
	}

	// 尝试使用 BatchSave
	if batchSaver, ok := bw.storage.(interface {
		BatchSave(context.Context, []interface{}) error
	}); ok {
		if err := batchSaver.BatchSave(ctx, interfaceData); err != nil {
			bw.stats.FlushErrors++
			return err
		}
	} else {
		// 回退到逐个保存
		for _, data := range dataToFlush {
			if err := bw.storage.Save(ctx, data); err != nil {
				bw.stats.FlushErrors++
				fmt.Printf("BatchWriter StructuredData save error: %v\n", err)
			}
		}
	}

	// 更新统计
	bw.stats.StructuredDataBatches++
	bw.stats.StructuredDataRecords += int64(len(dataToFlush))
	bw.stats.StructuredDataFlushes++
	bw.stats.TotalRecords += int64(len(dataToFlush))
	bw.stats.LastFlush = time.Now()

	return nil
}

// flushBuffer 是实际的刷新操作实现（需要在锁内调用）。
func (bw *BatchWriter) flushBuffer(ctx context.Context) error {
	if len(bw.buffer) == 0 {
		return nil
	}

	dataToFlush := make([]interface{}, len(bw.buffer))
	copy(dataToFlush, bw.buffer)
	bw.buffer = bw.buffer[:0]

	if batchSaver, ok := bw.storage.(interface {
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

	// 计算 StructuredData 缓冲区大小
	if bw.config.EnableStructuredDataOptim {
		structuredDataBufferSize := 0
		for _, schemaBuffer := range bw.structuredDataBuffer {
			structuredDataBufferSize += len(schemaBuffer)
		}
		stats.StructuredDataBufferSize = structuredDataBufferSize
	}

	return stats
}

// DefaultBatchWriterConfig 返回一个默认的 BatchWriter 配置实例。
func DefaultBatchWriterConfig() BatchWriterConfig {
	return BatchWriterConfig{
		BatchSize:                 100,
		FlushInterval:             5 * time.Second,
		MaxBufferSize:             1000,
		EnableAsync:               true,
		EnableStructuredDataOptim: true,
		StructuredDataBatchSize:   50,              // 更小的批次大小用于更频繁的刷新
		StructuredDataFlushDelay:  2 * time.Second, // 更短的延迟时间
	}
}

// OptimizedBatchWriterConfig 返回一个针对 StructuredData 优化的配置
func OptimizedBatchWriterConfig() BatchWriterConfig {
	return BatchWriterConfig{
		BatchSize:                 200,
		FlushInterval:             3 * time.Second,
		MaxBufferSize:             2000,
		EnableAsync:               true,
		EnableStructuredDataOptim: true,
		StructuredDataBatchSize:   100,
		StructuredDataFlushDelay:  1 * time.Second,
	}
}
