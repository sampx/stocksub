package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type Entry = logrus.Entry

var (
	// Logger 全局日志实例
	Logger *logrus.Logger
)

// Config 日志配置
type Config struct {
	Level  string `json:"level"`  // debug, info, warn, error
	Format string `json:"format"` // text, json
}

// Init 初始化日志器
func Init(config Config) {
	Logger = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Logger.SetLevel(level)

	// 设置格式
	if config.Format == "json" {
		Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000",
		})
	} else {
		Logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000",
			FullTimestamp:   true,
			ForceColors:     true,
		})
	}

	Logger.SetOutput(os.Stdout)
}

// InitFromEnv 从环境变量初始化日志器
func InitFromEnv() {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		if os.Getenv("DEBUG") == "1" {
			level = "debug"
		} else {
			level = "info"
		}
	}

	format := os.Getenv("LOG_FORMAT")
	if format == "" {
		format = "text"
	}

	Init(Config{
		Level:  level,
		Format: format,
	})
}

// GetLogger 获取日志器实例
func GetLogger() *logrus.Logger {
	if Logger == nil {
		InitFromEnv()
	}
	return Logger
}

// WithComponent 创建带组件名的日志器
func WithComponent(component string) *logrus.Entry {
	return GetLogger().WithField("component", component)
}

// Debug 调试日志
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info 信息日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn 警告日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error 错误日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// SetLevel 设置日志级别
func SetLevel(level string) {
	l, err := logrus.ParseLevel(strings.ToLower(level))
	if err != nil {
		l = logrus.InfoLevel
	}
	GetLogger().SetLevel(l)
}
