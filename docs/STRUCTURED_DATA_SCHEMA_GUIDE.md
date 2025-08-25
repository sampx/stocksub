# StructuredData 自定义数据模式指南

## 概述

StructuredData 是 stocksub 项目中的灵活数据结构，允许您定义自定义的数据模式来处理各种类型的结构化数据。本指南将详细介绍如何创建和使用自定义数据模式。

## 核心概念

### DataSchema 数据模式

数据模式定义了数据的结构，包括字段定义、字段顺序和元数据信息。

```go
type DataSchema struct {
    Name        string                      // 模式名称
    Description string                      // 模式描述
    Fields      map[string]*FieldDefinition // 字段定义
    FieldOrder  []string                    // 字段顺序（用于CSV输出）
}
```

### FieldDefinition 字段定义

字段定义包含了单个字段的所有元信息。

```go
type FieldDefinition struct {
    Name         string                  // 字段名（英文）
    Type         FieldType               // 字段类型
    Description  string                  // 中文描述
    Comment      string                  // 中文字段备注（可选）
    Required     bool                    // 是否必填
    DefaultValue interface{}             // 默认值
    Validator    func(interface{}) error // 验证函数（不序列化）
}
```

### 支持的字段类型

```go
const (
    FieldTypeString  FieldType = iota // 字符串类型
    FieldTypeInt                      // 整数类型（支持int, int8, int16, int32, int64）
    FieldTypeFloat64                  // 浮点数类型（支持float32, float64）
    FieldTypeBool                     // 布尔类型
    FieldTypeTime                     // 时间类型
)
```

## 创建自定义模式示例

### 1. 简单的用户信息模式

```go
package main

import (
    "fmt"
    "time"
    "stocksub/pkg/subscriber"
)

// 创建用户信息模式
func createUserSchema() *subscriber.DataSchema {
    return &subscriber.DataSchema{
        Name:        "user_info",
        Description: "用户信息数据",
        Fields: map[string]*subscriber.FieldDefinition{
            "user_id": {
                Name:        "user_id",
                Type:        subscriber.FieldTypeString,
                Description: "用户ID",
                Required:    true,
                Validator: func(v interface{}) error {
                    if str, ok := v.(string); ok && len(str) < 3 {
                        return fmt.Errorf("用户ID长度不能少于3个字符")
                    }
                    return nil
                },
            },
            "username": {
                Name:        "username",
                Type:        subscriber.FieldTypeString,
                Description: "用户名",
                Required:    true,
            },
            "age": {
                Name:         "age",
                Type:         subscriber.FieldTypeInt,
                Description:  "年龄",
                Required:     false,
                DefaultValue: 0,
                Validator: func(v interface{}) error {
                    if age, ok := v.(int); ok && (age < 0 || age > 150) {
                        return fmt.Errorf("年龄必须在0-150之间")
                    }
                    return nil
                },
            },
            "email": {
                Name:        "email",
                Type:        subscriber.FieldTypeString,
                Description: "邮箱地址",
                Comment:     "用于接收通知邮件",
                Required:    false,
            },
            "is_active": {
                Name:         "is_active",
                Type:         subscriber.FieldTypeBool,
                Description:  "是否激活",
                Required:     false,
                DefaultValue: true,
            },
            "created_at": {
                Name:        "created_at",
                Type:        subscriber.FieldTypeTime,
                Description: "创建时间",
                Required:    true,
            },
        },
        FieldOrder: []string{"user_id", "username", "age", "email", "is_active", "created_at"},
    }
}

func main() {
    // 创建用户模式
    userSchema := createUserSchema()
    
    // 创建用户数据
    userData := subscriber.NewStructuredData(userSchema)
    
    // 设置字段值
    userData.SetField("user_id", "user_001")
    userData.SetField("username", "张三")
    userData.SetField("age", 25)
    userData.SetField("email", "zhangsan@example.com")
    userData.SetField("is_active", true)
    userData.SetField("created_at", time.Now())
    
    // 验证数据
    if err := userData.ValidateData(); err != nil {
        fmt.Printf("数据验证失败: %v\n", err)
        return
    }
    
    fmt.Println("用户数据创建成功!")
}
```

### 2. 复杂的新闻数据模式

