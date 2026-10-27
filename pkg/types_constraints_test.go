package apalache

import "testing"

func parsedType(t *testing.T, text string) Type {
	t.Helper()
	typ, err := ParseType(text)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func variableType(t *testing.T, name string) VarType {
	t.Helper()
	typ, err := ParseVarType(name)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func TestConstraintSolverSelectsUniqueOverload(t *testing.T) {
	solver := NewConstraintSolver(NewTypeVarPool(100), nil)
	a, b, c := variableType(t, "a"), variableType(t, "b"), variableType(t, "c")
	d := variableType(t, "d")
	tupleSignature, _ := NewOperatorType([]Type{b, c}, parsedType(t, "<<b, c>>"))
	sequenceSignature, _ := NewOperatorType([]Type{d, d}, parsedType(t, "Seq(d)"))
	if err := solver.Add(&OrClause{Clauses: []*EqClause{{Unknown: a, Term: tupleSignature}, {Unknown: a, Term: sequenceSignature}}}); err != nil {
		t.Fatal(err)
	}
	if err := solver.Add(&EqClause{Unknown: a, Term: parsedType(t, "(Int, Str) => e")}); err != nil {
		t.Fatal(err)
	}
	solution, ok, err := solver.Solve()
	if err != nil || !ok {
		t.Fatalf("Solve() = %v, %v", ok, err)
	}
	got, err := solution.ApplyRecursive(variableType(t, "e"))
	if err != nil || !EqualTypes(got, parsedType(t, "<<Int, Str>>")) {
		t.Fatalf("e = %v, %v", got, err)
	}
}

func TestConstraintSolverRejectsAmbiguousOverload(t *testing.T) {
	solver := NewConstraintSolver(NewTypeVarPool(100), nil)
	a, b, c := variableType(t, "a"), variableType(t, "b"), variableType(t, "c")
	d := variableType(t, "d")
	tupleSignature, _ := NewOperatorType([]Type{b, c}, parsedType(t, "<<b, c>>"))
	sequenceSignature, _ := NewOperatorType([]Type{d, d}, parsedType(t, "Seq(d)"))
	errors := 0
	overload := &OrClause{Clauses: []*EqClause{{Unknown: a, Term: tupleSignature}, {Unknown: a, Term: sequenceSignature}}, OnTypeError: func(*Substitution, []Type) { errors++ }}
	if err := solver.Add(overload); err != nil {
		t.Fatal(err)
	}
	if err := solver.Add(&EqClause{Unknown: a, Term: parsedType(t, "(Int, Int) => e")}); err != nil {
		t.Fatal(err)
	}
	if solution, ok, err := solver.Solve(); err != nil || ok || solution != nil || errors != 1 {
		t.Fatalf("Solve() = %v, %v, %v; errors %d", solution, ok, err, errors)
	}
}

func TestConstraintSolverPostponesReverseOrderedConstraints(t *testing.T) {
	solver := NewConstraintSolver(NewTypeVarPool(100), nil)
	a, b, c := variableType(t, "a"), variableType(t, "b"), variableType(t, "c")
	if err := solver.Add(&OrClause{Clauses: []*EqClause{
		{Unknown: a, Term: parsedType(t, "(b, c) => b")},
		{Unknown: a, Term: parsedType(t, "(b, c) => c")},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := solver.Add(&OrClause{Clauses: []*EqClause{
		{Unknown: b, Term: c}, {Unknown: b, Term: BoolT1},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, clause := range []*EqClause{
		{Unknown: c, Term: IntT1},
		{Unknown: a, Term: parsedType(t, "(Bool, d) => Int")},
	} {
		if err := solver.Add(clause); err != nil {
			t.Fatal(err)
		}
	}
	solution, ok, err := solver.Solve()
	if err != nil || !ok {
		t.Fatalf("Solve() = %v, %v", ok, err)
	}
	got, err := solution.ApplyRecursive(a)
	if err != nil || !EqualTypes(got, parsedType(t, "(Bool, Int) => Int")) {
		t.Fatalf("a = %v, %v", got, err)
	}
}

func TestConstraintSolverCallbacksAndFreeVariables(t *testing.T) {
	solver := NewConstraintSolver(NewTypeVarPool(20), nil)
	a := variableType(t, "a")
	var found Type
	clause := &EqClause{Unknown: a, Term: IntT1, OnTypeFound: func(typ Type) { found = typ }}
	if err := solver.Add(clause); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := solver.Solve(); err != nil || !ok {
		t.Fatalf("Solve() = %v, %v", ok, err)
	}
	if !EqualTypes(found, IntT1) || solver.IsFreeVar(a.Number()) || !solver.IsFreeVar(19) {
		t.Fatalf("callback/free variables: found=%v aFree=%v otherFree=%v", found, solver.IsFreeVar(a.Number()), solver.IsFreeVar(19))
	}
}
