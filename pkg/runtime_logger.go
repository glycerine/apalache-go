package apalache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RuntimeLogger struct {
	mutex     sync.Mutex
	console   io.Writer
	files     []io.Writer
	closers   []io.Closer
	now       func() time.Time
	decorated bool
}

// NewRuntimeLogger configures Scala-compatible console and detailed-file
// thresholds. With no run directories, only warnings and errors reach the
// undecorated console. Each distinct run directory receives detailed.log.
func NewRuntimeLogger(console io.Writer, now func() time.Time, runDirectories ...string) (*RuntimeLogger, error) {
	if console == nil {
		console = io.Discard
	}
	if now == nil {
		now = time.Now
	}
	logger := &RuntimeLogger{console: console, now: now}
	seen := make(map[string]struct{})
	for _, directory := range runDirectories {
		if directory == "" {
			continue
		}
		absolute, err := filepath.Abs(directory)
		if err != nil {
			logger.Close()
			return nil, fmt.Errorf("resolve log directory %s: %w", directory, err)
		}
		if _, exists := seen[absolute]; exists {
			continue
		}
		seen[absolute] = struct{}{}
		if err = os.MkdirAll(absolute, 0o755); err != nil {
			logger.Close()
			return nil, fmt.Errorf("create log directory %s: %w", absolute, err)
		}
		file, err := os.Create(filepath.Join(absolute, "detailed.log"))
		if err != nil {
			logger.Close()
			return nil, fmt.Errorf("create detailed log in %s: %w", absolute, err)
		}
		logger.files = append(logger.files, file)
		logger.closers = append(logger.closers, file)
	}
	logger.decorated = len(logger.files) != 0
	return logger, nil
}

func (logger *RuntimeLogger) Debug(message string) { logger.write("DEBUG", message) }
func (logger *RuntimeLogger) Info(message string)  { logger.write("INFO", message) }
func (logger *RuntimeLogger) Warn(message string)  { logger.write("WARN", message) }
func (logger *RuntimeLogger) Error(message string) { logger.write("ERROR", message) }

func (logger *RuntimeLogger) write(level, message string) {
	if logger == nil {
		return
	}
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	now := logger.now()
	if logger.decorated && level != "DEBUG" {
		_, _ = fmt.Fprintf(logger.console, "%-65s %.1s@%s\n", message, level, now.Format("15:04:05.000"))
	} else if !logger.decorated && (level == "WARN" || level == "ERROR") {
		_, _ = fmt.Fprintf(logger.console, "%-80s\n", message)
	}
	for _, file := range logger.files {
		_, _ = fmt.Fprintf(file, "%s [go] %-5s apalache - %s\n",
			now.Format("2006-01-02T15:04:05,000"), strings.ToUpper(level), message)
	}
}

func (logger *RuntimeLogger) Close() error {
	if logger == nil {
		return nil
	}
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	var failures []string
	for _, closer := range logger.closers {
		if err := closer.Close(); err != nil {
			failures = append(failures, err.Error())
		}
	}
	logger.closers = nil
	logger.files = nil
	if len(failures) != 0 {
		return fmt.Errorf("close detailed logs: %s", strings.Join(failures, "; "))
	}
	return nil
}

var _ PassLogger = (*RuntimeLogger)(nil)
