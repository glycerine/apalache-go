//go:build resource

package apalache

import (
	"context"
	"os"
	"runtime"
	"runtime/debug"
	"testing"
)

func processFileDescriptorCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("file-descriptor accounting requires /proc/self/fd: %v", err)
	}
	return len(entries)
}

func TestG3RepeatedContextsReleaseResources(t *testing.T) {
	baselineFDs := processFileDescriptorCount(t)
	baselineGoroutines := runtime.NumGoroutine()
	for iteration := 0; iteration < 128; iteration++ {
		solver, err := NewG3SolverContext()
		if err != nil {
			t.Fatalf("context %d: %v", iteration, err)
		}
		truth, err := solver.Bool(true)
		if err == nil {
			err = solver.Assert(truth)
		}
		if err == nil {
			var status SMTStatus
			status, err = solver.Check(context.Background())
			if err == nil && status != SMTSatisfiable {
				t.Fatalf("context %d status = %v", iteration, status)
			}
		}
		closeErr := solver.Close()
		if err != nil {
			t.Fatalf("context %d solve: %v", iteration, err)
		}
		if closeErr != nil {
			t.Fatalf("context %d close: %v", iteration, closeErr)
		}
	}
	runtime.GC()
	debug.FreeOSMemory()
	if got := processFileDescriptorCount(t); got != baselineFDs {
		t.Fatalf("file descriptors after repeated contexts = %d, want baseline %d", got, baselineFDs)
	}
	if got := runtime.NumGoroutine(); got > baselineGoroutines+2 {
		t.Fatalf("goroutines after repeated contexts = %d, baseline %d", got, baselineGoroutines)
	}
}
