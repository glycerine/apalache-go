package apalache

import (
	"context"
	"math/big"
	"strings"
	"testing"
)

func newTestG3Solver(t *testing.T) *G3SolverContext {
	t.Helper()
	solver, err := NewG3SolverContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := solver.Close(); err != nil {
			t.Error(err)
		}
	})
	return solver
}

func newTestG3SolverWithLogic(t *testing.T, logic string) *G3SolverContext {
	t.Helper()
	solver, err := NewG3SolverContextWithLogic(logic)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := solver.Close(); err != nil {
			t.Error(err)
		}
	})
	return solver
}

func TestG3SolverIntegerArithmeticAndScopes(t *testing.T) {
	solver := newTestG3Solver(t)
	x, err := solver.Declare("x", SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	zero, err := solver.Int(0)
	if err != nil {
		t.Fatal(err)
	}
	one, err := solver.Int(1)
	if err != nil {
		t.Fatal(err)
	}
	equalZero, err := solver.Equal(x, zero)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(equalZero); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("base check = %v, %v", status, err)
	}
	if err = solver.Push(); err != nil {
		t.Fatal(err)
	}
	lowerBound, err := solver.GreaterEqual(x, one)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(lowerBound); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("scoped contradiction = %v, %v", status, err)
	}
	if err = solver.Pop(); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("post-pop check = %v, %v", status, err)
	}
}

