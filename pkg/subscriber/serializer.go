package subscriber

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SerializationFormat 序列化格式
type SerializationFormat int

const (
	FormatCSV SerializationFormat = iota
	FormatJSON
)

// String returns the string representation of SerializationFormat
func (sf SerializationFormat) String() string {
	switch sf {
	case FormatCSV:
		return "csv"
	case FormatJSON:
		return "json"
	default:
		return "unknown"
	}
}

// StructuredDataSerializer 结构化数据序列化器
type StructuredDataSerializer struct {
	format   SerializationFormat
	timezone *time.Location // 时区设置，默认为上海时区
}

// NewStructuredDataSerializer 创建新的结构化数据序列化器
func NewStructuredDataSerializer(format SerializationFormat) *StructuredDataSerializer {
	// 设置上海时区
	shanghaiTZ, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 如果加载失败，使用 UTC+8
		shanghaiTZ = time.FixedZone("CST", 8*3600)
	}

	return &StructuredDataSerializer{
		format:   format,
		timezone: shanghaiTZ,
	}
}

// Serialize 将 StructuredData 序列化为字节数组
func (s *StructuredDataSerializer) Serialize(data interface{}) ([]byte, error) {
	switch s.format {
	case FormatCSV:
		return s.serializeToCSV(data)
	case FormatJSON:
		return s.serializeToJSON(data)
	default:
		return nil, fmt.Errorf("unsupported serialization format: %v", s.format)
	}
}

// Deserialize 将字节数组反序列化为 StructuredData
func (s *StructuredDataSerializer) Deserialize(data []byte, target interface{}) error {
	switch s.format {
	case FormatCSV:
		return s.deserializeFromCSV(data, target)
	case FormatJSON:
		return s.deserializeFromJSON(data, target)
	default:
		return fmt.Errorf("unsupported deserialization format: %v", s.format)
	}
}

