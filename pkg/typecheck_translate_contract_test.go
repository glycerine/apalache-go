package apalache

import "testing"

func tlaBool(value bool) Expr { return NewValueExpr(BoolValue(value), Untyped()) }

func TestToEtcExprScalarSetTemporalAndSequenceContracts(t *testing.T) {
	intSet := func() Expr { return tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)) }
	boolSet := func() Expr { return tlaOper("SET_ENUM", tlaBool(true), tlaBool(false)) }
	intSeq := func() Expr { return tlaOper("TUPLE", tlaInt(1), tlaInt(2)) }
	tests := []struct {
		name       string
		expression func() Expr
		want       Type
	}{
		{"unary minus", func() Expr { return tlaOper("UNARY_MINUS", tlaInt(1)) }, IntT1},
		{"minus", func() Expr { return tlaOper("MINUS", tlaInt(3), tlaInt(1)) }, IntT1},
		{"multiply", func() Expr { return tlaOper("MULT", tlaInt(3), tlaInt(2)) }, IntT1},
		{"integer divide", func() Expr { return tlaOper("DIV", tlaInt(4), tlaInt(2)) }, IntT1},
		{"modulo", func() Expr { return tlaOper("MOD", tlaInt(4), tlaInt(3)) }, IntT1},
		{"power", func() Expr { return tlaOper("POW", tlaInt(2), tlaInt(3)) }, IntT1},
		{"less equal", func() Expr { return tlaOper("LE", tlaInt(1), tlaInt(2)) }, BoolT1},
		{"greater", func() Expr { return tlaOper("GT", tlaInt(2), tlaInt(1)) }, BoolT1},
		{"greater equal", func() Expr { return tlaOper("GE", tlaInt(2), tlaInt(2)) }, BoolT1},
		{"integer range", func() Expr { return tlaOper("INT_RANGE", tlaInt(1), tlaInt(3)) }, mustSet(IntT1)},
		{"equality", func() Expr { return tlaOper("EQ", tlaString("a"), tlaString("b")) }, BoolT1},
		{"inequality", func() Expr { return tlaOper("NE", tlaInt(1), tlaInt(2)) }, BoolT1},
		{"and", func() Expr { return tlaOper("AND", tlaBool(true), tlaBool(false)) }, BoolT1},
		{"or", func() Expr { return tlaOper("OR", tlaBool(true), tlaBool(false), tlaBool(true)) }, BoolT1},
		{"implies", func() Expr { return tlaOper("IMPLIES", tlaBool(true), tlaBool(false)) }, BoolT1},
		{"equivalence", func() Expr { return tlaOper("EQUIV", tlaBool(true), tlaBool(false)) }, BoolT1},
		{"not", func() Expr { return tlaOper("NOT", tlaBool(true)) }, BoolT1},
		{"set membership", func() Expr { return tlaOper("SET_IN", tlaInt(1), intSet()) }, BoolT1},
		{"set nonmembership", func() Expr { return tlaOper("SET_NOT_IN", tlaInt(3), intSet()) }, BoolT1},
		{"subset", func() Expr { return tlaOper("SET_SUBSET_EQ", intSet(), intSet()) }, BoolT1},
		{"powerset", func() Expr { return tlaOper("SET_POWERSET", intSet()) }, mustSet(mustSet(IntT1))},
		{"unary union", func() Expr { return tlaOper("SET_UNARY_UNION", tlaOper("SET_ENUM", intSet(), intSet())) }, mustSet(IntT1)},
		{"set union", func() Expr { return tlaOper("SET_UNION2", intSet(), intSet()) }, mustSet(IntT1)},
		{"set intersection", func() Expr { return tlaOper("SET_INTERSECT", intSet(), intSet()) }, mustSet(IntT1)},
		{"set difference", func() Expr { return tlaOper("SET_MINUS", intSet(), intSet()) }, mustSet(IntT1)},
		{"cartesian product", func() Expr { return tlaOper("SET_TIMES", intSet(), boolSet()) }, mustSet(mustTuple(IntT1, BoolT1))},
		{"sequence set", func() Expr { return tlaOper("Sequences!Seq", intSet()) }, mustSet(mustSeq(IntT1))},
		{"sequence to set", func() Expr { return tlaOper("SequencesExt!ToSet", intSeq()) }, mustSet(IntT1)},
		{"head", func() Expr { return tlaOper("Sequences!Head", intSeq()) }, IntT1},
		{"tail", func() Expr { return tlaOper("Sequences!Tail", intSeq()) }, mustSeq(IntT1)},
		{"append", func() Expr { return tlaOper("Sequences!Append", intSeq(), tlaInt(3)) }, mustSeq(IntT1)},
		{"concat", func() Expr { return tlaOper("Sequences!Concat", intSeq(), intSeq()) }, mustSeq(IntT1)},
		{"length", func() Expr { return tlaOper("Sequences!Len", intSeq()) }, IntT1},
		{"subsequence", func() Expr { return tlaOper("Sequences!SubSeq", intSeq(), tlaInt(1), tlaInt(2)) }, mustSeq(IntT1)},
		{"finite", func() Expr { return tlaOper("FiniteSets!IsFiniteSet", intSet()) }, BoolT1},
		{"cardinality", func() Expr { return tlaOper("FiniteSets!Cardinality", intSet()) }, IntT1},
		{"prime", func() Expr { return tlaOper("PRIME", tlaInt(1)) }, IntT1},
		{"enabled", func() Expr { return tlaOper("ENABLED", tlaBool(true)) }, BoolT1},
		{"globally", func() Expr { return tlaOper("GLOBALLY", tlaBool(true)) }, BoolT1},
		{"eventually", func() Expr { return tlaOper("EVENTUALLY", tlaBool(true)) }, BoolT1},
		{"composition", func() Expr { return tlaOper("COMPOSE", tlaBool(true), tlaBool(false)) }, BoolT1},
		{"leads to", func() Expr { return tlaOper("LEADS_TO", tlaBool(true), tlaBool(false)) }, BoolT1},
		{"guarantees", func() Expr { return tlaOper("GUARANTEES", tlaBool(true), tlaBool(false)) }, BoolT1},
		{"weak fairness", func() Expr { return tlaOper("WEAK_FAIRNESS", tlaInt(1), tlaBool(true)) }, BoolT1},
		{"strong fairness", func() Expr { return tlaOper("STRONG_FAIRNESS", tlaInt(1), tlaBool(true)) }, BoolT1},
		{"stutter", func() Expr { return tlaOper("STUTTER", tlaBool(true), tlaInt(1)) }, BoolT1},
		{"no stutter", func() Expr { return tlaOper("NO_STUTTER", tlaBool(true), tlaInt(1)) }, BoolT1},
		{"unchanged scalar", func() Expr { return tlaOper("UNCHANGED", tlaInt(1)) }, BoolT1},
		{"unchanged tuple", func() Expr { return tlaOper("UNCHANGED", tlaOper("TUPLE", tlaInt(1), tlaBool(true))) }, BoolT1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, listener, ok := inferIRExpression(t, test.expression())
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, listener.events)
			}
		})
	}
}

