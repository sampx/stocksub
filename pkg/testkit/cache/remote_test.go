package cache

import (
	"context"
	"stocksub/pkg/testkit/core"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestMockCache() (*MockRemoteCache, RemoteCacheConfig) {
	cfg := RemoteCacheConfig{
		MaxSize:    10,
		DefaultTTL: 1 * time.Minute,
	}
	return NewMockRemoteCache(cfg), cfg
}

func Test_remoteCacheBase_Stats(t *testing.T) {
	base := newRemoteCacheBase(RemoteCacheConfig{MaxSize: 100, DefaultTTL: time.Minute})
	base.stats.HitCount = 3
	base.stats.MissCount = 1

	stats := base.Stats()
	assert.Equal(t, int64(100), stats.MaxSize)
	assert.Equal(t, time.Minute, stats.TTL)
	assert.Equal(t, int64(3), stats.HitCount)
	assert.Equal(t, int64(1), stats.MissCount)
	assert.Equal(t, 0.75, stats.HitRate)
	assert.NotNil(t, stats.LastCleanup)
}

func Test_remoteCacheBase_NotImplemented(t *testing.T) {
	base := newRemoteCacheBase(RemoteCacheConfig{})
	ctx := context.Background()

	_, err := base.Get(ctx, "key")
	assert.Error(t, err)
	assert.Equal(t, "Get method not implemented", err.Error())

	err = base.Set(ctx, "key", "value", 0)
	assert.Error(t, err)
	assert.Equal(t, "Set method not implemented", err.Error())

	err = base.Delete(ctx, "key")
	assert.Error(t, err)
	assert.Equal(t, "Delete method not implemented", err.Error())

	err = base.Clear(ctx)
	assert.Error(t, err)
	assert.Equal(t, "Clear method not implemented", err.Error())
}

func TestMockRemoteCache_ConnectAndPing(t *testing.T) {
	cache, _ := newTestMockCache()
	ctx := context.Background()

	// Ping before connect should fail
	err := cache.Ping(ctx)
	assert.Error(t, err)
	assert.False(t, cache.IsConnected())

	// Connect
	err = cache.Connect(ctx)
	assert.NoError(t, err)
	assert.True(t, cache.IsConnected())

	// Ping after connect should succeed
	err = cache.Ping(ctx)
	assert.NoError(t, err)

	// Close
	err = cache.Close()
	assert.NoError(t, err)
	assert.False(t, cache.IsConnected())
}

func TestMockRemoteCache_SetAndGet(t *testing.T) {
	cache, _ := newTestMockCache()
	ctx := context.Background()
	cache.Connect(ctx)

	// Get a non-existent key
	_, err := cache.Get(ctx, "key1")
	assert.Error(t, err)
	testKitErr, ok := err.(*core.TestKitError)
	assert.True(t, ok)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)

	// Set a key
	err = cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	assert.NoError(t, err)

	// Get the key
	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)

	// Wait for it to expire
	time.Sleep(60 * time.Millisecond)
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	testKitErr, ok = err.(*core.TestKitError)
	assert.True(t, ok)
	assert.Equal(t, core.ErrCacheMiss, testKitErr.Code)
	assert.Contains(t, testKitErr.Error(), "cache expired")
}

func TestMockRemoteCache_Delete(t *testing.T) {
	cache, _ := newTestMockCache()
	ctx := context.Background()
	cache.Connect(ctx)

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	assert.Equal(t, int64(2), cache.Stats().Size)

	// Delete a key
	err := cache.Delete(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), cache.Stats().Size)

	// Verify it's gone
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)

	// Verify other key is still there
	val, err := cache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.Equal(t, "value2", val)

	// Delete a non-existent key
	err = cache.Delete(ctx, "key_nonexistent")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), cache.Stats().Size)
}

func TestMockRemoteCache_Clear(t *testing.T) {
	cache, _ := newTestMockCache()
	ctx := context.Background()
	cache.Connect(ctx)

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	stats := cache.Stats()
	assert.Equal(t, int64(2), stats.Size)

	// Clear the cache
	err := cache.Clear(ctx)
	assert.NoError(t, err)

	// Verify it's empty
	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.HitCount)
	assert.Equal(t, int64(0), stats.MissCount)

	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
}

func TestMockRemoteCache_Eviction(t *testing.T) {
	cache, cfg := newTestMockCache()
	ctx := context.Background()
	cache.Connect(ctx)

	// Fill the cache up to its max size
	for i := 0; i < int(cfg.MaxSize); i++ {
		key := "key" + string(rune(i))
		cache.Set(ctx, key, "value", 10*time.Second)
	}
	assert.Equal(t, cfg.MaxSize, cache.Stats().Size)

	// Add one more item, which should trigger eviction
	cache.Set(ctx, "new_key", "new_value", 10*time.Second)

	// The size should still be MaxSize
	assert.Equal(t, cfg.MaxSize, cache.Stats().Size)
}

func TestMockRemoteCache_Stats(t *testing.T) {
	cache, _ := newTestMockCache()
	ctx := context.Background()
	cache.Connect(ctx)

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	cache.Get(ctx, "key1") // Hit
	cache.Get(ctx, "key2") // Hit
	cache.Get(ctx, "key3") // Miss

	stats := cache.Stats()
	assert.Equal(t, int64(2), stats.HitCount)
	assert.Equal(t, int64(1), stats.MissCount)
	assert.InDelta(t, 0.66, stats.HitRate, 0.01)
	assert.Equal(t, int64(2), stats.Size)

	remoteStats, err := cache.GetStats(ctx)
	assert.NoError(t, err)
	assert.Equal(t, true, remoteStats["connected"])
	assert.Equal(t, 2, remoteStats["items_count"])
	assert.Equal(t, int64(2), remoteStats["hit_count"])
	assert.Equal(t, int64(1), remoteStats["miss_count"])
	assert.Equal(t, int64(10), remoteStats["max_size"])
}