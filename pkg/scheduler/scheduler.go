package scheduler

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// DefaultJobScheduler 默认任务调度器实现
type DefaultJobScheduler struct {
	cron     *cron.Cron
	jobs     map[string]*Job
	executor JobExecutor
	mu       sync.RWMutex
	logger   *logrus.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewJobScheduler 创建新的任务调度器
func NewJobScheduler() *DefaultJobScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultJobScheduler{
		cron:   cron.New(cron.WithSeconds()),
		jobs:   make(map[string]*Job),
		logger: logrus.New(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// LoadConfig 从配置文件加载任务配置
func (s *DefaultJobScheduler) LoadConfig(configPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", configPath)
	}

	// 使用 viper 加载配置
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config JobsConfig
	if err := v.Unmarshal(&config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证并添加任务
	for _, jobConfig := range config.Jobs {
		if err := s.validateJobConfig(jobConfig); err != nil {
			s.logger.WithError(err).Warnf("跳过无效任务配置: %s", jobConfig.Name)
			continue
		}

		if err := s.addJobInternal(jobConfig); err != nil {
			s.logger.WithError(err).Errorf("添加任务失败: %s", jobConfig.Name)
			continue
		}
	}

	s.logger.Infof("成功加载 %d 个任务配置", len(s.jobs))
	return nil
}

// Start 启动调度器
func (s *DefaultJobScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor == nil {
		return fmt.Errorf("任务执行器未设置")
	}

	s.cron.Start()
	s.logger.Info("任务调度器已启动")

	// 更新任务的下次运行时间
	s.updateNextRunTimes()

	return nil
}

// Stop 停止调度器
func (s *DefaultJobScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancel()
	ctx := s.cron.Stop()

	// 等待所有任务完成
	select {
	case <-ctx.Done():
		s.logger.Info("任务调度器已停止")
	case <-time.After(30 * time.Second):
		s.logger.Warn("任务调度器停止超时")
	}

	return nil
}

// AddJob 添加任务
func (s *DefaultJobScheduler) AddJob(config JobConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateJobConfig(config); err != nil {
		return err
	}

	return s.addJobInternal(config)
}

// RemoveJob 移除任务
func (s *DefaultJobScheduler) RemoveJob(jobName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobName]
	if !exists {
		return fmt.Errorf("任务不存在: %s", jobName)
	}

	s.cron.Remove(job.EntryID)
	delete(s.jobs, jobName)

	s.logger.Infof("任务已移除: %s", jobName)
	return nil
}

// GetJob 获取任务状态
func (s *DefaultJobScheduler) GetJob(jobName string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[jobName]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", jobName)
	}

	// 创建副本避免并发修改
	jobCopy := *job
	return &jobCopy, nil
}

// GetAllJobs 获取所有任务
func (s *DefaultJobScheduler) GetAllJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}

	return jobs
}

// RunJob 手动执行任务
func (s *DefaultJobScheduler) RunJob(jobName string) error {
	s.mu.RLock()
	job, exists := s.jobs[jobName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务不存在: %s", jobName)
	}

	if !job.Config.Enabled {
		return fmt.Errorf("任务已禁用: %s", jobName)
	}

	// 在新的 goroutine 中执行任务
	go s.executeJob(job)
	return nil
}

// SetExecutor 设置任务执行器
func (s *DefaultJobScheduler) SetExecutor(executor JobExecutor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executor = executor
}

// validateJobConfig 验证任务配置
func (s *DefaultJobScheduler) validateJobConfig(config JobConfig) error {
	if config.Name == "" {
		return fmt.Errorf("任务名称不能为空")
	}

	if config.Schedule == "" {
		return fmt.Errorf("任务调度表达式不能为空")
	}

	// 验证 cron 表达式 - 支持秒级调度
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(config.Schedule); err != nil {
		return fmt.Errorf("无效的调度表达式 '%s': %w", config.Schedule, err)
	}

	if config.Provider.Name == "" {
		return fmt.Errorf("提供商名称不能为空")
	}

	if config.Provider.Type == "" {
		return fmt.Errorf("提供商类型不能为空")
	}

	return nil
}

// addJobInternal 内部添加任务方法（需要持有锁）
func (s *DefaultJobScheduler) addJobInternal(config JobConfig) error {
	// 检查任务是否已存在
	if _, exists := s.jobs[config.Name]; exists {
		return fmt.Errorf("任务已存在: %s", config.Name)
	}

	// 创建任务
	job := &Job{
		ID:     uuid.New().String(),
		Config: config,
		Status: JobStatusPending,
	}

	if !config.Enabled {
		job.Status = JobStatusDisabled
		s.jobs[config.Name] = job
		s.logger.Infof("任务已添加（已禁用）: %s", config.Name)
		return nil
	}

	// 添加到 cron 调度器
	entryID, err := s.cron.AddFunc(config.Schedule, func() {
		s.executeJob(job)
	})
	if err != nil {
		return fmt.Errorf("添加任务到调度器失败: %w", err)
	}

	job.EntryID = entryID
	s.jobs[config.Name] = job

	s.logger.Infof("任务已添加: %s (调度: %s)", config.Name, config.Schedule)
	return nil
}

// executeJob 执行任务
func (s *DefaultJobScheduler) executeJob(job *Job) {
	s.mu.Lock()
	if job.Status == JobStatusRunning {
		s.mu.Unlock()
		s.logger.Warnf("任务正在运行，跳过本次执行: %s", job.Config.Name)
		return
	}
	job.Status = JobStatusRunning
	now := time.Now()
	job.LastRun = &now
	job.RunCount++
	s.mu.Unlock()

	s.logger.Infof("开始执行任务: %s", job.Config.Name)

	// 执行任务
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute) // 默认5分钟超时
	defer cancel()

	err := s.executor.Execute(ctx, job)

	s.mu.Lock()
	if err != nil {
		job.Status = JobStatusError
		job.LastError = err
		job.ErrorCount++
		s.logger.WithError(err).Errorf("任务执行失败: %s", job.Config.Name)
	} else {
		job.Status = JobStatusPending
		job.LastError = nil
		s.logger.Infof("任务执行成功: %s", job.Config.Name)
	}
	s.mu.Unlock()
}

// updateNextRunTimes 更新所有任务的下次运行时间
func (s *DefaultJobScheduler) updateNextRunTimes() {
	entries := s.cron.Entries()
	for _, job := range s.jobs {
		if job.Config.Enabled {
			for _, entry := range entries {
				if entry.ID == job.EntryID {
					nextRun := entry.Next
					job.NextRun = &nextRun
					break
				}
			}
		}
	}
}