func TestToEtcExprConditionalAndApalacheContracts(t *testing.T) {
	intSet := func() Expr { return tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)) }
	intSeq := func() Expr { return tlaOper("TUPLE", tlaInt(1), tlaInt(2)) }
	tests := []struct {
		name       string
		expression func() Expr
		want       Type
	}{
		{"case", func() Expr { return tlaOper("CASE", tlaBool(true), tlaInt(1), tlaBool(false), tlaInt(2)) }, IntT1},
		{"case other", func() Expr { return tlaOper("CASE_OTHER", tlaInt(3), tlaBool(true), tlaInt(1)) }, IntT1},
		{"assignment", func() Expr { return tlaOper("Apalache!:=", tlaInt(1), tlaInt(2)) }, BoolT1},
		{"guess", func() Expr { return tlaOper("Apalache!Guess", intSet()) }, IntT1},
		{"expand", func() Expr { return tlaOper("Apalache!Expand", tlaInt(1)) }, IntT1},
		{"distinct", func() Expr { return tlaOper("Apalache!Distinct", tlaInt(1), tlaInt(2), tlaInt(3)) }, BoolT1},
		{"sequence capacity", func() Expr { return tlaOper("ApalacheInternal!__ApalacheSeqCapacity", intSeq()) }, IntT1},
		{"to string", func() Expr { return tlaOper("TLC!ToString", tlaInt(1)) }, StrT1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, listener, ok := inferIRExpression(t, test.expression())
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, listener.events)
			}
		})
	}
}

