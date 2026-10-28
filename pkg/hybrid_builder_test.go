package apalache

import (
	"fmt"
	"testing"
)

func requireEqualTypedExpression(t *testing.T, got, want Expr) {
	t.Helper()
	if !EqualTypedExpr(got, want) {
		t.Fatalf("typed expressions differ: got %T (%v), want %T (%v)", got, got.Tag(), want, want.Tag())
	}
}

func TestHybridPrimeEqMatchesScala(t *testing.T) {
	typed := NewTypedBuilder()
	scoped := NewScopedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		left, right := typed.Name("x", typ), typed.Name("y", typ)
		got, err := typed.PrimeEq(left, right)
		if err != nil {
			t.Fatal(err)
		}
		prime, _ := typed.Prime(left)
		want, _ := typed.Eql(prime, right)
		requireEqualTypedExpression(t, got, want)

		built, err := scoped.PrimeEq(scoped.Name("x", typ), scoped.Name("y", typ)).Build()
		if err != nil {
			t.Fatal(err)
		}
		requireEqualTypedExpression(t, built, want)
		if _, err = typed.PrimeEq(typed.Name("bad", differentConcreteType(typ)), right); err == nil {
			t.Errorf("accepted mismatched primed operand for %v", typ)
		}
		if _, err = scoped.PrimeEq(scoped.Name("x", typ), scoped.Name("x", differentConcreteType(typ))).Build(); err == nil {
			t.Errorf("accepted inconsistent scoped name for %v", typ)
		}
	}
}

func TestHybridDeclarationsMatchScalaOracle(t *testing.T) {
	typed := NewTypedBuilder()
	scoped := NewScopedBuilder()
	types := concreteActionBuilderTypes(t)
	for index := 0; index < 1000; index++ {
		typ := types[index%len(types)]
		name := fmt.Sprintf("p%d", index)
		parameter, err := NewTypedParameter(name, typ)
		if err != nil {
			t.Fatal(err)
		}
		body := scoped.Name(name, typ)
		explicit, err := scoped.Declaration("A", body, parameter).Build()
		if err != nil {
			t.Fatalf("case %d: %v", index, err)
		}
		inferred, err := scoped.DeclarationWithInferredParameterTypes("A", body, parameter.Parameter).Build()
		if err != nil {
			t.Fatalf("inferred case %d: %v", index, err)
		}
		want, err := typed.Declaration("A", typed.Name(name, typ), parameter)
		if err != nil {
			t.Fatal(err)
		}
		if !EqualTypedDecl(explicit, want) || !EqualTypedDecl(inferred, want) {
			t.Fatalf("case %d did not match explicit Scala declaration shape", index)
		}
	}

	operatorType := mustOperatorType(t, []Type{IntT1}, BoolT1)
	operatorParameter, _ := NewTypedParameter("pred", operatorType)
	operatorBody := scoped.ApplyOperator(scoped.Name("pred", operatorType), scoped.Name("n", IntT1))
	operatorDecl, err := scoped.Declaration("P", operatorBody, operatorParameter).Build()
	if err != nil {
		t.Fatal(err)
	}
	if got := operatorDecl.FormalParams(); len(got) != 1 || got[0] != (OperatorParam{Name: "pred", Arity: 1}) {
		t.Fatalf("operator parameters = %v", got)
	}
	requireExprType(t, operatorDecl.Body(), BoolT1)

	badParameter, _ := NewTypedParameter("p", BoolT1)
	if _, err = scoped.Declaration("Bad", scoped.Name("p", IntT1), badParameter).Build(); err == nil {
		t.Fatal("explicit declaration accepted a parameter used at another type")
	}
	if _, err = scoped.DeclarationWithInferredParameterTypes("Bad", scoped.Name("x", IntT1), ValueParam("missing")).Build(); err == nil {
		t.Fatal("inferred declaration accepted a parameter absent from scope")
	}
}

func TestHybridLetAndLambdaScopeMatchScala(t *testing.T) {
	typed := NewTypedBuilder()
	scoped := NewScopedBuilder()
	p, _ := NewTypedParameter("p", IntT1)
	decl := scoped.Declaration("F", scoped.Name("p", IntT1), p)
	got, err := scoped.LetIn(scoped.Name("F", mustOperatorType(t, []Type{IntT1}, IntT1)), decl).Build()
	if err != nil {
		t.Fatal(err)
	}
	typedDecl, _ := typed.Declaration("F", typed.Name("p", IntT1), p)
	want, _ := typed.LetIn(typed.Name("F", mustOperatorType(t, []Type{IntT1}, IntT1)), typedDecl)
	requireEqualTypedExpression(t, got, want)

	declG := scoped.Declaration("G", scoped.Name("x", BoolT1))
	nested, err := scoped.LetIn(scoped.Name("e", StrT1), decl, declG).Build()
	if err != nil {
		t.Fatal(err)
	}
	outer := nested.(*LetInExpr)
	inner, ok := outer.Body().(*LetInExpr)
	if !ok || outer.Declarations()[0].Name() != "F" || inner.Declarations()[0].Name() != "G" {
		t.Fatal("multiple declarations were not nested in Scala foldRight order")
	}
	if _, err = scoped.LetIn(scoped.Int64(0)).Build(); err == nil {
		t.Fatal("empty LET declaration list was accepted")
	}
	if _, err = scoped.LetIn(scoped.Int64(0), scoped.Declaration("F", scoped.Name("F", IntT1))).Build(); err == nil {
		t.Fatal("LET declaration shadowing itself was accepted")
	}
	if _, err = scoped.LetIn(
		scoped.Exists(scoped.Name("F", IntT1), scoped.Bool(true)),
		scoped.Declaration("F", scoped.Int64(0)),
	).Build(); err == nil {
		t.Fatal("LET body binding its declaration name was accepted")
	}

	lambda, err := scoped.Lambda("L", scoped.Name("p", IntT1), p).Build()
	if err != nil {
		t.Fatal(err)
	}
	wantLambda, _ := typed.Lambda("L", typed.Name("p", IntT1), p)
	requireEqualTypedExpression(t, lambda, wantLambda)

	// The Scala prepass still catches the synthetic name at the incompatible
	// body type before LET construction.
	lambdaType := mustOperatorType(t, nil, IntT1)
	if _, err = scoped.Lambda("L", scoped.Name("L", lambdaType)).Build(); err == nil {
		t.Fatal("lambda accepted an incompatible occurrence of its synthetic name")
	}
}

