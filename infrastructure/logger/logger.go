package logger

import (
	"go.uber.org/zap"
	"sync"
	"go.uber.org/zap/zapcore"
)


var (
	logger *zap.Logger
	sugar  *zap.SugaredLogger
	mtx    sync.Mutex
)

func Get() *zap.SugaredLogger {
	mtx.Lock()
	defer mtx.Unlock()

	if logger == nil {
		cfg := zap.NewDevelopmentConfig()
		cfg.OutputPaths = []string{"stdout", "./logger.log"}
		cfg.ErrorOutputPaths = []string{"./error.log"}

		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		lg, err := cfg.Build()
		if err != nil {
			panic(err)
		}
		logger = lg
		sugar = lg.Sugar()
	}
	return sugar
}