func TestToEtcExprBinderContracts(t *testing.T) {
	intSet := func() Expr { return tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)) }
	pairSet := func() Expr {
		return tlaOper("SET_ENUM",
			tlaOper("TUPLE", tlaInt(1), tlaBool(true)),
			tlaOper("TUPLE", tlaInt(2), tlaBool(false)))
	}
	tests := []struct {
		name       string
		expression func() Expr
		want       Type
	}{
		{
			"bounded choose", func() Expr {
				return tlaOper("CHOOSE3", tlaName("x"), intSet(), tlaOper("EQ", tlaName("x"), tlaInt(1)))
			}, IntT1,
		},
		{
			"unbounded choose", func() Expr {
				return tlaOper("CHOOSE2", tlaName("x"), tlaOper("EQ", tlaName("x"), tlaInt(1)))
			}, IntT1,
		},
		{
			"bounded exists", func() Expr {
				return tlaOper("EXISTS3", tlaName("x"), intSet(), tlaOper("GT", tlaName("x"), tlaInt(0)))
			}, BoolT1,
		},
		{
			"bounded forall", func() Expr {
				return tlaOper("FORALL3", tlaName("x"), intSet(), tlaOper("GT", tlaName("x"), tlaInt(0)))
			}, BoolT1,
		},
		{
			"unbounded exists", func() Expr {
				return tlaOper("EXISTS2", tlaName("x"), tlaOper("EQ", tlaName("x"), tlaInt(1)))
			}, BoolT1,
		},
		{
			"unbounded forall", func() Expr {
				return tlaOper("FORALL2", tlaName("x"), tlaOper("EQ", tlaName("x"), tlaInt(1)))
			}, BoolT1,
		},
		{
			"tuple exists", func() Expr {
				target := tlaOper("TUPLE", tlaName("x"), tlaName("y"))
				predicate := tlaOper("AND", tlaOper("EQ", tlaName("x"), tlaInt(1)), tlaOper("EQ", tlaName("y"), tlaBool(true)))
				return tlaOper("EXISTS3", target, pairSet(), predicate)
			}, BoolT1,
		},
		{
			"scalar filter", func() Expr {
				return tlaOper("SET_FILTER", tlaName("x"), intSet(), tlaOper("GT", tlaName("x"), tlaInt(0)))
			}, mustSet(IntT1),
		},
		{
			"tuple filter", func() Expr {
				target := tlaOper("TUPLE", tlaName("x"), tlaName("y"))
				predicate := tlaOper("AND", tlaOper("EQ", tlaName("x"), tlaInt(1)), tlaOper("EQ", tlaName("y"), tlaBool(true)))
				return tlaOper("SET_FILTER", target, pairSet(), predicate)
			}, mustSet(mustTuple(IntT1, BoolT1)),
		},
		{
			"scalar map", func() Expr {
				return tlaOper("SET_MAP", tlaOper("PLUS", tlaName("x"), tlaInt(1)), tlaName("x"), intSet())
			}, mustSet(IntT1),
		},
		{
			"tuple map", func() Expr {
				target := tlaOper("TUPLE", tlaName("x"), tlaName("y"))
				return tlaOper("SET_MAP", tlaName("x"), target, pairSet())
			}, mustSet(IntT1),
		},
		{
			"multi-domain map", func() Expr {
				return tlaOper("SET_MAP", tlaName("x"), tlaName("x"), intSet(), tlaName("y"), tlaOper("SET_ENUM", tlaBool(true)))
			}, mustSet(IntT1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, listener, ok := inferIRExpression(t, test.expression())
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, listener.events)
			}
		})
	}
}

