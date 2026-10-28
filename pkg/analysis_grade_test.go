package apalache

import "testing"

func TestExpressionGradeAnalysisScalaOracleCases(t *testing.T) {
	analyze := func(expression Expr, constants, variables []string) (ExprGrade, *ExprGradeStore) {
		constantSet := make(map[string]struct{}, len(constants))
		variableSet := make(map[string]struct{}, len(variables))
		for _, name := range constants {
			constantSet[name] = struct{}{}
		}
		for _, name := range variables {
			variableSet[name] = struct{}{}
		}
		store := NewExprGradeStore()
		return LabelExpressionGrades(store, constantSet, variableSet, expression), store
	}

	for _, test := range []struct {
		name       string
		expression Expr
		constants  []string
		variables  []string
		want       ExprGrade
	}{
		{"value", intLiteral(1), nil, nil, ExprGradeConstant},
		{"constant-name", NewNameExpr("C", Typed(IntT1)), []string{"C"}, nil, ExprGradeConstant},
		{"state-variable", NewNameExpr("x", Typed(IntT1)), nil, []string{"x"}, ExprGradeStateFree},
		{"bound-name", NewNameExpr("i", Typed(IntT1)), nil, nil, ExprGradeStateBound},
		{"free-action", MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), nil, []string{"x"}, ExprGradeActionFree},
		{"bound-action", MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("i", Typed(IntT1))), nil, nil, ExprGradeActionBound},
		{"generated", MustOperExpr(Op("Apalache!Gen"), Typed(IntT1), intLiteral(1)), nil, nil, ExprGradeNonCacheable},
		{"nullary", MustOperExpr(Op("AND"), Typed(BoolT1)), nil, nil, ExprGradeConstant},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, _ := analyze(test.expression, test.constants, test.variables)
			if got != test.want {
				t.Fatalf("grade = %v, want %v", got, test.want)
			}
		})
	}

	stateVariable := NewNameExpr("x", Typed(IntT1))
	literal := intLiteral(1)
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), stateVariable, literal)
	grade, store := analyze(equality, nil, []string{"x"})
	if grade != ExprGradeStateFree {
		t.Fatalf("ordinary operator grade = %v", grade)
	}
	for expression, want := range map[Expr]ExprGrade{
		equality: ExprGradeStateFree, stateVariable: ExprGradeStateFree, literal: ExprGradeConstant,
	} {
		if got, found := store.Get(expression.ID()); !found || got != want {
			t.Errorf("stored grade = %v, %v; want %v", got, found, want)
		}
	}

	temporalChild := NewNameExpr("x", Typed(BoolT1))
	temporal := MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), temporalChild)
	grade, store = analyze(temporal, nil, []string{"x"})
	if childGrade, found := store.Get(temporalChild.ID()); grade != ExprGradeHigher || !found || childGrade != ExprGradeStateFree {
		t.Fatalf("temporal grades = root %v, child %v, found %v", grade, childGrade, found)
	}

	for _, test := range []struct{ left, right, want ExprGrade }{
		{ExprGradeConstant, ExprGradeStateFree, ExprGradeStateFree},
		{ExprGradeStateFree, ExprGradeStateBound, ExprGradeStateBound},
		{ExprGradeActionFree, ExprGradeStateBound, ExprGradeActionBound},
		{ExprGradeHigher, ExprGradeConstant, ExprGradeHigher},
		{ExprGradeNonCacheable, ExprGradeHigher, ExprGradeNonCacheable},
	} {
		if got := JoinExprGrades(test.left, test.right); got != test.want {
			t.Errorf("join(%v, %v) = %v, want %v", test.left, test.right, got, test.want)
		}
		if got := JoinExprGrades(test.right, test.left); got != test.want {
			t.Errorf("reverse join(%v, %v) = %v, want %v", test.right, test.left, got, test.want)
		}
	}

	let := NewLetInExpr(boolExpr(true), Typed(BoolT1), boolDefinition("A", boolExpr(true)))
	if grade, _ := analyze(let, nil, nil); grade != ExprGradeHigher {
		t.Fatalf("LET grade = %v, want higher", grade)
	}
}
