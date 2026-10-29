//go:build oracle

package apalache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glycerine/apalache-go/internal/oraclectl"
)

func TestVariableWidthNonlinearDeadlockDocumentsScalaUnderapproximation(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	goExecutable := os.Getenv("APALACHE_GO")
	if scalaExecutable == "" || goExecutable == "" {
		t.Skip("set APALACHE_SCALA and APALACHE_GO to run the live nonlinear-deadlock oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	source := "pkg/testdata/oracle/VariableNonlinearDeadlock.tla"
	scalaRun := t.TempDir()
	scala, err := oraclectl.Run(ctx, oraclectl.Command{
		Executable: scalaExecutable,
		Args:       []string{"--run-dir=" + scalaRun, "check", "--length=0", source},
		Dir:        "..",
	})
	if err != nil {
		t.Fatal(err)
	}
	goTrace := filepath.Join(t.TempDir(), "go.itf.json")
	goResult, err := oraclectl.Run(ctx, oraclectl.Command{
		Executable: goExecutable,
		Args:       []string{"check", "-itf=" + goTrace, "-length", "0", source},
		Dir:        "..",
	})
	if err != nil {
		t.Fatal(err)
	}
	if scala.Verdict != oraclectl.VerdictSafe {
		t.Fatalf("Scala no longer misses the nonlinear deadlock (verdict %s); review this documented correction", scala.Verdict)
	}
	if goResult.Verdict != oraclectl.VerdictDeadlock || goResult.Depth != 0 {
		t.Fatalf("Go nonlinear deadlock = %s at depth %d", goResult.Verdict, goResult.Depth)
	}
	data, err := os.ReadFile(goTrace)
	if err != nil {
		t.Fatal(err)
	}
	states, err := UnmarshalITFTrace(data)
	if err != nil || len(states) != 1 {
		t.Fatalf("Go nonlinear deadlock trace = %v, %v", states, err)
	}
	value, err := FormatTLAExpression(states[0]["x"])
	if err != nil || value != "2" {
		t.Fatalf("Go nonlinear deadlock state x = %q, %v", value, err)
	}
}
