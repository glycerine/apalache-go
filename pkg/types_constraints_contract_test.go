package apalache

import "testing"

func TestConstraintSolverPartialThenResumeLifecycle(t *testing.T) {
	a := mustNumberedVar(t, 0)
	innerFound := 0
	outerFound := 0
	outerErrors := 0
	first := &EqClause{Unknown: a, Term: IntT1, OnTypeFound: func(Type) { innerFound++ }}
	second := &EqClause{Unknown: a, Term: BoolT1, OnTypeFound: func(Type) { innerFound++ }}
	overload := &OrClause{
		Clauses: []*EqClause{first, second},
		OnTypeFound: func(typ Type) {
			if EqualTypes(typ, IntT1) {
				outerFound++
			}
		},
		OnTypeError: func(*Substitution, []Type) { outerErrors++ },
	}
	solver := NewConstraintSolver(NewTypeVarPool(100), nil)
	if err := solver.Add(overload); err != nil {
		t.Fatal(err)
	}
	partial, consistent, err := solver.SolvePartially()
	if err != nil || !consistent || partial == nil || innerFound != 0 || outerFound != 0 || outerErrors != 0 {
		t.Fatalf("SolvePartially() = %v, %v, %v; callbacks %d/%d/%d", partial, consistent, err, innerFound, outerFound, outerErrors)
	}
	if err = solver.Add(&EqClause{Unknown: a, Term: IntT1}); err != nil {
		t.Fatal(err)
	}
	solution, consistent, err := solver.Solve()
	if err != nil || !consistent || solution == nil || innerFound != 0 || outerFound != 1 || outerErrors != 0 {
		t.Fatalf("resumed Solve() = %v, %v, %v; callbacks %d/%d/%d", solution, consistent, err, innerFound, outerFound, outerErrors)
	}
	resolved, err := solution.ApplyRecursive(a)
	if err != nil || !EqualTypes(resolved, IntT1) {
		t.Fatalf("resolved a = %v, %v", resolved, err)
	}
}

func TestConstraintSolverImmediateUnitFailureCallback(t *testing.T) {
	a := mustNumberedVar(t, 0)
	initial := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: IntT1})
	callbackCalls := 0
	var callbackSubstitution *Substitution
	var callbackTerms []Type
	clause := &EqClause{
		Unknown: a,
		Term:    BoolT1,
		OnTypeError: func(substitution *Substitution, terms []Type) {
			callbackCalls++
			callbackSubstitution = substitution
			callbackTerms = append([]Type(nil), terms...)
		},
	}
	solver := NewConstraintSolver(NewTypeVarPool(100), initial)
	if err := solver.Add(clause); err != nil {
		t.Fatal(err)
	}
	if solution, consistent, err := solver.SolvePartially(); err != nil || consistent || solution != nil {
		t.Fatalf("SolvePartially() = %v, %v, %v", solution, consistent, err)
	}
	if callbackCalls != 1 || !EqualSubstitutions(callbackSubstitution, initial) || len(callbackTerms) != 1 || !EqualTypes(callbackTerms[0], BoolT1) {
		t.Fatalf("error callback = %d, %v, %v", callbackCalls, callbackSubstitution, callbackTerms)
	}
	// Scala clears the failed unit constraint, so a later solve observes the
	// retained approximation with no remaining work and does not re-report.
	if solution, consistent, err := solver.Solve(); err != nil || !consistent || !EqualSubstitutions(solution, initial) || callbackCalls != 1 {
		t.Fatalf("Solve() after unit failure = %v, %v, %v; callbacks %d", solution, consistent, err, callbackCalls)
	}
}

func TestConstraintSolverAmbiguousErrorReportsSubstitutedSignatures(t *testing.T) {
	a, b := mustNumberedVar(t, 0), mustNumberedVar(t, 1)
	initial := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(1), Type: IntT1})
	var callbackTerms []Type
	overload := &OrClause{
		Clauses: []*EqClause{
			{Unknown: a, Term: mustSetType(t, b)},
			{Unknown: a, Term: mustSeqType(t, b)},
		},
		OnTypeError: func(_ *Substitution, terms []Type) { callbackTerms = append([]Type(nil), terms...) },
	}
	solver := NewConstraintSolver(NewTypeVarPool(100), initial)
	if err := solver.Add(overload); err != nil {
		t.Fatal(err)
	}
	if solution, consistent, err := solver.Solve(); err != nil || consistent || solution != nil {
		t.Fatalf("Solve() = %v, %v, %v", solution, consistent, err)
	}
	if len(callbackTerms) != 2 || !EqualTypes(callbackTerms[0], mustSetType(t, IntT1)) || !EqualTypes(callbackTerms[1], mustSeqType(t, IntT1)) {
		t.Fatalf("ambiguous callback terms = %v", callbackTerms)
	}
}

func TestConstraintSolverApproximationAndFreeVariables(t *testing.T) {
	a, b, c := mustNumberedVar(t, 0), mustNumberedVar(t, 1), mustNumberedVar(t, 2)
	initial := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: IntT1})
	solver := NewConstraintSolver(NewTypeVarPool(100), initial)
	if solver.IsFreeVar(a.Number()) != false || solver.IsFreeVar(b.Number()) != true {
		t.Fatalf("initial free variables: a=%v b=%v", solver.IsFreeVar(a.Number()), solver.IsFreeVar(b.Number()))
	}
	if err := solver.Add(&EqClause{Unknown: b, Term: c}); err != nil {
		t.Fatal(err)
	}
	solution, consistent, err := solver.Solve()
	if err != nil || !consistent || solution == nil {
		t.Fatalf("Solve() = %v, %v, %v", solution, consistent, err)
	}
	if solver.IsFreeVar(a.Number()) || solver.IsFreeVar(b.Number()) || solver.IsFreeVar(c.Number()) || !solver.IsFreeVar(99) {
		t.Fatalf("solved free variables: a=%v b=%v c=%v other=%v", solver.IsFreeVar(a.Number()), solver.IsFreeVar(b.Number()), solver.IsFreeVar(c.Number()), solver.IsFreeVar(99))
	}
}

func TestConstraintSolverRejectsMalformedGoConstraints(t *testing.T) {
	solver := NewConstraintSolver(NewTypeVarPool(0), nil)
	a := mustNumberedVar(t, 0)
	for _, clause := range []TypeClause{
		&EqClause{Unknown: a},
		&OrClause{},
		&OrClause{Clauses: []*EqClause{nil}},
		&OrClause{Clauses: []*EqClause{{Unknown: a}}},
	} {
		if err := solver.Add(clause); err == nil {
			t.Errorf("Add(%#v) succeeded", clause)
		}
	}
	if err := solver.Add(nil); err == nil {
		t.Fatal("Add(nil) succeeded")
	}
}
