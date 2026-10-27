package apalache

import (
	"strings"
	"testing"
)

func TestEtcExpressionEqualityIgnoresSourceAndExplanation(t *testing.T) {
	left := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(Int) => Bool")},
		NewEtcConst(BlameRef{SourceID: NewUID()}, IntT1))
	right := NewEtcApp(BlameRef{SourceID: NewUID()}, []Type{parsedType(t, "(Int) => Bool")},
		NewEtcConst(ExactRef{SourceID: NewUID()}, IntT1))
	left.SetTypeErrorExplanation(func(_, _ []Type) (string, bool) { return "left", true })
	if !EqualEtcExpr(left, right) {
		t.Fatalf("structurally equal ETC expressions differ:\n%s\n%s", left, right)
	}
	if explanation, ok := left.Explain(nil, nil); !ok || explanation != "left" {
		t.Fatalf("Explain() = %q, %v", explanation, ok)
	}
	if _, ok := right.Explain(nil, nil); ok {
		t.Fatal("unset explanation was reported")
	}
}

func TestEtcExpressionAccessorsOwnSlices(t *testing.T) {
	types := []Type{parsedType(t, "(Int) => Bool")}
	arguments := []EtcExpr{NewUniqueEtcConst(IntT1)}
	application := NewEtcApp(ExactRef{SourceID: NewUID()}, types, arguments...)
	types[0] = IntT1
	arguments[0] = NewUniqueEtcConst(BoolT1)
	gotTypes := application.OperatorTypes()
	gotArgs := application.Arguments()
	gotTypes[0] = BoolT1
	gotArgs[0] = NewUniqueEtcConst(StrT1)
	if !EqualTypes(application.OperatorTypes()[0], parsedType(t, "(Int) => Bool")) ||
		!EqualTypes(application.Arguments()[0].(*EtcConst).Polytype(), IntT1) {
		t.Fatal("ETC application leaked slice ownership")
	}
}

func TestEtcExpressionRendering(t *testing.T) {
	x := NewEtcName(ExactRef{SourceID: NewUID()}, "x")
	domain := NewBlameEtcConst(parsedType(t, "Set(Int)"))
	body := NewEtcAppByName(ExactRef{SourceID: NewUID()}, NewBlameEtcName("F"), x)
	abstraction := NewEtcAbs(ExactRef{SourceID: NewUID()}, body, EtcBinding{Name: x, Domain: domain})
	let := NewEtcLet(ExactRef{SourceID: NewUID()}, "Op", abstraction, body)
	declaration := NewEtcTypeDecl(ExactRef{SourceID: NewUID()}, "Op", parsedType(t, "(Int) => Bool"), let)
	for _, part := range []string{"@Op:", "@let Op =", "@λ", "x ∈", "@(B"} {
		if !strings.Contains(declaration.String(), part) {
			t.Fatalf("rendering %q lacks %q", declaration, part)
		}
	}
}
