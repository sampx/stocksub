package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stocksub/pkg/subscriber"
	"stocksub/pkg/testkit/storage"
)

func main() {
	fmt.Println("=== StructuredData 高级使用示例 ===")

	// 高级示例1：复杂数据模式设计
	complexSchemaExample()

	// 高级示例2：数据流水线处理
	dataPipelineExample()

	// 高级示例3：性能优化和监控
	performanceMonitoringExample()

	// 高级示例4：多模式数据管理
	multiSchemaManagementExample()

	// 高级示例5：错误恢复和容错机制
	errorRecoveryExample()
}

// 高级示例1：复杂数据模式设计
func complexSchemaExample() {
	fmt.Println("\n--- 高级示例1：复杂数据模式设计 ---")

	// 设计一个复杂的期权数据模式
	optionSchema := &subscriber.DataSchema{
		Name:        "option_data",
		Description: "期权数据模式",
		Fields: map[string]*subscriber.FieldDefinition{
			"option_code": {
				Name:        "option_code",
				Type:        subscriber.FieldTypeString,
				Description: "期权合约代码",
				Required:    true,
				Validator: func(value interface{}) error {
					if str, ok := value.(string); ok && len(str) >= 8 {
						return nil
					}
					return fmt.Errorf("期权代码长度必须至少8位")
				},
			},
			"underlying_asset": {
				Name:        "underlying_asset",
				Type:        subscriber.FieldTypeString,
				Description: "标的资产",
				Required:    true,
			},
			"option_type": {
				Name:        "option_type",
				Type:        subscriber.FieldTypeString,
				Description: "期权类型",
				Required:    true,
				Validator: func(value interface{}) error {
					if str, ok := value.(string); ok {
						if str == "CALL" || str == "PUT" {
							return nil
						}
					}
					return fmt.Errorf("期权类型必须是 CALL 或 PUT")
				},
			},
			"strike_price": {
				Name:        "strike_price",
				Type:        subscriber.FieldTypeFloat64,
				Description: "行权价格",
				Required:    true,
			},
			"expiry_date": {
				Name:        "expiry_date",
				Type:        subscriber.FieldTypeTime,
				Description: "到期日",
				Required:    true,
			},
			"current_price": {
				Name:        "current_price",
				Type:        subscriber.FieldTypeFloat64,
				Description: "当前价格",
				Required:    true,
			},
			"implied_volatility": {
				Name:        "implied_volatility",
				Type:        subscriber.FieldTypeFloat64,
				Description: "隐含波动率",
				Required:    false,
				DefaultValue: 0.0,
			},
			"delta": {
				Name:        "delta",
				Type:        subscriber.FieldTypeFloat64,
				Description: "Delta值",
				Required:    false,
			},
			"gamma": {
				Name:        "gamma",
				Type:        subscriber.FieldTypeFloat64,
				Description: "Gamma值",
				Required:    false,
			},
			"theta": {
				Name:        "theta",
				Type:        subscriber.FieldTypeFloat64,
				Description: "Theta值",
				Required:    false,
			},
			"vega": {
				Name:        "vega",
				Type:        subscriber.FieldTypeFloat64,
				Description: "Vega值",
				Required:    false,
			},
			"open_interest": {
				Name:        "open_interest",
				Type:        subscriber.FieldTypeInt,
				Description: "持仓量",
				Required:    false,
				DefaultValue: int64(0),
			},
			"trading_volume": {
				Name:        "trading_volume",
				Type:        subscriber.FieldTypeInt,
				Description: "成交量",
				Required:    false,
				DefaultValue: int64(0),
			},
		},
		FieldOrder: []string{
			"option_code", "underlying_asset", "option_type", "strike_price", "expiry_date",
			"current_price", "implied_volatility", "delta", "gamma", "theta", "vega",
			"open_interest", "trading_volume",
		},
	}

	// 验证复杂模式
	if err := subscriber.ValidateSchema(optionSchema); err != nil {
		log.Fatalf("复杂模式验证失败: %v", err)
	}

	// 创建期权数据实例
	optionData := subscriber.NewStructuredData(optionSchema)

	// 设置期权数据
	expiryDate := time.Now().AddDate(0, 3, 0) // 3个月后到期
	
	optionData.SetFieldSafe("option_code", "50ETF2503C3000")
	optionData.SetFieldSafe("underlying_asset", "510050")
	optionData.SetFieldSafe("option_type", "CALL")
	optionData.SetFieldSafe("strike_price", 3.0)
	optionData.SetFieldSafe("expiry_date", expiryDate)
	optionData.SetFieldSafe("current_price", 0.0850)
	optionData.SetFieldSafe("implied_volatility", 0.25)
	optionData.SetFieldSafe("delta", 0.4235)
	optionData.SetFieldSafe("gamma", 1.2345)
	optionData.SetFieldSafe("theta", -0.0123)
	optionData.SetFieldSafe("vega", 0.0567)
	optionData.SetFieldSafe("open_interest", int64(12500))
	optionData.SetFieldSafe("trading_volume", int64(856))

	// 验证数据
	if err := optionData.ValidateDataComplete(); err != nil {
		log.Fatalf("期权数据验证失败: %v", err)
	}

	fmt.Println("复杂期权数据模式创建并验证成功!")

	// 展示希腊字母信息
	delta, _ := optionData.GetField("delta")
	gamma, _ := optionData.GetField("gamma")
	theta, _ := optionData.GetField("theta")
	vega, _ := optionData.GetField("vega")
	
	fmt.Printf("希腊字母 - Delta: %.4f, Gamma: %.4f, Theta: %.4f, Vega: %.4f\n", 
		delta, gamma, theta, vega)
}

