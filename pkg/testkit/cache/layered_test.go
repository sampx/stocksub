package cache

import (
	"context"
	"errors"
	"stocksub/pkg/testkit/core"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks for Layered Cache Testing ---

type mockLayer struct {
	name        string
	data        map[string]interface{}
	mu          sync.RWMutex
	getDelay    time.Duration
	setError    error
	getError    error
	deleteError error
	clearError  error
	stats       core.CacheStats
}

func newMockLayer(name string) *mockLayer {
	return &mockLayer{
		name: name,
		data: make(map[string]interface{}),
	}
}

func (m *mockLayer) Get(ctx context.Context, key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getError != nil {
		return nil, m.getError
	}
	if m.getDelay > 0 {
		time.Sleep(m.getDelay)
	}
	if val, ok := m.data[key]; ok {
		m.stats.HitCount++
		return val, nil
	}
	m.stats.MissCount++
	return nil, core.NewTestKitError(core.ErrCacheMiss, "not found")
}

func (m *mockLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setError != nil {
		return m.setError
	}
	m.data[key] = value
	m.stats.Size++
	return nil
}

func (m *mockLayer) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteError != nil {
		return m.deleteError
	}
	if _, ok := m.data[key]; ok {
		delete(m.data, key)
		m.stats.Size--
	}
	return nil
}

func (m *mockLayer) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.clearError != nil {
		return m.clearError
	}
	m.data = make(map[string]interface{})
	m.stats.Size = 0
	return nil
}

func (m *mockLayer) Stats() core.CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *mockLayer) Close() error { return nil }

// mockFactory implements the LayerFactory interface for mock layers.
type mockFactory struct {
	layerType LayerType
	layer     core.Cache
}

func (f *mockFactory) CreateLayer(config LayerConfig, layerIndex int) (core.Cache, error) {
	return f.layer, nil
}

func (f *mockFactory) LayerType() LayerType {
	return f.layerType
}

// newTestLayeredCache is a helper to create a LayeredCache with mock layers.
func newTestLayeredCache(t *testing.T, layers ...*mockLayer) *LayeredCache {
	cfg := LayeredCacheConfig{PromoteEnabled: true, WriteThrough: true}
	factories := make(map[LayerType]LayerFactory)

	for _, layer := range layers {
		layerType := LayerType(layer.name)
		cfg.Layers = append(cfg.Layers, LayerConfig{Type: layerType, Enabled: true})
		factories[layerType] = &mockFactory{layerType: layerType, layer: layer}
	}

	cache, err := NewLayeredCacheWithFactories(cfg, factories)
	require.NoError(t, err)
	return cache
}

// --- Test Cases ---

func TestLayeredCache_Get(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	cache := newTestLayeredCache(t, memLayer, diskLayer)
	ctx := context.Background()

	// Case 1: Found in memory layer
	memLayer.Set(ctx, "key1", "value1_mem", 0)
	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1_mem", val)
	assert.Equal(t, int64(1), memLayer.Stats().HitCount)
	assert.Equal(t, int64(0), diskLayer.Stats().HitCount)

	// Case 2: Found in disk layer, should be promoted to memory
	diskLayer.Set(ctx, "key2", "value2_disk", 0)
	val, err = cache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.Equal(t, "value2_disk", val)
	assert.Equal(t, int64(1), memLayer.Stats().MissCount)
	assert.Equal(t, int64(1), diskLayer.Stats().HitCount)

	// Wait for promotion
	time.Sleep(50 * time.Millisecond)
	memVal, memErr := memLayer.Get(ctx, "key2")
	assert.NoError(t, memErr)
	assert.Equal(t, "value2_disk", memVal)

	// Case 3: Not found in any layer
	_, err = cache.Get(ctx, "key3")
	assert.Error(t, err)
}

func TestLayeredCache_BatchGet(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	cache := newTestLayeredCache(t, memLayer, diskLayer)
	ctx := context.Background()

	// Setup keys in different layers
	memLayer.Set(ctx, "key_mem", "val_mem", 0)
	diskLayer.Set(ctx, "key_disk", "val_disk", 0)
	memLayer.Set(ctx, "key_both", "val_mem_priority", 0)
	diskLayer.Set(ctx, "key_both", "val_disk_ignored", 0)

	keysToGet := []string{"key_mem", "key_disk", "key_both", "key_missing"}
	results, err := cache.BatchGet(ctx, keysToGet)

	assert.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, "val_mem", results["key_mem"])
	assert.Equal(t, "val_disk", results["key_disk"])
	assert.Equal(t, "val_mem_priority", results["key_both"])
	_, ok := results["key_missing"]
	assert.False(t, ok)

	// Verify promotion for key_disk
	time.Sleep(50 * time.Millisecond)
	memVal, memErr := memLayer.Get(ctx, "key_disk")
	assert.NoError(t, memErr)
	assert.Equal(t, "val_disk", memVal)
}

