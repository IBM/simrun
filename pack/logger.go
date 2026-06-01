package pack

import (
	"os"

	"github.com/sirupsen/logrus"
)

var baseLogger *logrus.Logger

func init() {
	baseLogger = logrus.New()
	baseLogger.SetOutput(os.Stderr)
	baseLogger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05Z07:00",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime: "ts",
		},
	})
	baseLogger.SetLevel(logrus.DebugLevel)
}

// logEntry creates a structured logger with simulation/execution metadata.
func logEntry(executionID string) *logrus.Entry {
	return baseLogger.WithFields(logrus.Fields{
		"simulation":   currentSimulation,
		"execution_id": executionID,
		"pack":         packName,
		"pack_version": packVersion,
	})
}

// Logger returns a structured logger for detonate operations.
func Logger(input DetonateInput) *logrus.Entry {
	return logEntry(input.ExecutionID)
}

// CleanupLogger returns a structured logger for cleanup operations.
func CleanupLogger(input CleanupInput) *logrus.Entry {
	return logEntry(input.ExecutionID)
}
