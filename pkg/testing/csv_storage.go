package testing

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DataPoint 表示一个数据采集点
type DataPoint struct {
	Timestamp    time.Time `json:"timestamp"`     // 数据记录时间戳
	Symbol       string    `json:"symbol"`        // 股票代码
	QueryTime    time.Time `json:"query_time"`    // 查询发起时间
	ResponseTime time.Time `json:"response_time"` // 响应接收时间
	QuoteTime    string    `json:"quote_time"`    // API返回的时间字段
	Price        float64   `json:"price"`         // 当前价格
	Volume       int64     `json:"volume"`        // 成交量
	Field30      string    `json:"field_30"`      // 原始字段30的值
	AllFields    []string  `json:"all_fields"`    // 完整字段用于分析
}

// PerformanceMetric 表示性能监控数据
type PerformanceMetric struct {
	Timestamp         time.Time `json:"timestamp"`
	Symbol            string    `json:"symbol"`
	RequestDurationMs int64     `json:"request_duration_ms"`
	ResponseSizeBytes int64     `json:"response_size_bytes"`
	ErrorOccurred     bool      `json:"error_occurred"`
	ErrorMessage      string    `json:"error_message"`
}

// TimeAnalysisData 表示时间分析数据
type TimeAnalysisData struct {
	Timestamp    time.Time `json:"timestamp"`
	Symbol       string    `json:"symbol"`
	LocalTime    time.Time `json:"local_time"`
	APITimeField string    `json:"api_time_field"`
	TimeDiffSecs float64   `json:"time_diff_seconds"`
	MarketStatus string    `json:"market_status"`
	TradingPhase string    `json:"trading_phase"`
}

// CSVStorage CSV数据存储器
type CSVStorage struct {
	dataDir    string
	mu         sync.RWMutex
	writers    map[string]*csv.Writer
	files      map[string]*os.File
	bufferSize int
}

// NewCSVStorage 创建新的CSV存储器
func NewCSVStorage(dataDir string) *CSVStorage {
	return &CSVStorage{
		dataDir:    dataDir,
		writers:    make(map[string]*csv.Writer),
		files:      make(map[string]*os.File),
		bufferSize: 100, // 缓冲100条记录再写入
	}
}

// ensureDataDir 确保数据目录存在
func (cs *CSVStorage) ensureDataDir() error {
	collectedDir := filepath.Join(cs.dataDir, "collected")
	analyzedDir := filepath.Join(cs.dataDir, "analyzed")
	reportsDir := filepath.Join(cs.dataDir, "reports")

	dirs := []string{collectedDir, analyzedDir, reportsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %v", dir, err)
		}
	}
	return nil
}

// getCSVWriter 获取或创建CSV写入器
func (cs *CSVStorage) getCSVWriter(filename string, headers []string) (*csv.Writer, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if writer, exists := cs.writers[filename]; exists {
		return writer, nil
	}

	// 确保目录存在
	if err := cs.ensureDataDir(); err != nil {
		return nil, err
	}

	// 构建完整文件路径
	fullPath := filepath.Join(cs.dataDir, "collected", filename)

	// 检查文件是否已存在
	fileExists := true
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		fileExists = false
	}

	// 打开或创建文件
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开CSV文件失败 %s: %v", fullPath, err)
	}

	writer := csv.NewWriter(file)

	// 如果是新文件，写入表头
	if !fileExists && headers != nil {
		if err := writer.Write(headers); err != nil {
			file.Close()
			return nil, fmt.Errorf("写入CSV表头失败: %v", err)
		}
		writer.Flush()
	}

	cs.writers[filename] = writer
	cs.files[filename] = file

	return writer, nil
}

// SaveDataPoint 保存数据点到CSV文件
func (cs *CSVStorage) SaveDataPoint(data DataPoint) error {
	// 按日期分割文件
	dateStr := data.Timestamp.Format("2006-01-02")
	filename := fmt.Sprintf("api_data_%s.csv", dateStr)

	headers := []string{
		"timestamp", "symbol", "query_time", "response_time",
		"quote_time", "price", "volume", "field_30", "all_fields_json",
	}

	writer, err := cs.getCSVWriter(filename, headers)
	if err != nil {
		return err
	}

	// 将所有字段转换为JSON字符串
	allFieldsJSON, err := json.Marshal(data.AllFields)
	if err != nil {
		return fmt.Errorf("序列化字段数据失败: %v", err)
	}

	record := []string{
		data.Timestamp.Format(time.RFC3339Nano),
		data.Symbol,
		data.QueryTime.Format(time.RFC3339Nano),
		data.ResponseTime.Format(time.RFC3339Nano),
		data.QuoteTime,
		strconv.FormatFloat(data.Price, 'f', 3, 64),
		strconv.FormatInt(data.Volume, 10),
		data.Field30,
		string(allFieldsJSON),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("写入CSV记录失败: %v", err)
	}

	writer.Flush()
	return writer.Error()
}

