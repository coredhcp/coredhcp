// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package logger

import (
	"io"
	"sync"

	log_prefixed "github.com/chappjc/logrus-prefix"
	"github.com/rifflock/lfshook"
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

// WithFile logs to the specified file in addition to the existing output.
func WithFile(log *logrus.Entry, logfile string) {
	log.Logger.AddHook(lfshook.NewHook(logfile, &logrus.TextFormatter{}))
}

// WithNoStdOutErr disables logging to stdout/stderr.
func WithNoStdOutErr(log *logrus.Entry) {
	log.Logger.SetOutput(io.Discard)
}
