package utils

import (
	"go.uber.org/zap"
	"os"
)

var Log *zap.SugaredLogger

func InitLogger() {
	cfg := zap.NewDevelopmentConfig() // au lieu de NewProduction
	cfg.EncoderConfig.TimeKey = ""    // (optionnel) masque le timestamp
	logger, _ := cfg.Build()

	Log = logger.Sugar()

	Info("Logger initialised.")
}

func Info(msg string, fields ...any) {
	Log.Infow(msg, fields...)
}

func Warn(msg string, fields ...any) {
	Log.Warnw(msg, fields...)
}

func Error(msg string, fields ...any) {
	Log.Errorw("❌  "+msg, fields...)
}

func Success(msg string, fields ...any) {
	Log.Infow("✅ "+msg, fields...)
}

func Fatal(msg string, fields ...any) {
	Log.Errorw("🔥 FATAL: "+msg, fields...)
	_ = Log.Sync() // flush le logger
	os.Exit(1)
}
