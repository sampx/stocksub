package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stocksub/pkg/logger"
	"stocksub/pkg/provider"
	"stocksub/pkg/provider/decorators"
	"stocksub/pkg/provider/sina"
	"stocksub/pkg/provider/tencent"
	"stocksub/pkg/scheduler"

	"github.com/go-redis/redis/v8"
)

var (
	configPath = flag.String("config", "config/jobs.yaml", "任务配置文件路径")
	redisAddr  = flag.String("redis", "localhost:6379", "Redis 服务器地址")
	redisPass  = flag.String("redis-pass", "", "Redis 密码")
	nodeID     = flag.String("node-id", "", "节点ID（默认自动生成）")
	logLevel   = flag.String("log-level", "info", "日志级别")
	logFormat  = flag.String("log-format", "json", "日志格式 (json 或 text)")
)

func main() {
	flag.Parse()

	// 初始化日志系统
	logger.Init(logger.Config{
		Level:  *logLevel,
		Format: *logFormat,
	})

	log := logger.WithComponent("fetcher")

	// 生成节点ID
	if *nodeID == "" {
		*nodeID = fmt.Sprintf("fetcher-%d", time.Now().Unix())
	}

	log.WithField("nodeID", *nodeID).Info("启动 Fetcher")
	log.Debugf("配置参数: config=%s, redis=%s, logLevel=%s, logFormat=%s", *configPath, *redisAddr, *logLevel, *logFormat)

	// 创建 Redis 客户端
	log.Debugf("创建 Redis 客户端: %s", *redisAddr)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: *redisPass,
		DB:       0,
	})

	// 测试 Redis 连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Errorf("无法连接到 Redis: %v", err)
		os.Exit(1)
	}
	log.Info("Redis 连接成功")

	// 创建提供商管理器
	log.Debug("初始化提供商管理器")
	providerManager := provider.NewProviderManager()

	// 注册腾讯提供商
	log.Debug("创建腾讯数据提供商")
	tencentProvider := tencent.NewClient()

	// 应用装饰器
	decoratedProvider, err := decorators.CreateDecoratedProvider(tencentProvider, decorators.DefaultDecoratorConfig())
	if err != nil {
		log.Warnf("应用腾讯提供商装饰器失败: %v，使用原始提供商", err)
		decoratedProvider = tencentProvider
	} else {
		log.Debug("腾讯提供商装饰器应用成功")
	}

	if realtimeProvider, ok := decoratedProvider.(provider.RealtimeStockProvider); ok {
		if err := providerManager.RegisterRealtimeStockProvider("tencent", realtimeProvider); err != nil {
			log.Errorf("注册腾讯提供商失败: %v", err)
			os.Exit(1)
		}
	} else {
		log.Error("装饰后的腾讯提供商未实现 RealtimeStockProvider 接口")
		os.Exit(1)
	}
	log.Info("腾讯数据提供商注册成功")

	// 注册新浪提供商
	log.Debug("创建新浪数据提供商")
	sinaProvider := sina.NewClient()
	decoratedSinaProvider, err := decorators.CreateDecoratedProvider(sinaProvider, decorators.DefaultDecoratorConfig())
	if err != nil {
		log.Warnf("应用新浪提供商装饰器失败: %v，使用原始提供商", err)
		decoratedSinaProvider = sinaProvider
	} else {
		log.Debug("新浪提供商装饰器应用成功")
	}
	if realtimeSinaProvider, ok := decoratedSinaProvider.(provider.RealtimeStockProvider); ok {
		if err := providerManager.RegisterRealtimeStockProvider("sina", realtimeSinaProvider); err != nil {
			log.Errorf("注册新浪提供商失败: %v", err)
			os.Exit(1)
		}
	} else {
		log.Error("装饰后的新浪提供商未实现 RealtimeStockProvider 接口")
		os.Exit(1)
	}
	log.Info("新浪数据提供商注册成功")

	// 创建任务执行器
	log.Debug("创建任务执行器")
	executor := NewFetcherExecutor(providerManager, redisClient, *nodeID, log)

	// 创建任务调度器
	log.Debug("创建任务调度器")
	jobScheduler := scheduler.NewJobScheduler()
	jobScheduler.SetExecutor(executor)

	// 加载配置
	log.Debugf("加载任务配置文件: %s", *configPath)
	if err := jobScheduler.LoadConfig(*configPath); err != nil {
		log.Errorf("加载任务配置失败: %v", err)
		os.Exit(1)
	}

	// 启动调度器
	log.Debug("启动任务调度器")
	if err := jobScheduler.Start(); err != nil {
		log.Errorf("启动任务调度器失败: %v", err)
		os.Exit(1)
	}

	// 打印任务状态
	jobs := jobScheduler.GetAllJobs()
	log.Infof("已加载 %d 个任务", len(jobs))
	for _, job := range jobs {
		status := "启用"
		if !job.Config.Enabled {
			status = "禁用"
		}
		log.Debugf("任务详情: %s (%s): %s", job.Config.Name, status, job.Config.Schedule)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Fetcher 运行中，按 Ctrl+C 停止...")
	<-sigChan

	log.Info("收到停止信号，正在优雅关闭...")

	// 停止调度器
	log.Debug("停止任务调度器")
	if err := jobScheduler.Stop(); err != nil {
		log.Errorf("停止任务调度器失败: %v", err)
	} else {
		log.Debug("任务调度器停止成功")
	}

	// 关闭 Redis 连接
	log.Debug("关闭 Redis 连接")
	if err := redisClient.Close(); err != nil {
		log.Errorf("关闭 Redis 连接失败: %v", err)
	} else {
		log.Debug("Redis 连接关闭成功")
	}

	log.Info("Fetcher 已停止")
}
