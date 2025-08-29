package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
)

// JobConfig 定义单个任务的配置
type JobConfig struct {
	Name     string                 `yaml:"name" json:"name"`
	Enabled  bool                   `yaml:"enabled" json:"enabled"`
	Schedule string                 `yaml:"schedule" json:"schedule"`
	Provider ProviderConfig         `yaml:"provider" json:"provider"`
	Params   map[string]interface{} `yaml:"params" json:"params"`
	Output   *OutputConfig          `yaml:"output,omitempty" json:"output,omitempty"`
}

// ProviderConfig 定义提供商配置
type ProviderConfig struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
}

// OutputConfig 定义输出配置
type OutputConfig struct {
	Type      string `yaml:"type" json:"type"`
	Directory string `yaml:"directory,omitempty" json:"directory,omitempty"`
	Stream    string `yaml:"stream,omitempty" json:"stream,omitempty"`
}

// JobsConfig 定义整个任务配置文件结构
type JobsConfig struct {
	Jobs []JobConfig `yaml:"jobs" json:"jobs"`
}

// Job 表示一个运行中的任务
type Job struct {
	ID         string
	Config     JobConfig
	EntryID    cron.EntryID
	Status     JobStatus
	LastRun    *time.Time
	NextRun    *time.Time
	RunCount   int64
	ErrorCount int64
	LastError  error
}

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusPending  JobStatus = "pending"
	JobStatusRunning  JobStatus = "running"
	JobStatusStopped  JobStatus = "stopped"
	JobStatusError    JobStatus = "error"
	JobStatusDisabled JobStatus = "disabled"
)

// JobExecutor 任务执行器接口
type JobExecutor interface {
	Execute(ctx context.Context, job *Job) error
}

// JobScheduler 任务调度器接口
type JobScheduler interface {
	// 加载配置
	LoadConfig(configPath string) error

	// 启动调度器
	Start() error

	// 停止调度器
	Stop() error

	// 添加任务
	AddJob(config JobConfig) error

	// 移除任务
	RemoveJob(jobName string) error

	// 获取任务状态
	GetJob(jobName string) (*Job, error)

	// 获取所有任务
	GetAllJobs() []*Job

	// 手动执行任务
	RunJob(jobName string) error

	// 设置任务执行器
	SetExecutor(executor JobExecutor)
}
