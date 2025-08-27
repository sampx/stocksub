//go:build integration
// +build integration

package integration

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

func TestPhase4EndToEnd(t *testing.T) {
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

	t.Run("CompleteDataFlow", func(t *testing.T) {
		// Step 1: Simulate provider_node publishing data
		stockData := []message.StockData{
			{
				Symbol:        "600000",
				Name:          "浦发银行",
				Price:         10.50,
				Change:        0.15,
				ChangePercent: 1.45,
				Volume:        1250000,
				Timestamp:     time.Now(),
			},
			{
				Symbol:        "000001",
				Name:          "平安银行",
				Price:         15.20,
				Change:        -0.30,
				ChangePercent: -1.94,
				Volume:        2100000,
				Timestamp:     time.Now(),
			},
		}

		msgFormat := message.MessageFormat{
			Header: message.MessageHeader{
				MessageID:   "e2e-test-message-1",
				Timestamp:   time.Now().Unix(),
				Version:     "1.0",
				Producer:    "e2e-test-producer",
				ContentType: "application/json",
			},
			Metadata: message.MessageMetadata{
				Provider:  "tencent",
				DataType:  "stock_realtime",
				BatchSize: len(stockData),
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

		// Publish to Redis stream (simulating provider_node)
		streamName := "stream:stock:realtime"
		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(msgData),
			},
		}).Result()
		require.NoError(t, err)

		// Step 2: Simulate redis_collector processing the message
		consumerGroup := "test_redis_collectors"
		consumerName := "test_redis_collector_1"

		// Create consumer group
		err = redisClient.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
		require.NoError(t, err)

		// Read message as redis_collector would
		result, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{streamName, ">"},
			Count:    1,
			Block:    time.Second,
		}).Result()
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Len(t, result[0].Messages, 1)

		// Process message (simulate redis_collector logic)
		msg := result[0].Messages[0]
		data, ok := msg.Values["data"].(string)
		require.True(t, ok)

		var receivedMsg message.MessageFormat
		err = json.Unmarshal([]byte(data), &receivedMsg)
		require.NoError(t, err)
		require.True(t, receivedMsg.VerifyChecksum())

		// Parse stock data
		var receivedStockData []message.StockData
		err = json.Unmarshal(receivedMsg.Payload, &receivedStockData)
		require.NoError(t, err)
		require.Len(t, receivedStockData, 2)

		// Store data in Redis (simulate redis_collector storage)
		pipe := redisClient.Pipeline()
		for _, stock := range receivedStockData {
			key := fmt.Sprintf("latest:stock:%s", stock.Symbol)

			hashData := map[string]interface{}{
				"symbol":         stock.Symbol,
				"name":           stock.Name,
				"price":          stock.Price,
				"change":         stock.Change,
				"change_percent": stock.ChangePercent,
				"volume":         stock.Volume,
				"timestamp":      stock.Timestamp.Unix(),
				"provider":       receivedMsg.Metadata.Provider,
				"market":         receivedMsg.Metadata.Market,
				"updated_at":     time.Now().Unix(),
			}

			pipe.HMSet(ctx, key, hashData)
			pipe.Expire(ctx, key, time.Hour)
			pipe.SAdd(ctx, "symbols:stock", stock.Symbol)
		}
		pipe.Expire(ctx, "symbols:stock", time.Hour)

		_, err = pipe.Exec(ctx)
		require.NoError(t, err)

		// Acknowledge message
		err = redisClient.XAck(ctx, streamName, consumerGroup, msg.ID).Err()
		require.NoError(t, err)

		// Step 3: Verify data is available for API queries
		// Test individual stock retrieval
		for _, expectedStock := range stockData {
			key := fmt.Sprintf("latest:stock:%s", expectedStock.Symbol)
			storedData, err := redisClient.HGetAll(ctx, key).Result()
			require.NoError(t, err)
			require.NotEmpty(t, storedData)

			assert.Equal(t, expectedStock.Symbol, storedData["symbol"])
			assert.Equal(t, expectedStock.Name, storedData["name"])
			assert.Equal(t, fmt.Sprintf("%.2f", expectedStock.Price), fmt.Sprintf("%.2f", parseFloat(storedData["price"])))
			assert.Equal(t, "tencent", storedData["provider"])
			assert.Equal(t, "A-share", storedData["market"])
		}

		// Test symbols set
		symbols, err := redisClient.SMembers(ctx, "symbols:stock").Result()
		require.NoError(t, err)
		assert.Contains(t, symbols, "600000")
		assert.Contains(t, symbols, "000001")

		// Step 4: Test API server data retrieval (simulated)
		// This would normally be done through HTTP requests to the API server
		// Here we simulate the API server logic directly

		// Simulate GET /api/v1/stocks/600000
		stockKey := "latest:stock:600000"
		apiResult, err := redisClient.HGetAll(ctx, stockKey).Result()
		require.NoError(t, err)
		require.NotEmpty(t, apiResult)

		// Verify API response format
		expectedAPIResponse := map[string]interface{}{
			"symbol":         "600000",
			"name":           "浦发银行",
			"price":          "10.5",
			"change":         "0.15",
			"change_percent": "1.45",
			"volume":         "1250000",
			"provider":       "tencent",
			"market":         "A-share",
		}

		for key, expectedValue := range expectedAPIResponse {
			assert.Equal(t, expectedValue, apiResult[key], "Mismatch for key: %s", key)
		}

		// Simulate GET /api/v1/stocks (all stocks)
		allSymbols, err := redisClient.SMembers(ctx, "symbols:stock").Result()
		require.NoError(t, err)
		assert.Len(t, allSymbols, 2)

		// Verify we can retrieve all stocks
		allStocksData := make(map[string]map[string]string)
		for _, symbol := range allSymbols {
			key := fmt.Sprintf("latest:stock:%s", symbol)
			stockData, err := redisClient.HGetAll(ctx, key).Result()
			require.NoError(t, err)
			allStocksData[symbol] = stockData
		}

		assert.Len(t, allStocksData, 2)
		assert.Contains(t, allStocksData, "600000")
		assert.Contains(t, allStocksData, "000001")
	})

	t.Run("MultipleConsumerGroups", func(t *testing.T) {
		// Test that both redis_collector and influxdb_collector can consume the same stream
		streamName := "stream:test:multiple-consumers"

		// Create test message
		testMsg := map[string]interface{}{
			"test":      "data",
			"timestamp": time.Now().Unix(),
		}
		msgData, err := json.Marshal(testMsg)
		require.NoError(t, err)

		// Publish message
		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(msgData),
			},
		}).Result()
		require.NoError(t, err)

		// Create multiple consumer groups
		redisGroup := "redis_collectors"
		influxGroup := "influxdb_collectors"

		err = redisClient.XGroupCreateMkStream(ctx, streamName, redisGroup, "0").Err()
		require.NoError(t, err)

		err = redisClient.XGroupCreateMkStream(ctx, streamName, influxGroup, "0").Err()
		require.NoError(t, err)

		// Both consumer groups should be able to read the same message
		// Redis collector reads
		redisResult, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    redisGroup,
			Consumer: "redis_consumer_1",
			Streams:  []string{streamName, ">"},
			Count:    1,
			Block:    time.Second,
		}).Result()
		require.NoError(t, err)
		require.Len(t, redisResult, 1)
		require.Len(t, redisResult[0].Messages, 1)

		// InfluxDB collector reads
		influxResult, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    influxGroup,
			Consumer: "influx_consumer_1",
			Streams:  []string{streamName, ">"},
			Count:    1,
			Block:    time.Second,
		}).Result()
		require.NoError(t, err)
		require.Len(t, influxResult, 1)
		require.Len(t, influxResult[0].Messages, 1)

		// Both should have received the same message
		redisMsg := redisResult[0].Messages[0]
		influxMsg := influxResult[0].Messages[0]
		assert.Equal(t, redisMsg.Values["data"], influxMsg.Values["data"])

		// Acknowledge both messages
		err = redisClient.XAck(ctx, streamName, redisGroup, redisMsg.ID).Err()
		require.NoError(t, err)

		err = redisClient.XAck(ctx, streamName, influxGroup, influxMsg.ID).Err()
		require.NoError(t, err)

		// Verify both consumer groups have no pending messages
		redisPending, err := redisClient.XPending(ctx, streamName, redisGroup).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), redisPending.Count)

		influxPending, err := redisClient.XPending(ctx, streamName, influxGroup).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), influxPending.Count)
	})

	t.Run("DataConsistencyAndIdempotency", func(t *testing.T) {
		// Test that processing the same message multiple times doesn't cause issues
		symbol := "600000"

		// Create test message
		stockData := []message.StockData{
			{
				Symbol:        symbol,
				Name:          "浦发银行",
				Price:         10.50,
				Change:        0.15,
				ChangePercent: 1.45,
				Volume:        1250000,
				Timestamp:     time.Now(),
			},
		}

		msgFormat := message.MessageFormat{
			Header: message.MessageHeader{
				MessageID:   "idempotency-test-1",
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

		payload, err := json.Marshal(stockData)
		require.NoError(t, err)
		msgFormat.Payload = payload
		msgFormat.CalculateChecksum()

		// Process the same message multiple times
		for i := 0; i < 3; i++ {
			// Simulate redis_collector processing
			var receivedStockData []message.StockData
			err = json.Unmarshal(msgFormat.Payload, &receivedStockData)
			require.NoError(t, err)

			// Store data (this should be idempotent)
			for _, stock := range receivedStockData {
				key := fmt.Sprintf("latest:stock:%s", stock.Symbol)

				hashData := map[string]interface{}{
					"symbol":         stock.Symbol,
					"name":           stock.Name,
					"price":          stock.Price,
					"change":         stock.Change,
					"change_percent": stock.ChangePercent,
					"volume":         stock.Volume,
					"timestamp":      stock.Timestamp.Unix(),
					"provider":       msgFormat.Metadata.Provider,
					"market":         msgFormat.Metadata.Market,
					"updated_at":     time.Now().Unix(),
				}

				err = redisClient.HMSet(ctx, key, hashData).Err()
				require.NoError(t, err)

				err = redisClient.SAdd(ctx, "symbols:stock", stock.Symbol).Err()
				require.NoError(t, err)
			}
		}

		// Verify data integrity after multiple processing
		key := fmt.Sprintf("latest:stock:%s", symbol)
		result, err := redisClient.HGetAll(ctx, key).Result()
		require.NoError(t, err)

		assert.Equal(t, symbol, result["symbol"])
		assert.Equal(t, "浦发银行", result["name"])
		assert.Equal(t, "10.5", result["price"])

		// Verify symbols set contains only one entry for the symbol
		symbols, err := redisClient.SMembers(ctx, "symbols:stock").Result()
		require.NoError(t, err)

		symbolCount := 0
		for _, s := range symbols {
			if s == symbol {
				symbolCount++
			}
		}
		assert.Equal(t, 1, symbolCount, "Symbol should appear only once in the set")
	})

	t.Run("ErrorHandlingAndRecovery", func(t *testing.T) {
		// Test handling of malformed messages
		streamName := "stream:test:error-handling"
		consumerGroup := "test_error_consumers"

		// Create consumer group
		err := redisClient.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
		require.NoError(t, err)

		// Publish malformed message
		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": "invalid json data",
			},
		}).Result()
		require.NoError(t, err)

		// Publish valid message after malformed one
		validMsg := message.MessageFormat{
			Header: message.MessageHeader{
				MessageID:   "valid-after-error",
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

		stockData := []message.StockData{
			{
				Symbol:    "600000",
				Name:      "测试股票",
				Price:     10.0,
				Timestamp: time.Now(),
			},
		}

		payload, err := json.Marshal(stockData)
		require.NoError(t, err)
		validMsg.Payload = payload
		validMsg.CalculateChecksum()

		validMsgData, err := json.Marshal(validMsg)
		require.NoError(t, err)

		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(validMsgData),
			},
		}).Result()
		require.NoError(t, err)

		// Read messages
		result, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: "error_test_consumer",
			Streams:  []string{streamName, ">"},
			Count:    2,
			Block:    time.Second,
		}).Result()
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Len(t, result[0].Messages, 2)

		// Process messages (simulate error handling)
		processedCount := 0
		errorCount := 0

		for _, msg := range result[0].Messages {
			data, ok := msg.Values["data"].(string)
			require.True(t, ok)

			var msgFormat message.MessageFormat
			if err := json.Unmarshal([]byte(data), &msgFormat); err != nil {
				// Handle malformed message
				errorCount++
				// In real implementation, this would be logged and the message would be acknowledged
				// to prevent reprocessing
				err = redisClient.XAck(ctx, streamName, consumerGroup, msg.ID).Err()
				require.NoError(t, err)
				continue
			}

			// Verify checksum
			if !msgFormat.VerifyChecksum() {
				errorCount++
				err = redisClient.XAck(ctx, streamName, consumerGroup, msg.ID).Err()
				require.NoError(t, err)
				continue
			}

			// Process valid message
			processedCount++
			err = redisClient.XAck(ctx, streamName, consumerGroup, msg.ID).Err()
			require.NoError(t, err)
		}

		assert.Equal(t, 1, processedCount, "Should process one valid message")
		assert.Equal(t, 1, errorCount, "Should handle one error message")

		// Verify no pending messages
		pending, err := redisClient.XPending(ctx, streamName, consumerGroup).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), pending.Count, "All messages should be acknowledged")
	})
}

// Helper function to parse float from string
func parseFloat(s string) float64 {
	if f, err := fmt.Sscanf(s, "%f", new(float64)); err == nil && f == 1 {
		var result float64
		fmt.Sscanf(s, "%f", &result)
		return result
	}
	return 0.0
}
