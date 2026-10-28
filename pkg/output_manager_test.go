package apalache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOutputManagerConfigurationSourceAndMirroredWriters(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "custom")
	source := NewStringInputSource("  ---- MODULE First ----\n====\n", nil, InputFormatTLA)
	manager := NewOutputManager()
	if manager.IsConfigured() {
		t.Fatal("fresh output manager is configured")
	}
	if _, err := manager.OutDir(); err == nil {
		t.Fatal("unconfigured out-dir was returned")
	}
	if err := manager.Configure(OutputManagerOptions{
		OutDir: filepath.Join(root, "out"), CustomRunDir: custom, Command: "first", Source: source,
		WriteIntermediate: true, Profiling: true,
		Now: func() time.Time { return time.Date(2026, 10, 28, 12, 34, 56, 0, time.UTC) },
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.InitSourceLines(source); err != nil {
		t.Fatal(err)
	}
	if got, found := manager.AllSource(); !found || got != "---- MODULE First ----\n====" {
		t.Fatalf("source = %q, %v", got, found)
	}
	outDir, _ := manager.OutDir()
	if outDir != filepath.Join(root, "out", "first") {
		t.Fatalf("out-dir = %q", outDir)
	}
	runDir, _ := manager.RunDir()
	if !strings.HasPrefix(filepath.Base(runDir), "2026-10-28T12-34-56_") {
		t.Fatalf("run-dir = %q", runDir)
	}
	write := func(writer io.Writer) error { _, err := fmt.Fprintln(writer, "value"); return err }
	if wrote, err := manager.WithWriterInRunDir([]string{"result.txt"}, write); err != nil || !wrote {
		t.Fatalf("run writer = %v, %v", wrote, err)
	}
	if wrote, err := manager.WithWriterInIntermediateDir([]string{"pass.txt"}, write); err != nil || !wrote {
		t.Fatalf("intermediate writer = %v, %v", wrote, err)
	}
	if wrote, err := manager.WithProfilingWriter(write); err != nil || !wrote {
		t.Fatalf("profile writer = %v, %v", wrote, err)
	}
	for _, path := range []string{
		filepath.Join(runDir, "result.txt"), filepath.Join(custom, "result.txt"),
		filepath.Join(runDir, IntermediateFolderName, "pass.txt"),
		filepath.Join(custom, IntermediateFolderName, "pass.txt"),
		filepath.Join(runDir, "profile-rules.txt"), filepath.Join(custom, "profile-rules.txt"),
	} {
		if content, err := ReadOutputFile(path); err != nil || content != "value" {
			t.Fatalf("output %s = %q, %v", path, content, err)
		}
	}
	if content, found, err := manager.ReadFileInRunDir("result.txt"); err != nil || !found || content != "value" {
		t.Fatalf("run readback = %q, %v, %v", content, found, err)
	}
}

func TestOutputManagerUsesFileBasenameForValueAndPointerSources(t *testing.T) {
	for _, pointer := range []bool{false, true} {
		root := t.TempDir()
		file := NewFileInputSourceWithFormat(filepath.Join(root, "Spec.tla"), InputFormatTLA)
		var source InputSource = file
		if pointer {
			source = &file
		}
		manager := NewOutputManager()
		if err := manager.Configure(OutputManagerOptions{OutDir: root, Command: "check", Source: source}); err != nil {
			t.Fatal(err)
		}
		outDir, err := manager.OutDir()
		if err != nil || outDir != filepath.Join(root, "Spec.tla") {
			t.Fatalf("pointer=%v out-dir = %q, %v", pointer, outDir, err)
		}
	}
}

func TestOutputManagerConditionalOutputsAndFirstSourceWins(t *testing.T) {
	manager := NewOutputManager()
	first := NewStringInputSource("first", nil, InputFormatTLA)
	if err := manager.InitSourceLines(first); err != nil {
		t.Fatal(err)
	}
	if err := manager.InitSourceLines(NewStringInputSource("second", nil, InputFormatTLA)); err != nil {
		t.Fatal(err)
	}
	if got, _ := manager.AllSource(); got != "first" {
		t.Fatalf("source was overwritten: %q", got)
	}
	write := func(io.Writer) error { t.Fatal("disabled writer was called"); return nil }
	if wrote, err := manager.WithWriterInRunDir([]string{"x"}, write); err != nil || wrote {
		t.Fatalf("unconfigured run writer = %v, %v", wrote, err)
	}
	if wrote, err := manager.WithWriterInIntermediateDir([]string{"x"}, write); err != nil || wrote {
		t.Fatalf("disabled intermediate writer = %v, %v", wrote, err)
	}
	if wrote, err := manager.WithProfilingWriter(write); err != nil || wrote {
		t.Fatalf("disabled profiling writer = %v, %v", wrote, err)
	}
}

func TestOutputManagerConfiguredConditionalOutputsStayDisabled(t *testing.T) {
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{OutDir: t.TempDir(), Command: "check"}); err != nil {
		t.Fatal(err)
	}
	write := func(io.Writer) error { t.Fatal("disabled writer was called"); return nil }
	if wrote, err := manager.WithWriterInIntermediateDir([]string{"x"}, write); err != nil || wrote {
		t.Fatalf("disabled intermediate writer = %v, %v", wrote, err)
	}
	if wrote, err := manager.WithProfilingWriter(write); err != nil || wrote {
		t.Fatalf("disabled profiling writer = %v, %v", wrote, err)
	}
}

func TestOutputManagerReportsDirectoryCreationFailure(t *testing.T) {
	root := t.TempDir()
	occupied := filepath.Join(root, "check")
	if err := os.WriteFile(occupied, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewOutputManager()
	err := manager.Configure(OutputManagerOptions{OutDir: root, Command: "check"})
	if err == nil || !strings.Contains(err.Error(), "could not find or create directory") {
		t.Fatalf("configuration error = %v", err)
	}
}

func TestOutputManagersAreConcurrentInvocationIsolated(t *testing.T) {
	root := t.TempDir()
	managers := []*OutputManager{NewOutputManager(), NewOutputManager()}
	var wait sync.WaitGroup
	errors := make(chan error, len(managers))
	for index, manager := range managers {
		wait.Add(1)
		go func(index int, manager *OutputManager) {
			defer wait.Done()
			command := fmt.Sprintf("run%d", index)
			source := NewStringInputSource(command, nil, InputFormatTLA)
			if err := manager.Configure(OutputManagerOptions{OutDir: root, Command: command}); err != nil {
				errors <- err
				return
			}
			if err := manager.InitSourceLines(source); err != nil {
				errors <- err
				return
			}
			_, err := manager.WithWriterInRunDir([]string{"result.txt"}, func(writer io.Writer) error {
				_, writeErr := io.WriteString(writer, command)
				return writeErr
			})
			errors <- err
		}(index, manager)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	for index, manager := range managers {
		runDir, _ := manager.RunDir()
		content, err := os.ReadFile(filepath.Join(runDir, "result.txt"))
		want := fmt.Sprintf("run%d", index)
		if err != nil || string(content) != want {
			t.Fatalf("manager %d output = %q, %v", index, content, err)
		}
		if source, _ := manager.AllSource(); source != want {
			t.Fatalf("manager %d source = %q", index, source)
		}
	}
}