```go
// 创建新闻数据模式
func createNewsSchema() *subscriber.DataSchema {
    return &subscriber.DataSchema{
        Name:        "news_data",
        Description: "新闻数据",
        Fields: map[string]*subscriber.FieldDefinition{
            "news_id": {
                Name:        "news_id",
                Type:        subscriber.FieldTypeString,
                Description: "新闻ID",
                Required:    true,
                Validator: func(v interface{}) error {
                    if str, ok := v.(string); ok && !strings.HasPrefix(str, "N") {
                        return fmt.Errorf("新闻ID必须以'N'开头")
                    }
                    return nil
                },
            },
            "title": {
                Name:        "title",
                Type:        subscriber.FieldTypeString,
                Description: "新闻标题",
                Required:    true,
                Validator: func(v interface{}) error {
                    if str, ok := v.(string); ok && len(str) > 200 {
                        return fmt.Errorf("新闻标题不能超过200个字符")
                    }
                    return nil
                },
            },
            "content": {
                Name:        "content",
                Type:        subscriber.FieldTypeString,
                Description: "新闻内容",
                Required:    true,
            },
            "author": {
                Name:         "author",
                Type:         subscriber.FieldTypeString,
                Description:  "作者",
                Required:     false,
                DefaultValue: "未知作者",
            },
            "source": {
                Name:        "source",
                Type:        subscriber.FieldTypeString,
                Description: "新闻来源",
                Required:    true,
            },
            "category": {
                Name:         "category",
                Type:         subscriber.FieldTypeString,
                Description:  "新闻分类",
                Required:     false,
                DefaultValue: "综合",
                Validator: func(v interface{}) error {
                    validCategories := []string{"财经", "科技", "体育", "娱乐", "综合"}
                    if str, ok := v.(string); ok {
                        for _, valid := range validCategories {
                            if str == valid {
                                return nil
                            }
                        }
                        return fmt.Errorf("无效的新闻分类，有效值: %v", validCategories)
                    }
                    return nil
                },
            },
            "view_count": {
                Name:         "view_count",
                Type:         subscriber.FieldTypeInt,
                Description:  "浏览次数",
                Required:     false,
                DefaultValue: int64(0),
            },
            "is_featured": {
                Name:         "is_featured",
                Type:         subscriber.FieldTypeBool,
                Description:  "是否为头条",
                Required:     false,
                DefaultValue: false,
            },
            "publish_time": {
                Name:        "publish_time",
                Type:        subscriber.FieldTypeTime,
                Description: "发布时间",
                Required:    true,
            },
        },
        FieldOrder: []string{
            "news_id", "title", "content", "author", "source", 
            "category", "view_count", "is_featured", "publish_time",
        },
    }
}
```

### 3. 金融交易数据模式

```go
// 创建交易数据模式
func createTradeSchema() *subscriber.DataSchema {
    return &subscriber.DataSchema{
        Name:        "trade_data",
        Description: "交易数据",
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
                Validator: func(v interface{}) error {
                    if str, ok := v.(string); ok && len(str) != 6 {
                        return fmt.Errorf("股票代码必须为6位")
                    }
                    return nil
                },
            },
            "price": {
                Name:        "price",
                Type:        subscriber.FieldTypeFloat64,
                Description: "交易价格",
                Required:    true,
                Validator: func(v interface{}) error {
                    if price, ok := v.(float64); ok && price <= 0 {
                        return fmt.Errorf("交易价格必须大于0")
                    }
                    return nil
                },
            },
            "quantity": {
                Name:        "quantity",
                Type:        subscriber.FieldTypeInt,
                Description: "交易数量",
                Required:    true,
                Validator: func(v interface{}) error {
                    if qty, ok := v.(int64); ok && qty <= 0 {
                        return fmt.Errorf("交易数量必须大于0")
                    }
                    return nil
                },
            },
            "side": {
                Name:        "side",
                Type:        subscriber.FieldTypeString,
                Description: "买卖方向",
                Required:    true,
                Validator: func(v interface{}) error {
                    if side, ok := v.(string); ok && side != "买入" && side != "卖出" {
                        return fmt.Errorf("买卖方向必须为'买入'或'卖出'")
                    }
                    return nil
                },
            },
            "commission": {
                Name:         "commission",
                Type:         subscriber.FieldTypeFloat64,
                Description:  "手续费",
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
        FieldOrder: []string{"trade_id", "symbol", "price", "quantity", "side", "commission", "trade_time"},
    }
}
```

## 最佳实践

### 1. 命名规范

- **模式名称**: 使用小写字母和下划线，如 `user_info`, `trade_data`
- **字段名称**: 使用小写字母和下划线，如 `user_id`, `created_at`
- **描述信息**: 使用简洁明了的中文描述

### 2. 字段设计

