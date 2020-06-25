package log

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	formatLogger *zap.SugaredLogger
	levelMap     = map[string]zapcore.Level{
		"debug": zapcore.DebugLevel,
		"info":  zapcore.InfoLevel,
		"warn":  zapcore.WarnLevel,
		"error": zapcore.ErrorLevel,
		"panic": zapcore.PanicLevel,
		"fatal": zapcore.FatalLevel,
	}
)

func InitLog() {
	logger := newLogger("debug")
	formatLogger = logger.Sugar()
}

func newLogger(level string) *zap.Logger {
	core := newCore(level)
	return zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.Development(),
	)
}

func newCore(level string) zapcore.Core {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "line",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder, //
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}
	return zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
		zap.NewAtomicLevelAt(getLoggerLevel(level)),
	)
}

func getLoggerLevel(lvl string) zapcore.Level {
	if level, ok := levelMap[strings.ToLower(lvl)]; ok {
		return level
	}
	return zapcore.InfoLevel
}

func Debug(args ...interface{}) {
	formatLogger.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	formatLogger.Debugf(template, args...)
}

func Info(args ...interface{}) {
	formatLogger.Info(args...)
}

func Infof(template string, args ...interface{}) {
	formatLogger.Infof(template, args...)
}

func Warn(args ...interface{}) {
	formatLogger.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	formatLogger.Warnf(template, args...)
}

func Error(args ...interface{}) {
	formatLogger.Error(args...)
}

func Errorf(template string, args ...interface{}) {
	formatLogger.Errorf(template, args...)
}

func Panic(args ...interface{}) {
	formatLogger.Panic(args...)
}

func Panicf(template string, args ...interface{}) {
	formatLogger.Panicf(template, args...)
}

func Fatal(args ...interface{}) {
	formatLogger.Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	formatLogger.Fatalf(template, args...)
}

func Println(args ...interface{}) {
	formatLogger.Debug(args...)
}
