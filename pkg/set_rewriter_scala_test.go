package apalache

import (
	"context"
	"testing"
)

func TestStratifiedSetScalaRegressionOracleCases(t *testing.T) {
	set := func(elementType Type, elements ...Expr) Expr {
		return MustOperExpr(Op("SET_ENUM"), Typed(mustSet(elementType)), elements...)
	}
	intSet := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intLiteral(value)
		}
		return set(IntT1, elements...)
	}
	equal := func(left, right Expr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), left, right)
	}
	membership := func(element, domain Expr) Expr {
		return MustOperExpr(Op("SET_IN"), Typed(BoolT1), element, domain)
	}
	check := func(t *testing.T, expression Expr, want bool) {
		t.Helper()
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, result, want)
	}
	assume := func(t *testing.T, expression Expr) {
		t.Helper()
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		solver := newTestG3Solver(t)
		encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
		if err != nil {
			t.Fatal(err)
		}
		if err = encoder.Assert(result.NameExpr()); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
			t.Fatalf("assumed set formula = %v, %v", status, checkErr)
		}
	}
	skolem := func(binder *NameExpr, domain, predicate Expr) Expr {
		exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, domain, predicate)
		return MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists)
	}

	t.Run("deeply nested empty-set membership and equality", func(t *testing.T) {
		setInt := mustSet(IntT1)
		set2Int := mustSet(setInt)
		set3Int := mustSet(set2Int)
		empty1 := set(IntT1)
		empty2 := set(setInt)
		empty3 := set(set2Int)
		empty4 := set(set3Int)
		doubleEmpty1 := set(setInt, empty1, empty1)
		left := set(set2Int, empty2, doubleEmpty1)
		right := set(set3Int, empty3, set(set2Int, empty2, doubleEmpty1))
		check(t, membership(left, right), true)

		notLeft := set(set3Int, empty3, set(set2Int, set(setInt, empty1)))
		notRight := set(mustSet(set3Int), empty4,
			set(set3Int, empty3, set(set2Int, empty2)))
		check(t, membership(notLeft, notRight), false)
		check(t, equal(set(setInt, empty1), empty2), false)
	})

	t.Run("filtered powerset keeps precisely subsets without two", func(t *testing.T) {
		setInt := mustSet(IntT1)
		base := intSet(1, 2, 3)
		power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setInt)), base)
		expanded := MustOperExpr(Op("Apalache!Expand"), Typed(mustSet(setInt)), power)
		binder := NewNameExpr("Q", Typed(setInt))
		filtered := MustOperExpr(Op("SET_FILTER"), Typed(mustSet(setInt)), binder, expanded,
			MustOperExpr(Op("NOT"), Typed(BoolT1), membership(intLiteral(2), binder)))
		check(t, membership(intSet(1, 3), filtered), true)
		check(t, membership(intSet(1, 2), filtered), false)
	})

	t.Run("Skolem powerset filter with function application remains satisfiable", func(t *testing.T) {
		setInt := mustSet(IntT1)
		for _, test := range []struct {
			name      string
			base      Expr
			funDomain func(Expr) Expr
		}{
			{name: "matching symbolic domain", base: intSet(1), funDomain: func(x Expr) Expr { return x }},
			{name: "fixed singleton domain", base: intSet(1, 2), funDomain: func(Expr) Expr { return intSet(1) }},
		} {
			t.Run(test.name, func(t *testing.T) {
				xSet := NewNameExpr("X", Typed(setInt))
				x := NewNameExpr("x", Typed(IntT1))
				y := NewNameExpr("y", Typed(IntT1))
				functionType := mustFun(IntT1, BoolT1)
				constantTrue := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), boolExpr(true), y, test.funDomain(xSet))
				predicate := MustOperExpr(Op("FUN_APP"), Typed(BoolT1), constantTrue, x)
				filtered := MustOperExpr(Op("SET_FILTER"), Typed(setInt), x, xSet, predicate)
				power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setInt)), test.base)
				assume(t, skolem(xSet, power, equal(intSet(), filtered)))
			})
		}
	})

	t.Run("mapped integer division has Scala membership", func(t *testing.T) {
		x := NewNameExpr("x", Typed(IntT1))
		mapped := MustOperExpr(Op("SET_MAP"), Typed(mustSet(IntT1)),
			MustOperExpr(Op("DIV"), Typed(IntT1), x, intLiteral(3)), x, intSet(1, 2, 3, 4))
		check(t, membership(intLiteral(0), mapped), true)
		check(t, membership(intLiteral(2), mapped), false)
	})

	t.Run("multi-domain map retains tuple products and filtered binders", func(t *testing.T) {
		pairType := mustTuple(IntT1, BoolT1)
		x := NewNameExpr("x", Typed(IntT1))
		y := NewNameExpr("y", Typed(BoolT1))
		pair := MustOperExpr(Op("TUPLE"), Typed(pairType), x, y)
		mapped := MustOperExpr(Op("SET_MAP"), Typed(mustSet(pairType)), pair,
			x, intSet(1, 2, 3), y, set(BoolT1, boolExpr(false), boolExpr(true)))
		check(t, membership(MustOperExpr(Op("TUPLE"), Typed(pairType), intLiteral(2), boolExpr(true)), mapped), true)

		singleBoolTuple := mustTuple(BoolT1)
		z := NewNameExpr("z", Typed(IntT1))
		filtered := MustOperExpr(Op("SET_FILTER"), Typed(mustSet(IntT1)), z, intSet(1, 2), equal(z, intLiteral(1)))
		onlyY := MustOperExpr(Op("TUPLE"), Typed(singleBoolTuple), y)
		mappedFiltered := MustOperExpr(Op("SET_MAP"), Typed(mustSet(singleBoolTuple)), onlyY,
			x, filtered, y, set(BoolT1, boolExpr(false), boolExpr(true)))
		check(t, membership(MustOperExpr(Op("TUPLE"), Typed(singleBoolTuple), boolExpr(true)), mappedFiltered), true)
	})

	t.Run("record powerset map preserves shared projected values", func(t *testing.T) {
		recordType, err := NewRecordType(map[string]Type{"a": StrT1, "b": IntT1})
		if err != nil {
			t.Fatal(err)
		}
		recordSetType := mustSet(recordType)
		key := func(name string) Expr { return NewValueExpr(StringValue(name), Typed(StrT1)) }
		record := func(b int64) Expr {
			return MustOperExpr(Op("RECORD"), Typed(recordType), key("a"), key("a"), key("b"), intLiteral(b))
		}
		base := set(recordType, record(1), record(2))
		power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(recordSetType)), base)
		setBinder := NewNameExpr("S", Typed(recordSetType))
		r := NewNameExpr("r", Typed(recordType))
		mapped := MustOperExpr(Op("SET_MAP"), Typed(mustSet(StrT1)),
			MustOperExpr(Op("FUN_APP"), Typed(StrT1), r, key("a")), r, setBinder)
		projected := membership(key("a"), mapped)
		x := NewNameExpr("x", Typed(recordType))
		allSecond := MustOperExpr(Op("FORALL3"), Typed(BoolT1), x, setBinder,
			equal(MustOperExpr(Op("FUN_APP"), Typed(IntT1), x, key("b")), intLiteral(2)))
		assume(t, skolem(setBinder, power, booleanAnd(projected, allSecond)))
	})

	t.Run("union subset and unary union preserve structural values", func(t *testing.T) {
		union := MustOperExpr(Op("SET_UNION2"), Typed(mustSet(IntT1)), intSet(1, 3), intSet(3, 4))
		check(t, equal(union, intSet(1, 3, 4)), true)
		for _, test := range []struct {
			left Expr
			want bool
		}{
			{left: intSet(1, 2), want: true},
			{left: intSet(1, 2, 3), want: true},
			{left: intSet(), want: true},
			{left: intSet(1, 4), want: false},
		} {
			check(t, MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), test.left, intSet(1, 2, 3)), test.want)
		}
		outer := set(mustSet(IntT1), intSet(1, 2), intSet(2, 3))
		check(t, equal(MustOperExpr(Op("SET_UNARY_UNION"), Typed(mustSet(IntT1)), outer), intSet(1, 2, 3)), true)
	})
}
