package apalache

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullyTaggedIRStillRequiresTypeInference(t *testing.T) {
	result := ParseAndTypeCheckFile(context.Background(), "testdata/code_review/BadTypedIR.json", FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if result.OK() {
		t.Fatal("fully tagged 1 + TRUE was accepted as type-checked")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("missing type error")
	}
}

func TestTypedQuintInputRequiresConsistentInference(t *testing.T) {
	for _, bad := range []bool{false, true} {
		source := quintBigIntFixture
		if bad {
			source = strings.Replace(source, `{"id":8,"kind":"int","value":10000}`, `{"id":8,"kind":"bool","value":true}`, 1)
			source = strings.Replace(source, `"8":{"type":{"kind":"int"}}`, `"8":{"type":{"kind":"bool"}}`, 1)
		}
		path := filepath.Join(t.TempDir(), "model.qnt.json")
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		result := ParseAndTypeCheckFile(context.Background(), path, FrontendOptions{}, TypeCheckOptions{UseRows: true})
		if result.OK() == bad {
			t.Fatalf("bad=%v: diagnostics=%v", bad, result.Diagnostics)
		}
	}
}

func TestExplorationRejectsInconsistentTypedQuint(t *testing.T) {
	source := strings.Replace(quintBigIntFixture, `{"id":8,"kind":"int","value":10000}`, `{"id":8,"kind":"bool","value":true}`, 1)
	source = strings.Replace(source, `"8":{"type":{"kind":"int"}}`, `"8":{"type":{"kind":"bool"}}`, 1)
	result, cleanup, err := loadExplorationSource(context.Background(), loadSpecParams{Sources: []string{source}, Format: "qnt"})
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if result.OK() {
		t.Fatal("explorer accepted ill-typed Quint")
	}
}