// MimeType 返回此序列化器对应的MIME类型
func (s *StructuredDataSerializer) MimeType() string {
	switch s.format {
	case FormatCSV:
		return "text/csv"
	case FormatJSON:
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// serializeToCSV 序列化为CSV格式
func (s *StructuredDataSerializer) serializeToCSV(data interface{}) ([]byte, error) {
	sd, ok := data.(*StructuredData)
	if !ok {
		return nil, fmt.Errorf("data must be *StructuredData, got %T", data)
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 生成CSV表头
	headers := s.generateCSVHeaders(sd.Schema)
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// 生成数据行
	record := s.generateCSVRecord(sd)
	if err := writer.Write(record); err != nil {
		return nil, fmt.Errorf("failed to write CSV record: %w", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return buf.Bytes(), nil
}

// serializeToJSON 序列化为JSON格式
func (s *StructuredDataSerializer) serializeToJSON(data interface{}) ([]byte, error) {
	sd, ok := data.(*StructuredData)
	if !ok {
		return nil, fmt.Errorf("data must be *StructuredData, got %T", data)
	}

	// 创建JSON兼容的结构
	jsonData := map[string]interface{}{
		"schema":    sd.Schema,
		"values":    sd.Values,
		"timestamp": sd.Timestamp.In(s.timezone).Format("2006-01-02 15:04:05"),
	}

	return json.Marshal(jsonData)
}

// deserializeFromCSV 从CSV格式反序列化
func (s *StructuredDataSerializer) deserializeFromCSV(data []byte, target interface{}) error {
	sd, ok := target.(*StructuredData)
	if !ok {
		return fmt.Errorf("target must be *StructuredData, got %T", target)
	}

	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV data: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV data must contain at least header and one data row")
	}

	headers := records[0]
	dataRow := records[1]

	if len(headers) != len(dataRow) {
		return fmt.Errorf("header count (%d) does not match data count (%d)", len(headers), len(dataRow))
	}

	// 解析表头，提取字段名并验证
	fieldMapping, err := s.parseAndValidateCSVHeaders(headers, sd.Schema)
	if err != nil {
		return err
	}

	// 解析数据行
	for i, fieldName := range fieldMapping {
		if fieldName == "" {
			continue // 跳过无法识别的字段
		}

		fieldDef := sd.Schema.Fields[fieldName]
		value, err := s.parseCSVValue(dataRow[i], fieldDef.Type)
		if err != nil {
			return NewStructuredDataError(ErrInvalidFieldType, fieldName, fmt.Sprintf("failed to parse value '%s': %v", dataRow[i], err))
		}

		if err := sd.SetField(fieldName, value); err != nil {
			return err
		}
	}

	return nil
}

// deserializeFromJSON 从JSON格式反序列化
func (s *StructuredDataSerializer) deserializeFromJSON(data []byte, target interface{}) error {
	sd, ok := target.(*StructuredData)
	if !ok {
		return fmt.Errorf("target must be *StructuredData, got %T", target)
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// 解析schema
	if schemaData, exists := jsonData["schema"]; exists {
		schemaBytes, err := json.Marshal(schemaData)
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}
		if err := json.Unmarshal(schemaBytes, &sd.Schema); err != nil {
			return fmt.Errorf("failed to unmarshal schema: %w", err)
		}
	}

	// 解析values
	if valuesData, exists := jsonData["values"]; exists {
		if values, ok := valuesData.(map[string]interface{}); ok {
			sd.Values = values
		}
	}

	// 解析timestamp
	if timestampData, exists := jsonData["timestamp"]; exists {
		if timestampStr, ok := timestampData.(string); ok {
			if timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", timestampStr, s.timezone); err == nil {
				sd.Timestamp = timestamp
			}
		}
	}

	return nil
}

// generateCSVHeaders 生成CSV表头（包含中文描述）
func (s *StructuredDataSerializer) generateCSVHeaders(schema *DataSchema) []string {
	headers := make([]string, len(schema.FieldOrder))

	for i, fieldName := range schema.FieldOrder {
		fieldDef, exists := schema.Fields[fieldName]
		if !exists {
			headers[i] = fieldName
			continue
		}

		// 格式：中文描述(英文字段名)
		if fieldDef.Description != "" {
			headers[i] = fmt.Sprintf("%s(%s)", fieldDef.Description, fieldName)
		} else {
			headers[i] = fieldName
		}
	}

	return headers
}

// generateCSVRecord 生成CSV数据行
func (s *StructuredDataSerializer) generateCSVRecord(sd *StructuredData) []string {
	record := make([]string, len(sd.Schema.FieldOrder))

	for i, fieldName := range sd.Schema.FieldOrder {
		value, err := sd.GetField(fieldName)
		if err != nil || value == nil {
			record[i] = ""
			continue
		}

		fieldDef := sd.Schema.Fields[fieldName]
		record[i] = s.formatCSVValue(value, fieldDef.Type)
	}

	return record
}

// formatCSVValue 格式化CSV值
func (s *StructuredDataSerializer) formatCSVValue(value interface{}, fieldType FieldType) string {
	if value == nil {
		return ""
	}

	switch fieldType {
	case FieldTypeString:
		if str, ok := value.(string); ok {
			return str
		}
	case FieldTypeInt:
		switch v := value.(type) {
		case int:
			return strconv.Itoa(v)
		case int32:
			return strconv.FormatInt(int64(v), 10)
		case int64:
			return strconv.FormatInt(v, 10)
		}
	case FieldTypeFloat64:
		switch v := value.(type) {
		case float32:
			return strconv.FormatFloat(float64(v), 'f', 2, 32)
		case float64:
			return strconv.FormatFloat(v, 'f', 2, 64)
		}
	case FieldTypeBool:
		if b, ok := value.(bool); ok {
			return strconv.FormatBool(b)
		}
	case FieldTypeTime:
		if t, ok := value.(time.Time); ok {
			// 使用上海时区格式化时间：YYYY-MM-DD HH:mm:ss
			return t.In(s.timezone).Format("2006-01-02 15:04:05")
		}
	}

	return fmt.Sprintf("%v", value)
}

// parseCSVHeaders 解析CSV表头，提取字段名
func (s *StructuredDataSerializer) parseCSVHeaders(headers []string) []string {
	fieldNames := make([]string, len(headers))

	for i, header := range headers {
		// 解析格式：中文描述(英文字段名) 或 英文字段名
		if strings.Contains(header, "(") && strings.Contains(header, ")") {
			// 提取括号内的字段名
			start := strings.LastIndex(header, "(")
			end := strings.LastIndex(header, ")")
			if start < end && start >= 0 && end >= 0 {
				fieldNames[i] = header[start+1 : end]
			} else {
				fieldNames[i] = header
			}
		} else {
			fieldNames[i] = header
		}
	}

	return fieldNames
}

// parseAndValidateCSVHeaders 解析并验证CSV表头
func (s *StructuredDataSerializer) parseAndValidateCSVHeaders(headers []string, schema *DataSchema) ([]string, error) {
	fieldNames := s.parseCSVHeaders(headers)

	// 验证字段是否存在于schema中，并提供详细的错误信息
	var unknownFields []string
	var validFields []string

	for i, fieldName := range fieldNames {
		if fieldName == "" {
			validFields = append(validFields, "")
			continue
		}

		if _, exists := schema.Fields[fieldName]; !exists {
			unknownFields = append(unknownFields, fmt.Sprintf("'%s' (from header '%s')", fieldName, headers[i]))
			validFields = append(validFields, "")
		} else {
			validFields = append(validFields, fieldName)
		}
	}

	// 如果有未知字段，返回详细错误信息
	if len(unknownFields) > 0 {
		availableFields := make([]string, 0, len(schema.Fields))
		for fieldName := range schema.Fields {
			availableFields = append(availableFields, fieldName)
		}

		return nil, NewStructuredDataError(
			ErrCSVHeaderMismatch,
			"",
			fmt.Sprintf("unknown fields in CSV header: %s. Available fields: %s",
				strings.Join(unknownFields, ", "),
				strings.Join(availableFields, ", ")))
	}

	return validFields, nil
}

// parseCSVValue 解析CSV值
func (s *StructuredDataSerializer) parseCSVValue(value string, fieldType FieldType) (interface{}, error) {
	if value == "" {
		return nil, nil
	}

	switch fieldType {
	case FieldTypeString:
		return value, nil
	case FieldTypeInt:
		return strconv.ParseInt(value, 10, 64)
	case FieldTypeFloat64:
		return strconv.ParseFloat(value, 64)
	case FieldTypeBool:
		return strconv.ParseBool(value)
	case FieldTypeTime:
		// 解析上海时区时间：YYYY-MM-DD HH:mm:ss
		return time.ParseInLocation("2006-01-02 15:04:05", value, s.timezone)
	default:
		return value, nil
	}
}

// DeserializeMultiple 批量反序列化多个 StructuredData
func (s *StructuredDataSerializer) DeserializeMultiple(data []byte, schema *DataSchema) ([]*StructuredData, error) {
	switch s.format {
	case FormatCSV:
		return s.deserializeMultipleFromCSV(data, schema)
	case FormatJSON:
		return s.deserializeMultipleFromJSON(data, schema)
	default:
		return nil, fmt.Errorf("unsupported deserialization format: %v", s.format)
	}
}

// deserializeMultipleFromCSV 从CSV格式批量反序列化
func (s *StructuredDataSerializer) deserializeMultipleFromCSV(data []byte, schema *DataSchema) ([]*StructuredData, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV data must contain at least header and one data row")
	}

	headers := records[0]
	dataRows := records[1:]

	// 解析表头，提取字段名并验证
	fieldMapping, err := s.parseAndValidateCSVHeaders(headers, schema)
	if err != nil {
		return nil, err
	}

	// 批量解析数据行
	result := make([]*StructuredData, 0, len(dataRows))
	for rowIndex, dataRow := range dataRows {
		if len(headers) != len(dataRow) {
			return nil, fmt.Errorf("row %d: header count (%d) does not match data count (%d)",
				rowIndex+2, len(headers), len(dataRow))
		}

		sd := NewStructuredData(schema)

		// 解析当前行的数据
		for i, fieldName := range fieldMapping {
			if fieldName == "" {
				continue // 跳过无法识别的字段
			}

			fieldDef := schema.Fields[fieldName]
			value, err := s.parseCSVValue(dataRow[i], fieldDef.Type)
			if err != nil {
				return nil, NewStructuredDataError(ErrInvalidFieldType, fieldName,
					fmt.Sprintf("row %d: failed to parse value '%s': %v", rowIndex+2, dataRow[i], err))
			}

			if err := sd.SetField(fieldName, value); err != nil {
				return nil, fmt.Errorf("row %d: %w", rowIndex+2, err)
			}
		}

		result = append(result, sd)
	}

	return result, nil
}

// deserializeMultipleFromJSON 从JSON格式批量反序列化
func (s *StructuredDataSerializer) deserializeMultipleFromJSON(data []byte, schema *DataSchema) ([]*StructuredData, error) {
	var jsonDataList []map[string]interface{}
	if err := json.Unmarshal(data, &jsonDataList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON array: %w", err)
	}

	result := make([]*StructuredData, 0, len(jsonDataList))
	for i, jsonData := range jsonDataList {
		sd := NewStructuredData(schema)

		// 解析values
		if valuesData, exists := jsonData["values"]; exists {
			if values, ok := valuesData.(map[string]interface{}); ok {
				for fieldName, value := range values {
					if err := sd.SetField(fieldName, value); err != nil {
						return nil, fmt.Errorf("item %d: failed to set field %s: %w", i, fieldName, err)
					}
				}
			}
		}

		// 解析timestamp
		if timestampData, exists := jsonData["timestamp"]; exists {
			if timestampStr, ok := timestampData.(string); ok {
				if timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", timestampStr, s.timezone); err == nil {
					sd.Timestamp = timestamp
					// 同时设置到Values中，因为timestamp是必填字段
					if err := sd.SetField("timestamp", timestamp); err != nil {
						return nil, fmt.Errorf("item %d: failed to set timestamp field: %w", i, err)
					}
				}
			}
		}

		// 验证数据完整性
		if err := sd.ValidateData(); err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}

		result = append(result, sd)
	}

	return result, nil
}

