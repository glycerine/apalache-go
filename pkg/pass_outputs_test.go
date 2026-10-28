package apalache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePassOutputsWritesBothFormatsAndCustomMirror(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "custom")
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{
		OutDir: filepath.Join(root, "out"), CustomRunDir: custom, Command: "check", WriteIntermediate: true,
	}); err != nil {
		t.Fatal(err)
	}
	declaration := NewOperDecl("Flag", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator(nil, BoolT1)))
	module := NewModule("Original", []Decl{declaration})
	annotations := NewAnnotationStore()
	annotations.Set(declaration.ID(), []Annotation{{Name: "marker"}})
	sources := NewSourceStore()
	if err := sources.Add(declaration.ID(), SourceLocation{
		Filename: "Spec.tla", Region: NewSourceRegion(2, 1, 2, 12),
	}); err != nil {
		t.Fatal(err)
	}
	written, err := WritePassOutputs(manager, 3, "OptPass", module, PassOutputOptions{
		Annotations: annotations, SourceStore: sources,
	})
	if err != nil || !written {
		t.Fatalf("write pass outputs = %v, %v", written, err)
	}
	if module.Name() != "Original" {
		t.Fatalf("input module was renamed to %q", module.Name())
	}
	runDir, _ := manager.RunDir()
	for _, directory := range []string{
		filepath.Join(runDir, IntermediateFolderName),
		filepath.Join(custom, IntermediateFolderName),
	} {
		tla, readErr := os.ReadFile(filepath.Join(directory, "03_OutOptPass.tla"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(tla), "MODULE id03_OutOptPass") ||
			!strings.Contains(string(tla), "EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants") ||
			!strings.Contains(string(tla), "@marker") {
			t.Fatalf("unexpected TLA+ output:\n%s", tla)
		}
		encoded, readErr := os.ReadFile(filepath.Join(directory, "03_OutOptPass.json"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		var root irJSONRoot
		if unmarshalErr := json.Unmarshal(encoded, &root); unmarshalErr != nil {
			t.Fatal(unmarshalErr)
		}
		if len(root.Modules) != 1 || root.Modules[0].Name != "03_OutOptPass" {
			t.Fatalf("unexpected JSON module: %#v", root.Modules)
		}
		if !strings.Contains(string(encoded), `"filename":"Spec.tla"`) {
			t.Fatalf("JSON source metadata is missing: %s", encoded)
		}
	}
}

func TestWritePassOutputsIsLazyWhenIntermediateOutputIsDisabled(t *testing.T) {
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{OutDir: t.TempDir(), Command: "check"}); err != nil {
		t.Fatal(err)
	}
	if written, err := WritePassOutputs(manager, 0, "Parser", nil, PassOutputOptions{}); err != nil || written {
		t.Fatalf("disabled pass output = %v, %v", written, err)
	}
}

func TestWritePassOutputsRejectsNilModuleWhenEnabled(t *testing.T) {
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{
		OutDir: t.TempDir(), Command: "check", WriteIntermediate: true,
	}); err != nil {
		t.Fatal(err)
	}
	written, err := WritePassOutputs(manager, 0, "Parser", nil, PassOutputOptions{})
	if err == nil || !written || err.Error() != "pass Parser cannot write a nil module" {
		t.Fatalf("nil pass output = %v, %v", written, err)
	}
}

func TestStandardTLAModulesReturnsDefensiveSlice(t *testing.T) {
	modules := StandardTLAModules()
	modules[0] = "Changed"
	if got := StandardTLAModules()[0]; got != "Integers" {
		t.Fatalf("standard modules were mutated: %q", got)
	}
}
