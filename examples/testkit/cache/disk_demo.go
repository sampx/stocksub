package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stocksub/pkg/cache"
)

func diskCacheDemo() {
	fmt.Println("\n5. 磁盘持久化缓存 (DiskCache) 演示")

	// 1. 指定一个固定的目录用于演示, 便于查看生成的文件
	cacheDir := "_data/cache"
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		fmt.Printf("  ✗ 创建演示目录 '%s' 失败: %v\n", cacheDir, err)
		return
	}
	fmt.Printf("  ✓ 缓存目录已指定: %s\n", cacheDir)
	// defer os.RemoveAll(cacheDir) // 演示需要，暂时禁用自动清理

	config := cache.DiskCacheConfig{
		BaseDir:         cacheDir,
		MaxSize:         100,
		DefaultTTL:      10 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		FilePrefix:      "demo_cache", // 使用自定义前缀
	}

	// 2. 创建第一个cache实例并写入数据
	fmt.Println("\n  → 步骤1: 创建实例并写入数据")
	cache1, err := cache.NewDiskCache(config)
	if err != nil {
		fmt.Printf("  ✗ 创建DiskCache实例1失败: %v\n", err)
		return
	}

	ctx := context.Background()
	stockData := map[string]string{"name": "中国平安", "price": "67.89", "code": "601318"}
	err = cache1.Set(ctx, "stock:601318", stockData, 0)
	if err != nil {
		fmt.Printf("  ✗ 写入数据失败: %v\n", err)
		cache1.Close()
		return
	}
	fmt.Println("  ✓ 已将 'stock:601318' 存入磁盘缓存")

	// 3. 关闭第一个实例, 触发元数据保存
	cache1.Close()
	fmt.Println("  ✓ 实例1已关闭, 元数据已保存到 metadata.json")

	// 4. 读取并展示生成的缓存文件内容
	fmt.Println("\n  → 步骤2: 查看磁盘上生成的缓存文件")
	cachePath := filepath.Join(cacheDir, "demo_cache")

	// 读取数据文件
	files, _ := filepath.Glob(filepath.Join(cachePath, "*.json"))
	var dataFilePath string
	for _, f := range files {
		if !strings.HasSuffix(f, "metadata.json") {
			dataFilePath = f
			break
		}
	}

	if dataFilePath != "" {
		content, _ := os.ReadFile(dataFilePath)
		fmt.Printf("  ✓ 读取数据文件 (%s) 内容:\n    %s\n", filepath.Base(dataFilePath), string(content))
	} else {
		fmt.Println("  ✗ 未找到数据文件 (*.json)")
	}

	// 读取元数据文件
	metadataPath := filepath.Join(cachePath, "metadata.json")
	if content, err := os.ReadFile(metadataPath); err == nil {
		fmt.Printf("  ✓ 读取元数据文件 (metadata.json) 内容:\n    %s\n", string(content))
	} else {
		fmt.Println("  ✗ 未找到元数据文件 (metadata.json)")
	}

	// 5. 创建第二个实例, 它会自动加载元数据
	fmt.Println("\n  → 步骤3: 创建新实例并从持久化数据中恢复")
	cache2, err := cache.NewDiskCache(config)
	if err != nil {
		fmt.Printf("  ✗ 创建DiskCache实例2失败: %v\n", err)
		return
	}
	defer cache2.Close()

	// 6. 从第二个实例中读取数据进行验证
	value, err := cache2.Get(ctx, "stock:601318")
	if err != nil {
		fmt.Printf("  ✗ 从实例2读取持久化数据失败: %v\n", err)
		return
	}

	if data, ok := value.(map[string]interface{}); ok {
		fmt.Printf("  ✓ 加载并验证成功: %s -> 名称:%s, 价格:%s\n", "stock:601318", data["name"], data["price"])
	} else {
		fmt.Printf("  ✗ 数据类型断言失败, 获取到的数据类型是: %T\n", value)
	}

	fmt.Println("\n  (提示: 演示程序已将缓存文件保留在 _data/cache 目录中供您检查。)")
}
