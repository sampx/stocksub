package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/core"
	"stocksub/pkg/testkit/storage"
)

func main() {
	fmt.Println("=== StructuredData 基本使用示例 ===")

	// 示例1：基本的 StructuredData 创建和操作
	basicExample()

	// 示例2：自定义数据模式
	customSchemaExample()

	// 示例3：数据验证和错误处理
	validationExample()

	// 示例4：与存储系统集成
	storageIntegrationExample()

	// 示例5：批量处理示例
	batchProcessingExample()

	// 示例6：StockData 转换示例
	stockDataConversionExample()
}

// 示例1：基本的 StructuredData 创建和操作
func basicExample() {
	fmt.Println("\n--- 示例1：基本操作 ---")

	// 使用预定义的股票数据模式创建结构化数据
	stockData := subscriber.NewStructuredData(subscriber.StockDataSchema)

	// 设置字段值
	if err := stockData.SetField("symbol", "600000"); err != nil {
		log.Fatalf("设置 symbol 字段失败: %v", err)
	}

	if err := stockData.SetField("name", "浦发银行"); err != nil {
		log.Fatalf("设置 name 字段失败: %v", err)
	}

	if err := stockData.SetField("price", 10.50); err != nil {
		log.Fatalf("设置 price 字段失败: %v", err)
	}

	if err := stockData.SetField("change", 0.15); err != nil {
		log.Fatalf("设置 change 字段失败: %v", err)
	}

	if err := stockData.SetField("volume", int64(1250000)); err != nil {
		log.Fatalf("设置 volume 字段失败: %v", err)
	}

	if err := stockData.SetField("timestamp", time.Now()); err != nil {
		log.Fatalf("设置 timestamp 字段失败: %v", err)
	}

	// 获取字段值
	symbol, err := stockData.GetField("symbol")
	if err != nil {
		log.Fatalf("获取 symbol 字段失败: %v", err)
	}
	fmt.Printf("股票代码: %s\n", symbol)

	price, err := stockData.GetField("price")
	if err != nil {
		log.Fatalf("获取 price 字段失败: %v", err)
	}
	fmt.Printf("当前价格: %.2f\n", price)

	// 验证数据完整性
	if err := stockData.ValidateData(); err != nil {
		log.Fatalf("数据验证失败: %v", err)
	}
	fmt.Println("数据验证通过!")
}

// 示例2：自定义数据模式
func customSchemaExample() {
	fmt.Println("\n--- 示例2：自定义数据模式 ---")

	// 定义自定义的交易记录模式
	tradeSchema := &subscriber.DataSchema{
		Name:        "trade_record",
		Description: "交易记录",
		Fields: map[string]*subscriber.FieldDefinition{
			"trade_id": {
				Name:        "trade_id",
				Type:        subscriber.FieldTypeString,
				Description: "交易ID",
				Required:    true,
			},
			"symbol": {
				Name:        "symbol",
				Type:        subscriber.FieldTypeString,
				Description: "股票代码",
				Required:    true,
			},
			"side": {
				Name:        "side",
				Type:        subscriber.FieldTypeString,
				Description: "买卖方向",
				Required:    true,
				Validator: func(value interface{}) error {
					if str, ok := value.(string); ok {
						if str == "BUY" || str == "SELL" {
							return nil
						}
					}
					return fmt.Errorf("side 必须是 BUY 或 SELL")
				},
			},
			"quantity": {
				Name:        "quantity",
				Type:        subscriber.FieldTypeInt,
				Description: "交易数量",
				Required:    true,
			},
			"price": {
				Name:        "price",
				Type:        subscriber.FieldTypeFloat64,
				Description: "交易价格",
				Required:    true,
			},
			"commission": {
				Name:         "commission",
				Type:         subscriber.FieldTypeFloat64,
				Description:  "佣金",
				Required:     false,
				DefaultValue: 0.0,
			},
			"trade_time": {
				Name:        "trade_time",
				Type:        subscriber.FieldTypeTime,
				Description: "交易时间",
				Required:    true,
			},
		},
		FieldOrder: []string{"trade_id", "symbol", "side", "quantity", "price", "commission", "trade_time"},
	}

	// 验证自定义模式
	if err := subscriber.ValidateSchema(tradeSchema); err != nil {
		log.Fatalf("模式验证失败: %v", err)
	}
	fmt.Println("自定义模式验证通过!")

	// 创建交易记录
	trade := subscriber.NewStructuredData(tradeSchema)

	// 设置交易数据
	trade.SetFieldSafe("trade_id", "T20250825001")
	trade.SetFieldSafe("symbol", "600000")
	trade.SetFieldSafe("side", "BUY")
	trade.SetFieldSafe("quantity", int64(1000))
	trade.SetFieldSafe("price", 10.50)
	trade.SetFieldSafe("trade_time", time.Now())
	// commission 使用默认值

	// 获取佣金（应该是默认值）
	commission, err := trade.GetField("commission")
	if err != nil {
		log.Fatalf("获取佣金失败: %v", err)
	}
	fmt.Printf("佣金: %.2f (默认值)\n", commission)

	fmt.Println("自定义交易记录创建成功!")
}

