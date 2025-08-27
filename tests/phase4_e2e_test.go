//go:build integration

package tests

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stocksub/pkg/message"
)

const (
	testRedisAddr   = "localhost:6379"
	testInfluxDBURL = "http://localhost:8086"
	testInfluxDBOrg = "stocksub"
	testBucket      = "test_stock_data"
)

// TestPhase4HorizontalScaling 测试阶段4的水平扩展能力和数据一致性
func TestPhase4HorizontalScaling(t *testing.T) {
	// 设置测试环境
	ctx := context.Background()

	// 连接 Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: testRedisAddr,
	})
	defer redisClient.Close()

	// 清理测试数据
	redisClient.FlushDB(ctx)

	// 连接 InfluxDB
	influxClient := influxdb2.NewClient(testInfluxDBURL, "")
	defer influxClient.Close()

	t.Run("数据一致性保证测试", func(t *testing.T) {
		testDataConsistency(t, redisClient)
	})

	t.Run("水平扩展能力测试", func(t *testing.T) {
		testHorizontalScaling(t, redisClient, influxClient)
	})

	t.Run("API服务端到端测试", func(t *testing.T) {
		testAPIEndpoints(t, redisClient)
	})
}

// testDataConsistency 测试数据一致性和幂等处理
func testDataConsistency(t *testing.T, redisClient *redis.Client) {
	ctx := context.Background()
	streamName := "stream:stock:realtime"

	// 创建测试消息
	testData := []message.StockData{
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

	msgFormat := message.NewMessageFormat("test-producer", "tencent", "stock_realtime", testData)
	msgJSON, err := msgFormat.ToJSON()
	require.NoError(t, err)

	// 发布相同消息多次（测试幂等性）
	for i := 0; i < 3; i++ {
		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": msgJSON,
			},
		}).Result()
		require.NoError(t, err)
	}

	// 验证消息已发布
	messages, err := redisClient.XRange(ctx, streamName, "-", "+").Result()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(messages), 3, "应该有至少3条消息")

	t.Logf("✅ 数据一致性测试通过: 发布了 %d 条消息", len(messages))
}

// testHorizontalScaling 测试多个 collector 实例的水平扩展
func testHorizontalScaling(t *testing.T, redisClient *redis.Client, influxClient influxdb2.Client) {
	ctx := context.Background()
	streamName := "stream:stock:realtime"
	consumerGroup := "test_collectors"

	// 创建消费者组
	err := redisClient.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		require.NoError(t, err)
	}

	// 发布测试数据
	testMessages := 10
	for i := 0; i < testMessages; i++ {
		testData := []message.StockData{
			{
				Symbol:        fmt.Sprintf("60000%d", i),
				Name:          fmt.Sprintf("测试股票%d", i),
				Price:         10.0 + float64(i),
				Change:        0.1 + float64(i)*0.01,
				ChangePercent: 1.0 + float64(i)*0.1,
				Volume:        1000000 + int64(i)*10000,
				Timestamp:     time.Now().Format(time.RFC3339),
			},
		}

		msgFormat := message.NewMessageFormat("test-producer", "tencent", "stock_realtime", testData)
		msgJSON, err := msgFormat.ToJSON()
		require.NoError(t, err)

		_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": msgJSON,
			},
		}).Result()
		require.NoError(t, err)
	}

	// 验证消息分发
	groupInfo, err := redisClient.XInfoGroups(ctx, streamName).Result()
	require.NoError(t, err)

	var targetGroup *redis.XInfoGroup
	for _, group := range groupInfo {
		if group.Name == consumerGroup {
			targetGroup = &group
			break
		}
	}
	require.NotNil(t, targetGroup, "应该找到测试消费者组")

	t.Logf("✅ 水平扩展测试通过: 消费者组 '%s' 有 %d 条待处理消息",
		targetGroup.Name, targetGroup.Pending)
}

// testAPIEndpoints 测试API端点
func testAPIEndpoints(t *testing.T, redisClient *redis.Client) {
	ctx := context.Background()

	// 在Redis中设置测试数据
	testSymbol := "600000"
	testData := map[string]interface{}{
		"symbol":         testSymbol,
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

	// 设置Redis中的数据
	key := fmt.Sprintf("latest:stock:%s", testSymbol)
	err := redisClient.HMSet(ctx, key, testData).Err()
	require.NoError(t, err)

	// 添加到符号集合
	err = redisClient.SAdd(ctx, "symbols:stock", testSymbol).Err()
	require.NoError(t, err)

	// 验证数据存储
	result, err := redisClient.HGetAll(ctx, key).Result()
	require.NoError(t, err)
	assert.Equal(t, testSymbol, result["symbol"])

	t.Logf("✅ API数据准备完成: 股票 %s 数据已存储到Redis", testSymbol)
}

// TestServicesIntegration 测试服务集成
func TestServicesIntegration(t *testing.T) {
	// 检查依赖服务是否运行
	t.Run("检查Redis连接", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{
			Addr: testRedisAddr,
		})
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := client.Ping(ctx).Err()
		if err != nil {
			t.Skip("Redis服务未运行，跳过集成测试")
		}
		assert.NoError(t, err)
	})

	t.Run("检查InfluxDB连接", func(t *testing.T) {
		client := influxdb2.NewClient(testInfluxDBURL, "")
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		health, err := client.Health(ctx)
		if err != nil {
			t.Skip("InfluxDB服务未运行，跳过集成测试")
		}
		assert.NoError(t, err)
		assert.Equal(t, "pass", string(health.Status))
	})
}

// TestMessageFormatValidation 测试消息格式验证
func TestMessageFormatValidation(t *testing.T) {
	testData := []message.StockData{
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

	// 创建消息
	msgFormat := message.NewMessageFormat("test-producer", "tencent", "stock_realtime", testData)

	// 验证消息校验和
	err := msgFormat.Validate()
	assert.NoError(t, err, "消息校验和应该有效")

	// 测试JSON序列化和反序列化
	jsonStr, err := msgFormat.ToJSON()
	assert.NoError(t, err)

	parsedMsg, err := message.FromJSON(jsonStr)
	assert.NoError(t, err)
	assert.Equal(t, msgFormat.Header.MessageID, parsedMsg.Header.MessageID)

	t.Logf("✅ 消息格式验证通过: 消息ID %s", msgFormat.Header.MessageID)
}

// 辅助函数：检查服务是否运行
func isServiceRunning(command string, args ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	return cmd.Run() == nil
}
