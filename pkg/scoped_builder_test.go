package apalache

import (
	"errors"
	"testing"
)

func TestScopedBuilderConsistentFreeNames(t *testing.T) {
	builder := NewScopedBuilder()
	instruction := builder.Tuple(builder.Name("x", IntT1), builder.Name("x", IntT1))
	expr, err := instruction.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, expr, mustTupleType(t, IntT1, IntT1))

	inconsistent := builder.Tuple(builder.Name("x", IntT1), builder.Name("x", BoolT1))
	if _, err := inconsistent.Build(); err == nil {
		t.Fatal("inconsistent free-name types accepted")
	} else {
		var scopeError *ScopeError
		if !errors.As(err, &scopeError) {
			t.Fatalf("error type = %T, want *ScopeError", err)
		}
	}
}

func TestScopedBuilderCanInferPriorNameType(t *testing.T) {
	builder := NewScopedBuilder()
	instruction := builder.Tuple(builder.Name("known", IntT1), builder.NameWithInferredType("known"))
	expr, err := instruction.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, expr, mustTupleType(t, IntT1, IntT1))
	if _, err := builder.NameWithInferredType("missing").Build(); err == nil {
		t.Fatal("out-of-scope type inference succeeded")
	}
}

func TestScopedBuilderBoundedVariableUse(t *testing.T) {
	builder := NewScopedBuilder()
	set := builder.Apply(Op("SET_ENUM"), builder.Int64(1), builder.Int64(2))
	body := builder.Apply(Op("GT"), builder.Name("x", IntT1), builder.Int64(0))
	exists := builder.BindBounded(Op("EXISTS3"), builder.Name("x", IntT1), set, body)
	expr, err := exists.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, expr, BoolT1)

	wrongBody := builder.Apply(Op("NOT"), builder.Name("x", BoolT1))
	wrong := builder.BindBounded(Op("EXISTS3"), builder.Name("x", IntT1), set, wrongBody)
	if _, err := wrong.Build(); err == nil {
		t.Fatal("bound variable used at an inconsistent type")
	}
}

func TestScopedBuilderRejectsBoundVariableInItsDomain(t *testing.T) {
	builder := NewScopedBuilder()
	domain := builder.Apply(Op("SET_ENUM"), builder.Name("x", IntT1))
	quantifier := builder.BindBounded(
		Op("FORALL3"), builder.Name("x", IntT1), domain, builder.Bool(true))
	if _, err := quantifier.Build(); err == nil {
		t.Fatal("bound variable occurring in its own domain was accepted")
	} else {
		var scopeError *ScopeError
		if !errors.As(err, &scopeError) {
			t.Fatalf("error type = %T, want *ScopeError: %v", err, err)
		}
	}
}

func TestScopedBuilderRejectsNestedShadowing(t *testing.T) {
	builder := NewScopedBuilder()
	domain := builder.Apply(Op("SET_ENUM"), builder.Int64(1))
	inner := builder.BindBounded(
		Op("EXISTS3"), builder.Name("x", IntT1), domain,
		builder.Apply(Op("EQ"), builder.Name("x", IntT1), builder.Int64(1)))
	outer := builder.BindBounded(Op("EXISTS3"), builder.Name("x", IntT1), domain, inner)
	if _, err := outer.Build(); err == nil {
		t.Fatal("nested binder shadowing was accepted")
	}
}

func TestScopedBuilderUnboundedAndVariadicBinders(t *testing.T) {
	builder := NewScopedBuilder()
	unbounded := builder.BindUnbounded(
		Op("EXISTS2"), builder.Name("flag", BoolT1), builder.Name("flag", BoolT1))
	expr, err := unbounded.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, expr, BoolT1)

	set := builder.Apply(Op("SET_ENUM"), builder.Int64(1), builder.Int64(2))
	mapped := builder.BindVariadic(
		Op("SET_MAP"),
		builder.Apply(Op("PLUS"), builder.Name("x", IntT1), builder.Name("y", IntT1)),
		BoundDomain{Variable: builder.Name("x", IntT1), Set: set},
		BoundDomain{Variable: builder.Name("y", IntT1), Set: set},
	)
	mappedExpr, err := mapped.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, mappedExpr, mustSetType(t, IntT1))

	duplicate := builder.BindVariadic(
		Op("SET_MAP"), builder.Name("x", IntT1),
		BoundDomain{Variable: builder.Name("x", IntT1), Set: set},
		BoundDomain{Variable: builder.Name("x", IntT1), Set: set},
	)
	if _, err := duplicate.Build(); err == nil {
		t.Fatal("duplicate variadic bound variables accepted")
	}
}
