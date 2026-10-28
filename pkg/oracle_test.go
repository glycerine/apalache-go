package apalache

import (
	"context"
	"sort"
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

func TestIntAndMockOracleScalaPropertyMatrices(t *testing.T) {
	base := InitialRewriterScope()
	for size := -10; size <= 10; size++ {
		scope, oracle, err := CreateIntOracle(base, size)
		if size < 0 {
			if err == nil {
				t.Errorf("integer oracle accepted negative size %d", size)
			}
			continue
		}
		if err != nil || oracle.Size() != size {
			t.Fatalf("integer oracle size %d = %#v, %v", size, oracle, err)
		}
		assertions := make([]Expr, size)
		elseAssertions := make([]Expr, size)
		for index := 0; index < size; index++ {
			assertions[index] = NewNameExpr("A"+string(rune('a'+index)), Typed(BoolT1))
			elseAssertions[index] = NewNameExpr("B"+string(rune('a'+index)), Typed(BoolT1))
			chosen, chosenErr := oracle.Chosen(scope, index)
			if chosenErr != nil {
				t.Fatal(chosenErr)
			}
			arguments := chosen.(*OperExpr).Arguments()
			if arguments[0].(*NameExpr).Name() != oracle.Cell.NameExpr().Name() ||
				arguments[1].(*ValueExpr).Value().(IntValue).BigInt().Int64() != int64(index) {
				t.Errorf("integer oracle choice %d/%d = %v", index, size, chosen)
			}
		}
		for _, otherwise := range [][]Expr{nil, elseAssertions} {
			cases, caseErr := oracle.Cases(scope, assertions, otherwise)
			if caseErr != nil {
				t.Fatal(caseErr)
			}
			switch size {
			case 0:
				if name, ok := cases.(*NameExpr); !ok || name.Name() != ArenaTrueName {
					t.Errorf("zero integer oracle cases = %v", cases)
				}
			case 1:
				if cases != assertions[0] {
					t.Errorf("singleton integer oracle cases = %v", cases)
				}
			default:
				conjunction := cases.(*OperExpr)
				if conjunction.Operator().Name() != "AND" || len(conjunction.Arguments()) != size {
					t.Errorf("integer oracle cases size %d = %v", size, cases)
				}
				for index, expression := range conjunction.Arguments() {
					ite := expression.(*OperExpr)
					args := ite.Arguments()
					if ite.Operator().Name() != "IF_THEN_ELSE" || args[1] != assertions[index] {
						t.Errorf("integer oracle ITE %d/%d = %v", index, size, ite)
					}
					if otherwise == nil {
						if value, ok := args[2].(*ValueExpr); !ok || !bool(value.Value().(BoolValue)) {
							t.Errorf("integer oracle default branch %d/%d = %v", index, size, args[2])
						}
					} else if args[2] != otherwise[index] {
						t.Errorf("integer oracle else branch %d/%d = %v", index, size, args[2])
					}
				}
			}
		}
		if _, caseErr := oracle.Cases(scope, append(assertions, boolExpr(true)), nil); caseErr == nil {
			t.Errorf("integer oracle size %d accepted wrong assertion length", size)
		}
	}

	for fixed := -10; fixed <= 10; fixed++ {
		oracle, err := NewMockOracle(fixed)
		if fixed < 0 {
			if err == nil {
				t.Errorf("mock oracle accepted negative value %d", fixed)
			}
			continue
		}
		if err != nil || oracle.Size() != fixed+1 {
			t.Fatalf("mock oracle %d = %#v, %v", fixed, oracle, err)
		}
		assertions := make([]Expr, oracle.Size())
		for index := range assertions {
			assertions[index] = NewNameExpr("A"+string(rune('a'+index)), Typed(BoolT1))
			chosen, _ := oracle.Chosen(base, index)
			if bool(chosen.(*ValueExpr).Value().(BoolValue)) != (index == fixed) {
				t.Errorf("mock oracle %d choice %d = %v", fixed, index, chosen)
			}
		}
		cases, caseErr := oracle.Cases(base, assertions, nil)
		if caseErr != nil || cases != assertions[fixed] {
			t.Errorf("mock oracle %d cases = %v, %v", fixed, cases, caseErr)
		}
		if decoded, decodeErr := oracle.Decode(nil); decodeErr != nil || decoded != fixed {
			t.Errorf("mock oracle %d decode = %d, %v", fixed, decoded, decodeErr)
		}
		if _, caseErr := oracle.Cases(base, assertions[:fixed], nil); caseErr == nil {
			t.Errorf("mock oracle %d accepted wrong assertion length", fixed)
		}
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

func TestSparseAndZipOracleScalaPropertyMatrices(t *testing.T) {
	base := InitialRewriterScope()
	for _, sourceValues := range [][]int{
		nil, {0}, {-10, 0, 10}, {5, 2, 9, 5, 2}, {20, 19, 18, 4, 3, 2, 1, 0},
	} {
		unique := append([]int(nil), sourceValues...)
		sort.Ints(unique)
		unique = compactInts(unique)
		scope, backend, err := CreateIntOracle(base, len(unique))
		if err != nil {
			t.Fatal(err)
		}
		oracle, err := NewSparseOracle(backend, sourceValues)
		if err != nil || oracle.Size() != len(unique) {
			t.Fatalf("sparse oracle %v = %#v, %v", sourceValues, oracle, err)
		}
		for index := -10; index <= 20; index++ {
			chosen, chosenErr := oracle.Chosen(scope, index)
			if chosenErr != nil {
				t.Fatal(chosenErr)
			}
			position := sort.SearchInts(unique, index)
			if position == len(unique) || unique[position] != index {
				if value, ok := chosen.(*ValueExpr); !ok || bool(value.Value().(BoolValue)) {
					t.Errorf("sparse oracle %v accepted absent value %d: %v", unique, index, chosen)
				}
				continue
			}
			arguments := chosen.(*OperExpr).Arguments()
			if arguments[1].(*ValueExpr).Value().(IntValue).BigInt().Int64() != int64(position) {
				t.Errorf("sparse oracle %v mapped %d to %v", unique, index, chosen)
			}
		}
	}
	_, wrongBackend, _ := CreateIntOracle(base, 1)
	if _, err := NewSparseOracle(wrongBackend, []int{1, 2}); err == nil {
		t.Fatal("sparse oracle accepted a backend with the wrong size")
	}

	_, backend, err := CreateIntOracle(base, 12)
	if err != nil {
		t.Fatal(err)
	}
	groupings := [][][]int{
		nil,
		{{0}, {1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}, {9}, {10}, {11}},
		{{0, 1}, {2, 3}, {4, 5}, {6, 7}, {8, 9}, {10, 11}},
		{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {9, 10, 11}},
		{{0, 1, 2, 3}, {4, 5, 6, 7}, {8, 9, 10, 11}},
		{{0, 1, 2, 3, 4, 5}, {6, 7, 8, 9, 10, 11}},
		{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
	}
	for _, groups := range groupings {
		oracle, oracleErr := NewZipOracle(backend, groups)
		if oracleErr != nil {
			t.Fatal(oracleErr)
		}
		for index := 0; index <= 10; index++ {
			chosen, chosenErr := oracle.Chosen(base, index)
			if chosenErr != nil {
				t.Fatal(chosenErr)
			}
			if index >= len(groups) {
				if value, ok := chosen.(*ValueExpr); !ok || bool(value.Value().(BoolValue)) {
					t.Errorf("zip oracle with %d groups accepted index %d: %v", len(groups), index, chosen)
				}
				continue
			}
			disjunction, ok := chosen.(*OperExpr)
			if !ok || disjunction.Operator().Name() != "OR" || len(disjunction.Arguments()) != len(groups[index]) {
				t.Errorf("zip oracle group %d/%d = %v", index, len(groups), chosen)
			}
		}
	}
	if _, err := NewZipOracle(backend, [][]int{{2, 1}}); err == nil {
		t.Fatal("zip oracle accepted an unsorted group")
	}
	if _, err := NewZipOracle(backend, [][]int{{12}}); err == nil {
		t.Fatal("zip oracle accepted an out-of-range backend index")
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

func TestUninterpretedConstOracleScalaPropertyMatrix(t *testing.T) {
	base := InitialRewriterScope()
	for size := -10; size <= 10; size++ {
		cache := NewUninterpretedLiteralCache()
		scope, oracle, constraints, err := CreateUninterpretedConstOracle(cache, base, size)
		if size < 0 {
			if err == nil {
				t.Errorf("uninterpreted oracle accepted negative size %d", size)
			}
			continue
		}
		if err != nil || oracle.Size() != size {
			t.Fatalf("uninterpreted oracle size %d = %#v, %v", size, oracle, err)
		}
		if len(constraints) != map[bool]int{true: 1, false: 0}[size > 0] {
			t.Errorf("uninterpreted oracle size %d constraints = %v", size, constraints)
		}
		assertions := make([]Expr, size)
		elseAssertions := make([]Expr, size)
		for index := range assertions {
			assertions[index] = NewNameExpr("A"+string(rune('a'+index)), Typed(BoolT1))
			elseAssertions[index] = NewNameExpr("B"+string(rune('a'+index)), Typed(BoolT1))
		}
		for index := -10; index <= 10; index++ {
			chosen, chosenErr := oracle.Chosen(scope, index)
			if chosenErr != nil {
				t.Fatal(chosenErr)
			}
			if index < 0 || index >= size {
				if value, ok := chosen.(*ValueExpr); !ok || bool(value.Value().(BoolValue)) {
					t.Errorf("uninterpreted oracle size %d accepted index %d: %v", size, index, chosen)
				}
				continue
			}
			arguments := chosen.(*OperExpr).Arguments()
			if arguments[0].(*NameExpr).Name() != oracle.Cell.NameExpr().Name() ||
				arguments[1].(*NameExpr).Name() != oracle.Values[index].NameExpr().Name() {
				t.Errorf("uninterpreted oracle size %d choice %d = %v", size, index, chosen)
			}
		}
		for _, otherwise := range [][]Expr{nil, elseAssertions} {
			cases, caseErr := oracle.Cases(scope, assertions, otherwise)
			if caseErr != nil {
				t.Fatal(caseErr)
			}
			switch size {
			case 0:
				if cases.(*NameExpr).Name() != ArenaTrueName {
					t.Errorf("zero uninterpreted cases = %v", cases)
				}
			case 1:
				if cases != assertions[0] {
					t.Errorf("singleton uninterpreted cases = %v", cases)
				}
			default:
				if conjunction := cases.(*OperExpr); conjunction.Operator().Name() != "AND" || len(conjunction.Arguments()) != size {
					t.Errorf("uninterpreted cases size %d = %v", size, cases)
				}
			}
		}
		if _, caseErr := oracle.Cases(scope, append(assertions, boolExpr(true)), nil); caseErr == nil {
			t.Errorf("uninterpreted oracle size %d accepted wrong assertion length", size)
		}
	}
}

func TestUninterpretedConstOracleScalaSolverCases(t *testing.T) {
	for selected := 0; selected < 2; selected++ {
		cache := NewUninterpretedLiteralCache()
		scope := InitialRewriterScope()
		var flag ArenaCell
		scope.Arena, flag = scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
		scope, oracle, constraints, err := CreateUninterpretedConstOracle(cache, scope, 2)
		if err != nil {
			t.Fatal(err)
		}
		cases, err := oracle.Cases(scope, []Expr{
			flag.NameExpr(), MustOperExpr(Op("NOT"), Typed(BoolT1), flag.NameExpr()),
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		choice, _ := oracle.Chosen(scope, selected)
		constraints = append(constraints, cache.Constraints()...)
		constraints = append(constraints, cases, choice)
		solver := newTestG3Solver(t)
		encoder, err := EncodeRewriterScope(solver, scope, constraints)
		if err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
			t.Fatalf("uninterpreted oracle selection %d = %v, %v", selected, status, checkErr)
		}
		if decoded, decodeErr := oracle.Decode(encoder); decodeErr != nil || decoded != selected {
			t.Fatalf("uninterpreted oracle decode %d = %d, %v", selected, decoded, decodeErr)
		}
		if value, evalErr := encoder.EvaluateBool(flag.NameExpr()); evalErr != nil || value != (selected == 0) {
			t.Fatalf("uninterpreted oracle flag %d = %v, %v", selected, value, evalErr)
		}
	}

	cache := NewUninterpretedLiteralCache()
	scope, oracle, constraints, err := CreateUninterpretedConstOracle(cache, InitialRewriterScope(), 0)
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("empty uninterpreted oracle = %v, %v", status, checkErr)
	}
	if decoded, decodeErr := oracle.Decode(encoder); decodeErr != nil || decoded != -1 {
		t.Fatalf("empty uninterpreted oracle decode = %d, %v", decoded, decodeErr)
	}
}

func TestPropositionalAndLegacySparseOracleScalaSolverCases(t *testing.T) {
	base := InitialRewriterScope()
	scope, oracle, constraints, err := CreatePropositionalOracle(base, 6)
	if err != nil || oracle.Size() != 6 || len(oracle.Bits) != 3 || len(constraints) != 2 {
		t.Fatalf("propositional oracle = %#v, constraints %d, error %v", oracle, len(constraints), err)
	}
	choice3, _ := oracle.Chosen(scope, 3)
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, append(append([]Expr(nil), constraints...), choice3))
	if err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("propositional choice 3 = %v, %v", status, checkErr)
	}
	if decoded, decodeErr := oracle.Decode(encoder); decodeErr != nil || decoded != 3 {
		t.Fatalf("propositional decode = %d, %v", decoded, decodeErr)
	}
	choice4, _ := oracle.Chosen(scope, 4)
	solver = newTestG3Solver(t)
	if _, err = EncodeRewriterScope(solver, scope, append(append(append([]Expr(nil), constraints...), choice3), choice4)); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("two propositional choices = %v, %v", status, checkErr)
	}

	for selected := 0; selected < 2; selected++ {
		caseScope := InitialRewriterScope()
		var flag ArenaCell
		caseScope.Arena, flag = caseScope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
		caseScope, caseOracle, caseConstraints, createErr := CreatePropositionalOracle(caseScope, 2)
		if createErr != nil {
			t.Fatal(createErr)
		}
		cases, caseErr := caseOracle.Cases(caseScope, []Expr{
			flag.NameExpr(), MustOperExpr(Op("NOT"), Typed(BoolT1), flag.NameExpr()),
		}, nil)
		if caseErr != nil {
			t.Fatal(caseErr)
		}
		choice, _ := caseOracle.Chosen(caseScope, selected)
		caseConstraints = append(caseConstraints, cases, choice)
		solver = newTestG3Solver(t)
		encoder, err = EncodeRewriterScope(solver, caseScope, caseConstraints)
		if err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
			t.Fatalf("propositional case %d = %v, %v", selected, status, checkErr)
		}
		if value, evalErr := encoder.EvaluateBool(flag.NameExpr()); evalErr != nil || value != (selected == 0) {
			t.Fatalf("propositional flag %d = %v, %v", selected, value, evalErr)
		}
	}

	sparseScope, backend, sparseConstraints, err := CreatePropositionalOracle(base, 3)
	if err != nil {
		t.Fatal(err)
	}
	sparse, err := NewSparseOracle(backend, []int{1, 5, 10})
	if err != nil {
		t.Fatal(err)
	}
	chosenFive, _ := sparse.Chosen(sparseScope, 5)
	solver = newTestG3Solver(t)
	encoder, err = EncodeRewriterScope(solver, sparseScope, append(sparseConstraints, chosenFive))
	if err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("legacy sparse choice = %v, %v", status, checkErr)
	}
	if decoded, decodeErr := sparse.Decode(encoder); decodeErr != nil || decoded != 5 {
		t.Fatalf("legacy sparse decode = %d, %v", decoded, decodeErr)
	}
}

func TestOracleFactoryScalaEncodings(t *testing.T) {
	factory := NewOracleFactory(nil)
	base := InitialRewriterScope()
	defaultScope, defaultOracle, defaultConstraints, err := factory.NewDefault(base, 3)
	if err != nil || defaultOracle.Size() != 3 || len(defaultConstraints) != 1 || defaultScope.Arena == base.Arena {
		t.Fatalf("default oracle = %#v, constraints %v, error %v", defaultOracle, defaultConstraints, err)
	}
	if _, ok := defaultOracle.(*UninterpretedConstOracle); !ok {
		t.Fatalf("default oracle type = %T", defaultOracle)
	}
	constScope, constOracle, constConstraints, err := factory.NewUninterpreted(defaultScope, 2)
	if err != nil || constOracle.Size() != 2 || len(constConstraints) != 1 || constScope.Arena == defaultScope.Arena {
		t.Fatalf("constant oracle = %#v, constraints %v, error %v", constOracle, constConstraints, err)
	}
	propScope, propOracle, propConstraints, err := factory.NewPropositional(base, 3)
	if err != nil || propOracle.Size() != 3 || len(propOracle.Bits) != 2 || len(propConstraints) != 1 || propScope.Arena == base.Arena {
		t.Fatalf("propositional oracle = %#v, constraints %v, error %v", propOracle, propConstraints, err)
	}
	intScope, intOracle, intConstraints, err := factory.NewInteger(base, 3)
	if err != nil || intOracle.Size() != 3 || len(intConstraints) != 0 || intScope.Arena == base.Arena {
		t.Fatalf("integer oracle = %#v, constraints %v, error %v", intOracle, intConstraints, err)
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
