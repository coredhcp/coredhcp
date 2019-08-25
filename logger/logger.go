package logger

import (
	"sync"

	log_prefixed "github.com/chappjc/logrus-prefix"
	"github.com/sirupsen/logrus"
)

var (
	globalLogger   *logrus.Logger
	getLoggerMutex sync.Mutex
)

// GetLogger returns a configured logger instance
func GetLogger(prefix string) *logrus.Entry {
	if prefix == "" {
		prefix = "<no prefix>"
	}
	if globalLogger == nil {
		getLoggerMutex.Lock()
		defer getLoggerMutex.Unlock()
		logger := logrus.New()
		logger.SetFormatter(&log_prefixed.TextFormatter{
			FullTimestamp: true,
		})
		globalLogger = logger
	}
	return globalLogger.WithField("prefix", prefix)
}
