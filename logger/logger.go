// Package logger 是对go.uber.org/zap包的简单封装
package logger

import (
	"kortho/config"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 日志记录器
var Logger *zap.Logger

// SugarLogger 简单日志记录器
var SugarLogger *zap.SugaredLogger

// InitLogger 初始化logger
func InitLogger(cfg *config.LogConfigInfo) (err error) {
	encoder := getEncoder()
	syncWriter := getLogWriter(cfg.FileName, cfg.MaxAge, cfg.MaxSize, cfg.MaxBackups, cfg.Comperss)

	level := new(zapcore.Level)
	err = level.UnmarshalText([]byte(cfg.Level))
	if err != nil {
		log.Panic(err)
		return
	}

	core := zapcore.NewCore(encoder, syncWriter, level)
	Logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	SugarLogger = Logger.Sugar()
	return
}

func getEncoder() zapcore.Encoder {
	encodeConfig := zap.NewProductionEncoderConfig()
	encodeConfig.TimeKey = "time"
	encodeConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encodeConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encodeConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encodeConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewJSONEncoder(encodeConfig)
}

func getLogWriter(filename string, maxAge, maxSize, maxBackups int, compress bool) zapcore.WriteSyncer {
	umberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxAge:     maxAge,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		Compress:   compress,
	}
	return zapcore.NewMultiWriteSyncer(zapcore.AddSync(umberJackLogger), zapcore.AddSync(os.Stdout))
}

// Debug logs a message at DebugLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Info logs a message at InfoLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Warn logs a message at WarnLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// With creates a child logger and adds structured context to it. Fields added
// to the child don't affect the parent, and vice versa.
func With(fields ...zap.Field) *zap.Logger {
	return Logger.With(fields...)
}
