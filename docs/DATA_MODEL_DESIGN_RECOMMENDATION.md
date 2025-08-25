# 数据模型设计建议：StructuredData vs StockData + Tag

## 总体建议：采用混合方案

经过深入分析，建议采用**混合方案**，在不同场景下使用最适合的数据模型。

## 使用场景划分

### 🎯 使用 StructuredData 的场景

#### 1. 多样化数据类型
```go
// 新闻数据
newsSchema := &DataSchema{
    Name: "news_data",
    Fields: map[string]*FieldDefinition{
        "title": {Type: FieldTypeString, Required: true},
        "content": {Type: FieldTypeString, Required: true},
        "publish_time": {Type: FieldTypeTime, Required: true},
    },
}

// 公告数据
announcementSchema := &DataSchema{
    Name: "announcement_data", 
    Fields: map[string]*FieldDefinition{
        "company": {Type: FieldTypeString, Required: true},
        "type": {Type: FieldTypeString, Required: true},
        "content": {Type: FieldTypeString, Required: true},
    },
}
```

#### 2. 用户自定义数据模式
```go
// 从配置文件动态加载模式
func LoadCustomSchema(configPath string) (*DataSchema, error) {
    config, err := ioutil.ReadFile(configPath)
    if err != nil {
        return nil, err
    }
    
    var schema DataSchema
    err = json.Unmarshal(config, &schema)
    return &schema, err
}

// 用户可以定义自己的投资组合数据结构
portfolioSchema := LoadCustomSchema("portfolio_schema.json")
```

#### 3. 版本兼容性需求
```go
// 支持多版本股票数据格式
func GetStockSchema(version string) *DataSchema {
    switch version {
    case "v1":
        return stockSchemaV1
    case "v2":
        return stockSchemaV2  // 新增字段
    case "v3":
        return stockSchemaV3  // 字段类型变更
    default:
        return StockDataSchema // 最新版本
    }
}
```

#### 4. 需要丰富元数据的场景
```go
// 带有业务规则的字段定义
"pe_ratio": {
    Name:        "pe_ratio",
    Type:        FieldTypeFloat64,
    Description: "市盈率",
    Comment:     "股价/每股收益，负值表示亏损",
    Required:    false,
    Validator: func(v interface{}) error {
        if pe, ok := v.(float64); ok && pe > 1000 {
            return fmt.Errorf("市盈率异常，请检查数据")
        }
        return nil
    },
}
```

### ⚡ 使用 StockData + Tag 的场景

#### 1. 高频交易数据处理
```go
// 增强版 StockData
type StockData struct {
    Symbol        string    `json:"symbol" validate:"required,stock_symbol" db:"symbol"`
    Price         float64   `json:"price" validate:"required,gt=0" db:"price"`
    Volume        int64     `json:"volume" validate:"gte=0" db:"volume"`
    Timestamp     time.Time `json:"timestamp" validate:"required" db:"timestamp"`
    
    // 内部字段，不序列化
    _lastUpdated  time.Time `json:"-"`
    _isValid      bool      `json:"-"`
}

// 注册自定义验证器
func init() {
    validate := validator.New()
    validate.RegisterValidation("stock_symbol", validateStockSymbol)
}

func validateStockSymbol(fl validator.FieldLevel) bool {
    symbol := fl.Field().String()
    // 中国股票代码验证逻辑
    return regexp.MustCompile(`^[0-9]{6}$`).MatchString(symbol)
}
```

#### 2. API 响应和序列化
```go
// 直接用于 JSON API 响应
func (api *StockAPI) GetStock(c *gin.Context) {
    symbol := c.Param("symbol")
    stockData, err := api.provider.FetchData(ctx, []string{symbol})
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // 验证数据
    if err := validate.Struct(stockData[0]); err != nil {
        c.JSON(400, gin.H{"error": "Invalid data", "details": err.Error()})
        return
    }
    
    c.JSON(200, stockData[0])  // 直接序列化，性能更好
}
```

