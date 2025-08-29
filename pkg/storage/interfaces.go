package storage

import (
	"context"

	"stocksub/pkg/core"
)

// Storage 定义了持久化存储的行为。
// 任何希望在 testkit 中作为存储后端（如CSV、数据库）的组件都必须实现此接口。
type Storage interface {
	// Save 保存一条数据记录到存储后端。
	Save(ctx context.Context, data interface{}) error
	// Load 根据查询条件从存储后端加载数据。
	Load(ctx context.Context, query core.Query) ([]interface{}, error)
	// Delete 根据查询条件从存储后端删除数据。
	Delete(ctx context.Context, query core.Query) error
	// Close 关闭存储连接并释放所有资源。
	Close() error
}

// ResourceManager 定义了对可复用资源（如缓冲区、CSV写入器）的管理接口。
type ResourceManager1 interface {
	AcquireCSVWriter() interface{}
	ReleaseCSVWriter(writer interface{})
	AcquireBuffer() interface{}
	ReleaseBuffer(buffer interface{})
	RegisterCleanup(fn func())
	Cleanup()
}

// Serializer 定义了对象和字节流之间相互转换的接口。
type Serializer interface {
	// Serialize 将任意对象序列化为字节数组。
	Serialize(data interface{}) ([]byte, error)
	// Deserialize 将字节数组反序列化为目标对象。
	Deserialize(data []byte, target interface{}) error
	// MimeType 返回此序列化器对应的MIME类型。
	MimeType() string
}