// SavePerformanceMetric 保存性能指标到CSV文件
func (cs *CSVStorage) SavePerformanceMetric(metric PerformanceMetric) error {
	dateStr := metric.Timestamp.Format("2006-01-02")
	filename := fmt.Sprintf("performance_%s.csv", dateStr)

	headers := []string{
		"timestamp", "symbol", "request_duration_ms", "response_size_bytes",
		"error_occurred", "error_message",
	}

	writer, err := cs.getCSVWriter(filename, headers)
	if err != nil {
		return err
	}

	record := []string{
		metric.Timestamp.Format(time.RFC3339Nano),
		metric.Symbol,
		strconv.FormatInt(metric.RequestDurationMs, 10),
		strconv.FormatInt(metric.ResponseSizeBytes, 10),
		strconv.FormatBool(metric.ErrorOccurred),
		metric.ErrorMessage,
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("写入性能指标失败: %v", err)
	}

	writer.Flush()
	return writer.Error()
}

// SaveTimeAnalysis 保存时间分析数据到CSV文件
func (cs *CSVStorage) SaveTimeAnalysis(data TimeAnalysisData) error {
	dateStr := data.Timestamp.Format("2006-01-02")
	filename := fmt.Sprintf("time_analysis_%s.csv", dateStr)

	headers := []string{
		"timestamp", "symbol", "local_time", "api_time_field",
		"time_diff_seconds", "market_status", "trading_phase",
	}

	writer, err := cs.getCSVWriter(filename, headers)
	if err != nil {
		return err
	}

	record := []string{
		data.Timestamp.Format(time.RFC3339Nano),
		data.Symbol,
		data.LocalTime.Format(time.RFC3339Nano),
		data.APITimeField,
		strconv.FormatFloat(data.TimeDiffSecs, 'f', 3, 64),
		data.MarketStatus,
		data.TradingPhase,
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("写入时间分析数据失败: %v", err)
	}

	writer.Flush()
	return writer.Error()
}

// ReadDataPoints 从CSV文件读取数据点
func (cs *CSVStorage) ReadDataPoints(startDate, endDate time.Time) ([]DataPoint, error) {
	var results []DataPoint

	// 遍历日期范围内的文件
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		filename := fmt.Sprintf("api_data_%s.csv", dateStr)
		filepath := filepath.Join(cs.dataDir, "collected", filename)

		// 检查文件是否存在
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			continue // 跳过不存在的文件
		}

		data, err := cs.readDataPointsFromFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("读取文件 %s 失败: %v", filepath, err)
		}

		results = append(results, data...)
	}

	return results, nil
}

// readDataPointsFromFile 从单个CSV文件读取数据点
func (cs *CSVStorage) readDataPointsFromFile(filepath string) ([]DataPoint, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, nil
	}

	// 跳过表头
	var results []DataPoint
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 9 {
			continue // 跳过格式不正确的记录
		}

		dataPoint, err := cs.parseDataPointRecord(record)
		if err != nil {
			// 记录错误但继续处理其他记录
			fmt.Printf("解析记录失败 %v: %v\n", record, err)
			continue
		}

		results = append(results, dataPoint)
	}

	return results, nil
}

