package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RunLogEntry is a single structured log entry written to the per-run JSONL file.
type RunLogEntry struct {
	Timestamp string         `json:"ts"`
	Level     string         `json:"level"`
	Message   string         `json:"msg"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// RunLogWriter writes structured log entries to a per-run JSONL file.
type RunLogWriter struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// NewRunLogWriter creates a new RunLogWriter that writes to <dataDir>/run-logs/{runID}.jsonl.
func NewRunLogWriter(dataDir, runID string) (*RunLogWriter, error) {
	dir := filepath.Join(dataDir, "run-logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create run-logs directory: %w", err)
	}

	path := filepath.Join(dir, runID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create run log file: %w", err)
	}

	return &RunLogWriter{
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

// Write appends a structured log entry to the JSONL file.
func (w *RunLogWriter) Write(entry RunLogEntry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enc.Encode(entry) //nolint:errcheck
}

// Close closes the underlying file.
func (w *RunLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// RunLogRegistry is a global thread-safe registry mapping runID → RunLogWriter.
// The RunLogHook uses this to route log entries to the correct per-run file.
type RunLogRegistry struct {
	mu      sync.RWMutex
	writers map[string]*RunLogWriter
}

// NewRunLogRegistry creates a new empty registry.
func NewRunLogRegistry() *RunLogRegistry {
	return &RunLogRegistry{
		writers: make(map[string]*RunLogWriter),
	}
}

// Register adds a writer for the given runID.
func (r *RunLogRegistry) Register(runID string, w *RunLogWriter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writers[runID] = w
}

// Unregister removes and closes the writer for the given runID.
func (r *RunLogRegistry) Unregister(runID string) {
	r.mu.Lock()
	w, ok := r.writers[runID]
	if ok {
		delete(r.writers, runID)
	}
	r.mu.Unlock()

	if ok && w != nil {
		if err := w.Close(); err != nil {
			logrus.WithError(err).WithField("run_id", runID).Warn("failed to close run log writer")
		}
	}
}

// get returns the writer for the given runID, or nil if not found.
func (r *RunLogRegistry) get(runID string) *RunLogWriter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.writers[runID]
}

// RunLogHook is a logrus Hook that routes entries to per-run log files
// based on the "run_id" field in the log entry.
type RunLogHook struct {
	registry *RunLogRegistry
	hub      *Hub
}

// NewRunLogHook creates a new logrus Hook backed by a RunLogRegistry.
// If hub is provided, log entries are also broadcast via WebSocket.
func NewRunLogHook(registry *RunLogRegistry, hub *Hub) *RunLogHook {
	return &RunLogHook{registry: registry, hub: hub}
}

// Levels returns all log levels this hook fires for.
func (h *RunLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is called by logrus when a log entry is fired.
func (h *RunLogHook) Fire(entry *logrus.Entry) error {
	runID, ok := entry.Data["run_id"].(string)
	if !ok || runID == "" {
		return nil // not a run-scoped entry
	}

	writer := h.registry.get(runID)
	if writer == nil {
		return nil // no active writer for this run
	}

	fields := make(map[string]any, len(entry.Data))
	for k, v := range entry.Data {
		if k == "run_id" {
			continue // don't duplicate run_id in fields
		}
		// Convert errors to their string representation so they
		// serialize properly in JSON (error structs have unexported
		// fields which marshal to {}).
		if err, ok := v.(error); ok {
			fields[k] = err.Error()
		} else {
			fields[k] = v
		}
	}

	logEntry := RunLogEntry{
		Timestamp: entry.Time.Format(time.RFC3339Nano),
		Level:     entry.Level.String(),
		Message:   entry.Message,
		Fields:    fields,
	}

	writer.Write(logEntry)

	// Broadcast via WebSocket for real-time streaming
	if h.hub != nil {
		h.hub.BroadcastToRun(runID, WSMessage{
			Type: "scenario_log",
			Data: logEntry,
		})
	}

	return nil
}

// DeleteRunLog removes a run's JSONL log file from disk.
func DeleteRunLog(dataDir, runID string) {
	_ = os.Remove(filepath.Join(dataDir, "run-logs", runID+".jsonl"))
}

// SweepRunLogs deletes run-log JSONL files in <dataDir>/run-logs whose last
// modification time is older than days. It is a no-op when enabled is false.
// Deleting a log file does not touch the corresponding runs row — only the
// verbose log expires. Best-effort: per-file failures are logged, not fatal.
func SweepRunLogs(dataDir string, enabled bool, days int) {
	if !enabled {
		return
	}

	dir := filepath.Join(dataDir, "run-logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.WithError(err).WithField("dir", dir).Warn("run-log sweep: failed to read run-logs directory")
		}
		return
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, e.Name())
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				logrus.WithError(err).WithField("path", path).Warn("run-log sweep: failed to remove aged log file")
			}
		}
	}
}

// ReadRunLog reads a run's JSONL log file and returns the entries.
func ReadRunLog(dataDir, runID string) ([]RunLogEntry, error) {
	path := filepath.Join(dataDir, "run-logs", runID+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunLogEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read run log: %w", err)
	}

	var entries []RunLogEntry
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var entry RunLogEntry
		if err := dec.Decode(&entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}

	if entries == nil {
		entries = []RunLogEntry{}
	}
	return entries, nil
}