// 示例3：数据验证和错误处理
func validationExample() {
	fmt.Println("\n--- 示例3：数据验证和错误处理 ---")

	stockData := subscriber.NewStructuredData(subscriber.StockDataSchema)

	// 演示各种验证错误
	fmt.Println("演示验证错误:")

	// 1. 无效字段名
	if err := stockData.SetField("invalid_field", "test"); err != nil {
		if structErr, ok := err.(*subscriber.StructuredDataError); ok {
			fmt.Printf("错误类型: %s, 字段: %s, 消息: %s\n",
				structErr.Code, structErr.Field, structErr.Message)
		}
	}

	// 2. 无效字段类型
	if err := stockData.SetField("price", "not_a_number"); err != nil {
		if structErr, ok := err.(*subscriber.StructuredDataError); ok {
			fmt.Printf("错误类型: %s, 字段: %s, 消息: %s\n",
				structErr.Code, structErr.Field, structErr.Message)
		}
	}

	// 3. 使用安全设置方法（包含范围验证）
	if err := stockData.SetFieldSafe("price", -10.0); err != nil {
		if structErr, ok := err.(*subscriber.StructuredDataError); ok {
			fmt.Printf("错误类型: %s, 字段: %s, 消息: %s\n",
				structErr.Code, structErr.Field, structErr.Message)
		}
	}

	// 4. 正确设置数据
	stockData.SetFieldSafe("symbol", "600000")
	stockData.SetFieldSafe("name", "浦发银行")
	stockData.SetFieldSafe("price", 10.50)
	stockData.SetFieldSafe("timestamp", time.Now())

	fmt.Println("正确的数据设置完成!")

	// 5. 完整数据验证
	if err := stockData.ValidateDataComplete(); err != nil {
		fmt.Printf("完整验证失败: %v\n", err)
	} else {
		fmt.Println("完整数据验证通过!")
	}
}

