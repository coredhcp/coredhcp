package logger

import (
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	globalLogger   *logrus.Logger
	getLoggerMutex sync.Mutex
)

// GetLogger returns a configured logger instance
func GetLogger() *logrus.Logger {
	if globalLogger == nil {
		getLoggerMutex.Lock()
		defer getLoggerMutex.Unlock()
		logger := logrus.New()
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
		globalLogger = logger
	}
	return globalLogger
}