#### 3. 数据库操作
```go
// 使用 GORM 等 ORM
type StockData struct {
    ID            uint      `gorm:"primaryKey"`
    Symbol        string    `gorm:"index;not null" validate:"required,stock_symbol"`
    Price         float64   `gorm:"not null" validate:"required,gt=0"`
    CreatedAt     time.Time `gorm:"autoCreateTime"`
    UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}
```

## 实施策略

### 第一阶段：增强现有实现

#### 1. 为 StockData 添加验证器支持
```go
// pkg/subscriber/validation.go
package subscriber

import "github.com/go-playground/validator/v10"

var validate *validator.Validate

func init() {
    validate = validator.New()
    registerCustomValidators()
}

func registerCustomValidators() {
    validate.RegisterValidation("stock_symbol", validateStockSymbol)
    validate.RegisterValidation("positive_price", validatePositivePrice)
    validate.RegisterValidation("valid_volume", validateVolume)
}

// 为 StockData 添加验证方法
func (sd *StockData) Validate() error {
    return validate.Struct(sd)
}

func (sd *StockData) ValidateField(field string) error {
    return validate.Var(sd, field)
}
```

#### 2. 添加 Tag 支持到 StockData
```go
// 更新 types.go 中的 StockData 定义
type StockData struct {
    // 基本信息
    Symbol        string  `json:"symbol" validate:"required,stock_symbol" csv:"股票代码" db:"symbol"`
    Name          string  `json:"name" validate:"required,min=1,max=50" csv:"股票名称" db:"name"`
    Price         float64 `json:"price" validate:"required,positive_price" csv:"现价" db:"price"`
    Change        float64 `json:"change" validate:"numeric" csv:"涨跌额" db:"change_amount"`
    ChangePercent float64 `json:"change_percent" validate:"gte=-100,lte=100" csv:"涨跌幅%" db:"change_percent"`
    
    // 交易数据
    Volume        int64   `json:"volume" validate:"valid_volume" csv:"成交量" db:"volume"`
    Turnover      float64 `json:"turnover" validate:"gte=0" csv:"成交额" db:"turnover"`
    
    // 时间信息
    Timestamp     time.Time `json:"timestamp" validate:"required" csv:"时间" db:"created_at"`
}
```

### 第二阶段：构建转换和兼容层

#### 1. 增强转换函数
```go
// pkg/subscriber/converter.go
package subscriber

// ConvertWithValidation 转换并验证
func (sd *StockData) ToStructuredDataWithValidation() (*StructuredData, error) {
    // 先验证 StockData
    if err := sd.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // 转换为 StructuredData
    return StockDataToStructuredData(*sd)
}

// 从 StructuredData 转换并验证
func StructuredDataToStockDataWithValidation(sd *StructuredData) (*StockData, error) {
    // 先验证 StructuredData
    if err := sd.ValidateDataComplete(); err != nil {
        return nil, fmt.Errorf("structured data validation failed: %w", err)
    }
    
    // 转换为 StockData
    stockData, err := StructuredDataToStockData(sd)
    if err != nil {
        return nil, err
    }
    
    // 再次验证转换后的数据
    if err := stockData.Validate(); err != nil {
        return nil, fmt.Errorf("converted data validation failed: %w", err)
    }
    
    return stockData, nil
}
```

