// Package storage 提供了 testkit 的存储层实现，包括CSV文件存储、内存存储和批量写入等功能。
package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"stocksub/pkg/core"
)

// MemoryStorage 是一种完全在内存中实现的 core.Storage 接口。
// 它用于快速、无I/O的测试，所有数据在程序结束时会丢失。
type MemoryStorage struct {
	data    map[string][]interface{}
	mu      sync.RWMutex
	indexes map[string]*MemoryIndex
	config  MemoryStorageConfig
	stats   MemoryStorageStats
}

// MemoryStorageConfig 定义了 MemoryStorage 的配置选项。
type MemoryStorageConfig struct {
	MaxRecords      int           `yaml:"max_records"`      // 每个"表"中存储的最大记录数。
	EnableIndex     bool          `yaml:"enable_index"`     // 是否为数据启用索引以加速查询。
	TTL             time.Duration `yaml:"ttl"`              // 记录的生存时间。
	CleanupInterval time.Duration `yaml:"cleanup_interval"` // 清理过期记录的后台任务运行间隔。
}

// MemoryStorageStats 包含了 MemoryStorage 的运行统计信息。
type MemoryStorageStats struct {
	TotalRecords int64     `json:"total_records"` // 存储的总记录数。
	TotalTables  int       `json:"total_tables"`  // 内部"表"的数量。
	IndexCount   int       `json:"index_count"`   // 创建的索引数量。
	LastCleanup  time.Time `json:"last_cleanup"`  // 最后一次清理的时间。
}

// MemoryIndex 为内存中的数据表提供索引功能。
type MemoryIndex struct {
	fieldIndex map[string][]int // 字段值到记录索引的映射
	timeIndex  []TimeRecord     // 时间索引
	mu         sync.RWMutex
}

// TimeRecord 是用于时间索引的内部结构。
type TimeRecord struct {
	Index     int
	Timestamp time.Time
}

// NewMemoryStorage 创建一个新的 MemoryStorage 实例。
func NewMemoryStorage(config MemoryStorageConfig) *MemoryStorage {
	ms := &MemoryStorage{
		data:    make(map[string][]interface{}),
		indexes: make(map[string]*MemoryIndex),
		config:  config,
		stats:   MemoryStorageStats{},
	}

	if config.CleanupInterval > 0 {
		go ms.startPeriodicCleanup()
	}

	return ms
}

// Save 将数据保存到内存中。
func (ms *MemoryStorage) Save(ctx context.Context, data interface{}) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	tableName := ms.getTableName(data)

	if len(ms.data[tableName]) >= ms.config.MaxRecords {
		ms.data[tableName] = ms.data[tableName][1:]
	}

	index := len(ms.data[tableName])
	ms.data[tableName] = append(ms.data[tableName], data)

	if ms.config.EnableIndex {
		ms.updateIndex(tableName, data, index)
	}

	ms.stats.TotalRecords++
	return nil
}

// Load 从内存中加载数据。
func (ms *MemoryStorage) Load(ctx context.Context, query core.Query) ([]interface{}, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	results := make([]interface{}, 0)

	for _, records := range ms.data {
		for _, record := range records {
			if ms.matchesQuery(record, query) {
				results = append(results, record)

				if query.Limit > 0 && len(results) >= query.Limit {
					return results, nil
				}
			}
		}
	}

	return results, nil
}

// Delete 从内存中删除数据。
func (ms *MemoryStorage) Delete(ctx context.Context, query core.Query) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for tableName, records := range ms.data {
		newRecords := make([]interface{}, 0, len(records))

		for _, record := range records {
			if !ms.matchesQuery(record, query) {
				newRecords = append(newRecords, record)
			}
		}

		ms.data[tableName] = newRecords
	}

	return nil
}

// Close 清空所有内存数据。
func (ms *MemoryStorage) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.data = make(map[string][]interface{})
	ms.indexes = make(map[string]*MemoryIndex)

	return nil
}

