package helpers

import (
	"bufio"
	"encoding/csv"
	"os"
	"sync"
	"sync/atomic"

	"stocksub/pkg/testkit/core"
)

// ResourceManager 资源管理器实现
type ResourceManager struct {
	// 资源池
	csvWriterPool *sync.Pool
	bufferPool    *sync.Pool
	filePool      *sync.Pool

	// 清理函数
	cleanupFuncs []func()
	cleanupMu    sync.Mutex

	// 统计信息
	csvWriterAcquired int64
	csvWriterReleased int64
	bufferAcquired    int64
	bufferReleased    int64

	// 配置
	config ResourceConfig
}

// ResourceConfig 资源管理配置
type ResourceConfig struct {
	BufferSize    int  `yaml:"buffer_size"`    // 缓冲区大小
	MaxBuffers    int  `yaml:"max_buffers"`    // 最大缓冲区数量
	MaxWriters    int  `yaml:"max_writers"`    // 最大写入器数量
	EnablePooling bool `yaml:"enable_pooling"` // 是否启用池化
}

// NewResourceManager 创建资源管理器
func NewResourceManager(config ResourceConfig) *ResourceManager {
	rm := &ResourceManager{
		config:       config,
		cleanupFuncs: make([]func(), 0),
	}

	if config.EnablePooling {
		rm.initPools()
	}

	return rm
}

// initPools 初始化资源池
func (rm *ResourceManager) initPools() {
	// CSV写入器池
	rm.csvWriterPool = &sync.Pool{
		New: func() interface{} {
			buffer := rm.acquireBufferInternal()
			return csv.NewWriter(buffer.(*bufio.Writer))
		},
	}

	// 缓冲区池
	rm.bufferPool = &sync.Pool{
		New: func() interface{} {
			return bufio.NewWriterSize(nil, rm.config.BufferSize)
		},
	}

	// 文件池（用于复用文件句柄，虽然CSV通常不适用）
	rm.filePool = &sync.Pool{
		New: func() interface{} {
			return nil // 文件不能预创建，返回nil
		},
	}
}

// AcquireCSVWriter 获取CSV写入器
func (rm *ResourceManager) AcquireCSVWriter() interface{} {
	atomic.AddInt64(&rm.csvWriterAcquired, 1)

	if !rm.config.EnablePooling || rm.csvWriterPool == nil {
		buffer := rm.AcquireBuffer().(*bufio.Writer)
		return csv.NewWriter(buffer)
	}

	writer := rm.csvWriterPool.Get().(*csv.Writer)
	return writer
}

// ReleaseCSVWriter 释放CSV写入器
func (rm *ResourceManager) ReleaseCSVWriter(writer interface{}) {
	atomic.AddInt64(&rm.csvWriterReleased, 1)

	switch w := writer.(type) {
	case *csv.Writer:
		if !rm.config.EnablePooling || rm.csvWriterPool == nil {
			w.Flush()
			return
		}

		w.Flush()
		rm.csvWriterPool.Put(w)
	}
}

// AcquireBuffer 获取缓冲区
func (rm *ResourceManager) AcquireBuffer() interface{} {
	return rm.acquireBufferInternal()
}

// acquireBufferInternal 内部获取缓冲区方法
func (rm *ResourceManager) acquireBufferInternal() interface{} {
	atomic.AddInt64(&rm.bufferAcquired, 1)

	if !rm.config.EnablePooling || rm.bufferPool == nil {
		return bufio.NewWriterSize(nil, rm.config.BufferSize)
	}

	buffer := rm.bufferPool.Get().(*bufio.Writer)
	buffer.Reset(nil) // 重置缓冲区
	return buffer
}

// ReleaseBuffer 释放缓冲区
func (rm *ResourceManager) ReleaseBuffer(buffer interface{}) {
	atomic.AddInt64(&rm.bufferReleased, 1)

	if !rm.config.EnablePooling || rm.bufferPool == nil {
		if b, ok := buffer.(*bufio.Writer); ok {
			b.Flush()
		}
		return
	}

	if b, ok := buffer.(*bufio.Writer); ok {
		b.Flush()
		rm.bufferPool.Put(b)
	}
}

// RegisterCleanup 注册清理函数
func (rm *ResourceManager) RegisterCleanup(fn func()) {
	rm.cleanupMu.Lock()
	defer rm.cleanupMu.Unlock()

	rm.cleanupFuncs = append(rm.cleanupFuncs, fn)
}

// Cleanup 执行清理
func (rm *ResourceManager) Cleanup() {
	rm.cleanupMu.Lock()
	defer rm.cleanupMu.Unlock()

	for _, fn := range rm.cleanupFuncs {
		if fn != nil {
			fn()
		}
	}

	rm.cleanupFuncs = rm.cleanupFuncs[:0] // 清空但保留容量
}

// GetStats 获取资源使用统计
func (rm *ResourceManager) GetStats() ResourceStats {
	return ResourceStats{
		CSVWriterAcquired: atomic.LoadInt64(&rm.csvWriterAcquired),
		CSVWriterReleased: atomic.LoadInt64(&rm.csvWriterReleased),
		BufferAcquired:    atomic.LoadInt64(&rm.bufferAcquired),
		BufferReleased:    atomic.LoadInt64(&rm.bufferReleased),
		PoolingEnabled:    rm.config.EnablePooling,
		BufferSize:        rm.config.BufferSize,
	}
}