// SerializeMultiple 批量序列化多个 StructuredData
func (s *StructuredDataSerializer) SerializeMultiple(dataList []*StructuredData) ([]byte, error) {
	if len(dataList) == 0 {
		return []byte{}, nil
	}

	switch s.format {
	case FormatCSV:
		return s.serializeMultipleToCSV(dataList)
	case FormatJSON:
		return s.serializeMultipleToJSON(dataList)
	default:
		return nil, fmt.Errorf("unsupported serialization format: %v", s.format)
	}
}

// serializeMultipleToCSV 批量序列化为CSV格式
func (s *StructuredDataSerializer) serializeMultipleToCSV(dataList []*StructuredData) ([]byte, error) {
	if len(dataList) == 0 {
		return []byte{}, nil
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 使用第一个数据的schema生成表头
	firstData := dataList[0]
	headers := s.generateCSVHeaders(firstData.Schema)
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// 写入所有数据行
	for _, sd := range dataList {
		// 验证schema一致性
		if sd.Schema.Name != firstData.Schema.Name {
			return nil, fmt.Errorf("inconsistent schema: expected %s, got %s", firstData.Schema.Name, sd.Schema.Name)
		}

		record := s.generateCSVRecord(sd)
		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return buf.Bytes(), nil
}

// serializeMultipleToJSON 批量序列化为JSON格式
func (s *StructuredDataSerializer) serializeMultipleToJSON(dataList []*StructuredData) ([]byte, error) {
	jsonDataList := make([]map[string]interface{}, len(dataList))

	for i, sd := range dataList {
		jsonDataList[i] = map[string]interface{}{
			"schema":    sd.Schema,
			"values":    sd.Values,
			"timestamp": sd.Timestamp.In(s.timezone).Format("2006-01-02 15:04:05"),
		}
	}

	return json.Marshal(jsonDataList)
}