// BatchSave 批量保存数据到内存中（支持 StructuredData 优化）
func (ms *MemoryStorage) BatchSave(ctx context.Context, dataList []interface{}) error {
	if len(dataList) == 0 {
		return nil
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 按表名分组数据
	tableGroups := make(map[string][]interface{})
	for _, data := range dataList {
		tableName := ms.getTableName(data)
		tableGroups[tableName] = append(tableGroups[tableName], data)
	}

	// 为每个表批量处理数据
	for tableName, tableData := range tableGroups {
		if err := ms.batchSaveToTable(tableName, tableData); err != nil {
			return fmt.Errorf("batch save to table %s failed: %w", tableName, err)
		}
	}

	ms.stats.TotalRecords += int64(len(dataList))
	return nil
}

// batchSaveToTable 批量保存数据到指定表中
func (ms *MemoryStorage) batchSaveToTable(tableName string, tableData []interface{}) error {
	// 确保表存在
	if _, exists := ms.data[tableName]; !exists {
		ms.data[tableName] = make([]interface{}, 0)
	}

	// 检查容量限制
	currentSize := len(ms.data[tableName])
	newDataSize := len(tableData)
	totalSize := currentSize + newDataSize

	// 如果总大小超过限制，移除旧数据
	if totalSize > ms.config.MaxRecords {
		excessCount := totalSize - ms.config.MaxRecords
		if excessCount >= currentSize {
			// 新数据太多，只保留最新的
			ms.data[tableName] = ms.data[tableName][:0]
			keepCount := ms.config.MaxRecords
			if keepCount > newDataSize {
				keepCount = newDataSize
			}
			tableData = tableData[newDataSize-keepCount:]
		} else {
			// 移除一些旧数据
			ms.data[tableName] = ms.data[tableName][excessCount:]
		}
	}

	// 批量添加数据并建立索引
	startIndex := len(ms.data[tableName])
	ms.data[tableName] = append(ms.data[tableName], tableData...)

	// 为新添加的数据建立索引
	if ms.config.EnableIndex {
		for i, data := range tableData {
			ms.updateIndex(tableName, data, startIndex+i)
		}
	}

	return nil
}

func (ms *MemoryStorage) getTableName(data interface{}) string {
	switch d := data.(type) {
	case *StructuredData:
		// 对于 StructuredData，使用 schema 名称作为表名
		if d.Schema != nil {
			return fmt.Sprintf("table_structured_%s", d.Schema.Name)
		}
		return "table_structured_unknown"
	case map[string]interface{}:
		if m, ok := data.(map[string]interface{}); ok {
			if t, exists := m["type"]; exists {
				return fmt.Sprintf("table_%v", t)
			}
		}
		return "table_generic"
	default:
		return fmt.Sprintf("table_%T", data)
	}
}

func (ms *MemoryStorage) matchesQuery(record interface{}, query core.Query) bool {
	// 如果是 StructuredData，使用专门的查询逻辑
	if structData, ok := record.(*StructuredData); ok {
		return ms.queryStructuredData(structData, query)
	}

	// 对于其他类型，返回 true（保持原有行为）
	return true
}

// updateIndex 更新指定表的索引信息
//
// 参数:
//
//	tableName: string - 需要更新索引的表名
//	data: interface{} - 需要被索引的数据，可以是任意类型
//	index: int - 数据在存储中的位置索引
//
// 功能说明:
//  1. 如果表不存在索引结构，则创建新的 MemoryIndex
//  2. 根据数据类型进行不同的索引处理：
//     - 如果是 StructuredData 类型，调用 indexStructuredData 进行特殊索引处理
//     - 否则使用当前时间作为时间戳
//  3. 更新时间索引，记录数据位置和时间戳
func (ms *MemoryStorage) updateIndex(tableName string, data interface{}, index int) {
	if _, exists := ms.indexes[tableName]; !exists {
		ms.indexes[tableName] = &MemoryIndex{
			fieldIndex: make(map[string][]int),
			timeIndex:  make([]TimeRecord, 0),
		}
	}

	idx := ms.indexes[tableName]

	// 处理 StructuredData 的特殊索引
	if structData, ok := data.(*StructuredData); ok {
		ms.indexStructuredData(idx, structData, index)
		// 使用 StructuredData 的时间戳
		idx.timeIndex = append(idx.timeIndex, TimeRecord{
			Index:     index,
			Timestamp: structData.Timestamp,
		})
	} else {
		// 为非 StructuredData 使用当前时间
		idx.timeIndex = append(idx.timeIndex, TimeRecord{
			Index:     index,
			Timestamp: time.Now(),
		})
	}
}

func (ms *MemoryStorage) startPeriodicCleanup() {
	ticker := time.NewTicker(ms.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ms.cleanup()
	}
}

func (ms *MemoryStorage) cleanup() {
	if ms.config.TTL <= 0 {
		return
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	cutoff := time.Now().Add(-ms.config.TTL)

	for tableName, index := range ms.indexes {
		if index == nil {
			continue
		}

		validRecords := make([]TimeRecord, 0)
		invalidIndexes := make(map[int]bool)

		for _, timeRecord := range index.timeIndex {
			if timeRecord.Timestamp.After(cutoff) {
				validRecords = append(validRecords, timeRecord)
			} else {
				invalidIndexes[timeRecord.Index] = true
			}
		}

		index.timeIndex = validRecords

		if records, exists := ms.data[tableName]; exists {
			validData := make([]interface{}, 0)
			for i, record := range records {
				if !invalidIndexes[i] {
					validData = append(validData, record)
				}
			}
			ms.data[tableName] = validData
		}
	}

	ms.stats.LastCleanup = time.Now()
}

// DefaultMemoryStorageConfig 返回一个默认的 MemoryStorage 配置实例。
func DefaultMemoryStorageConfig() MemoryStorageConfig {
	return MemoryStorageConfig{
		MaxRecords:      10000,
		EnableIndex:     true,
		TTL:             24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}
}

var _ Storage = (*MemoryStorage)(nil)

// indexStructuredData 为结构化数据建立内存索引，包括字段值索引、前缀索引和特殊字段索引
//
// 参数:
//   - index: MemoryIndex 指针，用于存储索引数据
//   - data: StructuredData 指针，包含需要建立索引的结构化数据
//   - recordIndex: int 类型，表示当前记录在存储中的位置索引
//
// 功能说明:
//  1. 为数据中的每个非空字段值建立索引，索引格式为 "字段名:值"
//  2. 对于字符串类型的值，额外建立前缀索引，支持长度1-10的前缀匹配
//  3. 为特殊的 "symbol" 字段建立单独的索引，格式为 "symbol_index:值"
//  4. 所有索引都存储在 index.fieldIndex 中，值为包含 recordIndex 的切片
//
// 线程安全:
//   - 使用互斥锁 (index.mu) 确保索引操作的线程安全
func (ms *MemoryStorage) indexStructuredData(index *MemoryIndex, data *StructuredData, recordIndex int) {
	index.mu.Lock()
	defer index.mu.Unlock()

	// 为每个字段值建立索引
	for fieldName, value := range data.Values {
		if value == nil {
			continue
		}

		// 生成索引键：字段名:值
		indexKey := fmt.Sprintf("%s:%v", fieldName, value)
		if _, exists := index.fieldIndex[indexKey]; !exists {
			index.fieldIndex[indexKey] = make([]int, 0)
		}
		index.fieldIndex[indexKey] = append(index.fieldIndex[indexKey], recordIndex)

		// 为字符串类型建立前缀索引
		if strValue, ok := value.(string); ok {
			// 建立前缀索引
			for i := 1; i <= len(strValue) && i <= 10; i++ { // 限制前缀长度
				prefixKey := fmt.Sprintf("%s:prefix:%s", fieldName, strValue[:i])
				if _, exists := index.fieldIndex[prefixKey]; !exists {
					index.fieldIndex[prefixKey] = make([]int, 0)
				}
				index.fieldIndex[prefixKey] = append(index.fieldIndex[prefixKey], recordIndex)
			}
		}
	}

	// 为 symbol 字段建立特殊索引（如果存在）
	if symbol, exists := data.Values["symbol"]; exists {
		if strSymbol, ok := symbol.(string); ok {
			symbolKey := fmt.Sprintf("symbol_index:%s", strSymbol)
			if _, exists := index.fieldIndex[symbolKey]; !exists {
				index.fieldIndex[symbolKey] = make([]int, 0)
			}
			index.fieldIndex[symbolKey] = append(index.fieldIndex[symbolKey], recordIndex)
		}
	}
}

// queryStructuredData 检查给定的结构化数据是否匹配查询条件
// 参数:
//   - data: 要检查的结构化数据指针
//   - query: 包含查询条件的Query结构体，包括股票代码列表和时间范围
//
// 返回值:
//   - bool: 如果数据匹配查询条件返回true，否则返回false
//
// 查询逻辑:
//  1. 如果查询条件为空(无股票代码且无时间范围)，匹配所有记录
//  2. 检查股票代码是否在查询的股票代码列表中
//  3. 检查数据时间戳是否在查询的时间范围内
func (ms *MemoryStorage) queryStructuredData(data *StructuredData, query core.Query) bool {
	// 如果没有查询条件，匹配所有记录
	if len(query.Symbols) == 0 && query.StartTime.IsZero() && query.EndTime.IsZero() {
		return true
	}

	// 检查股票代码匹配
	if len(query.Symbols) > 0 {
		if symbol, exists := data.Values["symbol"]; exists {
			if symbolStr, ok := symbol.(string); ok {
				found := false
				for _, targetSymbol := range query.Symbols {
					if symbolStr == targetSymbol {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			} else {
				return false
			}
		} else {
			return false
		}
	}

	// 检查时间范围
	if !query.StartTime.IsZero() && data.Timestamp.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && data.Timestamp.After(query.EndTime) {
		return false
	}

	return true
}

// GetStats 返回内存存储的统计信息
// 该方法会获取当前内存存储的状态信息，包括总表数和索引数
// 返回值:
//   - MemoryStorageStats: 包含存储统计信息的结构体，其中TotalTables表示总表数，IndexCount表示索引数
//
// 注意: 该方法使用读锁保证线程安全，不会阻塞其他读操作
func (ms *MemoryStorage) GetStats() MemoryStorageStats {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	stats := ms.stats
	stats.TotalTables = len(ms.data)
	stats.IndexCount = len(ms.indexes)

	return stats
}

// QueryBySymbol 根据交易品种代码查询相关的结构化数据 StructuredData
//
// 参数:
//
//	ctx context.Context - 上下文信息，用于控制请求的超时和取消
//	symbol string - 交易品种代码，用于指定要查询的交易品种
//
// 返回值:
//
//	[]*StructuredData - 查询到的结构化数据切片
//	error - 查询过程中发生的错误，如果查询成功则返回nil
func (ms *MemoryStorage) QueryBySymbol(ctx context.Context, symbol string) ([]*StructuredData, error) {
	query := core.Query{
		Symbols: []string{symbol},
	}

	results, err := ms.Load(ctx, query)
	if err != nil {
		return nil, err
	}

	// 过滤出 StructuredData 类型的结果
	var structuredResults []*StructuredData
	for _, result := range results {
		if structData, ok := result.(*StructuredData); ok {
			structuredResults = append(structuredResults, structData)
		}
	}

	return structuredResults, nil
}

// QueryByTimeRange 根据时间范围查询 StructuredData
func (ms *MemoryStorage) QueryByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*StructuredData, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var results []*StructuredData

	// 遍历所有表中的 StructuredData
	for tableName, records := range ms.data {
		if !strings.HasPrefix(tableName, "table_structured_") {
			continue
		}

		for _, record := range records {
			if structData, ok := record.(*StructuredData); ok {
				// 检查时间范围
				if (structData.Timestamp.Equal(startTime) || structData.Timestamp.After(startTime)) &&
					(structData.Timestamp.Equal(endTime) || structData.Timestamp.Before(endTime)) {
					results = append(results, structData)
				}
			}
		}
	}

	return results, nil
}