// 高级示例2：数据流水线处理
func dataPipelineExample() {
	fmt.Println("\n--- 高级示例2：数据流水线处理 ---")

	// 创建存储系统
	memConfig := storage.DefaultMemoryStorageConfig()
	memStorage := storage.NewMemoryStorage(memConfig)
	defer memStorage.Close()

	// 创建高性能批量写入器
	batchConfig := storage.BatchWriterConfig{
		BatchSize:                 50,
		FlushInterval:             1 * time.Second,
		MaxBufferSize:             500,
		EnableAsync:               true,
		EnableStructuredDataOptim: true,
		StructuredDataBatchSize:   25,
		StructuredDataFlushDelay:  500 * time.Millisecond,
	}
	batchWriter := storage.NewBatchWriter(memStorage, batchConfig)
	defer batchWriter.Close()

	ctx := context.Background()

	// 模拟实时数据流
	fmt.Println("启动模拟数据流水线...")
	
	// 数据生成器
	dataChannel := make(chan *subscriber.StructuredData, 100)
	errorChannel := make(chan error, 10)

	// 启动数据生成goroutine
	go func() {
		defer close(dataChannel)
		
		for i := 0; i < 100; i++ {
			stockData := subscriber.NewStructuredData(subscriber.StockDataSchema)
			
			symbol := fmt.Sprintf("60%04d", i%50) // 模拟50只股票
			name := fmt.Sprintf("股票%d", i%50)
			price := 10.0 + float64(i%100)*0.1 // 价格在10.0-19.9之间变动
			volume := int64(1000000 + i*10000)
			
			stockData.SetFieldSafe("symbol", symbol)
			stockData.SetFieldSafe("name", name)
			stockData.SetFieldSafe("price", price)
			stockData.SetFieldSafe("volume", volume)
			stockData.SetFieldSafe("timestamp", time.Now().Add(time.Duration(i)*time.Millisecond))

			dataChannel <- stockData
			
			// 模拟数据流延迟
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// 启动数据处理goroutine
	go func() {
		defer close(errorChannel)
		
		for stockData := range dataChannel {
			// 数据预处理：添加计算字段
			price, _ := stockData.GetField("price")
			volume, _ := stockData.GetField("volume")
			
			// 计算成交额
			turnover := price.(float64) * float64(volume.(int64))
			stockData.SetFieldSafe("turnover", turnover)
			
			// 写入存储
			if err := batchWriter.Write(ctx, stockData); err != nil {
				errorChannel <- err
				continue
			}
		}
	}()

	// 监控处理结果
	processed := 0
	errors := 0

	// 等待数据处理完成
	for {
		select {
		case err := <-errorChannel:
			if err != nil {
				errors++
				fmt.Printf("处理错误: %v\n", err)
			} else {
				// errorChannel 已关闭
				goto done
			}
		case <-time.After(5 * time.Second):
			// 超时保护
			goto done
		}
	}

done:
	// 确保所有数据都已刷新
	batchWriter.Flush()

	// 统计处理结果
	batchStats := batchWriter.GetStats()
	processed = int(batchStats.StructuredDataRecords)
	
	fmt.Printf("数据流水线处理完成: 成功处理 %d 条记录, 错误 %d 条\n", processed, errors)
	fmt.Printf("批量写入统计: 总批次 %d, 总刷新次数 %d\n", 
		batchStats.StructuredDataBatches, batchStats.StructuredDataFlushes)
}

// 高级示例3：性能优化和监控
func performanceMonitoringExample() {
	fmt.Println("\n--- 高级示例3：性能优化和监控 ---")

	// 创建多个存储实例进行性能比较
	configs := []struct {
		name   string
		config storage.MemoryStorageConfig
	}{
		{
			name: "标准配置",
			config: storage.MemoryStorageConfig{
				MaxRecords:      1000,
				EnableIndex:     true,
				TTL:             0,
				CleanupInterval: 0,
			},
		},
		{
			name: "高容量配置",
			config: storage.MemoryStorageConfig{
				MaxRecords:      10000,
				EnableIndex:     true,
				TTL:             0,
				CleanupInterval: 0,
			},
		},
		{
			name: "无索引配置",
			config: storage.MemoryStorageConfig{
				MaxRecords:      1000,
				EnableIndex:     false,
				TTL:             0,
				CleanupInterval: 0,
			},
		},
	}

	ctx := context.Background()

	for _, cfg := range configs {
		fmt.Printf("\n测试配置: %s\n", cfg.name)
		
		ms := storage.NewMemoryStorage(cfg.config)
		
		// 准备测试数据
		testData := make([]*subscriber.StructuredData, 500)
		for i := 0; i < 500; i++ {
			stockData := subscriber.NewStructuredData(subscriber.StockDataSchema)
			stockData.SetFieldSafe("symbol", fmt.Sprintf("60%04d", i%100))
			stockData.SetFieldSafe("name", fmt.Sprintf("股票%d", i%100))
			stockData.SetFieldSafe("price", 10.0+float64(i%50)*0.5)
			stockData.SetFieldSafe("volume", int64(1000000+i*1000))
			stockData.SetFieldSafe("timestamp", time.Now().Add(time.Duration(i)*time.Minute))
			testData[i] = stockData
		}

		// 测试批量保存性能
		start := time.Now()
		
		var dataList []interface{}
		for _, data := range testData {
			dataList = append(dataList, data)
		}
		
		err := ms.BatchSave(ctx, dataList)
		if err != nil {
			log.Printf("批量保存失败: %v", err)
			continue
		}
		
		saveTime := time.Since(start)

		// 测试查询性能
		start = time.Now()
		
		// 查询测试
		queryCount := 50
		for i := 0; i < queryCount; i++ {
			symbol := fmt.Sprintf("60%04d", i%100)
			results, err := ms.QueryBySymbol(ctx, symbol)
			if err != nil {
				log.Printf("查询失败: %v", err)
				continue
			}
			_ = results // 使用结果
		}
		
		queryTime := time.Since(start)

		// 获取统计信息
		stats := ms.GetStats()
		
		fmt.Printf("  保存性能: %d 条记录耗时 %v (%.2f 条/秒)\n", 
			len(testData), saveTime, float64(len(testData))/saveTime.Seconds())
		fmt.Printf("  查询性能: %d 次查询耗时 %v (%.2f 查询/秒)\n", 
			queryCount, queryTime, float64(queryCount)/queryTime.Seconds())
		fmt.Printf("  存储统计: 记录数=%d, 表数=%d, 索引数=%d\n", 
			stats.TotalRecords, stats.TotalTables, stats.IndexCount)

		ms.Close()
	}
}

// 高级示例4：多模式数据管理
func multiSchemaManagementExample() {
	fmt.Println("\n--- 高级示例4：多模式数据管理 ---")

	// 创建存储系统
	memStorage := storage.NewMemoryStorage(storage.DefaultMemoryStorageConfig())
	defer memStorage.Close()

	batchWriter := storage.NewBatchWriter(memStorage, storage.OptimizedBatchWriterConfig())
	defer batchWriter.Close()

	ctx := context.Background()

	// 定义多个数据模式
	schemas := map[string]*subscriber.DataSchema{
		"stock": subscriber.StockDataSchema,
		"trade": createTradeSchema(),
		"order": createOrderSchema(),
		"news":  createNewsSchema(),
	}

	// 为每种模式创建测试数据
	allData := make([]interface{}, 0)

	// 股票数据
	for i := 0; i < 10; i++ {
		stockData := subscriber.NewStructuredData(schemas["stock"])
		stockData.SetFieldSafe("symbol", fmt.Sprintf("60000%d", i))
		stockData.SetFieldSafe("name", fmt.Sprintf("股票%d", i))
		stockData.SetFieldSafe("price", 10.0+float64(i))
		stockData.SetFieldSafe("timestamp", time.Now())
		allData = append(allData, stockData)
	}

	// 交易数据
	for i := 0; i < 5; i++ {
		tradeData := subscriber.NewStructuredData(schemas["trade"])
		tradeData.SetFieldSafe("trade_id", fmt.Sprintf("T%05d", i))
		tradeData.SetFieldSafe("symbol", fmt.Sprintf("60000%d", i%3))
		tradeData.SetFieldSafe("side", []string{"BUY", "SELL"}[i%2])
		tradeData.SetFieldSafe("quantity", int64(1000*(i+1)))
		tradeData.SetFieldSafe("price", 10.0+float64(i)*0.5)
		tradeData.SetFieldSafe("trade_time", time.Now())
		allData = append(allData, tradeData)
	}

	// 订单数据
	for i := 0; i < 8; i++ {
		orderData := subscriber.NewStructuredData(schemas["order"])
		orderData.SetFieldSafe("order_id", fmt.Sprintf("O%05d", i))
		orderData.SetFieldSafe("symbol", fmt.Sprintf("60000%d", i%4))
		orderData.SetFieldSafe("side", []string{"BUY", "SELL"}[i%2])
		orderData.SetFieldSafe("quantity", int64(500*(i+1)))
		orderData.SetFieldSafe("price", 10.0+float64(i)*0.3)
		orderData.SetFieldSafe("status", "PENDING")
		orderData.SetFieldSafe("order_time", time.Now())
		allData = append(allData, orderData)
	}

	// 新闻数据
	for i := 0; i < 3; i++ {
		newsData := subscriber.NewStructuredData(schemas["news"])
		newsData.SetFieldSafe("news_id", fmt.Sprintf("N%05d", i))
		newsData.SetFieldSafe("title", fmt.Sprintf("重要新闻%d", i))
		newsData.SetFieldSafe("content", fmt.Sprintf("这是第%d条重要新闻的内容...", i))
		newsData.SetFieldSafe("source", "财经网")
		newsData.SetFieldSafe("publish_time", time.Now())
		allData = append(allData, newsData)
	}

	// 批量写入所有数据
	fmt.Printf("写入多模式数据: 总计 %d 条记录\n", len(allData))
	
	for _, data := range allData {
		if err := batchWriter.Write(ctx, data); err != nil {
			log.Printf("写入失败: %v", err)
		}
	}

	batchWriter.Flush()

	// 获取最终统计
	batchStats := batchWriter.GetStats()
	memStats := memStorage.GetStats()

	fmt.Printf("多模式数据写入完成:\n")
	fmt.Printf("  批量写入统计: 总记录=%d, StructuredData记录=%d\n", 
		batchStats.TotalRecords, batchStats.StructuredDataRecords)
	fmt.Printf("  内存存储统计: 总记录=%d, 总表数=%d\n", 
		memStats.TotalRecords, memStats.TotalTables)
	fmt.Printf("  期望的表数量: %d (每个模式一个表)\n", len(schemas))
}

// 高级示例5：错误恢复和容错机制
func errorRecoveryExample() {
	fmt.Println("\n--- 高级示例5：错误恢复和容错机制 ---")

	memStorage := storage.NewMemoryStorage(storage.DefaultMemoryStorageConfig())
	defer memStorage.Close()

	// 创建容错配置的批量写入器
	tolerantConfig := storage.BatchWriterConfig{
		BatchSize:                 10,
		FlushInterval:             1 * time.Second,
		MaxBufferSize:             100,
		EnableAsync:               false, // 同步模式便于错误处理
		EnableStructuredDataOptim: true,
		StructuredDataBatchSize:   5,
		StructuredDataFlushDelay:  200 * time.Millisecond,
	}
	
	batchWriter := storage.NewBatchWriter(memStorage, tolerantConfig)
	defer batchWriter.Close()

	ctx := context.Background()

	// 创建混合数据：正常数据和异常数据
	testData := []interface{}{
		// 正常的 StructuredData
		createValidStockData("600000", "正常股票1", 10.50),
		createValidStockData("600001", "正常股票2", 12.80),
		
		// 异常的 StructuredData (缺少 schema)
		&subscriber.StructuredData{
			Schema:    nil, // 这会导致错误
			Values:    map[string]interface{}{"symbol": "600002"},
			Timestamp: time.Now(),
		},
		
		// 更多正常数据
		createValidStockData("600003", "正常股票3", 15.20),
		createValidStockData("600004", "正常股票4", 8.90),
		
		// 普通数据（非StructuredData）
		map[string]interface{}{
			"type": "misc",
			"data": "普通数据",
		},
	}

	// 容错写入处理
	successCount := 0
	errorCount := 0

	fmt.Println("开始容错写入测试...")
	
	for i, data := range testData {
		err := batchWriter.Write(ctx, data)
		if err != nil {
			errorCount++
			fmt.Printf("数据 %d 写入失败: %v\n", i, err)
			// 容错机制：记录错误但继续处理
			continue
		}
		successCount++
	}

	// 强制刷新以确保所有有效数据都被保存
	if err := batchWriter.Flush(); err != nil {
		fmt.Printf("刷新时出现错误: %v\n", err)
	}

	fmt.Printf("容错写入完成: 成功 %d 条, 失败 %d 条\n", successCount, errorCount)

	// 验证成功保存的数据
	allResults, err := memStorage.Load(ctx, storage.Query{})
	if err != nil {
		log.Printf("验证查询失败: %v", err)
		return
	}

	fmt.Printf("最终存储验证: 共保存 %d 条有效数据\n", len(allResults))

	// 显示每条保存的数据信息
	for i, result := range allResults {
		switch data := result.(type) {
		case *subscriber.StructuredData:
			symbol, _ := data.GetField("symbol")
			name, _ := data.GetField("name")
			fmt.Printf("  记录 %d: StructuredData - %s %s\n", i+1, symbol, name)
		default:
			fmt.Printf("  记录 %d: 其他类型数据 - %T\n", i+1, data)
		}
	}

	// 获取最终统计信息
	batchStats := batchWriter.GetStats()
	fmt.Printf("容错统计: 总刷新错误 %d 次\n", batchStats.FlushErrors)
}

// 辅助函数：创建交易模式
func createTradeSchema() *subscriber.DataSchema {
	return &subscriber.DataSchema{
		Name:        "trade",
		Description: "交易记录",
		Fields: map[string]*subscriber.FieldDefinition{
			"trade_id": {
				Name: "trade_id", Type: subscriber.FieldTypeString,
				Description: "交易ID", Required: true,
			},
			"symbol": {
				Name: "symbol", Type: subscriber.FieldTypeString,
				Description: "股票代码", Required: true,
			},
			"side": {
				Name: "side", Type: subscriber.FieldTypeString,
				Description: "买卖方向", Required: true,
			},
			"quantity": {
				Name: "quantity", Type: subscriber.FieldTypeInt,
				Description: "数量", Required: true,
			},
			"price": {
				Name: "price", Type: subscriber.FieldTypeFloat64,
				Description: "价格", Required: true,
			},
			"trade_time": {
				Name: "trade_time", Type: subscriber.FieldTypeTime,
				Description: "交易时间", Required: true,
			},
		},
		FieldOrder: []string{"trade_id", "symbol", "side", "quantity", "price", "trade_time"},
	}
}

// 辅助函数：创建订单模式
func createOrderSchema() *subscriber.DataSchema {
	return &subscriber.DataSchema{
		Name:        "order",
		Description: "订单记录",
		Fields: map[string]*subscriber.FieldDefinition{
			"order_id": {
				Name: "order_id", Type: subscriber.FieldTypeString,
				Description: "订单ID", Required: true,
			},
			"symbol": {
				Name: "symbol", Type: subscriber.FieldTypeString,
				Description: "股票代码", Required: true,
			},
			"side": {
				Name: "side", Type: subscriber.FieldTypeString,
				Description: "买卖方向", Required: true,
			},
			"quantity": {
				Name: "quantity", Type: subscriber.FieldTypeInt,
				Description: "数量", Required: true,
			},
			"price": {
				Name: "price", Type: subscriber.FieldTypeFloat64,
				Description: "价格", Required: true,
			},
			"status": {
				Name: "status", Type: subscriber.FieldTypeString,
				Description: "订单状态", Required: true,
			},
			"order_time": {
				Name: "order_time", Type: subscriber.FieldTypeTime,
				Description: "下单时间", Required: true,
			},
		},
		FieldOrder: []string{"order_id", "symbol", "side", "quantity", "price", "status", "order_time"},
	}
}

// 辅助函数：创建新闻模式
func createNewsSchema() *subscriber.DataSchema {
	return &subscriber.DataSchema{
		Name:        "news",
		Description: "新闻数据",
		Fields: map[string]*subscriber.FieldDefinition{
			"news_id": {
				Name: "news_id", Type: subscriber.FieldTypeString,
				Description: "新闻ID", Required: true,
			},
			"title": {
				Name: "title", Type: subscriber.FieldTypeString,
				Description: "标题", Required: true,
			},
			"content": {
				Name: "content", Type: subscriber.FieldTypeString,
				Description: "内容", Required: true,
			},
			"source": {
				Name: "source", Type: subscriber.FieldTypeString,
				Description: "来源", Required: true,
			},
			"publish_time": {
				Name: "publish_time", Type: subscriber.FieldTypeTime,
				Description: "发布时间", Required: true,
			},
		},
		FieldOrder: []string{"news_id", "title", "content", "source", "publish_time"},
	}
}

// 辅助函数：创建有效的股票数据
func createValidStockData(symbol, name string, price float64) *subscriber.StructuredData {
	stockData := subscriber.NewStructuredData(subscriber.StockDataSchema)
	stockData.SetFieldSafe("symbol", symbol)
	stockData.SetFieldSafe("name", name)
	stockData.SetFieldSafe("price", price)
	stockData.SetFieldSafe("timestamp", time.Now())
	return stockData
}