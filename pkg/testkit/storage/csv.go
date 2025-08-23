package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/helpers"
)

// CSVStorage 实现了 core.Storage 接口，提供了将测试数据以CSV格式持久化到磁盘的功能。
// 它支持按日期和类型自动分割文件，并利用资源池来提高性能。
type CSVStorage struct {
	config      CSVStorageConfig
	resourceMgr *helpers.ResourceManager
	fileMgr     *helpers.FileManager
	writerCache map[string]*helpers.CSVWriterWrapper
	mu          sync.RWMutex
	serializer  core.Serializer
	stats       CSVStorageStats
}

// CSVStorageConfig 定义了 CSVStorage 的所有可配置选项。
type CSVStorageConfig struct {
	Directory      string                 `yaml:"directory"`      // CSV文件的存储目录。
	FilePrefix     string                 `yaml:"file_prefix"`     // CSV文件名的前缀。
	DateFormat     string                 `yaml:"date_format"`     // 用于生成每日文件名的时间格式。
	MaxFileSize    int64                  `yaml:"max_file_size"`   // 单个CSV文件的最大大小（字节）。
	RotateInterval time.Duration          `yaml:"rotate_interval"` // 文件轮转的时间间隔。
	EnableCompress bool                   `yaml:"enable_compress"` // 是否对归档的CSV文件启用压缩。
	BatchSize      int                    `yaml:"batch_size"`      // 批量写入的批次大小。
	FlushInterval  time.Duration          `yaml:"flush_interval"`  // 定期将缓冲区数据刷新到磁盘的间隔。
	ResourceConfig helpers.ResourceConfig `yaml:"resource_config"` // 底层资源管理器（如缓冲区、写入器）的配置。
}

// CSVStorageStats 包含了 CSVStorage 的运行统计信息。
type CSVStorageStats struct {
	TotalRecords  int64                 `json:"total_records"`  // 已写入的总记录数。
	TotalFiles    int64                 `json:"total_files"`    // 当前管理的总文件数。
	TotalSize     int64                 `json:"total_size"`     // 所有文件的总大小（字节）。
	WriteErrors   int64                 `json:"write_errors"`   // 写入失败的次数。
	BatchWrites   int64                 `json:"batch_writes"`   // 完成的批量写入操作次数。
	ResourceStats helpers.ResourceStats `json:"resource_stats"` // 底层资源的统计信息。
	LastWrite     time.Time             `json:"last_write"`     // 最后一次写入操作的时间。
	LastFlush     time.Time             `json:"last_flush"`     // 最后一次刷新到磁盘的时间。
}

// NewCSVStorage 创建并返回一个新的 CSVStorage 实例。
func NewCSVStorage(config CSVStorageConfig) (*CSVStorage, error) {
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	resourceMgr := helpers.NewResourceManager(config.ResourceConfig)
	fileMgr := helpers.NewFileManager(resourceMgr)

	storage := &CSVStorage{
		config:      config,
		resourceMgr: resourceMgr,
		fileMgr:     fileMgr,
		writerCache: make(map[string]*helpers.CSVWriterWrapper),
		serializer:  NewJSONSerializer(),
		stats:       CSVStorageStats{},
	}

	if config.FlushInterval > 0 {
		go storage.startPeriodicFlush()
	}

	return storage, nil
}

// Save 将一条数据记录保存到对应的CSV文件中。
func (cs *CSVStorage) Save(ctx context.Context, data interface{}) error {
	record, err := cs.convertToRecord(data)
	if err != nil {
		cs.stats.WriteErrors++
		return fmt.Errorf("数据转换失败: %w", err)
	}

	writer, err := cs.getOrCreateWriter(record.Type, record.Date)
	if err != nil {
		cs.stats.WriteErrors++
		return fmt.Errorf("获取写入器失败: %w", err)
	}

	if err := writer.Write(record.Fields); err != nil {
		cs.stats.WriteErrors++
		return fmt.Errorf("写入记录失败: %w", err)
	}

	cs.stats.TotalRecords++
	cs.stats.LastWrite = time.Now()

	return nil
}

// BatchSave 将多条数据记录批量保存到对应的CSV文件中，以提高性能。
func (cs *CSVStorage) BatchSave(ctx context.Context, dataList []interface{}) error {
	if len(dataList) == 0 {
		return nil
	}

	groups := make(map[string][][]string)

	for _, data := range dataList {
		record, err := cs.convertToRecord(data)
		if err != nil {
			cs.stats.WriteErrors++
			continue
		}

		key := fmt.Sprintf("%s_%s", record.Type, record.Date)
		groups[key] = append(groups[key], record.Fields)
	}

	for key, records := range groups {
		parts := strings.SplitN(key, "_", 2)
		if len(parts) != 2 {
			continue
		}

		writer, err := cs.getOrCreateWriter(parts[0], parts[1])
		if err != nil {
			cs.stats.WriteErrors++
			continue
		}

		if err := writer.WriteAll(records); err != nil {
			cs.stats.WriteErrors++
			continue
		}

		cs.stats.TotalRecords += int64(len(records))
	}

	cs.stats.BatchWrites++
	cs.stats.LastWrite = time.Now()

	return nil
}

// Load 根据查询条件从CSV文件加载数据。注意：此功能当前尚未实现。
func (cs *CSVStorage) Load(ctx context.Context, query core.Query) ([]interface{}, error) {
	return nil, fmt.Errorf("CSV加载功能待实现")
}

