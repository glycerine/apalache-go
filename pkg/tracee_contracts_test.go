package apalache

import (
	"reflect"
	"testing"
)

func TestTraceePackageAliasesRetainStateShapes(t *testing.T) {
	state := TraceeState{"x": intLiteral(1)}
	sequence := TraceeStateSeq{state}
	if len(sequence) != 1 || !EqualExpr(sequence[0]["x"], intLiteral(1)) {
		t.Fatalf("tracee state sequence = %#v", sequence)
	}
}

func TestTraceeBridgingPassIsIdentityWithAnalyzedTransformation(t *testing.T) {
	pass := NewTraceeBridgingPass()
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1))})
	got, err := pass.Execute(module)
	if err != nil || got != module {
		t.Fatalf("bridge execute = %p, %v; want %p", got, err, module)
	}
	if pass.Name() != "BridgingPass" || len(pass.Dependencies()) != 0 ||
		!reflect.DeepEqual(pass.Transformations(), []ModuleProperty{PropertyAnalyzed}) {
		t.Fatalf("bridge metadata = %q, %#v, %#v", pass.Name(), pass.Dependencies(), pass.Transformations())
	}
	// The executor, rather than Execute, applies transformation metadata.
	if module.HasProperty(PropertyAnalyzed) {
		t.Fatal("identity bridge mutated its input module")
	}
	executor := NewPassChainExecutor(ModulePassFunc{
		PassName:  "LoadTraceeModule",
		Transform: func(*Module) (*Module, error) { return module, nil },
	}, pass)
	bridged, err := executor.Run()
	if err != nil || !bridged.HasProperty(PropertyAnalyzed) {
		t.Fatalf("bridged pass chain = %#v, %v", bridged, err)
	}
}