#### 2. 统一的数据接口
```go
// pkg/subscriber/data_interface.go
package subscriber

// DataModel 统一数据模型接口
type DataModel interface {
    Validate() error
    ToJSON() ([]byte, error)
    FromJSON([]byte) error
    GetTimestamp() time.Time
    GetIdentifier() string  // 获取唯一标识符（如股票代码）
}

// StockData 实现 DataModel 接口
func (sd *StockData) GetTimestamp() time.Time {
    return sd.Timestamp
}

func (sd *StockData) GetIdentifier() string {
    return sd.Symbol
}

func (sd *StockData) ToJSON() ([]byte, error) {
    if err := sd.Validate(); err != nil {
        return nil, err
    }
    return json.Marshal(sd)
}

func (sd *StockData) FromJSON(data []byte) error {
    if err := json.Unmarshal(data, sd); err != nil {
        return err
    }
    return sd.Validate()
}

// StructuredData 也实现 DataModel 接口
func (sd *StructuredData) GetTimestamp() time.Time {
    return sd.Timestamp
}

func (sd *StructuredData) GetIdentifier() string {
    if id, err := sd.GetField("symbol"); err == nil && id != nil {
        if str, ok := id.(string); ok {
            return str
        }
    }
    return ""
}
```

### 第三阶段：性能优化和监控

#### 1. 性能基准测试
```go
// tests/performance_comparison_test.go
func BenchmarkStockDataValidation(b *testing.B) {
    stockData := createTestStockData()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = stockData.Validate()
    }
}

func BenchmarkStructuredDataValidation(b *testing.B) {
    structuredData := createTestStructuredData()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = structuredData.ValidateData()
    }
}

func BenchmarkConversion(b *testing.B) {
    stockData := createTestStockData()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = StockDataToStructuredData(stockData)
    }
}
```

#### 2. 监控和指标
```go
// pkg/metrics/data_metrics.go
type DataMetrics struct {
    ValidationErrors   int64
    ConversionErrors   int64
    PerformanceStats   map[string]time.Duration
}

func (dm *DataMetrics) RecordValidation(dataType string, duration time.Duration, success bool) {
    if !success {
        atomic.AddInt64(&dm.ValidationErrors, 1)
    }
    // 记录性能指标
}
```

## 配置和使用指南

### 配置文件示例
```yaml
# config/data_models.yaml
data_models:
  stock_data:
    use_validation: true
    validation_mode: "strict"  # strict, lenient, off
    performance_mode: true     # 优化性能
    
  structured_data:
    enable_dynamic_schema: true
    schema_cache_size: 1000
    validation_timeout: "100ms"
    
  conversion:
    enable_bidirectional: true
    validate_after_conversion: true
    performance_logging: true
```

### 使用示例
```go
// examples/mixed_data_models/main.go
func main() {
    // 高频股票数据处理
    stockData := &StockData{
        Symbol: "000001",
        Price:  10.50,
        Volume: 1000000,
        Timestamp: time.Now(),
    }
    
    if err := stockData.Validate(); err != nil {
        log.Printf("Stock data validation failed: %v", err)
        return
    }
    
    // 需要灵活性的场景
    newsSchema := createNewsSchema()
    newsData := NewStructuredData(newsSchema)
    newsData.SetField("title", "重要公告")
    newsData.SetField("content", "...")
    
    if err := newsData.ValidateData(); err != nil {
        log.Printf("News data validation failed: %v", err)
        return
    }
    
    // 数据转换
    structuredStock, err := stockData.ToStructuredDataWithValidation()
    if err != nil {
        log.Printf("Conversion failed: %v", err)
        return
    }
    
    fmt.Println("Both models working together successfully!")
}
```

## 迁移计划

### 阶段 1（1-2周）：增强现有代码
- [ ] 为 StockData 添加验证器支持
- [ ] 添加 Tag 定义
- [ ] 创建转换和兼容层
- [ ] 编写测试用例

### 阶段 2（2-3周）：优化和集成
- [ ] 性能基准测试和优化
- [ ] 添加监控和指标
- [ ] 文档更新
- [ ] 示例代码

### 阶段 3（1周）：部署和验证
- [ ] 生产环境测试
- [ ] 性能验证
- [ ] 用户反馈收集

## 总结

混合方案充分发挥了两种实现方式的优势：
- **StockData + Tag**：高性能、类型安全、适合核心业务
- **StructuredData**：灵活、可扩展、适合多样化需求

这种设计完美契合项目的可扩展性和松耦合设计理念，为未来的发展留出了充足的空间。