func TestG3SolverRejectsCrossContextAndCancellation(t *testing.T) {
	left := newTestG3Solver(t)
	right := newTestG3Solver(t)
	term, err := left.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = right.Assert(term); err == nil {
		t.Fatal("cross-context term was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if status, err := left.Check(ctx); err != context.Canceled || status != SMTUnknown {
		t.Fatalf("canceled check = %v, %v", status, err)
	}
}

func TestG3SolverEvaluatesBooleanAndIntegerModels(t *testing.T) {
	solver := newTestG3Solver(t)
	x, err := solver.Declare("model_x", SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	want := new(big.Int).Neg(new(big.Int).Exp(big.NewInt(2), big.NewInt(90), nil))
	value, err := solver.BigInt(want)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := solver.Equal(x, value)
	if err != nil {
		t.Fatal(err)
	}
	if err := solver.Assert(equal); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("model check = %v, %v", status, err)
	}
	got, err := solver.EvaluateBigInt(x)
	if err != nil || got.Cmp(want) != 0 {
		t.Fatalf("integer model value = %v, %v", got, err)
	}
	truth, err := solver.EvaluateBool(equal)
	if err != nil || !truth {
		t.Fatalf("Boolean model value = %v, %v", truth, err)
	}
}

func TestG3SolverArrayStoreSelect(t *testing.T) {
	solver := newTestG3Solver(t)
	array, err := solver.DeclareArray("array_a", SMTInt, SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	index, err := solver.Int(1)
	if err != nil {
		t.Fatal(err)
	}
	value, err := solver.Int(42)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := solver.Store(array, index, value)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := solver.Select(stored, index)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := solver.Equal(selected, value)
	if err != nil {
		t.Fatal(err)
	}
	contradiction, err := solver.Not(equal)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(contradiction); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("array store/select contradiction = %v, %v", status, err)
	}
	truth, err := solver.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = solver.Store(array, truth, value); err == nil {
		t.Fatal("Boolean array index was accepted for an integer-indexed array")
	}
}

func TestG3SolverBooleanArrayMap(t *testing.T) {
	solver := newTestG3Solver(t)
	falsehood, err := solver.Bool(false)
	if err != nil {
		t.Fatal(err)
	}
	truth, err := solver.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	left, err := solver.ConstantArray(SMTInt, "", falsehood)
	if err != nil {
		t.Fatal(err)
	}
	right, err := solver.ConstantArray(SMTInt, "", falsehood)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := solver.Int(1)
	two, _ := solver.Int(2)
	left, err = solver.Store(left, one, truth)
	if err != nil {
		t.Fatal(err)
	}
	right, err = solver.Store(right, one, truth)
	if err != nil {
		t.Fatal(err)
	}
	right, err = solver.Store(right, two, truth)
	if err != nil {
		t.Fatal(err)
	}
	intersection, err := solver.ArrayMapBoolean("AND", left, right)
	if err != nil {
		t.Fatal(err)
	}
	union, err := solver.ArrayMapBoolean("OR", left, right)
	if err != nil {
		t.Fatal(err)
	}
	atIntersection, _ := solver.Select(intersection, one)
	atUnion, _ := solver.Select(union, two)
	notIntersection, _ := solver.Not(atIntersection)
	notUnion, _ := solver.Not(atUnion)
	bad, _ := solver.Or(notIntersection, notUnion)
	if err = solver.Assert(bad); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("Boolean array maps = %v, %v", status, checkErr)
	}
	integerArray, err := solver.DeclareArray("integer_map", SMTInt, SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = solver.ArrayMapBoolean("AND", integerArray, integerArray); err == nil {
		t.Fatal("Boolean map accepted integer-valued arrays")
	}
	if _, err = solver.ArrayMapBoolean("XOR", left, right); err == nil {
		t.Fatal("unsupported Boolean map operator was accepted")
	}
}

func TestG3SolverNestedArrays(t *testing.T) {
	t.Run("array-valued result", func(t *testing.T) {
		solver := newTestG3Solver(t)
		falsehood, _ := solver.Bool(false)
		truth, _ := solver.Bool(true)
		inner, err := solver.ConstantArray(SMTInt, "", falsehood)
		if err != nil {
			t.Fatal(err)
		}
		two, _ := solver.Int(2)
		inner, err = solver.Store(inner, two, truth)
		if err != nil {
			t.Fatal(err)
		}
		outer, err := solver.constantArrayWithIndexSpec(scalarSMTSortSpec(SMTInt, ""), inner)
		if err != nil {
			t.Fatal(err)
		}
		one, _ := solver.Int(1)
		outer, err = solver.Store(outer, one, inner)
		if err != nil {
			t.Fatal(err)
		}
		selectedInner, err := solver.Select(outer, one)
		if err != nil {
			t.Fatal(err)
		}
		selectedValue, err := solver.Select(selectedInner, two)
		if err != nil {
			t.Fatal(err)
		}
		negated, _ := solver.Not(selectedValue)
		if err = solver.Assert(negated); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
			t.Fatalf("nested array result = %v, %v", status, checkErr)
		}
	})

	t.Run("array-valued index", func(t *testing.T) {
		solver := newTestG3Solver(t)
		falsehood, _ := solver.Bool(false)
		truth, _ := solver.Bool(true)
		inner, err := solver.ConstantArray(SMTInt, "", falsehood)
		if err != nil {
			t.Fatal(err)
		}
		outer, err := solver.declareArrayWithSpecs(
			"set_of_sets", smtTermSortSpec(inner), scalarSMTSortSpec(SMTBool, ""))
		if err != nil {
			t.Fatal(err)
		}
		outer, err = solver.Store(outer, inner, truth)
		if err != nil {
			t.Fatal(err)
		}
		selected, err := solver.Select(outer, inner)
		if err != nil {
			t.Fatal(err)
		}
		negated, _ := solver.Not(selected)
		if err = solver.Assert(negated); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
			t.Fatalf("nested array index = %v, %v", status, checkErr)
		}
	})
}

func TestG3SolverUninterpretedAndConstantArrays(t *testing.T) {
	solver := newTestG3Solver(t)
	element, err := solver.DeclareUninterpreted("array_element", "Elem")
	if err != nil {
		t.Fatal(err)
	}
	array, err := solver.DeclareArrayWithSortIDs("membership", SMTUninterpreted, "Elem", SMTBool, "")
	if err != nil {
		t.Fatal(err)
	}
	truth, err := solver.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := solver.Store(array, element, truth)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := solver.Select(stored, element)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(selected); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("uninterpreted-index array = %v, %v", status, checkErr)
	}

	falsehood, err := solver.Bool(false)
	if err != nil {
		t.Fatal(err)
	}
	constant, err := solver.ConstantArray(SMTUninterpreted, "Elem", falsehood)
	if err != nil {
		t.Fatal(err)
	}
	constantValue, err := solver.Select(constant, element)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(constantValue); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("constant false array = %v, %v", status, checkErr)
	}

	functionSolver := newTestG3Solver(t)
	functionElement, err := functionSolver.DeclareUninterpreted("function_element", "Elem")
	if err != nil {
		t.Fatal(err)
	}
	result, err := functionSolver.DeclareUninterpreted("array_result", "Result")
	if err != nil {
		t.Fatal(err)
	}
	function, err := functionSolver.DeclareArrayWithSortIDs(
		"function", SMTUninterpreted, "Elem", SMTUninterpreted, "Result")
	if err != nil {
		t.Fatal(err)
	}
	function, err = functionSolver.Store(function, functionElement, result)
	if err != nil {
		t.Fatal(err)
	}
	application, err := functionSolver.Select(function, functionElement)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := functionSolver.Equal(application, result)
	if err != nil {
		t.Fatal(err)
	}
	negated, err := functionSolver.Not(equal)
	if err != nil {
		t.Fatal(err)
	}
	if err = functionSolver.Assert(negated); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := functionSolver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("uninterpreted function array = %v, %v", status, checkErr)
	}
}

