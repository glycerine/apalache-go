package apalache

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeLoggerConsoleOnlyWarnThreshold(t *testing.T) {
	var console bytes.Buffer
	logger, err := NewRuntimeLogger(&console, func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warning")
	logger.Error("failure")
	if got := console.String(); strings.Contains(got, "debug") || strings.Contains(got, "info") ||
		!strings.Contains(got, "warning") || !strings.Contains(got, "failure") || strings.Contains(got, "@") {
		t.Fatalf("console-only log = %q", got)
	}
}

func TestRuntimeLoggerDuplicatesDetailedLogsAndDecoratesConsole(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	var console bytes.Buffer
	now := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC) }
	logger, err := NewRuntimeLogger(&console, now, first, second, first)
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("details")
	logger.Info("visible")
	logger.Warn("warning")
	if err = logger.Close(); err != nil {
		t.Fatal(err)
	}
	if got := console.String(); strings.Contains(got, "details") || !strings.Contains(got, "visible") ||
		!strings.Contains(got, "I@03:04:05.006") || !strings.Contains(got, "W@03:04:05.006") {
		t.Fatalf("decorated console log = %q", got)
	}
	for _, directory := range []string{first, second} {
		data, readErr := os.ReadFile(filepath.Join(directory, "detailed.log"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(data)
		if !strings.Contains(text, "2026-01-02T03:04:05,006 [go] DEBUG apalache - details") ||
			!strings.Contains(text, "INFO  apalache - visible") || !strings.Contains(text, "WARN  apalache - warning") {
			t.Fatalf("detailed log in %s = %q", directory, text)
		}
	}
}

func TestRuntimeLoggerCreatesDirectoriesAndRejectsFileDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "new", "run")
	logger, err := NewRuntimeLogger(nil, nil, nested)
	if err != nil {
		t.Fatal(err)
	}
	_ = logger.Close()
	if _, err = os.Stat(filepath.Join(nested, "detailed.log")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "ordinary-file")
	if err = os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = NewRuntimeLogger(nil, nil, file); err == nil {
		t.Fatal("ordinary file accepted as a log directory")
	}
}
