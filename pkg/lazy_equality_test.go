package apalache

import "testing"

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