// Delete 根据查询条件删除CSV文件中的数据。注意：此功能当前尚未实现。
func (cs *CSVStorage) Delete(ctx context.Context, query core.Query) error {
	return fmt.Errorf("CSV删除功能待实现")
}

// Close 关闭所有打开的CSV文件和写入器，并释放相关资源。
func (cs *CSVStorage) Close() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, writer := range cs.writerCache {
		writer.Close()
	}
	cs.writerCache = make(map[string]*helpers.CSVWriterWrapper)

	cs.fileMgr.CloseAll()

	return cs.resourceMgr.Close()
}

// Flush 将所有内部缓冲区的数据刷新到底层的CSV文件。
func (cs *CSVStorage) Flush() error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, writer := range cs.writerCache {
		if err := writer.Flush(); err != nil {
			return err
		}
	}

	cs.stats.LastFlush = time.Now()
	return nil
}

// GetStats 返回当前存储实例的运行统计信息。
func (cs *CSVStorage) GetStats() CSVStorageStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	stats := cs.stats
	stats.ResourceStats = cs.resourceMgr.GetStats()
	stats.TotalFiles = int64(len(cs.writerCache))

	return stats
}

// convertToRecord 将任意数据转换为内部的 core.Record 格式，以便于存储。
func (cs *CSVStorage) convertToRecord(data interface{}) (*core.Record, error) {
	record := &core.Record{
		Timestamp: time.Now(),
		Date:      time.Now().Format(cs.config.DateFormat),
		Data:      data,
	}

	switch v := data.(type) {
	case subscriber.StockData:
		record.Type = "stock_data"
		record.Symbol = v.Symbol
	case map[string]interface{}:
		if recordType, ok := v["type"].(string); ok {
			record.Type = recordType
		} else {
			record.Type = "generic"
		}
		if symbol, ok := v["symbol"].(string); ok {
			record.Symbol = symbol
		}
	default:
		record.Type = "unknown"
		record.Symbol = ""
	}

	jsonData, err := cs.serializer.Serialize(data)
	if err != nil {
		return nil, err
	}

	record.Fields = []string{
		record.Timestamp.Format(time.RFC3339),
		record.Type,
		record.Symbol,
		string(jsonData),
	}

	return record, nil
}

// getOrCreateWriter 根据记录类型和日期获取或创建一个新的CSV写入器。
func (cs *CSVStorage) getOrCreateWriter(recordType, date string) (*helpers.CSVWriterWrapper, error) {
	key := fmt.Sprintf("%s_%s", recordType, date)

	cs.mu.RLock()
	if writer, exists := cs.writerCache[key]; exists {
		cs.mu.RUnlock()
		return writer, nil
	}
	cs.mu.RUnlock()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if writer, exists := cs.writerCache[key]; exists {
		return writer, nil
	}

	filename := fmt.Sprintf("%s_%s_%s.csv", cs.config.FilePrefix, recordType, date)
	filepath := filepath.Join(cs.config.Directory, filename)

	file, err := cs.fileMgr.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}

	writer := helpers.NewCSVWriterWrapper(file, cs.resourceMgr)

	if stat, err := file.Stat(); err == nil && stat.Size() == 0 {
		headers := cs.getCSVHeaders(recordType)
		if err := writer.Write(headers); err != nil {
			writer.Close()
			return nil, fmt.Errorf("写入头部失败: %w", err)
		}
	}

	cs.writerCache[key] = writer
	return writer, nil
}

// getCSVHeaders 返回所有CSV文件统一使用的表头。
func (cs *CSVStorage) getCSVHeaders(recordType string) []string {
	return []string{"timestamp", "type", "symbol", "data"}
}

// startPeriodicFlush 启动一个后台goroutine，按固定间隔自动刷新缓冲区。
func (cs *CSVStorage) startPeriodicFlush() {
	ticker := time.NewTicker(cs.config.FlushInterval)
	defer ticker.Stop()

	for range ticker.C {
		cs.Flush()
	}
}

// JSONSerializer 实现了 core.Serializer 接口，使用JSON进行序列化和反序列化。
type JSONSerializer struct{}

// NewJSONSerializer 创建一个新的 JSONSerializer 实例。
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize 将任意对象序列化为JSON字节数组。
func (js *JSONSerializer) Serialize(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// Deserialize 将JSON字节数组反序列化到 target 对象中。
func (js *JSONSerializer) Deserialize(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}

// MimeType 返回 "application/json"。
func (js *JSONSerializer) MimeType() string {
	return "application/json"
}

// DefaultCSVStorageConfig 返回一个默认的CSVStorage配置实例。
func DefaultCSVStorageConfig() CSVStorageConfig {
	return CSVStorageConfig{
		Directory:      "./testdata",
		FilePrefix:     "stocksub",
		DateFormat:     "2006-01-02",
		MaxFileSize:    100 * 1024 * 1024, // 100MB
		RotateInterval: 24 * time.Hour,
		EnableCompress: false,
		BatchSize:      100,
		FlushInterval:  10 * time.Second,
		ResourceConfig: helpers.DefaultResourceConfig(),
	}
}

// 确保 CSVStorage 实现了 core.Storage 接口。
var _ core.Storage = (*CSVStorage)(nil)