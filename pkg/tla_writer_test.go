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

func TestPrettyTLAWriterWithAnnotationsMatchesScalaDeclarationLayouts(t *testing.T) {
	functionType, err := NewFunType(IntT1, BoolT1)
	if err != nil {
		t.Fatal(err)
	}
	typedVariable := NewVarDecl("myFun", Typed(functionType))
	untypedVariable := NewVarDecl("plain", Untyped())
	constant := NewConstDecl("N", Untyped())
	operator := NewOperDecl("RecOper", []OperatorParam{ValueParam("x"), ValueParam("y")},
		NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator([]Type{IntT1, StrT1}, BoolT1)))
	operator.SetRecursive(true)
	assumption := NewAssumeDecl(nil,
		MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Untyped()), NewValueExpr(BoolValue(true), Typed(BoolT1))),
		Typed(BoolT1))
	store := NewAnnotationStore()
	store.Set(constant.ID(), []Annotation{
		NewTypeAnnotation("Int"), {Name: "sweet", Arguments: []AnnotationArg{AnnotationBoolean{Value: true}}},
	})
	store.Set(operator.ID(), []Annotation{{Name: "stored"}})

	tests := []struct {
		name        string
		declaration Decl
		want        string
	}{
		{"typed variable", typedVariable, "VARIABLE\n  (*\n    @type: (Int -> Bool);\n  *)\n  myFun\n\n"},
		{"untyped variable", untypedVariable, "VARIABLE plain\n\n"},
		{"stored constant", constant, "CONSTANT\n  (*\n    @type: Int;\n    @sweet(true)\n  *)\n  N\n\n"},
		{"recursive operator", operator, "RECURSIVE RecOper(_, _)\n(*\n  @type: ((Int, Str) => Bool);\n  @stored\n*)\nRecOper(x, y) == TRUE\n\n"},
		{"unnamed assumption", assumption, "(*\n  @type: Bool;\n*)\nASSUME(x = TRUE)\n\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := NewPrettyTLAWriterWithAnnotations(&output, store).WriteDeclaration(test.declaration); err != nil {
				t.Fatal(err)
			}
			if output.String() != test.want {
				t.Fatalf("annotated declaration:\n%s\nwant:\n%s", output.String(), test.want)
			}
		})
	}
}

func TestPrettyTLAWriterWithAnnotationsWritesTypedModule(t *testing.T) {
	constant := NewConstDecl("N", Typed(IntT1))
	module := NewModule("M", []Decl{constant})
	var output bytes.Buffer
	if err := NewPrettyTLAWriterWithAnnotations(&output, NewAnnotationStore()).WriteModule(module, []string{"Integers"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"MODULE M", "EXTENDS Integers", "@type: Int;", "CONSTANT\n  (*"} {
		if !strings.Contains(output.String(), fragment) {
			t.Fatalf("typed module is missing %q:\n%s", fragment, output.String())
		}
	}
}

func TestTLADeclarationAnnotatorDecoratesDirectAndModuleOutput(t *testing.T) {
	declaration := NewOperDecl("Flag", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Untyped())
	layout := TextLayout{TextWidth: 41, Indent: 3}
	calls := 0
	annotator := TLADeclarationAnnotatorFunc(func(gotLayout TextLayout, gotDeclaration Decl) ([]string, bool) {
		calls++
		if gotLayout != layout || gotDeclaration.ID() != declaration.ID() {
			t.Fatalf("annotator arguments = %+v, %s", gotLayout, gotDeclaration.ID())
		}
		return []string{"@custom(true)"}, true
	})
	var direct bytes.Buffer
	if err := NewPrettyTLAWriterWithAnnotator(&direct, layout, annotator).WriteDeclaration(declaration); err != nil {
		t.Fatal(err)
	}
	if direct.String() != "(*\n  @custom(true)\n*)\nFlag == TRUE\n\n" {
		t.Fatalf("direct decorated output = %q", direct.String())
	}
	var module bytes.Buffer
	if err := NewPrettyTLAWriterWithAnnotator(&module, layout, annotator).
		WriteModule(NewModule("M", []Decl{declaration}), nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !strings.Contains(module.String(), "@custom(true)") {
		t.Fatalf("module decoration calls/output = %d, %q", calls, module.String())
	}
}

func TestTLADeclarationAnnotatorDefaultsToNoDecoration(t *testing.T) {
	declaration := NewConstDecl("N", Untyped())
	var output bytes.Buffer
	if err := NewPrettyTLAWriterWithAnnotator(&output, DefaultTextLayout(), nil).WriteDeclaration(declaration); err != nil {
		t.Fatal(err)
	}
	if output.String() != "CONSTANT N\n\n" {
		t.Fatalf("default annotator output = %q", output.String())
	}
	if annotations, found := (NoTLADeclarationAnnotations{}).Annotate(DefaultTextLayout(), declaration); found || annotations != nil {
		t.Fatalf("default annotations = %#v, %v", annotations, found)
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