func TestHybridExceptConstructorsMatchScala(t *testing.T) {
	typed := NewTypedBuilder()
	scoped := NewScopedBuilder()
	functionType := mustFunType(t, IntT1, StrT1)
	function := typed.Name("f", functionType)
	one, two := typed.Int64(1), typed.Int64(2)
	a, _ := typed.String("a")
	b, _ := typed.String("b")
	got, err := typed.ExceptMany(function,
		ExceptUpdate{Argument: one, Replacement: a},
		ExceptUpdate{Argument: two, Replacement: b},
	)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := typed.Except(function, one, a)
	want, _ := typed.Except(first, two, b)
	requireEqualTypedExpression(t, got, want)
	if _, err = typed.ExceptMany(function); err == nil {
		t.Fatal("empty EXCEPT update list was accepted")
	}
	scopedMany, err := scoped.ExceptMany(scoped.Name("f", functionType),
		ScopedExceptUpdate{Argument: scoped.Int64(1), Replacement: scoped.Unchecked(a)},
		ScopedExceptUpdate{Argument: scoped.Int64(2), Replacement: scoped.Unchecked(b)},
	).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireEqualTypedExpression(t, scopedMany, want)

	nestedType := mustFunType(t, IntT1, mustFunType(t, StrT1, BoolT1))
	nested := typed.Name("nested", nestedType)
	key, _ := typed.String("key")
	replacement := typed.Bool(true)
	deep, err := typed.ExceptDeep(nested, replacement, one, key)
	if err != nil {
		t.Fatal(err)
	}
	selected, _ := typed.App(nested, one)
	inner, _ := typed.Except(selected, key, replacement)
	wantDeep, _ := typed.Except(nested, one, inner)
	requireEqualTypedExpression(t, deep, wantDeep)
	if _, err = typed.ExceptDeep(nested, replacement); err == nil {
		t.Fatal("empty deep EXCEPT path was accepted")
	}

	general, err := typed.ExceptGeneral(nested,
		ExceptPath{Replacement: replacement, Arguments: []Expr{one, key}},
		ExceptPath{Replacement: typed.Bool(false), Arguments: []Expr{two, key}},
	)
	if err != nil {
		t.Fatal(err)
	}
	wantGeneral, _ := typed.ExceptDeep(deep, typed.Bool(false), two, key)
	requireEqualTypedExpression(t, general, wantGeneral)
	if _, err = typed.ExceptGeneral(nested,
		ExceptPath{Replacement: replacement, Arguments: []Expr{one}},
		ExceptPath{Replacement: replacement, Arguments: []Expr{two, key}},
	); err == nil {
		t.Fatal("nonuniform general EXCEPT paths were accepted")
	}
}

func TestHybridUncheckedAndVariableDeclarationName(t *testing.T) {
	typed := NewTypedBuilder()
	scoped := NewScopedBuilder()
	unchecked := NewNameExpr("x", Typed(IntT1))
	if typed.Unchecked(unchecked) != unchecked {
		t.Fatal("unsafe unchecked was not the Scala-compatible identity")
	}
	parameter, err := typed.Param("p", IntT1)
	if err != nil || parameter.Parameter != ValueParam("p") || !EqualTypes(parameter.Type, IntT1) {
		t.Fatalf("typed Param = %v, %v", parameter, err)
	}
	if _, err = scoped.Param("bad", mustOperatorType(t, nil, IntT1)); err == nil {
		t.Fatal("scoped Param accepted a nullary operator type")
	}
	result, err := scoped.Exists(scoped.Unchecked(unchecked), scoped.Bool(true)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, result, BoolT1)

	declaration := NewVarDecl("state", Typed(StrT1))
	name, err := typed.VarDeclAsNameEx(declaration)
	if err != nil || name.Name() != "state" {
		t.Fatalf("typed declaration name = %v, %v", name, err)
	}
	requireExprType(t, name, StrT1)
	built, err := scoped.VarDeclAsNameEx(declaration).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireEqualTypedExpression(t, built, name)
	if _, err = typed.VarDeclAsNameEx(nil); err == nil {
		t.Fatal("nil variable declaration was accepted")
	}
	if _, err = scoped.VarDeclAsNameEx(NewVarDecl("u", Untyped())).Build(); err == nil {
		t.Fatal("untyped variable declaration was accepted")
	}
	operDecl, _ := typed.Declaration("A", typed.Int64(1))
	if typed.UncheckedDecl(operDecl) != operDecl {
		t.Fatal("unsafe uncheckedDecl was not the Scala-compatible identity")
	}
	if builtDecl, buildErr := scoped.UncheckedDecl(operDecl).Build(); buildErr != nil || builtDecl != operDecl {
		t.Fatalf("scoped uncheckedDecl = %v, %v", builtDecl, buildErr)
	}
}
