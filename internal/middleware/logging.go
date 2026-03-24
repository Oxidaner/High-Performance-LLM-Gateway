package middleware

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log        *zap.Logger
	SugaredLog *zap.SugaredLogger
)

// Config holds logger configuration
type Config struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // json, console
	OutputPath string `yaml:"output_path"` // stdout, stderr, or file path
}

// DefaultConfig returns default logger configuration
func DefaultConfig() Config {
	return Config{
		Level:      "info",
		Format:     "console",
		OutputPath: "stdout",
	}
}

// Init initializes the global logger
func Init(cfg Config) error {
	encoding := "console"
	if cfg.Format == "json" {
		encoding = "json"
	}

	// Parse log level
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",                           // 时间键名，默认是time
		LevelKey:       "level",                          // 日志级别键名，默认是level
		NameKey:        "logger",                         // 日志记录器键名，默认是logger
		CallerKey:      "caller",                         // 调用者键名，默认是caller
		FunctionKey:    zapcore.OmitKey,                  // 函数调用键名，默认是function，这里省略
		MessageKey:     "msg",                            // 日志消息键名，默认是msg
		StacktraceKey:  "stacktrace",                     // 栈跟踪信息键名，默认是stacktrace
		LineEnding:     zapcore.DefaultLineEnding,        // 日志行结束符，默认是换行符
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // 日志级别编码器，默认是大写颜色编码
		EncodeTime:     zapcore.ISO8601TimeEncoder,       // 时间编码器，默认是ISO8601时间编码
		EncodeDuration: zapcore.SecondsDurationEncoder,   // 持续时间编码器，默认是秒级持续时间编码
		EncodeCaller:   zapcore.ShortCallerEncoder,       // 调用者编码器，默认是短调用者编码
	}

	// Create output writer 日志写入方式，默认stdout
	var writeSyncer zapcore.WriteSyncer
	if cfg.OutputPath == "stdout" {
		// 将日志写入stdout 就是打印到控制台 普通日志
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else if cfg.OutputPath == "stderr" {
		// 将日志写入stderr 就是打印到控制台 错误日志
		writeSyncer = zapcore.AddSync(os.Stderr)
	} else {
		// 将日志写入文件
		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644) // 打开文件，创建如果不存在，追加写入，权限为0644
		if err != nil {
			return err
		}
		writeSyncer = zapcore.AddSync(file)
	}

	// Create core
	var encoder zapcore.Encoder
	if encoding == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig) //json格式
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig) //console格式
	}

	//zapcore.Core需要三个配置——Encoder，WriteSyncer，LogLevel。
	core := zapcore.NewCore(
		encoder,     //如何编码日志消息（例如 JSON 格式或控制台格式）
		writeSyncer, //在哪里输出日志消息（例如控制台、文件等）
		level,       //决定哪些日志消息会被记录（基于日志级别）
	)

	// Create logger
	// core 是 Zap 日志库的核心组件，通常是一个 zapcore.Core 接口的实现。
	// zap.AddCaller() 会在日志中添加调用者的信息，包括文件路径、函数名、行号等。
	// zap.AddCallerSkip(1) 用于调整调用者信息的准确性，默认值为0。
	// 		当设置为1时，会跳过调用者的当前函数，直接记录调用者的上一级函数。
	//		这在创建日志包装函数时非常有用，确保记录的是实际调用日志的代码位置，
	//		而不是包装函数内部的位置
	Log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	// SugaredLog 是一个方便的日志记录器，它提供了一种更简单的日志记录方式，
	SugaredLog = Log.Sugar()

	return nil
}

// Sync flushes any buffered log entries
// 刷新日志缓冲区，确保所有日志消息都被写入输出
func Sync() {
	if Log != nil {
		Log.Sync()
	}
}

// Debug logs a debug message
// 记录调试级别的日志消息
func Debug(msg string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.Debug(msg, fields...)
}

// Info logs an info message
// 记录信息级别的日志消息
func Info(msg string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.Info(msg, fields...)
}

// Warn logs a warning message
// 记录警告级别的日志消息
func Warn(msg string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.Warn(msg, fields...)
}

// Error logs an error message
// 记录错误级别的日志消息
func Error(msg string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.Error(msg, fields...)
}

// DPanic logs a panic message if development mode is enabled, otherwise logs an error message
// 记录开发模式下的恐慌级别的日志消息，否则记录错误级别的日志消息
func DPanic(msg string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.DPanic(msg, fields...)
}

// Panic logs a panic message
// 记录恐慌级别的日志消息
func Panic(msg string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.Panic(msg, fields...)
}

// Fatal logs a fatal message and exits
// 记录致命级别的日志消息，并退出程序
func Fatal(msg string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.Fatal(msg, fields...)
}

// Debugf logs a formatted debug message
// 记录格式化的调试级别的日志消息
func Debugf(template string, args ...interface{}) {
	if SugaredLog == nil {
		return
	}
	SugaredLog.Debugf(template, args...)
}

// Infof logs a formatted info message
// 记录格式化的信息级别的日志消息
func Infof(template string, args ...interface{}) {
	if SugaredLog == nil {
		return
	}
	SugaredLog.Infof(template, args...)
}

// Warnf logs a formatted warning message
// 记录格式化的警告级别的日志消息
func Warnf(template string, args ...interface{}) {
	if SugaredLog == nil {
		return
	}
	SugaredLog.Warnf(template, args...)
}

// Errorf logs a formatted error message
// 记录格式化的错误级别的日志消息
func Errorf(template string, args ...interface{}) {
	if SugaredLog == nil {
		return
	}
	SugaredLog.Errorf(template, args...)
}

// String creates a string field
// 创建一个字符串字段
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int creates an int field
// 创建一个整数字段
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Err creates an error field
// 创建一个错误字段
func Err(err error) zap.Field {
	return zap.Error(err)
}
