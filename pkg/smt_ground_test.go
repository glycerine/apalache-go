package apalache

import (
	"context"
	"math/big"
	"testing"
)

func declareArenaForEncoder(t *testing.T, encoder *G3GroundEncoder, arena *PureArena) {
	t.Helper()
	for _, name := range []string{ArenaFalseName, ArenaTrueName, ArenaBooleanSet, ArenaNatSet, ArenaIntSet} {
		cell, err := arena.FindCellByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := encoder.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
}

func TestG3GroundEncoderBooleanArithmeticAndMembership(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		t.Fatal(err)
	}
	arena := InitialPureArena()
	declareArenaForEncoder(t, encoder, arena)
	if err := encoder.DeclareMembership(arena.BooleanSetCell(), arena.FalseCell()); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), arena.FalseCell().NameExpr())); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(arena.TrueCell().NameExpr()); err != nil {
		t.Fatal(err)
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), arena.FalseCell().NameExpr(), arena.BooleanSetCell().NameExpr())
	if err := encoder.Assert(membership); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("base ground encoding = %v, %v", status, err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), membership)); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("contradictory membership = %v, %v", status, err)
	}
}

func TestG3GroundEncoderBigIntegersAndUninterpretedDistinct(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, _ := NewG3GroundEncoder(solver)
	arena := EmptyPureArena()
	arena, integer := arena.AppendCellType(BasicCellType{Type: IntT1})
	typeA, _ := NewConstType("A")
	arena, values := arena.AppendCellTypes(BasicCellType{Type: typeA}, BasicCellType{Type: typeA})
	for _, cell := range append([]ArenaCell{integer}, values...) {
		if err := encoder.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
	huge := new(big.Int).Exp(big.NewInt(2), big.NewInt(100), nil)
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), integer.NameExpr(), NewValueExpr(NewIntValue(huge), Typed(IntT1)))
	if err := encoder.Assert(equality); err != nil {
		t.Fatal(err)
	}
	distinct := MustOperExpr(Op("Apalache!Distinct"), Typed(BoolT1), values[0].NameExpr(), values[1].NameExpr())
	if err := encoder.Assert(distinct); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("EQ"), Typed(BoolT1), values[0].NameExpr(), values[1].NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("uninterpreted distinct contradiction = %v, %v", status, err)
	}
	if len(encoder.DeclaredCellNames()) != arena.CellCount() {
		t.Fatalf("declared cell names = %v", encoder.DeclaredCellNames())
	}
}

func TestG3GroundEncoderRejectsMissingAndUnsupportedTerms(t *testing.T) {
	encoder, err := NewG3GroundEncoder(newTestG3Solver(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encoder.Encode(NewNameExpr("$C$99", Typed(IntT1))); err == nil {
		t.Fatal("undeclared arena cell was encoded")
	}
	if _, err := encoder.Encode(NewValueExpr(StringValue("x"), Typed(StrT1))); err == nil {
		t.Fatal("raw string literal was encoded")
	}
	cell := NewArenaCell(0, BasicCellType{Type: IntT1})
	set := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
	if err := encoder.DeclareCell(cell); err != nil {
		t.Fatal(err)
	}
	if err := encoder.DeclareCell(set); err != nil {
		t.Fatal(err)
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), cell.NameExpr(), set.NameExpr())
	if _, err := encoder.Encode(membership); err == nil {
		t.Fatal("undeclared membership predicate was encoded")
	}
	if err := encoder.DeclareCell(cell); err == nil {
		t.Fatal("duplicate cell declaration was accepted")
	}
}

func TestG3GroundEncoderUsesOOPSLAEdgesForFunctionMembership(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		t.Fatal(err)
	}
	element := NewArenaCell(0, BasicCellType{Type: IntT1})
	function := NewArenaCell(1, BasicCellType{Type: mustFunType(t, IntT1, BoolT1)})
	if err = encoder.DeclareCell(element); err != nil {
		t.Fatal(err)
	}
	if err = encoder.DeclareCell(function); err != nil {
		t.Fatal(err)
	}
	if err = encoder.DeclareMembership(function, element); err != nil {
		t.Fatal(err)
	}
	selected := MustOperExpr(Op("Apalache!SelectInFun"), Typed(BoolT1), element.NameExpr(), function.NameExpr())
	notStored := MustOperExpr(Op("Apalache!storeNotInFun"), Typed(BoolT1), element.NameExpr(), function.NameExpr())
	if err = encoder.Assert(selected); err != nil {
		t.Fatal(err)
	}
	if err = encoder.Assert(notStored); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("function edge contradiction = %v, %v", status, checkErr)
	}
}

