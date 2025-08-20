package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// Logger 日志记录器
type Logger struct {
	level      Level
	output     io.Writer
	mu         sync.Mutex
	prefix     string
	timeFormat string
}

// Config 日志配置
type Config struct {
	Level      string `json:"level"`       // debug, info, warn, error
	Output     string `json:"output"`      // console, file, both
	Filename   string `json:"filename"`    // 日志文件名
	MaxSize    int    `json:"max_size"`    // 最大文件大小(MB)
	MaxBackups int    `json:"max_backups"` // 最大备份数
	MaxAge     int    `json:"max_age"`     // 最大保存天数
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Init 初始化全局日志器
func Init(config Config) error {
	var err error
	once.Do(func() {
		defaultLogger, err = NewLogger(config)
	})
	return err
}

// NewLogger 创建新的日志器
func NewLogger(config Config) (*Logger, error) {
	level := parseLevel(config.Level)

	var output io.Writer
	switch strings.ToLower(config.Output) {
	case "console":
		output = os.Stdout
	case "file":
		if config.Filename == "" {
			config.Filename = "stocksub.log"
		}
		file, err := openLogFile(config.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = file
	case "both":
		if config.Filename == "" {
			config.Filename = "stocksub.log"
		}
		file, err := openLogFile(config.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = io.MultiWriter(os.Stdout, file)
	default:
		output = os.Stdout
	}

	return &Logger{
		level:      level,
		output:     output,
		prefix:     "[StockSub]",
		timeFormat: "2006-01-02 15:04:05",
	}, nil
}

// parseLevel 解析日志级别
func parseLevel(levelStr string) Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// openLogFile 打开日志文件
func openLogFile(filename string) (*os.File, error) {
	dir := filepath.Dir(filename)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// log 通用日志记录方法
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format(l.timeFormat)
	levelName := levelNames[level]
	message := fmt.Sprintf(format, args...)

	logLine := fmt.Sprintf("%s %s [%s] %s\n", timestamp, l.prefix, levelName, message)
	l.output.Write([]byte(logLine))
}

// Debug 调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Info 信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Error 错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetPrefix 设置日志前缀
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// 全局日志方法
func Debug(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Debug(format, args...)
	} else {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Info(format, args...)
	} else {
		log.Printf("[INFO] "+format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warn(format, args...)
	} else {
		log.Printf("[WARN] "+format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Error(format, args...)
	} else {
		log.Printf("[ERROR] "+format, args...)
	}
}

// GetDefaultLogger 获取默认日志器
func GetDefaultLogger() *Logger {
	return defaultLogger
}
