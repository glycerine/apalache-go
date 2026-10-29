package apalache

import (
	"context"
	"testing"
)

func prepareSafetyModelFixture(t *testing.T, source string, invariants []string, view string) SafetyPipelineResult {
	t.Helper()
	frontend := ParseAndTypeCheckSource(context.Background(), "Review.tla", source, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend: %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: invariants, View: view,
		}, Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}
