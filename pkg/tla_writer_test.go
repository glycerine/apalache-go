package apalache

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrettyTLAWriterWritesEveryNodeShape(t *testing.T) {
	expression := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("x", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	declaration := NewOperDecl("P", nil, expression, Typed(mustOperator(nil, BoolT1)))
	module := NewModule("M", []Decl{declaration})

	var expressionOutput bytes.Buffer
	if err := NewPrettyTLAWriter(&expressionOutput, nil).WriteExpression(expression); err != nil {
		t.Fatal(err)
	}
	if expressionOutput.String() != "x = 1" {
		t.Fatalf("expression output = %q", expressionOutput.String())
	}

	annotations := NewAnnotationStore()
	annotations.Set(declaration.ID(), []Annotation{{Name: "marker"}})
	var declarationOutput bytes.Buffer
	if err := NewPrettyTLAWriter(&declarationOutput, annotations).WriteDeclaration(declaration); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(declarationOutput.String(), "@marker") || !strings.Contains(declarationOutput.String(), "P == x = 1") {
		t.Fatalf("declaration output = %q", declarationOutput.String())
	}

	var moduleOutput bytes.Buffer
	if err := NewPrettyTLAWriter(&moduleOutput, annotations).WriteModule(module, []string{"Integers"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(moduleOutput.String(), "MODULE M") || !strings.Contains(moduleOutput.String(), "EXTENDS Integers") {
		t.Fatalf("module output = %q", moduleOutput.String())
	}
}

func TestJSONTLAWriterWritesEveryNodeShapeWithSources(t *testing.T) {
	expression := NewValueExpr(Int64Value(1), Typed(IntT1))
	declaration := NewOperDecl("N", nil, expression, Typed(mustOperator(nil, IntT1)))
	module := NewModule("M", []Decl{declaration})
	store := NewSourceStore()
	if err := store.Add(expression.ID(), SourceLocation{
		Filename: "M.tla", Region: NewSourceRegion(2, 6, 2, 6),
	}); err != nil {
		t.Fatal(err)
	}

	for name, write := range map[string]func(TLAWriter) error{
		"expression":  func(writer TLAWriter) error { return writer.WriteExpression(expression) },
		"declaration": func(writer TLAWriter) error { return writer.WriteDeclaration(declaration) },
		"module":      func(writer TLAWriter) error { return writer.WriteModule(module, []string{"Ignored"}) },
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := write(NewJSONTLAWriter(&output, store)); err != nil {
				t.Fatal(err)
			}
			var value any
			if err := json.Unmarshal(output.Bytes(), &value); err != nil {
				t.Fatalf("invalid JSON %q: %v", output.String(), err)
			}
			if name == "expression" && !strings.Contains(output.String(), `"filename":"M.tla"`) {
				t.Fatalf("source metadata is missing: %s", output.String())
			}
		})
	}
}

func TestTLAWriterFactoryWritesIntermediateMirrorsAndExplicitFiles(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "custom")
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{
		OutDir: root, CustomRunDir: custom, Command: "check", WriteIntermediate: true,
	}); err != nil {
		t.Fatal(err)
	}
	module := NewModule("Snapshot", []Decl{
		NewOperDecl("Flag", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator(nil, BoolT1))),
	})
	factory := TLAWriterFactory{}
	written, err := factory.WriteModuleAllFormats(manager, module, StandardTLAModules())
	if err != nil || !written {
		t.Fatalf("all formats = %v, %v", written, err)
	}
	runDir, _ := manager.RunDir()
	for _, directory := range []string{filepath.Join(runDir, IntermediateFolderName), filepath.Join(custom, IntermediateFolderName)} {
		for _, extension := range []string{".tla", ".json"} {
			if _, err := os.Stat(filepath.Join(directory, "Snapshot"+extension)); err != nil {
				t.Fatalf("missing %s mirror in %s: %v", extension, directory, err)
			}
		}
	}
	tlaPath, jsonPath := filepath.Join(root, "explicit.tla"), filepath.Join(root, "explicit.json")
	if err := factory.WriteModuleTLAFile(tlaPath, module, nil); err != nil {
		t.Fatal(err)
	}
	if err := factory.WriteModuleJSONFile(jsonPath, module); err != nil {
		t.Fatal(err)
	}
	if tla, err := os.ReadFile(tlaPath); err != nil || !strings.Contains(string(tla), "MODULE Snapshot") {
		t.Fatalf("explicit TLA+ = %q, %v", tla, err)
	}
	if encoded, err := os.ReadFile(jsonPath); err != nil || !strings.Contains(string(encoded), `"name":"Snapshot"`) {
		t.Fatalf("explicit JSON = %q, %v", encoded, err)
	}
}

func TestTLAWriterFactorySkipsDisabledIntermediateOutput(t *testing.T) {
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{OutDir: t.TempDir(), Command: "check"}); err != nil {
		t.Fatal(err)
	}
	if written, err := (TLAWriterFactory{}).WriteModuleAllFormats(manager, nil, nil); err != nil || written {
		t.Fatalf("disabled output = %v, %v", written, err)
	}
}

func TestTLAWritersRejectMissingOutput(t *testing.T) {
	if err := NewPrettyTLAWriter(nil, nil).WriteExpression(NewValueExpr(BoolValue(true), Typed(BoolT1))); err == nil {
		t.Fatal("pretty writer accepted a nil output")
	}
	if err := NewJSONTLAWriter(nil, nil).WriteExpression(NewValueExpr(BoolValue(true), Typed(BoolT1))); err == nil {
		t.Fatal("JSON writer accepted a nil output")
	}
}