func TestG3GroundEncoderArraySetDeclarationsAndSelection(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoderWithEncoding(solver, SMTEncodingArrays)
	if err != nil {
		t.Fatal(err)
	}
	element := NewArenaCell(0, BasicCellType{Type: IntT1})
	emptySet := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
	fullSet := NewArenaCell(2, InfiniteSetCellType{Element: BasicCellType{Type: IntT1}})
	for _, cell := range []ArenaCell{element, emptySet, fullSet} {
		if err = encoder.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
	selectFrom := func(set ArenaCell) Expr {
		return MustOperExpr(Op("Apalache!SelectInSet"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	}
	contradiction := MustOperExpr(Op("OR"), Typed(BoolT1),
		selectFrom(emptySet), MustOperExpr(Op("NOT"), Typed(BoolT1), selectFrom(fullSet)))
	if err = encoder.Assert(contradiction); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("constant set arrays = %v, %v", status, checkErr)
	}
}

func TestG3GroundEncoderArrayBooleanUniverse(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoderWithEncoding(solver, SMTEncodingArrays)
	if err != nil {
		t.Fatal(err)
	}
	arena := InitialPureArena()
	declareArenaForEncoder(t, encoder, arena)
	for _, element := range []ArenaCell{arena.FalseCell(), arena.TrueCell()} {
		membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), arena.BooleanSetCell().NameExpr())
		if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), membership)); err != nil {
			t.Fatal(err)
		}
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("Boolean universe arrays = %v, %v", status, checkErr)
	}
}

func TestEncodeRewriterScopeUsesNativeEmptySetArray(t *testing.T) {
	setType := mustSet(IntT1)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType))
	membership := MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), intLiteral(1), set)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(membership, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScopeWithEncoding(solver, scope, rewriter.Constraints(), SMTEncodingArrays)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoder.memberships) != 0 {
		t.Fatalf("full-array scope declared OOPSLA membership predicates: %v", encoder.memberships)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("native empty-set array = %v, %v", status, checkErr)
	}
	if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("contradictory native empty-set membership = %v, %v", status, checkErr)
	}
}

func TestEncodeRewriterScopePopulatesNativeSetArray(t *testing.T) {
	rewriter := NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{SMTEncoding: SMTEncodingArrays})
	setExpression := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1))
	scope, set, err := rewriter.Rewrite(setExpression, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	elements := scope.Arena.GetHas(set)
	if len(elements) != 1 {
		t.Fatalf("enumerated set pointers = %v", elements)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScopeWithEncoding(solver, scope, rewriter.Constraints(), SMTEncodingArrays)
	if err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("populated native set array = %v, %v", status, checkErr)
	}
	selected := MustOperExpr(Op("Apalache!SelectInSet"), Typed(BoolT1),
		elements[0].Element().NameExpr(), set.NameExpr())
	if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), selected)); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("stored native set member = %v, %v", status, checkErr)
	}
}

