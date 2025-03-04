package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// levelToColor renvoie une chaîne colorée selon le niveau de log
func levelToColor(l zapcore.Level) string {
	switch l {
	case zapcore.DebugLevel:
		return colorBlue + "DEBUG" + colorReset
	case zapcore.InfoLevel:
		return colorGreen + "INFO " + colorReset
	case zapcore.WarnLevel:
		return colorYellow + "WARN " + colorReset
	case zapcore.ErrorLevel:
		return colorRed + "ERROR" + colorReset
	default:
		return fmt.Sprintf("%s", l)
	}
}

// customTimeEncoder formatte la date/heure selon la norme ISO8601
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format("2006-01-02T15:04:05Z07:00"))
}

// customLevelEncoder renvoie soit le nom du niveau brut, soit le nom coloré
// en fonction du paramètre "useColor".
func customLevelEncoder(useColor bool) zapcore.LevelEncoder {
	return func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		if useColor {
			enc.AppendString(levelToColor(l))
		} else {
			enc.AppendString(l.CapitalString())
		}
	}
}

// Logger defines the interface for structured logging.
type Logger interface {
	IsDebug() bool
	IsInfo() bool
	IsWarn() bool
	IsError() bool
	// Debug logs a message at debug level.
	Debug(msg string, args ...interface{})
	// Info logs a message at info level.
	Info(msg string, args ...interface{})
	// Warn logs a message at warning level.
	Warn(msg string, args ...interface{})
	// Error logs a message at error level.
	Error(msg string, args ...interface{})
	// Fatal logs a message at fatal level and exits the application.
	Fatal(msg string, args ...interface{})
	// Json converts an object to a JSON string.
	Json(v interface{}) string
}

// logger implements the Logger interface using zap.
type logger struct {
	zapLogger *zap.SugaredLogger
}

// NewLogger creates and returns a new instance of logger.
func NewLogger(level zapcore.Level, useColor bool) Logger {
	// Configuration for console output (with or without color depending on the flag)
	consoleEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",  // Clé de la date
		LevelKey:       "level", // Clé du niveau
		MessageKey:     "msg",   // Clé du message
		CallerKey:      "caller",
		EncodeTime:     customTimeEncoder,
		EncodeLevel:    customLevelEncoder(useColor),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Configuration for file output (always without color)
	fileEncoderConfig := consoleEncoderConfig
	fileEncoderConfig.EncodeLevel = customLevelEncoder(false)

	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(consoleEncoderConfig),
		zapcore.AddSync(zapcore.Lock(os.Stdout)),
		level,
	)

	errorFile, err := os.OpenFile("error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("failed to open error.log file: %v", err))
	}
	fileCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(fileEncoderConfig),
		zapcore.AddSync(errorFile),
		zapcore.ErrorLevel,
	)

	// Combine the two cores: console and file
	core := zapcore.NewTee(consoleCore, fileCore)

	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &logger{
		zapLogger: zapLogger.Sugar(),
	}
}

func (l *logger) IsDebug() bool {
	return l.zapLogger.Desugar().Core().Enabled(zapcore.DebugLevel)
}

func (l *logger) IsInfo() bool {
	return l.zapLogger.Desugar().Core().Enabled(zapcore.InfoLevel)
}

func (l *logger) IsWarn() bool {
	return l.zapLogger.Desugar().Core().Enabled(zapcore.WarnLevel)
}

func (l *logger) IsError() bool {
	return l.zapLogger.Desugar().Core().Enabled(zapcore.ErrorLevel)
}

// Debug logs a debug message with optional formatting arguments.
func (l *logger) Debug(msg string, args ...interface{}) {
	l.zapLogger.Debugf(msg, args...)
}

// Info logs an informational message with optional formatting arguments.
func (l *logger) Info(msg string, args ...interface{}) {
	l.zapLogger.Infof(msg, args...)
}

// Warn logs a warning message with optional formatting arguments.
func (l *logger) Warn(msg string, args ...interface{}) {
	l.zapLogger.Warnf(msg, args...)
}

// Error logs an error message with optional formatting arguments.
func (l *logger) Error(msg string, args ...interface{}) {
	l.zapLogger.Errorf(msg, args...)
}

// Fatal logs a fatal error message with optional formatting arguments and exits.
func (l *logger) Fatal(msg string, args ...interface{}) {
	l.zapLogger.Fatalf(msg, args...)
}

// Json converts a value to a JSON string, logging any errors.
func (l *logger) Json(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		l.Error("Failed to marshal JSON", "value", v, "error", err)
		return "{}"
	}
	return string(b)
}