func TestToEtcExprFunctionConstructorContracts(t *testing.T) {
	intSet := func() Expr { return tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)) }
	boolSet := func() Expr { return tlaOper("SET_ENUM", tlaBool(true), tlaBool(false)) }
	pairSet := func() Expr {
		return tlaOper("SET_ENUM",
			tlaOper("TUPLE", tlaInt(1), tlaBool(true)),
			tlaOper("TUPLE", tlaInt(2), tlaBool(false)))
	}
	tests := []struct {
		name       string
		expression func() Expr
		want       Type
	}{
		{
			"unary", func() Expr { return tlaOper("FUN_CTOR", tlaName("x"), tlaName("x"), intSet()) },
			mustFun(IntT1, IntT1),
		},
		{
			"binary", func() Expr {
				return tlaOper("FUN_CTOR", tlaName("x"), tlaName("x"), intSet(), tlaName("y"), boolSet())
			}, mustFun(mustTuple(IntT1, BoolT1), IntT1),
		},
		{
			"tuple binder", func() Expr {
				return tlaOper("FUN_CTOR", tlaName("x"), tlaOper("TUPLE", tlaName("x"), tlaName("y")), pairSet())
			}, mustFun(mustTuple(IntT1, BoolT1), IntT1),
		},
		{
			"recursive unary", func() Expr { return tlaOper("FUN_REC_CTOR", tlaName("x"), tlaName("x"), intSet()) },
			mustFun(IntT1, IntT1),
		},
		{
			"recursive binary", func() Expr {
				return tlaOper("FUN_REC_CTOR", tlaName("x"), tlaName("x"), intSet(), tlaName("y"), boolSet())
			}, mustFun(mustTuple(IntT1, BoolT1), IntT1),
		},
		{
			"recursive tuple binder", func() Expr {
				return tlaOper("FUN_REC_CTOR", tlaName("x"), tlaOper("TUPLE", tlaName("x"), tlaName("y")), pairSet())
			}, mustFun(mustTuple(IntT1, BoolT1), IntT1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, listener, ok := inferIRExpression(t, test.expression())
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, listener.events)
			}
		})
	}
}

func TestToEtcExprFunctionApplicationDomainAndExceptContracts(t *testing.T) {
	intSet := func() Expr { return tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)) }
	function := func() Expr { return tlaOper("FUN_CTOR", tlaName("x"), tlaName("x"), intSet()) }
	sequence := func() Expr { return tlaOper("TUPLE", tlaInt(1), tlaInt(2)).WithTag(Typed(mustSeq(IntT1))) }
	record := func() Expr { return tlaOper("RECORD", tlaString("foo"), tlaInt(1), tlaString("bar"), tlaBool(true)) }
	tests := []struct {
		name       string
		expression func() Expr
		want       Type
	}{
		{"function application", func() Expr { return tlaOper("FUN_APP", function(), tlaInt(2)) }, IntT1},
		{"sequence application", func() Expr { return tlaOper("FUN_APP", sequence(), tlaInt(2)) }, IntT1},
		{"record application", func() Expr { return tlaOper("FUN_APP", record(), tlaString("foo")) }, IntT1},
		{"function domain", func() Expr { return tlaOper("DOMAIN", function()) }, mustSet(IntT1)},
		{"sequence domain", func() Expr { return tlaOper("DOMAIN", sequence()) }, mustSet(IntT1)},
		{"record domain", func() Expr { return tlaOper("DOMAIN", record()) }, mustSet(StrT1)},
		{
			"function except", func() Expr {
				return tlaOper("EXCEPT", function(), tlaOper("TUPLE", tlaInt(1)), tlaInt(3))
			}, mustFun(IntT1, IntT1),
		},
		{
			"record except", func() Expr {
				return tlaOper("EXCEPT", record(), tlaOper("TUPLE", tlaString("foo")), tlaInt(3))
			}, parsedType(t, "[bar: Bool, foo: Int]"),
		},
		{
			"two function updates", func() Expr {
				return tlaOper("EXCEPT", function(),
					tlaOper("TUPLE", tlaInt(1)), tlaInt(3),
					tlaOper("TUPLE", tlaInt(2)), tlaInt(4))
			}, mustFun(IntT1, IntT1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, listener, ok := inferIRExpression(t, test.expression())
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, listener.events)
			}
		})
	}

	bad := tlaOper("EXCEPT", function(), tlaName("not-a-path"), tlaInt(3))
	if _, err := newTranslator().TranslateExpression(bad); err == nil || err.Error() != "EXCEPT accessor must be tuple" {
		t.Fatalf("malformed EXCEPT error = %v", err)
	}
}

