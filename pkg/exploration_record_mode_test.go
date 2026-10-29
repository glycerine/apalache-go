package apalache

import (
	"context"
	"testing"
)

func TestExplorationFrontendDefaultsToRows(t *testing.T) {
	result, cleanup, err := loadExplorationSource(context.Background(), loadSpecParams{Sources: []string{"---- MODULE M ----\nR == {[a|->1],[b|->2]}\n===="}, Format: "tla"})
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if result.OK() {
		t.Fatal("exploration accepted heterogeneous record rows using legacy inference")
	}
}
