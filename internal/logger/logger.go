package logger

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
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create output writer
	var writeSyncer zapcore.WriteSyncer
	if cfg.OutputPath == "stdout" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else if cfg.OutputPath == "stderr" {
		writeSyncer = zapcore.AddSync(os.Stderr)
	} else {
		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		writeSyncer = zapcore.AddSync(file)
	}

	// Create core
	var encoder zapcore.Encoder
	if encoding == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		writeSyncer,
		level,
	)

	// Create logger
	Log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	SugaredLog = Log.Sugar()

	return nil
}

// Sync flushes any buffered log entries
func Sync() {
	if Log != nil {
		Log.Sync()
	}
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// Debugf logs a formatted debug message
func Debugf(template string, args ...interface{}) {
	SugaredLog.Debugf(template, args...)
}

// Infof logs a formatted info message
func Infof(template string, args ...interface{}) {
	SugaredLog.Infof(template, args...)
}

// Warnf logs a formatted warning message
func Warnf(template string, args ...interface{}) {
	SugaredLog.Warnf(template, args...)
}

// Errorf logs a formatted error message
func Errorf(template string, args ...interface{}) {
	SugaredLog.Errorf(template, args...)
}

// String creates a string field
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int creates an int field
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Err creates an error field
func Err(err error) zap.Field {
	return zap.Error(err)
}