// ResourceStats 资源使用统计
type ResourceStats struct {
	CSVWriterAcquired int64 `json:"csv_writer_acquired"`
	CSVWriterReleased int64 `json:"csv_writer_released"`
	BufferAcquired    int64 `json:"buffer_acquired"`
	BufferReleased    int64 `json:"buffer_released"`
	PoolingEnabled    bool  `json:"pooling_enabled"`
	BufferSize        int   `json:"buffer_size"`
}

// Close 关闭资源管理器
func (rm *ResourceManager) Close() error {
	rm.Cleanup()
	return nil
}

// CSVWriterWrapper CSV写入器包装器，提供自动资源管理
type CSVWriterWrapper struct {
	writer  *csv.Writer
	buffer  *bufio.Writer
	file    *os.File
	manager *ResourceManager
	closed  bool
	mu      sync.Mutex
}

// NewCSVWriterWrapper 创建CSV写入器包装器
func NewCSVWriterWrapper(file *os.File, manager *ResourceManager) *CSVWriterWrapper {
	buffer := manager.AcquireBuffer().(*bufio.Writer)
	buffer.Reset(file)

	writerInterface := manager.AcquireCSVWriter()

	// 获取底层缓冲区写入器，如果writer不是基于我们的buffer创建的
	if csvWriter, ok := writerInterface.(*csv.Writer); ok {
		// 重新创建writer，确保使用我们的buffer
		writer := csv.NewWriter(buffer)
		manager.ReleaseCSVWriter(csvWriter) // 释放之前的writer

		wrapper := &CSVWriterWrapper{
			writer:  writer,
			buffer:  buffer,
			file:    file,
			manager: manager,
		}

		// 注册清理函数
		manager.RegisterCleanup(func() {
			wrapper.Close()
		})

		return wrapper
	}

	// 如果类型断言失败，直接创建新的writer
	writer := csv.NewWriter(buffer)

	wrapper := &CSVWriterWrapper{
		writer:  writer,
		buffer:  buffer,
		file:    file,
		manager: manager,
	}

	// 注册清理函数
	manager.RegisterCleanup(func() {
		wrapper.Close()
	})

	return wrapper
}

// Write 写入记录
func (cw *CSVWriterWrapper) Write(record []string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.closed {
		return core.NewTestKitError(core.ErrResourceClosed, "resource has been closed")
	}

	return cw.writer.Write(record)
}

// WriteAll 写入所有记录
func (cw *CSVWriterWrapper) WriteAll(records [][]string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.closed {
		return core.NewTestKitError(core.ErrResourceClosed, "resource has been closed")
	}

	return cw.writer.WriteAll(records)
}

// Flush 刷新缓冲区
func (cw *CSVWriterWrapper) Flush() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.closed {
		return core.NewTestKitError(core.ErrResourceClosed, "resource has been closed")
	}

	cw.writer.Flush()
	return cw.buffer.Flush()
}

// Close 关闭写入器
func (cw *CSVWriterWrapper) Close() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.closed {
		return nil
	}

	// 刷新数据
	cw.writer.Flush()
	cw.buffer.Flush()

	// 释放资源
	cw.manager.ReleaseCSVWriter(cw.writer)
	cw.manager.ReleaseBuffer(cw.buffer)

	// 关闭文件
	if cw.file != nil {
		cw.file.Close()
	}

	cw.closed = true
	return nil
}

// Error 获取CSV写入器的错误
func (cw *CSVWriterWrapper) Error() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	return cw.writer.Error()
}

// DefaultResourceConfig 默认资源配置
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		BufferSize:    64 * 1024, // 64KB
		MaxBuffers:    100,
		MaxWriters:    50,
		EnablePooling: true,
	}
}

// FileManager 文件管理器
type FileManager struct {
	openFiles map[string]*os.File
	mu        sync.RWMutex
	manager   *ResourceManager
}

// NewFileManager 创建文件管理器
func NewFileManager(manager *ResourceManager) *FileManager {
	return &FileManager{
		openFiles: make(map[string]*os.File),
		manager:   manager,
	}
}

// OpenFile 打开文件
func (fm *FileManager) OpenFile(filename string, flag int, perm os.FileMode) (*os.File, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 检查是否已经打开
	if file, exists := fm.openFiles[filename]; exists {
		return file, nil
	}

	// 打开新文件
	file, err := os.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}

	fm.openFiles[filename] = file

	// 注册清理函数
	fm.manager.RegisterCleanup(func() {
		fm.closeFile(filename)
	})

	return file, nil
}

// closeFile 关闭指定文件
func (fm *FileManager) closeFile(filename string) {
	if file, exists := fm.openFiles[filename]; exists {
		file.Close()
		delete(fm.openFiles, filename)
	}
}

// CloseAll 关闭所有文件
func (fm *FileManager) CloseAll() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	var lastErr error
	for filename, file := range fm.openFiles {
		if err := file.Close(); err != nil {
			lastErr = err
		}
		delete(fm.openFiles, filename)
	}

	return lastErr
}
