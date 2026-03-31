// Package logger 提供统一的日志接口，基于 zap 实现
package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger    *zap.Logger
	sugar     *zap.SugaredLogger
	atomLevel zap.AtomicLevel
	logFile   *os.File
)

// Options 日志初始化选项
type Options struct {
	Debug     bool
	WriteFile bool
	LogDir    string
}

// Init 初始化日志系统
func Init(debug bool) error {
	return InitWithOptions(Options{Debug: debug})
}

// InitWithOptions 使用选项初始化日志系统
func InitWithOptions(opts Options) error {
	atomLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	if opts.Debug {
		atomLevel = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	encoderConfig := zap.NewDevelopmentEncoderConfig()
	if !opts.Debug {
		encoderConfig = zap.NewProductionEncoderConfig()
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	if !opts.Debug && !opts.WriteFile {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var writeSyncer zapcore.WriteSyncer

	if opts.WriteFile {
		logDir := opts.LogDir
		if logDir == "" {
			homeDir, _ := os.UserHomeDir()
			logDir = filepath.Join(homeDir, ".simpleclaw", "logs")
		}
		if err := os.MkdirAll(logDir, 0755); err != nil {
			writeSyncer = zapcore.AddSync(os.Stderr)
		} else {
			logPath := filepath.Join(logDir, "simpleclaw.log")
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				writeSyncer = zapcore.AddSync(os.Stderr)
			} else {
				logFile = f
				writeSyncer = zapcore.AddSync(f)
			}
		}
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	core := zapcore.NewCore(encoder, writeSyncer, atomLevel)
	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	sugar = logger.Sugar()

	return nil
}

// Close 关闭日志系统
func Close() error {
	if logFile != nil {
		return logFile.Close()
	}
	return nil
}

// SetLevel 动态设置日志级别
func SetLevel(level zapcore.Level) {
	if atomLevel.Level() != level {
		atomLevel.SetLevel(level)
	}
}

// IsDebug 是否为调试模式
func IsDebug() bool {
	return atomLevel.Level() == zapcore.DebugLevel
}

// GetLogger 获取原始 zap logger
func GetLogger() *zap.Logger {
	if logger == nil {
		// 未初始化时使用默认配置
		logger = zap.NewNop()
		sugar = logger.Sugar()
	}
	return logger
}

// GetSugar 获取 sugared logger（用于格式化日志）
func GetSugar() *zap.SugaredLogger {
	if sugar == nil {
		// 未初始化时使用默认配置
		logger = zap.NewNop()
		sugar = logger.Sugar()
	}
	return sugar
}

// Debug 记录调试级别日志
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Info 记录信息级别日志
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Warn 记录警告级别日志
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Error 记录错误级别日志
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Fatal 记录致命级别日志并退出程序
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// Debugf 记录格式化调试级别日志
func Debugf(format string, args ...any) {
	GetSugar().Debugf(format, args...)
}

// Infof 记录格式化信息级别日志
func Infof(format string, args ...any) {
	GetSugar().Infof(format, args...)
}

// Warnf 记录格式化警告级别日志
func Warnf(format string, args ...any) {
	GetSugar().Warnf(format, args...)
}

// Errorf 记录格式化错误级别日志
func Errorf(format string, args ...any) {
	GetSugar().Errorf(format, args...)
}

// Fatalf 记录格式化致命级别日志并退出程序
func Fatalf(format string, args ...any) {
	GetSugar().Fatalf(format, args...)
}

// Sync 刷新日志缓冲
func Sync() error {
	if logger != nil {
		return logger.Sync()
	}
	return nil
}

// init 默认初始化，避免未调用 Init 时 panic
func init() {
	// 默认使用 nop logger，避免空指针
	logger = zap.NewNop()
	sugar = logger.Sugar()
	atomLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)
}