```go
// 好的字段定义示例
{
    Name:         "user_age",
    Type:         subscriber.FieldTypeInt,
    Description:  "用户年龄",
    Comment:      "范围: 0-150岁",
    Required:     false,
    DefaultValue: 0,
    Validator: func(v interface{}) error {
        if age, ok := v.(int); ok && (age < 0 || age > 150) {
            return fmt.Errorf("年龄必须在0-150之间")
        }
        return nil
    },
}
```

### 3. 验证器使用

```go
// 字符串长度验证
Validator: func(v interface{}) error {
    if str, ok := v.(string); ok && len(str) > 100 {
        return fmt.Errorf("长度不能超过100个字符")
    }
    return nil
}

// 数值范围验证
Validator: func(v interface{}) error {
    if num, ok := v.(float64); ok && (num < 0 || num > 1000) {
        return fmt.Errorf("数值必须在0-1000之间")
    }
    return nil
}

// 枚举值验证
Validator: func(v interface{}) error {
    validValues := []string{"A", "B", "C"}
    if str, ok := v.(string); ok {
        for _, valid := range validValues {
            if str == valid {
                return nil
            }
        }
        return fmt.Errorf("无效值，有效选项: %v", validValues)
    }
    return nil
}
```

### 4. 默认值设置

```go
// 基本类型默认值
DefaultValue: 0          // 整数
DefaultValue: 0.0        // 浮点数
DefaultValue: ""         // 字符串
DefaultValue: false      // 布尔值
DefaultValue: time.Now() // 时间
```

### 5. 字段顺序

```go
// 重要字段在前，辅助字段在后
FieldOrder: []string{
    "id",           // 主键
    "name",         // 主要信息
    "price",        // 核心业务字段
    "description",  // 描述信息
    "created_at",   // 时间字段
    "updated_at",   // 更新时间
}
```

## 使用示例

### 创建和使用自定义数据

```go
func main() {
    // 1. 创建模式
    schema := createUserSchema()
    
    // 2. 创建数据实例
    data := subscriber.NewStructuredData(schema)
    
    // 3. 设置字段值
    data.SetField("user_id", "U001")
    data.SetField("username", "张三")
    data.SetField("age", 25)
    data.SetField("created_at", time.Now())
    
    // 4. 验证数据
    if err := data.ValidateData(); err != nil {
        log.Fatalf("验证失败: %v", err)
    }
    
    // 5. 序列化为CSV
    serializer := subscriber.NewStructuredDataSerializer(subscriber.FormatCSV)
    csvData, err := serializer.Serialize(data)
    if err != nil {
        log.Fatalf("序列化失败: %v", err)
    }
    
    fmt.Println("CSV数据:")
    fmt.Println(string(csvData))
}
```

### 与存储系统集成

```go
func main() {
    // 创建内存存储
    storage := storage.NewMemoryStorage(storage.DefaultMemoryStorageConfig())
    defer storage.Close()
    
    // 创建数据
    schema := createUserSchema()
    data := subscriber.NewStructuredData(schema)
    data.SetField("user_id", "U001")
    data.SetField("username", "张三")
    
    // 保存数据
    ctx := context.Background()
    if err := storage.Save(ctx, data); err != nil {
        log.Fatalf("保存失败: %v", err)
    }
    
    // 查询数据
    query := core.Query{
        Filters: map[string]interface{}{
            "user_id": "U001",
        },
    }
    
    results, err := storage.Load(ctx, query)
    if err != nil {
        log.Fatalf("查询失败: %v", err)
    }
    
    fmt.Printf("找到 %d 条记录\n", len(results))
}
```

## 注意事项

1. **性能考虑**: 验证器函数会在每次设置字段值时调用，避免在验证器中执行耗时操作
2. **类型安全**: 确保字段类型与实际值类型匹配
3. **内存管理**: 大量数据时注意内存使用，考虑使用批量操作
4. **并发安全**: StructuredData 本身不是并发安全的，在多协程环境中需要额外的同步机制
5. **向后兼容**: 修改现有模式时要考虑数据的向后兼容性

## 模式演进

当需要修改现有模式时，推荐的方法：

1. **添加新字段**: 设置为非必填并提供默认值
2. **废弃字段**: 保留字段定义但不再使用
3. **重命名字段**: 创建新字段并逐步迁移数据
4. **修改类型**: 创建新模式版本并进行数据迁移

通过遵循这些指南，您可以充分利用 StructuredData 的灵活性来处理各种类型的结构化数据需求。