// 示例4：与存储系统集成
func storageIntegrationExample() {
	fmt.Println("\n--- 示例4：与存储系统集成 ---")

	// 创建内存存储
	config := storage.DefaultMemoryStorageConfig()
	memStorage := storage.NewMemoryStorage(config)
	defer memStorage.Close()

	ctx := context.Background()

	// 创建多个股票数据
	symbols := []string{"600000", "000001", "000002"}
	names := []string{"浦发银行", "平安银行", "万科A"}
	prices := []float64{10.50, 12.80, 25.30}

	for i, symbol := range symbols {
		stockData := subscriber.NewStructuredData(subscriber.StockDataSchema)
		stockData.SetFieldSafe("symbol", symbol)
		stockData.SetFieldSafe("name", names[i])
		stockData.SetFieldSafe("price", prices[i])
		stockData.SetFieldSafe("timestamp", time.Now().Add(time.Duration(i)*time.Minute))

		// 保存到存储系统
		if err := memStorage.Save(ctx, stockData); err != nil {
			log.Fatalf("保存数据失败: %v", err)
		}
	}

	fmt.Printf("成功保存 %d 条股票数据\n", len(symbols))

	// 查询特定股票
	results, err := memStorage.QueryBySymbol(ctx, "600000")
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("查询到 %d 条 600000 的记录\n", len(results))
	if len(results) > 0 {
		result := results[0]
		name, _ := result.GetField("name")
		price, _ := result.GetField("price")
		fmt.Printf("股票信息: %s, 价格: %.2f\n", name, price)
	}

	// 时间范围查询
	now := time.Now()
	timeResults, err := memStorage.QueryByTimeRange(ctx, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	if err != nil {
		log.Fatalf("时间范围查询失败: %v", err)
	}
	fmt.Printf("时间范围内查询到 %d 条记录\n", len(timeResults))

	// 获取存储统计信息
	stats := memStorage.GetStats()
	fmt.Printf("存储统计: 总记录数=%d, 总表数=%d, 索引数=%d\n",
		stats.TotalRecords, stats.TotalTables, stats.IndexCount)
}

// 示例5：批量处理示例
func batchProcessingExample() {
	fmt.Println("\n--- 示例5：批量处理示例 ---")

	// 创建内存存储
	memConfig := storage.DefaultMemoryStorageConfig()
	memStorage := storage.NewMemoryStorage(memConfig)
	defer memStorage.Close()

	// 创建优化的批量写入器
	batchConfig := storage.OptimizedBatchWriterConfig()
	batchWriter := storage.NewBatchWriter(memStorage, batchConfig)
	defer batchWriter.Close()

	ctx := context.Background()

	// 批量创建股票数据
	fmt.Println("批量创建股票数据...")
	var stockDataList []interface{}

	for i := 0; i < 10; i++ {
		stockData := subscriber.NewStructuredData(subscriber.StockDataSchema)
		stockData.SetFieldSafe("symbol", fmt.Sprintf("60000%d", i))
		stockData.SetFieldSafe("name", fmt.Sprintf("测试股票%d", i))
		stockData.SetFieldSafe("price", float64(10+i))
		stockData.SetFieldSafe("volume", int64(1000000*(i+1)))
		stockData.SetFieldSafe("timestamp", time.Now().Add(time.Duration(i)*time.Second))

		stockDataList = append(stockDataList, stockData)
	}

	// 批量写入
	start := time.Now()
	for _, data := range stockDataList {
		if err := batchWriter.Write(ctx, data); err != nil {
			log.Fatalf("批量写入失败: %v", err)
		}
	}

	// 确保所有数据都已刷新
	if err := batchWriter.Flush(); err != nil {
		log.Fatalf("刷新失败: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("批量写入 %d 条记录耗时: %v\n", len(stockDataList), duration)

	// 获取批量写入统计信息
	batchStats := batchWriter.GetStats()
	fmt.Printf("批量写入统计: 总记录=%d, StructuredData记录=%d, 批次数=%d\n",
		batchStats.TotalRecords, batchStats.StructuredDataRecords, batchStats.StructuredDataBatches)

	// 验证数据
	totalResults, err := memStorage.Load(ctx, core.Query{})
	if err != nil {
		log.Fatalf("加载数据失败: %v", err)
	}
	fmt.Printf("验证: 存储中共有 %d 条记录\n", len(totalResults))
}

// 示例6：StockData 转换示例
func stockDataConversionExample() {
	fmt.Println("\n--- 示例6：StockData 转换示例 ---")

	// 创建传统的 StockData
	originalStock := subscriber.StockData{
		Symbol:        "600000",
		Name:          "浦发银行",
		Price:         10.50,
		Change:        0.15,
		ChangePercent: 1.45,
		Volume:        1250000,
		Open:          10.35,
		High:          10.55,
		Low:           10.30,
		PrevClose:     10.35,
		Timestamp:     time.Now(),
	}

	fmt.Printf("原始 StockData: %s %s %.2f\n",
		originalStock.Symbol, originalStock.Name, originalStock.Price)

	// 转换为 StructuredData
	structuredData, err := subscriber.StockDataToStructuredData(originalStock)
	if err != nil {
		log.Fatalf("转换为 StructuredData 失败: %v", err)
	}

	fmt.Println("成功转换为 StructuredData!")

	// 从 StructuredData 获取数据
	symbol, _ := structuredData.GetField("symbol")
	name, _ := structuredData.GetField("name")
	price, _ := structuredData.GetField("price")
	fmt.Printf("StructuredData 中的数据: %s %s %.2f\n", symbol, name, price)

	// 转换回 StockData
	convertedStock, err := subscriber.StructuredDataToStockData(structuredData)
	if err != nil {
		log.Fatalf("转换回 StockData 失败: %v", err)
	}

	fmt.Printf("转换回的 StockData: %s %s %.2f\n",
		convertedStock.Symbol, convertedStock.Name, convertedStock.Price)

	// 验证转换的准确性
	if originalStock.Symbol == convertedStock.Symbol &&
		originalStock.Name == convertedStock.Name &&
		originalStock.Price == convertedStock.Price {
		fmt.Println("转换成功且数据一致!")
	} else {
		fmt.Println("转换后数据不一致!")
	}
}
