//go:build deterministic

package apalache

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStandaloneCounterexampleIsDeterministicAcrossWorkerCounts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	directory := t.TempDir()
	binary := filepath.Join(directory, "apalache")
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", binary, "./cmd/apalache")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build checker: %v\n%s", err, output)
	}
	source := filepath.Join(directory, "Deterministic.tla")
	if err := os.WriteFile(source, []byte(`---- MODULE Deterministic ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == x < 2
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(directory, "trace.itf.json")
	var wantOutput, wantTrace []byte
	for _, workers := range []string{"1", "8"} {
		for repetition := 0; repetition < 4; repetition++ {
			command := exec.CommandContext(ctx, binary, "check", "--length=2", "--no-deadlock", "--inv=Inv", "--itf="+trace, source)
			command.Dir = directory
			command.Env = []string{
				"GOMAXPROCS=" + workers,
				"HOME=" + directory,
				"PATH=" + filepath.Join(directory, "no-executables"),
			}
			output, err := command.CombinedOutput()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 12 {
				t.Fatalf("workers %s repetition %d: exit = %v\n%s", workers, repetition, err, output)
			}
			traceData, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			if wantOutput == nil {
				wantOutput = append([]byte(nil), output...)
				wantTrace = append([]byte(nil), traceData...)
				continue
			}
			if !bytes.Equal(output, wantOutput) {
				t.Fatalf("workers %s repetition %d: output changed\nwant:\n%s\ngot:\n%s", workers, repetition, wantOutput, output)
			}
			if !bytes.Equal(traceData, wantTrace) {
				t.Fatalf("workers %s repetition %d: ITF trace changed\nwant:\n%s\ngot:\n%s", workers, repetition, wantTrace, traceData)
			}
		}
	}
}
