package main

import (
	"fmt"
	"log"

	"stocksub/pkg/storage"
)

func csvDeserializationDemo() {
	fmt.Println("=== CSV 反序列化功能演示 ===")

	// 创建CSV序列化器
	serializer := storage.NewStructuredDataSerializer(storage.FormatCSV)

	// 模拟从文件读取的CSV数据（包含中文表头）
	csvData := `股票代码(symbol),股票名称(name),当前价格(price),涨跌额(change),涨跌幅(%)(change_percent),成交量(volume),数据时间(timestamp)
600000,浦发银行,10.50,0.15,1.45,1250000,2025-08-24 18:30:00
000001,平安银行,12.80,-0.05,-0.39,980000,2025-08-24 18:35:00
600036,招商银行,45.20,1.20,2.73,2100000,2025-08-24 18:40:00`

	fmt.Println("原始CSV数据:")
	fmt.Println(csvData)
	fmt.Println()

	// 1. 批量反序列化CSV数据
	fmt.Println("1. 批量反序列化CSV数据:")
	dataList, err := serializer.DeserializeMultiple([]byte(csvData), storage.StockDataSchema)
	if err != nil {
		log.Fatalf("反序列化失败: %v", err)
	}

	fmt.Printf("成功反序列化 %d 条记录:\n", len(dataList))
	for i, sd := range dataList {
		symbol, _ := sd.GetField("symbol")
		name, _ := sd.GetField("name")
		price, _ := sd.GetField("price")
		change, _ := sd.GetField("change")
		volume, _ := sd.GetField("volume")
		timestamp, _ := sd.GetField("timestamp")

		fmt.Printf("记录 %d: %s (%s) - 价格: %.2f, 涨跌: %.2f, 成交量: %d, 时间: %v\n",
			i+1, symbol, name, price, change, volume, timestamp)
	}
	fmt.Println()

	// 2. 单条记录反序列化演示
	fmt.Println("2. 单条记录反序列化演示:")
	singleCSV := `股票代码(symbol),股票名称(name),当前价格(price),数据时间(timestamp)
600519,贵州茅台,1680.50,2025-08-24 19:00:00`

	singleData := storage.NewStructuredData(storage.StockDataSchema)
	err = serializer.Deserialize([]byte(singleCSV), singleData)
	if err != nil {
		log.Fatalf("单条反序列化失败: %v", err)
	}

	symbol, _ := singleData.GetField("symbol")
	name, _ := singleData.GetField("name")
	price, _ := singleData.GetField("price")
	timestamp, _ := singleData.GetField("timestamp")

	fmt.Printf("单条记录: %s (%s) - 价格: %.2f, 时间: %v\n", symbol, name, price, timestamp)
	fmt.Println()

	// 3. 错误处理演示
	fmt.Println("3. 错误处理演示:")

	// 3.1 未知字段错误
	fmt.Println("3.1 未知字段错误:")
	invalidCSV := `symbol,unknown_field,price
600000,test,10.50`

	_, err = serializer.DeserializeMultiple([]byte(invalidCSV), storage.StockDataSchema)
	if err != nil {
		fmt.Printf("预期错误: %v\n", err)
	}

	// 3.2 数据类型错误
	fmt.Println("3.2 数据类型错误:")
	typeErrorCSV := `股票代码(symbol),当前价格(price)
600000,invalid_price`

	_, err = serializer.DeserializeMultiple([]byte(typeErrorCSV), storage.StockDataSchema)
	if err != nil {
		fmt.Printf("预期错误: %v\n", err)
	}
	fmt.Println()

	// 4. 时区处理演示
	fmt.Println("4. 时区处理演示:")
	timeCSV := `股票代码(symbol),数据时间(timestamp)
600000,2025-08-24 18:30:00`

	timeDataList, err := serializer.DeserializeMultiple([]byte(timeCSV), storage.StockDataSchema)
	if err != nil {
		log.Fatalf("时间反序列化失败: %v", err)
	}

	timeData := timeDataList[0]
	timestamp, _ = timeData.GetField("timestamp")
	fmt.Printf("解析的时间 (上海时区): %v\n", timestamp)

	fmt.Println("\n=== CSV 反序列化功能演示完成 ===")
}
