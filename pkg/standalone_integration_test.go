//go:build standalone

package apalache

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStandaloneBinaryWithoutExternalRuntime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "apalache")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/apalache")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build standalone binary: %v\n%s", err, output)
	}

	spec := `---- MODULE Standalone ----
EXTENDS Integers, SequencesExt

Init == TRUE
Next == TRUE
Inv == Reverse(<<1, 2, 3>>) = <<3, 2, 1>>
====
`
	specPath := filepath.Join(tempDir, "Standalone.tla")
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}

	check := exec.CommandContext(ctx, binary,
		"check", "-length", "0", "-deadlock=false", "-inv", "Inv", specPath)
	check.Dir = tempDir
	check.Env = []string{
		"HOME=" + tempDir,
		"PATH=" + filepath.Join(tempDir, "no-executables"),
	}
	output, err := check.CombinedOutput()
	if err != nil {
		t.Fatalf("run standalone binary in clean environment: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "No counterexample found through depth 0") {
		t.Fatalf("unexpected standalone output:\n%s", output)
	}
}
