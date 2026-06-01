package detonators

import (
	"log"
	"strings"

	"github.com/sirupsen/logrus"
)

// LogrusWriter redirects standard log output to logrus with global fields
type LogrusWriter struct {
	fields logrus.Fields
}

// NewLogrusWriter creates a new LogrusWriter with the given fields
func NewLogrusWriter(fields logrus.Fields) *LogrusWriter {
	return &LogrusWriter{fields: fields}
}

func (w *LogrusWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	logrus.WithFields(w.fields).Debug(msg)
	return len(p), nil
}

// SetupLogging redirects the standard library's log package to logrus
// (decorated with the supplied fields). Log level, format, and silencing
// are controlled at process startup — the CLI sets them via flags, the
// server via Bootstrap.Debug — so this function no longer touches them.
func SetupLogging(fields logrus.Fields) {
	writer := NewLogrusWriter(fields)
	log.SetOutput(writer)
	log.SetFlags(0) // logrus adds its own
}
