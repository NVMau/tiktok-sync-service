package logger

import (
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log  *zap.Logger
	once sync.Once
)

// Config holds logger configuration
type Config struct {
	Level       string // debug, info, warn, error
	Environment string // development, production
	OutputPath  string // stdout, stderr, or file path
}

// Init initializes the global logger
func Init(cfg *Config) {
	once.Do(func() {
		Log = newLogger(cfg)
	})
}

// newLogger creates a new zap logger based on config
func newLogger(cfg *Config) *zap.Logger {
	if cfg == nil {
		cfg = &Config{
			Level:       "info",
			Environment: "development",
			OutputPath:  "stdout",
		}
	}

	// Parse log level
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if cfg.Environment == "production" {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Configure output
	var writeSyncer zapcore.WriteSyncer
	switch cfg.OutputPath {
	case "stdout", "":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		// Ensure log directory exists
		logDir := filepath.Dir(cfg.OutputPath)
		if logDir != "" && logDir != "." {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				writeSyncer = zapcore.AddSync(os.Stdout)
				break
			}
		}
		file, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			writeSyncer = zapcore.AddSync(os.Stdout)
		} else {
			writeSyncer = zapcore.AddSync(file)
		}
	}

	// Create encoder based on environment
	var encoder zapcore.Encoder
	if cfg.Environment == "production" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger
}

// Sync flushes any buffered log entries
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

// Helper functions for common logging patterns

// Info logs an info message with fields
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Debug logs a debug message with fields
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Warn logs a warning message with fields
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Error logs an error message with fields
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// WithContext returns a logger with additional context fields
func WithContext(fields ...zap.Field) *zap.Logger {
	return Log.With(fields...)
}

// Field constructors for common types
func String(key, val string) zap.Field    { return zap.String(key, val) }
func Int(key string, val int) zap.Field   { return zap.Int(key, val) }
func Uint(key string, val uint) zap.Field { return zap.Uint(key, val) }
func Float64(key string, val float64) zap.Field { return zap.Float64(key, val) }
func Bool(key string, val bool) zap.Field { return zap.Bool(key, val) }
func Err(err error) zap.Field             { return zap.Error(err) }
func Any(key string, val interface{}) zap.Field { return zap.Any(key, val) }
