package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockJobExecutor 模拟任务执行器
type MockJobExecutor struct {
	executedJobs []string
	shouldError  bool
	errorMsg     string
}

func (m *MockJobExecutor) Execute(ctx context.Context, job *Job) error {
	m.executedJobs = append(m.executedJobs, job.Config.Name)
	if m.shouldError {
		return fmt.Errorf(m.errorMsg)
	}
	return nil
}

func TestNewJobScheduler(t *testing.T) {
	scheduler := NewJobScheduler()

	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.cron)
	assert.NotNil(t, scheduler.jobs)
	assert.NotNil(t, scheduler.logger)
	assert.NotNil(t, scheduler.ctx)
}

func TestJobScheduler_LoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		expectJobs  int
	}{
		{
			name: "有效配置",
			configYAML: `
jobs:
  - name: "test-job-1"
    enabled: true
    schedule: "*/5 * * * * *"
    provider:
      name: "test-provider"
      type: "RealtimeStock"
    params:
      symbols: ["600000"]
  - name: "test-job-2"
    enabled: false
    schedule: "0 * * * * *"
    provider:
      name: "test-provider"
      type: "RealtimeStock"
    params:
      symbols: ["000001"]
`,
			expectError: false,
			expectJobs:  2,
		},
		{
			name: "无效的 cron 表达式",
			configYAML: `
jobs:
  - name: "invalid-job"
    enabled: true
    schedule: "invalid-cron"
    provider:
      name: "test-provider"
      type: "RealtimeStock"
    params:
      symbols: ["600000"]
`,
			expectError: false, // 无效任务会被跳过，不会导致整体失败
			expectJobs:  0,
		},
		{
			name: "缺少必要字段",
			configYAML: `
jobs:
  - name: ""
    enabled: true
    schedule: "*/5 * * * * *"
    provider:
      name: "test-provider"
      type: "RealtimeStock"
`,
			expectError: false, // 无效任务会被跳过
			expectJobs:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建临时配置文件
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")

			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			require.NoError(t, err)

			// 创建调度器并加载配置
			scheduler := NewJobScheduler()
			err = scheduler.LoadConfig(configPath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, scheduler.jobs, tt.expectJobs)
			}
		})
	}
}

func TestJobScheduler_AddJob(t *testing.T) {
	scheduler := NewJobScheduler()

	validJob := JobConfig{
		Name:     "test-job",
		Enabled:  true,
		Schedule: "*/5 * * * * *",
		Provider: ProviderConfig{
			Name: "test-provider",
			Type: "RealtimeStock",
		},
		Params: map[string]interface{}{
			"symbols": []string{"600000"},
		},
	}

	// 测试添加有效任务
	err := scheduler.AddJob(validJob)
	assert.NoError(t, err)

	// 验证任务已添加
	job, err := scheduler.GetJob("test-job")
	assert.NoError(t, err)
	assert.Equal(t, "test-job", job.Config.Name)
	assert.Equal(t, JobStatusPending, job.Status)

	// 测试添加重复任务
	err = scheduler.AddJob(validJob)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务已存在")

	// 测试添加无效任务
	invalidJob := JobConfig{
		Name:     "invalid-job",
		Enabled:  true,
		Schedule: "invalid-cron",
		Provider: ProviderConfig{
			Name: "test-provider",
			Type: "RealtimeStock",
		},
	}

	err = scheduler.AddJob(invalidJob)
	assert.Error(t, err)
}

func TestJobScheduler_RemoveJob(t *testing.T) {
	scheduler := NewJobScheduler()

	// 添加测试任务
	job := JobConfig{
		Name:     "test-job",
		Enabled:  true,
		Schedule: "*/5 * * * * *",
		Provider: ProviderConfig{
			Name: "test-provider",
			Type: "RealtimeStock",
		},
	}

	err := scheduler.AddJob(job)
	require.NoError(t, err)

	// 测试移除存在的任务
	err = scheduler.RemoveJob("test-job")
	assert.NoError(t, err)

	// 验证任务已移除
	_, err = scheduler.GetJob("test-job")
	assert.Error(t, err)

	// 测试移除不存在的任务
	err = scheduler.RemoveJob("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务不存在")
}

func TestJobScheduler_GetAllJobs(t *testing.T) {
	scheduler := NewJobScheduler()

	// 初始状态应该没有任务
	jobs := scheduler.GetAllJobs()
	assert.Len(t, jobs, 0)

	// 添加几个任务
	for i := 0; i < 3; i++ {
		job := JobConfig{
			Name:     fmt.Sprintf("test-job-%d", i),
			Enabled:  true,
			Schedule: "*/5 * * * * *",
			Provider: ProviderConfig{
				Name: "test-provider",
				Type: "RealtimeStock",
			},
		}
		err := scheduler.AddJob(job)
		require.NoError(t, err)
	}

	// 验证返回所有任务
	jobs = scheduler.GetAllJobs()
	assert.Len(t, jobs, 3)

	// 验证返回的是副本，不会影响原始数据
	jobs[0].Status = JobStatusError
	originalJob, err := scheduler.GetJob("test-job-0")
	require.NoError(t, err)
	assert.NotEqual(t, JobStatusError, originalJob.Status)
}

