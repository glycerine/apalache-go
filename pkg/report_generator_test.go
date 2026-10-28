package apalache

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareBugReportWritesSourceLogSystemAndMirror(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "custom")
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{OutDir: root, CustomRunDir: custom, Command: "check"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.InitSourceLines(NewStringInputSource("---- MODULE M ----\n====", nil, InputFormatTLA)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.WithWriterInRunDir([]string{"detailed.log"}, func(writer io.Writer) error {
		_, writeErr := io.WriteString(writer, `price=$5 path=C:\spec`)
		return writeErr
	}); err != nil {
		t.Fatal(err)
	}
	path, err := PrepareBugReport(manager, BugReportOptions{
		Command: "apalache check M.tla", Version: "0.99", OS: "TestOS", GoVersion: "go-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	runDir, _ := manager.RunDir()
	if path != filepath.Join(runDir, BugReportFilename) {
		t.Fatalf("report path = %q", path)
	}
	for _, reportPath := range []string{path, filepath.Join(custom, BugReportFilename)} {
		data, readErr := os.ReadFile(reportPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		report := string(data)
		for _, fragment := range []string{
			"---- MODULE M ----\n====\n````", "apalache check M.tla", `price=\$5 path=C:\\spec`,
			"Apalache version: `0.99`", "OS: `TestOS`", "Go version: `go-test`",
		} {
			if !strings.Contains(report, fragment) {
				t.Fatalf("report %s is missing %q:\n%s", reportPath, fragment, report)
			}
		}
	}
}

func TestPrepareBugReportUsesMissingDataFallbacks(t *testing.T) {
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{OutDir: t.TempDir(), Command: "check"}); err != nil {
		t.Fatal(err)
	}
	path, err := PrepareBugReport(manager, BugReportOptions{Version: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "<!-- TLA+ specification not found. -->") ||
		!strings.Contains(string(report), "Go version: `") {
		t.Fatalf("fallback report:\n%s", report)
	}
}

func TestPrepareBugReportRequiresConfiguredManager(t *testing.T) {
	if _, err := PrepareBugReport(nil, BugReportOptions{}); err == nil {
		t.Fatal("nil manager was accepted")
	}
	if _, err := PrepareBugReport(NewOutputManager(), BugReportOptions{}); err == nil {
		t.Fatal("unconfigured manager was accepted")
	}
}

func TestQuoteBugReportLogEscapesReplacementCharacters(t *testing.T) {
	if got := QuoteBugReportLog(`$x\y`); got != `\$x\\y` {
		t.Fatalf("quoted log = %q", got)
	}
}