func TestLayeredCache_Set(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	cache := newTestLayeredCache(t, memLayer, diskLayer)
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	// Verify it's in both layers (due to WriteThrough)
	memVal, _ := memLayer.Get(ctx, "key1")
	diskVal, _ := diskLayer.Get(ctx, "key1")
	assert.Equal(t, "value1", memVal)
	assert.Equal(t, "value1", diskVal)
}

func TestLayeredCache_DeleteAndClear(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	cache := newTestLayeredCache(t, memLayer, diskLayer)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	// Test Delete
	err := cache.Delete(ctx, "key1")
	assert.NoError(t, err)

	_, err = memLayer.Get(ctx, "key1")
	assert.Error(t, err)
	_, err = diskLayer.Get(ctx, "key1")
	assert.Error(t, err)

	// Test Clear
	err = cache.Clear(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), memLayer.Stats().Size)
	assert.Equal(t, int64(0), diskLayer.Stats().Size)
}

func TestLayeredCache_ErrorHandling(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	ctx := context.Background()

	// Set error on disk layer
	diskLayer.setError = errors.New("disk write error")
	cache := newTestLayeredCache(t, memLayer, diskLayer)

	// Set should return the error
	err := cache.Set(ctx, "key1", "value1", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disk write error")

	// Clear errors for next test
	diskLayer.setError = nil

	// Delete error on disk layer
	cache.Set(ctx, "key2", "value2", 0)
	diskLayer.deleteError = errors.New("disk delete error")
	err = cache.Delete(ctx, "key2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disk delete error")
	diskLayer.deleteError = nil

	// Clear error on disk layer
	diskLayer.clearError = errors.New("disk clear error")
	err = cache.Clear(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disk clear error")
}

func TestNewLayeredCache_Factories(t *testing.T) {
	// Test with default factories
	config := DefaultLayeredCacheConfig()
	config.Layers = []LayerConfig{
		{Type: LayerMemory, Enabled: true, MaxSize: 10},
	}
	cache, err := NewLayeredCache(config)
	require.NoError(t, err)
	assert.NotNil(t, cache)

	// Test with missing factory
	config.Layers = []LayerConfig{
		{Type: "non_existent_type", Enabled: true},
	}
	_, err = NewLayeredCache(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的缓存层类型")
}

func TestLayeredCache_GetLayerStats(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	cache := newTestLayeredCache(t, memLayer, diskLayer)
	ctx := context.Background()

	memLayer.Set(ctx, "key1", "val1", 0)
	memLayer.Get(ctx, "key1")

	stats := cache.GetLayerStats()
	require.Len(t, stats.LayerStats, 2)

	memStats := stats.LayerStats[0]
	assert.Equal(t, int64(1), memStats.Size)
	assert.Equal(t, int64(1), memStats.HitCount)
}

func TestLayeredCache_Warm(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	cache := newTestLayeredCache(t, memLayer, diskLayer)
	ctx := context.Background()

	// Pre-seed disk layer
	diskLayer.Set(ctx, "key1", "val1", 0)
	diskLayer.Set(ctx, "key2", "val2", 0)

	// Warm up the cache
	dataToWarm := map[string]interface{}{
		"key1": "val1",
		"key2": "val2",
	}
	err := cache.Warm(ctx, dataToWarm)
	assert.NoError(t, err)

	// Check if memory layer is populated
	val1, err1 := memLayer.Get(ctx, "key1")
	val2, err2 := memLayer.Get(ctx, "key2")
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, "val1", val1)
	assert.Equal(t, "val2", val2)
	assert.Equal(t, int64(2), memLayer.Stats().Size)
}

func TestLayeredCache_BatchGet_Error(t *testing.T) {
	memLayer := newMockLayer("memory")
	diskLayer := newMockLayer("disk")
	cache := newTestLayeredCache(t, memLayer, diskLayer)
	ctx := context.Background()

	// Configure disk layer to fail on Get
	diskLayer.getError = errors.New("disk read failed")

	// key1 is not in memory, so it will try to fetch from disk
	_, err := cache.BatchGet(ctx, []string{"key1"})

	require.Error(t, err, "BatchGet should fail when underlying Get fails")
	assert.Contains(t, err.Error(), "disk read failed")
}