func TestJobScheduler_RunJob(t *testing.T) {
	scheduler := NewJobScheduler()
	executor := &MockJobExecutor{}
	scheduler.SetExecutor(executor)

	// 添加测试任务
	job := JobConfig{
		Name:     "test-job",
		Enabled:  true,
		Schedule: "*/5 * * * * *",
		Provider: ProviderConfig{
			Name: "test-provider",
			Type: "RealtimeStock",
		},
	}

	err := scheduler.AddJob(job)
	require.NoError(t, err)

	// 测试手动执行任务
	err = scheduler.RunJob("test-job")
	assert.NoError(t, err)

	// 等待任务执行完成
	time.Sleep(100 * time.Millisecond)

	// 验证任务已执行
	assert.Contains(t, executor.executedJobs, "test-job")

	// 测试执行不存在的任务
	err = scheduler.RunJob("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务不存在")

	// 测试执行禁用的任务
	disabledJob := JobConfig{
		Name:     "disabled-job",
		Enabled:  false,
		Schedule: "*/5 * * * * *",
		Provider: ProviderConfig{
			Name: "test-provider",
			Type: "RealtimeStock",
		},
	}

	err = scheduler.AddJob(disabledJob)
	require.NoError(t, err)

	err = scheduler.RunJob("disabled-job")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务已禁用")
}

func TestJobScheduler_StartStop(t *testing.T) {
	scheduler := NewJobScheduler()
	executor := &MockJobExecutor{}
	scheduler.SetExecutor(executor)

	// 测试启动调度器
	err := scheduler.Start()
	assert.NoError(t, err)

	// 测试停止调度器
	err = scheduler.Stop()
	assert.NoError(t, err)

	// 测试没有执行器时启动
	scheduler2 := NewJobScheduler()
	err = scheduler2.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务执行器未设置")
}

func TestJobScheduler_validateJobConfig(t *testing.T) {
	scheduler := NewJobScheduler()

	tests := []struct {
		name        string
		config      JobConfig
		expectError bool
	}{
		{
			name: "有效配置",
			config: JobConfig{
				Name:     "valid-job",
				Schedule: "*/5 * * * * *",
				Provider: ProviderConfig{
					Name: "test-provider",
					Type: "RealtimeStock",
				},
			},
			expectError: false,
		},
		{
			name: "缺少任务名称",
			config: JobConfig{
				Name:     "",
				Schedule: "*/5 * * * * *",
				Provider: ProviderConfig{
					Name: "test-provider",
					Type: "RealtimeStock",
				},
			},
			expectError: true,
		},
		{
			name: "缺少调度表达式",
			config: JobConfig{
				Name:     "test-job",
				Schedule: "",
				Provider: ProviderConfig{
					Name: "test-provider",
					Type: "RealtimeStock",
				},
			},
			expectError: true,
		},
		{
			name: "无效的调度表达式",
			config: JobConfig{
				Name:     "test-job",
				Schedule: "invalid-cron",
				Provider: ProviderConfig{
					Name: "test-provider",
					Type: "RealtimeStock",
				},
			},
			expectError: true,
		},
		{
			name: "缺少提供商名称",
			config: JobConfig{
				Name:     "test-job",
				Schedule: "*/5 * * * * *",
				Provider: ProviderConfig{
					Name: "",
					Type: "RealtimeStock",
				},
			},
			expectError: true,
		},
		{
			name: "缺少提供商类型",
			config: JobConfig{
				Name:     "test-job",
				Schedule: "*/5 * * * * *",
				Provider: ProviderConfig{
					Name: "test-provider",
					Type: "",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scheduler.validateJobConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJobScheduler_Integration(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "integration-test.yaml")

	configYAML := `
jobs:
  - name: "integration-test-job"
    enabled: true
    schedule: "*/1 * * * * *"  # 每秒执行一次
    provider:
      name: "test-provider"
      type: "RealtimeStock"
    params:
      symbols: ["600000", "000001"]
`

	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	require.NoError(t, err)

	// 创建调度器和执行器
	scheduler := NewJobScheduler()
	executor := &MockJobExecutor{}
	scheduler.SetExecutor(executor)

	// 加载配置
	err = scheduler.LoadConfig(configPath)
	require.NoError(t, err)

	// 验证任务已加载
	jobs := scheduler.GetAllJobs()
	require.Len(t, jobs, 1)
	assert.Equal(t, "integration-test-job", jobs[0].Config.Name)
	assert.True(t, jobs[0].Config.Enabled)

	// 启动调度器
	err = scheduler.Start()
	require.NoError(t, err)

	// 等待任务执行几次
	time.Sleep(2500 * time.Millisecond)

	// 停止调度器
	err = scheduler.Stop()
	require.NoError(t, err)

	// 验证任务已执行
	assert.True(t, len(executor.executedJobs) >= 2, "任务应该至少执行2次")

	for _, executedJob := range executor.executedJobs {
		assert.Equal(t, "integration-test-job", executedJob)
	}

	// 验证任务状态
	job, err := scheduler.GetJob("integration-test-job")
	require.NoError(t, err)
	assert.True(t, job.RunCount >= 2, "运行次数应该至少为2")
	assert.NotNil(t, job.LastRun)
}
