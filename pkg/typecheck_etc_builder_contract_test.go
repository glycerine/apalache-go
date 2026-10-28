package apalache

import "testing"

func TestEtcBuilderEveryUniqueAndBlameConstructor(t *testing.T) {
	constant := NewUniqueEtcConst(IntT1)
	name := NewUniqueEtcName("x")
	domain := NewBlameEtcConst(mustSetType(t, IntT1))
	binding := EtcBinding{Name: name, Domain: domain}
	operatorType := mustOperatorType(t, []Type{IntT1}, BoolT1)

	pairs := [][2]EtcExpr{
		{NewUniqueEtcConst(IntT1), NewBlameEtcConst(IntT1)},
		{NewUniqueEtcName("x"), NewBlameEtcName("x")},
		{NewUniqueEtcAbs(constant, binding), NewBlameEtcAbs(constant, binding)},
		{NewUniqueEtcApp([]Type{operatorType}, constant), NewBlameEtcApp([]Type{operatorType}, constant)},
		{NewUniqueEtcAppByName(name, constant), NewBlameEtcAppByName(name, constant)},
		{NewUniqueEtcLet("F", constant, name), NewBlameEtcLet("F", constant, name)},
		{NewUniqueEtcTypeDecl("F", operatorType, name), NewBlameEtcTypeDecl("F", operatorType, name)},
	}
	seen := make(map[uint64]struct{})
	for index, pair := range pairs {
		if _, ok := pair[0].SourceRef().(ExactRef); !ok {
			t.Errorf("pair %d unique source = %T", index, pair[0].SourceRef())
		}
		if _, ok := pair[1].SourceRef().(BlameRef); !ok {
			t.Errorf("pair %d blame source = %T", index, pair[1].SourceRef())
		}
		if !EqualEtcExpr(pair[0], pair[1]) {
			t.Errorf("pair %d differs structurally:\n%s\n%s", index, pair[0], pair[1])
		}
		for _, expression := range pair {
			id := expression.SourceRef().ID().Uint64()
			if _, duplicate := seen[id]; duplicate {
				t.Errorf("builder reused UID %d", id)
			}
			seen[id] = struct{}{}
		}
	}
}

func TestEtcExprEveryExactRenderingContract(t *testing.T) {
	ref := ExactRef{SourceID: NullUID()}
	blame := BlameRef{SourceID: NullUID()}
	constant := NewEtcConst(ref, IntT1)
	name := NewEtcName(blame, "x")
	domain := NewEtcConst(blame, mustSetType(t, IntT1))
	abstraction := NewEtcAbs(ref, name, EtcBinding{Name: name, Domain: domain})
	operatorType := mustOperatorType(t, []Type{IntT1}, BoolT1)
	application := NewEtcApp(ref, []Type{operatorType, mustOperatorType(t, []Type{IntT1}, IntT1)}, constant)
	namedApplication := NewEtcAppByName(ref, name, constant)
	let := NewEtcLet(ref, "F", abstraction, namedApplication)
	declaration := NewEtcTypeDecl(ref, "F", operatorType, let)

	tests := []struct {
		expression EtcExpr
		want       string
	}{
		{constant, "E0@Int"},
		{name, "B0@x"},
		{abstraction, "E0@λ B0@x ∈ B0@Set(Int). B0@x"},
		{application, "E0@((((Int) => Bool) | ((Int) => Int)) E0@Int)"},
		{namedApplication, "E0@(B0@x E0@Int)"},
		{let, "E0@let F = E0@λ B0@x ∈ B0@Set(Int). B0@x in E0@(B0@x E0@Int)"},
		{declaration, "E0@F: ((Int) => Bool) in E0@let F = E0@λ B0@x ∈ B0@Set(Int). B0@x in E0@(B0@x E0@Int)"},
	}
	for _, test := range tests {
		if got := test.expression.String(); got != test.want {
			t.Errorf("%T.String() = %q, want %q", test.expression, got, test.want)
		}
	}
}

func TestEtcExprExplanationDefensiveInputs(t *testing.T) {
	expression := NewUniqueEtcConst(IntT1)
	expected := []Type{IntT1}
	actual := []Type{BoolT1}
	expression.SetTypeErrorExplanation(func(gotExpected, gotActual []Type) (string, bool) {
		gotExpected[0] = StrT1
		gotActual[0] = StrT1
		return "custom", true
	})
	if explanation, ok := expression.Explain(expected, actual); !ok || explanation != "custom" {
		t.Fatalf("Explain() = %q, %v", explanation, ok)
	}
	if !EqualTypes(expected[0], IntT1) || !EqualTypes(actual[0], BoolT1) {
		t.Fatal("Explain exposed caller slice ownership")
	}
}