func TestToEtcExprRecordAndVariantContracts(t *testing.T) {
	recordSet := tlaOper("RECORD_SET",
		tlaString("x"), tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)),
		tlaString("y"), tlaOper("SET_ENUM", tlaBool(true), tlaBool(false)))
	got, listener, ok := inferIRExpression(t, recordSet)
	if !ok || !EqualTypes(got, mustSet(parsedType(t, "[x: Int, y: Bool]"))) {
		t.Fatalf("record set inferred %v; events %+v", got, listener.events)
	}

	variant := func() Expr { return tlaOper("Variants!Variant", tlaString("A"), tlaInt(1)) }
	variantTests := []struct {
		name       string
		expression func() Expr
		want       Type
	}{
		{"tag", func() Expr { return tlaOper("Variants!VariantTag", variant()) }, StrT1},
		{"unsafe get", func() Expr { return tlaOper("Variants!VariantGetUnsafe", tlaString("A"), variant()) }, IntT1},
		{"get or else", func() Expr { return tlaOper("Variants!VariantGetOrElse", tlaString("A"), variant(), tlaInt(2)) }, IntT1},
		{"filter", func() Expr {
			return tlaOper("Variants!VariantFilter", tlaString("A"), tlaOper("SET_ENUM", variant(), variant()))
		}, mustSet(IntT1)},
	}
	for _, test := range variantTests {
		t.Run(test.name, func(t *testing.T) {
			got, listener, ok := inferIRExpression(t, test.expression())
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, listener.events)
			}
		})
	}

	invalidRecord := tlaOper("RECORD", tlaName("not-a-string"), tlaInt(1))
	if _, err := newTranslator().TranslateExpression(invalidRecord); err == nil || err.Error() != "record key is *apalache.NameExpr" {
		t.Fatalf("invalid record error = %v", err)
	}
	invalidVariant := tlaOper("Variants!Variant", tlaName("not-a-string"), tlaInt(1))
	if _, err := newTranslator().TranslateExpression(invalidVariant); err == nil || err.Error() != "the first argument of a variant operation must be a string" {
		t.Fatalf("invalid variant error = %v", err)
	}
}

func TestToEtcExprStringIndexRespectsRowsMode(t *testing.T) {
	record := tlaOper("RECORD", tlaString("foo"), tlaInt(1), tlaString("bar"), tlaBool(true))
	updated := tlaOper("EXCEPT", record, tlaOper("TUPLE", tlaString("foo")), tlaInt(3))
	translator := NewToEtcExpr(nil, nil, NewTypeVarPool(1000), true)
	translated, err := translator.TranslateExpression(updated)
	if err != nil {
		t.Fatal(err)
	}
	listener := &collectingTypeListener{}
	got, ok, err := NewEtcTypeChecker(translator.pool, true).Compute(listener, EmptyTypeContext(), translated)
	if err != nil || !ok || !EqualTypes(got, parsedType(t, "{ bar: Bool, foo: Int }")) {
		t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
	}
}
