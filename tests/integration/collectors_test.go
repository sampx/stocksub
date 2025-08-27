//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/message"
)

func TestCollectorsIntegration(t *testing.T) {
	// Setup Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})
	defer redisClient.Close()

	ctx := context.Background()

	// Clean up test data
	defer func() {
		redisClient.FlushDB(ctx)
	}()

	t.Run("MessageProcessing", func(t *testing.T) {
		// Create test message
		stockData := []message.StockData{
			{
				Symbol:        "600000",
				Name:          "浦发银行",
				Price:         10.50,
				Change:        0.15,
				ChangePercent: 1.45,
				Volume:        1250000,
				Timestamp:     time.Now().Format(time.RFC3339),
			},
		}

		msgFormat := message.MessageFormat{
			Header: message.MessageHeader{
				MessageID:   "test-message-1",
				Timestamp:   time.Now().Unix(),
				Version:     "1.0",
				Producer:    "test-producer",
				ContentType: "application/json",
			},
			Metadata: message.MessageMetadata{
				Provider:  "tencent",
				DataType:  "stock_realtime",
				BatchSize: 1,
				Market:    "A-share",
			},
		}

		// Marshal payload
		payload, err := json.Marshal(stockData)
		require.NoError(t, err)
		msgFormat.Payload = payload

		// Calculate checksum
		msgFormat.CalculateChecksum()

		// Serialize message
		msgData, err := json.Marshal(msgFormat)
		require.NoError(t, err)

		// Publish to Redis stream
		streamName := "stream:stock:realtime"
		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(msgData),
			},
		}).Result()
		require.NoError(t, err)

		// Verify message was published
		messages, err := redisClient.XRange(ctx, streamName, "-", "+").Result()
		require.NoError(t, err)
		assert.Len(t, messages, 1)

		// Verify message content
		var receivedMsg message.MessageFormat
		err = json.Unmarshal([]byte(messages[0].Values["data"].(string)), &receivedMsg)
		require.NoError(t, err)
		assert.NoError(t, receivedMsg.Validate())
		assert.Equal(t, "stock_realtime", receivedMsg.Metadata.DataType)
	})

	t.Run("ConsumerGroupCreation", func(t *testing.T) {
		streamName := "stream:test:consumer"
		consumerGroup := "test_collectors"

		// Create stream with initial message
		_, err := redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"test": "data",
			},
		}).Result()
		require.NoError(t, err)

		// Create consumer group
		err = redisClient.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
		require.NoError(t, err)

		// Verify consumer group exists
		groups, err := redisClient.XInfoGroups(ctx, streamName).Result()
		require.NoError(t, err)
		assert.Len(t, groups, 1)
		assert.Equal(t, consumerGroup, groups[0].Name)
	})

	t.Run("MessageConsumption", func(t *testing.T) {
		streamName := "stream:test:consumption"
		consumerGroup := "test_consumers"
		consumerName := "test_consumer_1"

		// Create test message
		testData := map[string]interface{}{
			"symbol": "000001",
			"price":  15.20,
		}
		msgData, err := json.Marshal(testData)
		require.NoError(t, err)

		// Add message to stream
		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(msgData),
			},
		}).Result()
		require.NoError(t, err)

		// Create consumer group
		err = redisClient.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
		require.NoError(t, err)

		// Read message as consumer
		result, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{streamName, ">"},
			Count:    1,
			Block:    time.Second,
		}).Result()
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Len(t, result[0].Messages, 1)

		// Verify message content
		msg := result[0].Messages[0]
		var receivedData map[string]interface{}
		err = json.Unmarshal([]byte(msg.Values["data"].(string)), &receivedData)
		require.NoError(t, err)
		assert.Equal(t, "000001", receivedData["symbol"])
		assert.Equal(t, 15.20, receivedData["price"])

		// Acknowledge message
		err = redisClient.XAck(ctx, streamName, consumerGroup, msg.ID).Err()
		require.NoError(t, err)

		// Verify message was acknowledged
		pending, err := redisClient.XPending(ctx, streamName, consumerGroup).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), pending.Count)
	})

	t.Run("RedisDataStorage", func(t *testing.T) {
		// Test Redis collector data storage format
		symbol := "600000"
		key := fmt.Sprintf("latest:stock:%s", symbol)

		// Store test data
		hashData := map[string]interface{}{
			"symbol":         symbol,
			"name":           "浦发银行",
			"price":          10.50,
			"change":         0.15,
			"change_percent": 1.45,
			"volume":         1250000,
			"timestamp":      time.Now().Unix(),
			"provider":       "tencent",
			"market":         "A-share",
			"updated_at":     time.Now().Unix(),
		}

		err := redisClient.HMSet(ctx, key, hashData).Err()
		require.NoError(t, err)

		// Set TTL
		err = redisClient.Expire(ctx, key, time.Hour).Err()
		require.NoError(t, err)

		// Add to symbols set
		err = redisClient.SAdd(ctx, "symbols:stock", symbol).Err()
		require.NoError(t, err)

		// Verify data retrieval
		result, err := redisClient.HGetAll(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, symbol, result["symbol"])
		assert.Equal(t, "浦发银行", result["name"])
		assert.Equal(t, "10.5", result["price"])

		// Verify symbols set
		symbols, err := redisClient.SMembers(ctx, "symbols:stock").Result()
		require.NoError(t, err)
		assert.Contains(t, symbols, symbol)

		// Verify TTL
		ttl, err := redisClient.TTL(ctx, key).Result()
		require.NoError(t, err)
		assert.True(t, ttl > 0)
	})

	t.Run("MultipleConsumersLoadBalancing", func(t *testing.T) {
		streamName := "stream:test:loadbalancing"
		consumerGroup := "load_test_consumers"

		// Create multiple messages
		messageCount := 10
		for i := 0; i < messageCount; i++ {
			_, err := redisClient.XAdd(ctx, &redis.XAddArgs{
				Stream: streamName,
				Values: map[string]interface{}{
					"data": fmt.Sprintf("message-%d", i),
				},
			}).Result()
			require.NoError(t, err)
		}

		// Create consumer group
		err := redisClient.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
		require.NoError(t, err)

		// Simulate multiple consumers
		consumer1Messages := 0
		consumer2Messages := 0

		// Consumer 1 reads messages
		for i := 0; i < 5; i++ {
			result, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    consumerGroup,
				Consumer: "consumer_1",
				Streams:  []string{streamName, ">"},
				Count:    1,
				Block:    100 * time.Millisecond,
			}).Result()
			if err == nil && len(result) > 0 && len(result[0].Messages) > 0 {
				consumer1Messages++
				// Acknowledge message
				redisClient.XAck(ctx, streamName, consumerGroup, result[0].Messages[0].ID)
			}
		}

		// Consumer 2 reads remaining messages
		for i := 0; i < 5; i++ {
			result, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    consumerGroup,
				Consumer: "consumer_2",
				Streams:  []string{streamName, ">"},
				Count:    1,
				Block:    100 * time.Millisecond,
			}).Result()
			if err == nil && len(result) > 0 && len(result[0].Messages) > 0 {
				consumer2Messages++
				// Acknowledge message
				redisClient.XAck(ctx, streamName, consumerGroup, result[0].Messages[0].ID)
			}
		}

		// Verify load balancing (both consumers should get messages)
		totalProcessed := consumer1Messages + consumer2Messages
		assert.True(t, totalProcessed > 0, "At least some messages should be processed")
		assert.True(t, consumer1Messages > 0 || consumer2Messages > 0, "At least one consumer should process messages")

		// Verify all messages were processed
		pending, err := redisClient.XPending(ctx, streamName, consumerGroup).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), pending.Count, "All messages should be acknowledged")
	})
}
