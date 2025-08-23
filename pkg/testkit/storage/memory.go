// Package storage 提供了 testkit 的存储层实现，包括CSV文件存储、内存存储和批量写入等功能。
package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stocksub/pkg/testkit/core"
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

func (ms *MemoryStorage) getTableName(data interface{}) string {
	switch data.(type) {
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
	return true
}

func (ms *MemoryStorage) updateIndex(tableName string, data interface{}, index int) {
	if _, exists := ms.indexes[tableName]; !exists {
		ms.indexes[tableName] = &MemoryIndex{
			fieldIndex: make(map[string][]int),
			timeIndex:  make([]TimeRecord, 0),
		}
	}

	idx := ms.indexes[tableName]

	idx.timeIndex = append(idx.timeIndex, TimeRecord{
		Index:     index,
		Timestamp: time.Now(),
	})
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

// 确保 MemoryStorage 实现了 core.Storage 接口。
var _ core.Storage = (*MemoryStorage)(nil)
