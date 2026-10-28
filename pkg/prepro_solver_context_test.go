package apalache

import (
	"context"
	"testing"
)

func TestPreproSolverContextCachesEqualityAndMembershipFacts(t *testing.T) {
	base, err := CreateSolverContext(func() SolverConfig { c := DefaultSolverConfig(); c.Debug = true; return c }())
	if err != nil {
		t.Fatal(err)
	}
	prepro, err := NewPreproSolverContext(base)
	if err != nil {
		t.Fatal(err)
	}
	defer prepro.Dispose()
	x := NewArenaCell(0, BasicCellType{Type: IntT1})
	y := NewArenaCell(1, BasicCellType{Type: IntT1})
	set := NewArenaCell(2, BasicCellType{Type: mustSet(IntT1)})
	for _, cell := range []ArenaCell{x, y, set} {
		if err = prepro.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
	if err = prepro.DeclareMembership(set, x); err != nil {
		t.Fatal(err)
	}
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), x.NameExpr(), y.NameExpr())
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), x.NameExpr(), set.NameExpr())
	if err = prepro.Assert(equality); err != nil {
		t.Fatal(err)
	}
	if err = prepro.Assert(membership); err != nil {
		t.Fatal(err)
	}
	if value, ok := boolConstant(prepro.Preprocess(MustOperExpr(Op("NE"), Typed(BoolT1), y.NameExpr(), x.NameExpr()))); !ok || value {
		t.Fatalf("cached inequality = %#v", prepro.Preprocess(MustOperExpr(Op("NE"), Typed(BoolT1), y.NameExpr(), x.NameExpr())))
	}
	if value, ok := boolConstant(prepro.Preprocess(MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), x.NameExpr(), set.NameExpr()))); !ok || value {
		t.Fatalf("cached nonmembership = %#v", prepro.Preprocess(MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), x.NameExpr(), set.NameExpr())))
	}
	if status, checkErr := prepro.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("preprocessed solver = %v, %v", status, checkErr)
	}
	if len(prepro.Logs()) != 2 {
		t.Fatalf("preprocessor logs = %#v", prepro.Logs())
	}
}

func TestPreproSolverContextCachesNegativeMembership(t *testing.T) {
	base, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	prepro, err := NewPreproSolverContext(base)
	if err != nil {
		t.Fatal(err)
	}
	defer prepro.Dispose()
	element := NewArenaCell(0, BasicCellType{Type: IntT1})
	set := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
	for _, cell := range []ArenaCell{element, set} {
		if err = prepro.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
	if err = prepro.DeclareMembership(set, element); err != nil {
		t.Fatal(err)
	}
	notin := MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	if err = prepro.Assert(notin); err != nil {
		t.Fatal(err)
	}
	in := MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	if value, ok := boolConstant(prepro.Preprocess(in)); !ok || value {
		t.Fatalf("cached membership = %#v", prepro.Preprocess(in))
	}
	if status, checkErr := prepro.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("negative-membership solver = %v, %v", status, checkErr)
	}
}

func TestPreproSolverContextUsesSMTShallowSimplification(t *testing.T) {
	if got, err := simplifyPreproShallow(NewNameExpr(ArenaFalseName, Typed(BoolT1))); err != nil {
		t.Fatal(err)
	} else if value, ok := boolConstant(got); !ok || value {
		t.Fatalf("false arena cell simplified to %#v", got)
	}

	nested := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(1)),
		intLiteral(2))
	got, err := simplifyPreproShallow(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualExpr(got, nested) {
		t.Fatalf("shallow simplifier recursed: %#v", got)
	}

	membership := MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1),
		intLiteral(1), MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1)))
	got, err = simplifyPreproShallow(membership)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualExpr(got, membership) {
		t.Fatalf("membership simplifier changed sort-bearing expression: %#v", got)
	}
}

func TestPreproSolverContextRollsBackFactsWithSolverStack(t *testing.T) {
	base, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	prepro, err := NewPreproSolverContext(base)
	if err != nil {
		t.Fatal(err)
	}
	defer prepro.Dispose()
	x := NewArenaCell(0, BasicCellType{Type: IntT1})
	y := NewArenaCell(1, BasicCellType{Type: IntT1})
	if err = prepro.DeclareCell(x); err != nil {
		t.Fatal(err)
	}
	if err = prepro.DeclareCell(y); err != nil {
		t.Fatal(err)
	}
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), x.NameExpr(), y.NameExpr())
	if err = prepro.Push(); err != nil {
		t.Fatal(err)
	}
	if err = prepro.Assert(equality); err != nil {
		t.Fatal(err)
	}
	if _, cached := boolConstant(prepro.Preprocess(equality)); !cached {
		t.Fatal("pushed equality was not cached")
	}
	if err = prepro.Pop(); err != nil {
		t.Fatal(err)
	}
	if _, cached := boolConstant(prepro.Preprocess(equality)); cached {
		t.Fatal("popped equality remained cached")
	}
}

func TestPreproSolverContextRestoresOverwrittenFactOnPop(t *testing.T) {
	base, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	prepro, err := NewPreproSolverContext(base)
	if err != nil {
		t.Fatal(err)
	}
	defer prepro.Dispose()
	x := NewArenaCell(0, BasicCellType{Type: IntT1})
	y := NewArenaCell(1, BasicCellType{Type: IntT1})
	for _, cell := range []ArenaCell{x, y} {
		if err = prepro.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), x.NameExpr(), y.NameExpr())
	inequality := MustOperExpr(Op("NE"), Typed(BoolT1), x.NameExpr(), y.NameExpr())
	if err = prepro.Assert(equality); err != nil {
		t.Fatal(err)
	}
	if err = prepro.Push(); err != nil {
		t.Fatal(err)
	}
	// Exercise the cache stack directly: normal assertion preprocessing turns
	// this contradictory fact into FALSE before it reaches the cache.
	prepro.cacheTopLevel(inequality)
	if value, ok := boolConstant(prepro.Preprocess(equality)); !ok || value {
		t.Fatalf("overwritten equality = %#v", prepro.Preprocess(equality))
	}
	if err = prepro.Pop(); err != nil {
		t.Fatal(err)
	}
	if value, ok := boolConstant(prepro.Preprocess(equality)); !ok || !value {
		t.Fatalf("restored equality = %#v", prepro.Preprocess(equality))
	}
}

func TestPreproSolverContextOnlyCachesArenaCellNames(t *testing.T) {
	base, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	prepro, err := NewPreproSolverContext(base)
	if err != nil {
		t.Fatal(err)
	}
	defer prepro.Dispose()
	expression := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("x", Typed(IntT1)), NewNameExpr("y", Typed(IntT1)))
	if err = prepro.Assert(expression); err == nil {
		t.Fatal("delegate unexpectedly accepted undeclared non-cell names")
	}
	if _, cached := boolConstant(prepro.Preprocess(expression)); cached {
		t.Fatal("non-cell equality was cached")
	}
}

func TestPreproSolverContextPreservesSortBearingMembershipOperands(t *testing.T) {
	base, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	prepro, err := NewPreproSolverContext(base)
	if err != nil {
		t.Fatal(err)
	}
	defer prepro.Dispose()
	complex := MustOperExpr(Op("SET_IN"), Typed(BoolT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(1)),
		NewNameExpr("S", Typed(mustSet(IntT1))))
	if got := prepro.Preprocess(complex); !EqualExpr(got, complex) {
		t.Fatalf("complex membership was recursively changed: %#v", got)
	}
}
