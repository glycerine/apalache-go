package apalache

import (
	"context"
	"testing"
)

func TestIntOracleChoicesAndCases(t *testing.T) {
	scope, oracle, err := CreateIntOracle(InitialRewriterScope(), 3)
	if err != nil || oracle.Size() != 3 || scope.Arena.CellCount() != 6 {
		t.Fatalf("integer oracle = %#v, count %d, error %v", oracle, scope.Arena.CellCount(), err)
	}
	chosen, err := oracle.Chosen(scope, 2)
	if err != nil || chosen.(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("oracle choice = %#v, %v", chosen, err)
	}
	assertions := []Expr{
		NewNameExpr("p0", Typed(BoolT1)), NewNameExpr("p1", Typed(BoolT1)), NewNameExpr("p2", Typed(BoolT1)),
	}
	cases, err := oracle.Cases(scope, assertions, nil)
	if err != nil {
		t.Fatal(err)
	}
	conjunction := cases.(*OperExpr)
	if conjunction.Operator().Name() != "AND" || len(conjunction.Arguments()) != 3 {
		t.Fatalf("oracle cases = %#v", cases)
	}
	if _, err := oracle.Cases(scope, assertions[:2], nil); err == nil {
		t.Fatal("wrong assertion count was accepted")
	}
}

func TestZeroSingletonAndMockOracleCases(t *testing.T) {
	scope := InitialRewriterScope()
	_, zero, err := CreateIntOracle(scope, 0)
	if err != nil {
		t.Fatal(err)
	}
	zeroCases, err := zero.Cases(scope, nil, nil)
	if err != nil || zeroCases.(*NameExpr).Name() != ArenaTrueName {
		t.Fatalf("zero oracle cases = %#v, %v", zeroCases, err)
	}
	_, singleton, _ := CreateIntOracle(scope, 1)
	p := NewNameExpr("p", Typed(BoolT1))
	singleCases, err := singleton.Cases(scope, []Expr{p}, []Expr{NewValueExpr(BoolValue(false), Typed(BoolT1))})
	if err != nil || singleCases != p {
		t.Fatalf("singleton oracle cases = %#v, %v", singleCases, err)
	}
	mock, err := NewMockOracle(2)
	if err != nil || mock.Size() != 3 {
		t.Fatalf("mock oracle = %#v, %v", mock, err)
	}
	selected, err := mock.Cases(scope, []Expr{
		NewNameExpr("a", Typed(BoolT1)), NewNameExpr("b", Typed(BoolT1)), NewNameExpr("c", Typed(BoolT1)),
	}, nil)
	if err != nil || selected.(*NameExpr).Name() != "c" {
		t.Fatalf("mock cases = %#v, %v", selected, err)
	}
	condition, _ := mock.Chosen(scope, 1)
	if bool(condition.(*ValueExpr).Value().(BoolValue)) {
		t.Fatal("mock oracle selected the wrong index")
	}
}

func TestSparseAndZipOracles(t *testing.T) {
	scope, backend, err := CreateIntOracle(InitialRewriterScope(), 3)
	if err != nil {
		t.Fatal(err)
	}
	sparse, err := NewSparseOracle(backend, []int{9, 2, 5, 5})
	if err != nil || sparse.Size() != 3 {
		t.Fatalf("sparse oracle = %#v, %v", sparse, err)
	}
	missing, _ := sparse.Chosen(scope, 4)
	if bool(missing.(*ValueExpr).Value().(BoolValue)) {
		t.Fatal("sparse oracle accepted an absent index")
	}
	chosen, err := sparse.Chosen(scope, 5)
	if err != nil || chosen.(*OperExpr).Arguments()[1].(*ValueExpr).Value().(IntValue).BigInt().Int64() != 1 {
		t.Fatalf("sparse mapped choice = %#v, %v", chosen, err)
	}
	zip, err := NewZipOracle(backend, [][]int{{0, 2}, {1}})
	if err != nil || zip.Size() != 2 {
		t.Fatalf("zip oracle = %#v, %v", zip, err)
	}
	zipped, err := zip.Chosen(scope, 0)
	if err != nil || zipped.(*OperExpr).Operator().Name() != "OR" || len(zipped.(*OperExpr).Arguments()) != 2 {
		t.Fatalf("zipped choice = %#v, %v", zipped, err)
	}
	if _, err := NewZipOracle(backend, [][]int{{2, 1}}); err == nil {
		t.Fatal("unsorted zip group was accepted")
	}
}

func TestUninterpretedConstOracleCreation(t *testing.T) {
	cache := NewUninterpretedLiteralCache()
	scope, oracle, constraints, err := CreateUninterpretedConstOracle(cache, InitialRewriterScope(), 3)
	if err != nil || oracle.Size() != 3 || len(constraints) != 1 || scope.Arena.CellCount() != 9 {
		t.Fatalf("uninterpreted oracle = %#v, constraints %d, count %d, error %v", oracle, len(constraints), scope.Arena.CellCount(), err)
	}
	if constraints[0].(*OperExpr).Operator().Name() != "OR" {
		t.Fatalf("oracle range constraint = %#v", constraints[0])
	}
	chosen, err := oracle.Chosen(scope, 1)
	if err != nil || chosen.(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("uninterpreted choice = %#v, %v", chosen, err)
	}
	missing, _ := oracle.Chosen(scope, 10)
	if bool(missing.(*ValueExpr).Value().(BoolValue)) {
		t.Fatal("out-of-range uninterpreted choice was true")
	}
}

func TestOracleModelDecodingThroughG3(t *testing.T) {
	scope, intOracle, err := CreateIntOracle(InitialRewriterScope(), 3)
	if err != nil {
		t.Fatal(err)
	}
	choice, _ := intOracle.Chosen(scope, 2)
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, []Expr{choice})
	if err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("integer oracle model = %v, %v", status, err)
	}
	if decoded, err := intOracle.Decode(encoder); err != nil || decoded != 2 {
		t.Fatalf("decoded integer oracle = %d, %v", decoded, err)
	}

	cache := NewUninterpretedLiteralCache()
	scope, uninterpreted, constraints, err := CreateUninterpretedConstOracle(cache, InitialRewriterScope(), 3)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := uninterpreted.Chosen(scope, 1)
	constraints = append(constraints, cache.Constraints()...)
	constraints = append(constraints, second)
	solver = newTestG3Solver(t)
	encoder, err = EncodeRewriterScope(solver, scope, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("uninterpreted oracle model = %v, %v", status, err)
	}
	if decoded, err := uninterpreted.Decode(encoder); err != nil || decoded != 1 {
		t.Fatalf("decoded uninterpreted oracle = %d, %v", decoded, err)
	}
}