func TestEncodeRewriterScopeStoresDynamicSetMembership(t *testing.T) {
	rewriter := NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{SMTEncoding: SMTEncodingArrays})
	setType := mustSet(IntT1)
	binder := NewNameExpr("x", Typed(IntT1))
	domain := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2))
	predicate := MustOperExpr(Op("EQ"), Typed(BoolT1), binder, intLiteral(1))
	filter := MustOperExpr(Op("SET_FILTER"), Typed(setType), binder, domain, predicate)
	scope, result, err := rewriter.Rewrite(filter, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	pointers := scope.Arena.GetHas(result)
	if len(pointers) != 2 {
		t.Fatalf("filtered set pointers = %v", pointers)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScopeWithEncoding(solver, scope, rewriter.Constraints(), SMTEncodingArrays)
	if err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("dynamic native set array = %v, %v", status, checkErr)
	}
	selectAt := func(index int) Expr {
		return MustOperExpr(Op("Apalache!SelectInSet"), Typed(BoolT1),
			pointers[index].Element().NameExpr(), result.NameExpr())
	}
	badMembership := MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("NOT"), Typed(BoolT1), selectAt(0)), selectAt(1))
	if err = encoder.Assert(badMembership); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("dynamic stored memberships = %v, %v", status, checkErr)
	}
}

func TestG3GroundEncoderArraySSAStoresAndUnconstraining(t *testing.T) {
	t.Run("set store", func(t *testing.T) {
		solver := newTestG3Solver(t)
		encoder, err := NewG3GroundEncoderWithEncoding(solver, SMTEncodingArrays)
		if err != nil {
			t.Fatal(err)
		}
		element := NewArenaCell(0, BasicCellType{Type: IntT1})
		set := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
		for _, cell := range []ArenaCell{element, set} {
			if err = encoder.DeclareCell(cell); err != nil {
				t.Fatal(err)
			}
		}
		store := MustOperExpr(Op("Apalache!StoreInSet"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
		selectElement := MustOperExpr(Op("Apalache!SelectInSet"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
		if err = encoder.Assert(store); err != nil {
			t.Fatal(err)
		}
		if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), selectElement)); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
			t.Fatalf("stored set member = %v, %v", status, checkErr)
		}
	})

	t.Run("function store", func(t *testing.T) {
		solver := newTestG3Solver(t)
		encoder, err := NewG3GroundEncoderWithEncoding(solver, SMTEncodingFunArrays)
		if err != nil {
			t.Fatal(err)
		}
		argument := NewArenaCell(0, BasicCellType{Type: IntT1})
		function := NewArenaCell(1, BasicCellType{Type: mustFunType(t, IntT1, BoolT1)})
		for _, cell := range []ArenaCell{argument, function} {
			if err = encoder.DeclareCell(cell); err != nil {
				t.Fatal(err)
			}
		}
		store := MustOperExpr(Op("Apalache!StoreInSet"), Typed(BoolT1),
			boolExpr(true), function.NameExpr(), argument.NameExpr())
		application := MustOperExpr(Op("Apalache!SelectInFun"), Typed(BoolT1),
			argument.NameExpr(), function.NameExpr())
		if err = encoder.Assert(store); err != nil {
			t.Fatal(err)
		}
		if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), application)); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
			t.Fatalf("stored function result = %v, %v", status, checkErr)
		}
	})

	t.Run("unconstrained set version", func(t *testing.T) {
		solver := newTestG3Solver(t)
		encoder, err := NewG3GroundEncoderWithEncoding(solver, SMTEncodingArrays)
		if err != nil {
			t.Fatal(err)
		}
		element := NewArenaCell(0, BasicCellType{Type: IntT1})
		set := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
		for _, cell := range []ArenaCell{element, set} {
			if err = encoder.DeclareCell(cell); err != nil {
				t.Fatal(err)
			}
		}
		unconstrain := MustOperExpr(Op("Apalache!UnconstrainArray"), Typed(BoolT1), set.NameExpr())
		selectElement := MustOperExpr(Op("Apalache!SelectInSet"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
		if err = encoder.Assert(MustOperExpr(Op("AND"), Typed(BoolT1), unconstrain, selectElement)); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
			t.Fatalf("unconstrained set array = %v, %v", status, checkErr)
		}
	})
}
