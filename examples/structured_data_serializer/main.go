package main

import (
	"fmt"
	"log"
	"time"

	"stocksub/pkg/subscriber"
)

func main() {
	fmt.Println("=== StructuredDataSerializer 示例 ===")

	// 1. 创建结构化数据
	sd := subscriber.NewStructuredData(subscriber.StockDataSchema)

	// 设置字段值
	sd.SetField("symbol", "600000")
	sd.SetField("name", "浦发银行")
	sd.SetField("price", 10.50)
	sd.SetField("change", 0.15)
	sd.SetField("change_percent", 1.45)
	sd.SetField("volume", int64(1250000))
	sd.SetField("timestamp", time.Now())

	fmt.Println("1. 创建的结构化数据:")
	fmt.Printf("   股票代码: %v\n", sd.Values["symbol"])
	fmt.Printf("   股票名称: %v\n", sd.Values["name"])
	fmt.Printf("   当前价格: %v\n", sd.Values["price"])
	fmt.Printf("   数据时间: %v\n", sd.Values["timestamp"])
	fmt.Println()

	// 2. CSV 序列化
	csvSerializer := subscriber.NewStructuredDataSerializer(subscriber.FormatCSV)

	csvData, err := csvSerializer.Serialize(sd)
	if err != nil {
		log.Fatalf("CSV序列化失败: %v", err)
	}

	fmt.Println("2. CSV 序列化结果:")
	fmt.Printf("   MIME类型: %s\n", csvSerializer.MimeType())
	fmt.Printf("   CSV内容:\n%s\n", string(csvData))

	// 3. JSON 序列化
	jsonSerializer := subscriber.NewStructuredDataSerializer(subscriber.FormatJSON)

	jsonData, err := jsonSerializer.Serialize(sd)
	if err != nil {
		log.Fatalf("JSON序列化失败: %v", err)
	}

	fmt.Println("3. JSON 序列化结果:")
	fmt.Printf("   MIME类型: %s\n", jsonSerializer.MimeType())
	fmt.Printf("   JSON内容:\n%s\n", string(jsonData))

	// 4. CSV 反序列化
	deserializedSD := subscriber.NewStructuredData(subscriber.StockDataSchema)
	err = csvSerializer.Deserialize(csvData, deserializedSD)
	if err != nil {
		log.Fatalf("CSV反序列化失败: %v", err)
	}

	fmt.Println("4. CSV 反序列化结果:")
	symbol, _ := deserializedSD.GetField("symbol")
	name, _ := deserializedSD.GetField("name")
	price, _ := deserializedSD.GetField("price")
	fmt.Printf("   股票代码: %v\n", symbol)
	fmt.Printf("   股票名称: %v\n", name)
	fmt.Printf("   当前价格: %v\n", price)
	fmt.Println()

	// 5. 批量序列化示例
	fmt.Println("5. 批量序列化示例:")

	// 创建第二个数据
	sd2 := subscriber.NewStructuredData(subscriber.StockDataSchema)
	sd2.SetField("symbol", "000001")
	sd2.SetField("name", "平安银行")
	sd2.SetField("price", 12.80)
	sd2.SetField("change", -0.05)
	sd2.SetField("change_percent", -0.39)
	sd2.SetField("volume", int64(980000))
	sd2.SetField("timestamp", time.Now())

	// 批量序列化
	dataList := []*subscriber.StructuredData{sd, sd2}
	batchCSVData, err := csvSerializer.SerializeMultiple(dataList)
	if err != nil {
		log.Fatalf("批量CSV序列化失败: %v", err)
	}

	fmt.Printf("   批量CSV内容:\n%s\n", string(batchCSVData))

	// 6. 从 StockData 转换示例
	fmt.Println("6. 从 StockData 转换示例:")

	stockData := subscriber.StockData{
		Symbol:        "601398",
		Name:          "工商银行",
		Price:         5.25,
		Change:        0.02,
		ChangePercent: 0.38,
		Volume:        2500000,
		Timestamp:     time.Now(),
	}

	convertedSD, err := subscriber.StockDataToStructuredData(stockData)
	if err != nil {
		log.Fatalf("StockData转换失败: %v", err)
	}

	convertedCSV, err := csvSerializer.Serialize(convertedSD)
	if err != nil {
		log.Fatalf("转换后序列化失败: %v", err)
	}

	fmt.Printf("   转换后的CSV:\n%s\n", string(convertedCSV))

	fmt.Println("=== 示例完成 ===")

	csvDeserializationDemo()
}