func TestG3SolverQuantifiers(t *testing.T) {
	for _, test := range []struct {
		name       string
		forall     bool
		wantStatus SMTStatus
	}{
		{name: "forall", forall: true, wantStatus: SMTUnsatisfiable},
		{name: "exists", forall: false, wantStatus: SMTSatisfiable},
	} {
		t.Run(test.name, func(t *testing.T) {
			solver := newTestG3SolverWithLogic(t, "AUFNIA")
			bound, err := solver.Bound(0, SMTInt)
			if err != nil {
				t.Fatal(err)
			}
			zero, err := solver.Int(0)
			if err != nil {
				t.Fatal(err)
			}
			body, err := solver.GreaterEqual(bound, zero)
			if !test.forall {
				body, err = solver.Greater(bound, zero)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = solver.Forall("not_bound", zero, body); err == nil {
				t.Fatal("ordinary integer term was accepted as a quantified binder")
			}
			var quantified SMTTerm
			if test.forall {
				quantified, err = solver.Forall("x", bound, body)
			} else {
				quantified, err = solver.Exists("x", bound, body)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = solver.Assert(quantified); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != test.wantStatus {
				t.Fatalf("quantified integer formula = %v, %v", status, err)
			}
		})
	}
}

func TestG3SolverSMTLib2TracksActiveScopes(t *testing.T) {
	solver := newTestG3Solver(t)
	x, err := solver.Declare("x", SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := solver.Int(1)
	four, _ := solver.Int(4)
	lower, err := solver.Greater(x, one)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(lower); err != nil {
		t.Fatal(err)
	}
	if err = solver.Push(); err != nil {
		t.Fatal(err)
	}
	upper, err := solver.Less(x, four)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(upper); err != nil {
		t.Fatal(err)
	}
	nested, err := solver.SMTLib2()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"(declare-fun x () Int)", "(assert (> x 1))", "(assert (< x 4))"} {
		if !strings.Contains(nested, fragment) {
			t.Fatalf("nested SMT-LIB2 lacks %q:\n%s", fragment, nested)
		}
	}
	if err = solver.Pop(); err != nil {
		t.Fatal(err)
	}
	base, err := solver.SMTLib2()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(base, "(assert (> x 1))") || strings.Contains(base, "(< x 4)") {
		t.Fatalf("base SMT-LIB2 has wrong active assertions:\n%s", base)
	}
	if err = solver.Pop(); err == nil || !strings.Contains(err.Error(), "base SMT scope") {
		t.Fatalf("base-scope pop error = %v", err)
	}
}

func TestG3SolverSMTLib2RendersNestedArraySorts(t *testing.T) {
	solver := newTestG3Solver(t)
	inner := arraySMTSortSpec(scalarSMTSortSpec(SMTInt, ""), scalarSMTSortSpec(SMTBool, ""))
	array, err := solver.declareArrayWithSpecs("nested", scalarSMTSortSpec(SMTInt, ""), inner)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(mustSMTEqual(t, solver, array, array)); err != nil {
		t.Fatal(err)
	}
	dump, err := solver.SMTLib2()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump, "(declare-fun nested () (Array Int (Array Int Bool)))") {
		t.Fatalf("nested-array SMT-LIB2:\n%s", dump)
	}
}

func mustSMTEqual(t *testing.T, solver *G3SolverContext, left, right SMTTerm) SMTTerm {
	t.Helper()
	equal, err := solver.Equal(left, right)
	if err != nil {
		t.Fatal(err)
	}
	return equal
}
