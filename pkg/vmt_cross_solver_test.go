//go:build solver

package apalache

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func runVMTOracle(t *testing.T, executable string, arguments []string, document string) string {
	t.Helper()
	path, err := exec.LookPath(executable)
	if err != nil {
		t.Skipf("%s is not installed; skipping optional cross-solver oracle", executable)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, arguments...)
	command.Stdin = strings.NewReader(document)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err = command.Run(); err != nil {
		t.Fatalf("%s rejected VMT query: %v\nstdout:\n%s\nstderr:\n%s\nquery:\n%s",
			executable, err, &stdout, &stderr, document)
	}
	if ctx.Err() != nil {
		t.Fatalf("%s VMT query timed out: %v", executable, ctx.Err())
	}
	fields := strings.Fields(stdout.String())
	if len(fields) == 0 {
		t.Fatalf("%s produced no status\nstderr:\n%s", executable, &stderr)
	}
	return fields[0]
}

func stripVMTMetadataDefinitions(document string) string {
	lines := strings.Split(document, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "(define-fun .") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func TestVMTQueriesAgreeAcrossZ3AndCVC5(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "CrossSolver.tla", `
---- MODULE CrossSolver ----
\* @type: Bool;
VARIABLE b
Init == b = FALSE
Next == b' = ~b
Inv == ~b
====`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics: %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := FormatVMTModuleWithInvariants(prepared.Module, prepared.Predicates.Invariants)
	if err != nil {
		t.Fatal(err)
	}
	base := stripVMTMetadataDefinitions(stripVMTAttributes(document))
	tests := []struct {
		name       string
		assertions []string
		want       string
	}{
		{"initial state", []string{"Init$0000"}, "sat"},
		{"contradictory initial state", []string{"Init$0000", "b"}, "unsat"},
		{"one transition", []string{"Init$0000", "Next$0000", "b^"}, "sat"},
		{"incorrect successor", []string{"Init$0000", "Next$0000", "(not b^)"}, "unsat"},
		{"initial invariant violation", []string{"Init$0000", "(not VCInv$0)"}, "unsat"},
	}
	solvers := []struct {
		name string
		args []string
	}{
		{"z3", []string{"-in", "-smt2"}},
		{"cvc5", []string{"--lang=smt2"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := base
			for _, assertion := range test.assertions {
				query += fmt.Sprintf("(assert %s)\n", assertion)
			}
			query += "(check-sat)\n"
			for _, solver := range solvers {
				t.Run(solver.name, func(t *testing.T) {
					if got := runVMTOracle(t, solver.name, solver.args, query); got != test.want {
						t.Fatalf("status = %s, want %s\nquery:\n%s", got, test.want, query)
					}
				})
			}
		})
	}
}