// parseDataPointRecord 解析CSV记录为DataPoint
func (cs *CSVStorage) parseDataPointRecord(record []string) (DataPoint, error) {
	var dp DataPoint
	var err error

	// 解析时间戳
	dp.Timestamp, err = time.Parse(time.RFC3339Nano, record[0])
	if err != nil {
		return dp, fmt.Errorf("解析timestamp失败: %v", err)
	}

	dp.Symbol = record[1]

	dp.QueryTime, err = time.Parse(time.RFC3339Nano, record[2])
	if err != nil {
		return dp, fmt.Errorf("解析query_time失败: %v", err)
	}

	dp.ResponseTime, err = time.Parse(time.RFC3339Nano, record[3])
	if err != nil {
		return dp, fmt.Errorf("解析response_time失败: %v", err)
	}

	dp.QuoteTime = record[4]

	dp.Price, err = strconv.ParseFloat(record[5], 64)
	if err != nil {
		return dp, fmt.Errorf("解析price失败: %v", err)
	}

	dp.Volume, err = strconv.ParseInt(record[6], 10, 64)
	if err != nil {
		return dp, fmt.Errorf("解析volume失败: %v", err)
	}

	dp.Field30 = record[7]

	// 解析JSON字段
	if err := json.Unmarshal([]byte(record[8]), &dp.AllFields); err != nil {
		return dp, fmt.Errorf("解析all_fields_json失败: %v", err)
	}

	return dp, nil
}

// FilterDataPoints 筛选数据点
func (cs *CSVStorage) FilterDataPoints(data []DataPoint, symbol string, startTime, endTime time.Time) []DataPoint {
	var filtered []DataPoint

	for _, dp := range data {
		// 按股票代码筛选
		if symbol != "" && dp.Symbol != symbol {
			continue
		}

		// 按时间范围筛选
		if !startTime.IsZero() && dp.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && dp.Timestamp.After(endTime) {
			continue
		}

		filtered = append(filtered, dp)
	}

	return filtered
}

// GetFileSizes 获取CSV文件大小统计
func (cs *CSVStorage) GetFileSizes() (map[string]int64, error) {
	sizes := make(map[string]int64)

	collectedDir := filepath.Join(cs.dataDir, "collected")

	err := filepath.Walk(collectedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".csv") {
			sizes[info.Name()] = info.Size()
		}

		return nil
	})

	return sizes, err
}

// RotateFiles 文件轮转：压缩超过指定大小的文件
func (cs *CSVStorage) RotateFiles(maxSizeBytes int64) error {
	sizes, err := cs.GetFileSizes()
	if err != nil {
		return err
	}

	for filename, size := range sizes {
		if size > maxSizeBytes {
			// 这里可以实现文件压缩逻辑
			fmt.Printf("文件 %s 大小 %d 字节，需要轮转\n", filename, size)
		}
	}

	return nil
}

// Close 关闭所有打开的文件和写入器
func (cs *CSVStorage) Close() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 刷新所有写入器
	for _, writer := range cs.writers {
		writer.Flush()
	}

	// 关闭所有文件
	var errors []string
	for filename, file := range cs.files {
		if err := file.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("关闭文件 %s 失败: %v", filename, err))
		}
	}

	// 清空映射
	cs.writers = make(map[string]*csv.Writer)
	cs.files = make(map[string]*os.File)

	if len(errors) > 0 {
		return fmt.Errorf("关闭文件时发生错误: %s", strings.Join(errors, "; "))
	}

	return nil
}

// BatchSaveDataPoints 批量保存数据点（提高性能）
func (cs *CSVStorage) BatchSaveDataPoints(dataPoints []DataPoint) error {
	// 按日期分组
	groups := make(map[string][]DataPoint)
	for _, dp := range dataPoints {
		dateStr := dp.Timestamp.Format("2006-01-02")
		groups[dateStr] = append(groups[dateStr], dp)
	}

	// 批量写入每个日期的数据
	for dateStr, points := range groups {
		filename := fmt.Sprintf("api_data_%s.csv", dateStr)

		headers := []string{
			"timestamp", "symbol", "query_time", "response_time",
			"quote_time", "price", "volume", "field_30", "all_fields_json",
		}

		writer, err := cs.getCSVWriter(filename, headers)
		if err != nil {
			return err
		}

		// 批量写入所有记录
		for _, dp := range points {
			allFieldsJSON, err := json.Marshal(dp.AllFields)
			if err != nil {
				continue // 跳过错误记录
			}

			record := []string{
				dp.Timestamp.Format(time.RFC3339Nano),
				dp.Symbol,
				dp.QueryTime.Format(time.RFC3339Nano),
				dp.ResponseTime.Format(time.RFC3339Nano),
				dp.QuoteTime,
				strconv.FormatFloat(dp.Price, 'f', 3, 64),
				strconv.FormatInt(dp.Volume, 10),
				dp.Field30,
				string(allFieldsJSON),
			}

			if err := writer.Write(record); err != nil {
				return fmt.Errorf("批量写入记录失败: %v", err)
			}
		}

		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
	}

	return nil
}
