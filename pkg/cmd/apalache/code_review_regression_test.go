package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewCheckUnsatisfiableInit(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Review.tla")
	if err := os.WriteFile(source, []byte(`---- MODULE Review ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0 /\ x > 0
Next == x' = x
Inv == x >= 0
====`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, deadlocks := range []bool{true, false} {
		t.Run(fmt.Sprint(deadlocks), func(t *testing.T) {
			trace := filepath.Join(t.TempDir(), "trace.itf.json")
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"check", "--init=Init", "--next=Next", "--inv=Inv", "--length=1",
				fmt.Sprintf("--no-deadlock=%v", !deadlocks), "--itf=" + trace, "--out-dir=" + t.TempDir(), source}, &stdout, &stderr)
			want := exitOK
			if deadlocks {
				want = exitCounterexample
			}
			if code != want || !strings.Contains(stdout.String(), "no satisfiable initial state") ||
				strings.Contains(stdout.String(), "No counterexample found through") {
				t.Fatalf("exit=%d want=%d stdout=%s stderr=%s", code, want, &stdout, &stderr)
			}
			if _, err := os.Stat(trace); !os.IsNotExist(err) {
				t.Fatalf("fabricated trace: %v", err)
			}
		})
	}
}
