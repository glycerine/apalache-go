package apalache

import (
	"strings"
	"testing"
)

func TestLazyEqualityCachesFiniteSetDefinition(t *testing.T) {
	arena := InitialPureArena()
	arena, elements := arena.AppendCellTypes(
		BasicCellType{Type: IntT1}, BasicCellType{Type: IntT1},
	)
	setType := BasicCellType{Type: mustSet(IntT1)}
	arena, sets := arena.AppendCellTypes(setType, setType)
	arena = arena.AppendHas(sets[0], FixedElementPointer{Cell: elements[0]})
	arena = arena.AppendHas(sets[1], FixedElementPointer{Cell: elements[1]})
	constraints := make([]Expr, 0)
	equality := NewLazyEquality(func(expression Expr) { constraints = append(constraints, expression) })

	first, err := equality.Equal(arena, sets[0], sets[1])
	if err != nil {
		t.Fatal(err)
	}
	second, err := equality.Equal(arena, sets[1], sets[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(constraints) != 1 {
		t.Fatalf("set equality definitions = %d", len(constraints))
	}
	firstArgs := first.(*OperExpr).Arguments()
	secondArgs := second.(*OperExpr).Arguments()
	if !EqualExpr(firstArgs[0], secondArgs[1]) || !EqualExpr(firstArgs[1], secondArgs[0]) {
		t.Fatalf("cached symmetric equality changed its cell pair: %#v and %#v", first, second)
	}
	definition := constraints[0].(*OperExpr)
	if definition.Operator().Name() != "EQUIV" {
		t.Fatalf("set equality definition = %#v", definition)
	}
}

func TestLazyEqualityBuildsGuardedSubsetConditions(t *testing.T) {
	arena := InitialPureArena()
	arena, element := arena.AppendCellType(BasicCellType{Type: BoolT1})
	setType := BasicCellType{Type: mustSet(BoolT1)}
	arena, sets := arena.AppendCellTypes(setType, setType)
	arena = arena.AppendHas(sets[0], SMTElementPointer{Cell: element, Expression: arena.FalseCell().NameExpr()})
	arena = arena.AppendHas(sets[1], FixedElementPointer{Cell: element})
	var definition Expr
	equality := NewLazyEquality(func(expression Expr) { definition = expression })
	if _, err := equality.Equal(arena, sets[0], sets[1]); err != nil {
		t.Fatal(err)
	}
	if definition == nil {
		t.Fatal("guarded set equality produced no definition")
	}
	formatted, err := FormatTLAExpression(definition)
	if err != nil || formatted == "" {
		t.Fatalf("guarded set equality definition could not be formatted: %q, %v", formatted, err)
	}
}

func TestLazyEqualitySafeCachedMagicAndBatchContracts(t *testing.T) {
	arena := InitialPureArena()
	arena, cells := arena.AppendCellTypes(
		BasicCellType{Type: IntT1}, BasicCellType{Type: IntT1}, BasicCellType{Type: BoolT1},
	)
	equality := NewLazyEquality(nil)
	if expression, err := equality.SafeEqual(cells[0], cells[0]); err != nil || !EqualExpr(expression, boolExpr(true)) {
		t.Fatalf("self equality = %#v, %v", expression, err)
	}
	if _, err := equality.CachedEqual(cells[0], cells[1]); err == nil || !strings.Contains(err.Error(), "VORSICHT") {
		t.Fatalf("uncached equality error = %v", err)
	}
	if err := equality.CacheEqualities(arena, []ArenaCellPair{{Left: cells[0], Right: cells[1]}}); err != nil {
		t.Fatal(err)
	}
	if expression, err := equality.SafeEqual(cells[1], cells[0]); err != nil || expression.(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("safe cached equality = %#v, %v", expression, err)
	}
	if err := equality.CacheAsSMTEqualityByMagic(cells[0], cells[2]); err != nil {
		t.Fatal(err)
	}
	if expression, err := equality.Equal(arena, cells[0], cells[2]); err != nil || expression.(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("magic unlike-type equality = %#v, %v", expression, err)
	}
	if err := equality.CacheOneEquality(arena, cells[0], cells[1]); err != nil {
		t.Fatal(err)
	}
}

func TestLazyEqualityUsesDirectEqualityForUnknownCells(t *testing.T) {
	arena := InitialPureArena()
	arena, cells := arena.AppendCellTypes(UnknownCellType{}, UnknownCellType{})
	equality := NewLazyEquality(nil)
	expression, err := equality.Equal(arena, cells[0], cells[1])
	if err != nil {
		t.Fatal(err)
	}
	operator, ok := expression.(*OperExpr)
	if !ok || operator.Operator().Name() != "EQ" {
		t.Fatalf("unknown-cell equality = %#v", expression)
	}
}

func TestLazyEqualityHonorsStaticCacheEntriesAndCountsNonStaticEntries(t *testing.T) {
	arena := InitialPureArena()
	arena, cells := arena.AppendCellTypes(
		BasicCellType{Type: IntT1}, BasicCellType{Type: IntT1}, BasicCellType{Type: IntT1},
	)
	equality := NewLazyEquality(nil)
	if err := equality.Cache().Put(cells[0], cells[1], EqualityFalse{}); err != nil {
		t.Fatal(err)
	}
	if expression, err := equality.CachedEqual(cells[1], cells[0]); err != nil || !EqualExpr(expression, boolExpr(false)) {
		t.Fatalf("cached false = %#v, %v", expression, err)
	}
	if err := equality.CacheAsSMTEqualityByMagic(cells[1], cells[2]); err != nil {
		t.Fatal(err)
	}
	if err := equality.Cache().Put(cells[0], cells[2], EqualityExpression{Expression: boolExpr(true)}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	constant, nonStatic, err := equality.CountConstantEqualities(func(expression Expr) (bool, error) {
		calls++
		value, ok := boolConstant(expression)
		return ok && value, nil
	})
	if err != nil || constant != 1 || nonStatic != 2 || calls != 2 {
		t.Fatalf("constant=%d nonStatic=%d calls=%d err=%v", constant, nonStatic, calls, err)
	}
}

func TestLazyEqualityFiniteFunctionSetsAndPublicSubset(t *testing.T) {
	arena := InitialPureArena()
	arena, elements := arena.AppendCellTypes(
		BasicCellType{Type: IntT1}, BasicCellType{Type: IntT1},
	)
	setType := BasicCellType{Type: mustSet(IntT1)}
	arena, sets := arena.AppendCellTypes(setType, setType, setType, setType)
	arena = arena.AppendHas(sets[0], FixedElementPointer{Cell: elements[0]})
	arena = arena.AppendHas(sets[1], FixedElementPointer{Cell: elements[0]})
	arena = arena.AppendHas(sets[2], FixedElementPointer{Cell: elements[1]})
	arena = arena.AppendHas(sets[3], FixedElementPointer{Cell: elements[1]})
	functionSetType, err := NewFiniteFunctionSetCellType(setType, setType)
	if err != nil {
		t.Fatal(err)
	}
	arena, functionSets := arena.AppendCellTypes(functionSetType, functionSetType)
	arena = arena.SetDomain(functionSets[0], sets[0]).SetCodomain(functionSets[0], sets[2])
	arena = arena.SetDomain(functionSets[1], sets[1]).SetCodomain(functionSets[1], sets[3])
	constraints := make([]Expr, 0)
	equality := NewLazyEquality(func(expression Expr) { constraints = append(constraints, expression) })
	if _, err = equality.SubsetEqual(arena, sets[0], sets[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = equality.Equal(arena, functionSets[0], functionSets[1]); err != nil {
		t.Fatal(err)
	}
	if len(constraints) != 3 {
		t.Fatalf("function-set structural definitions = %d, want domain, codomain, outer", len(constraints))
	}
}
