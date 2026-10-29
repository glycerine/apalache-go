package apalache

import (
	"errors"
	"testing"
)

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
	if _, err := newTranslator().TranslateExpression(invalidVariant); err == nil ||
		err.Error() != "The first argument of a variant operation must be a string, found: not_a_string" {
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

func TestToEtcExprScalaRowsExactSignatures(t *testing.T) {
	fresh := func(number int) Type {
		variable, err := NewVarType(number)
		if err != nil {
			t.Fatal(err)
		}
		return variable
	}
	rowType := func(fields map[string]Type, tailNumber int) Type {
		var tail *VarType
		if tailNumber >= 0 {
			variable, err := NewVarType(tailNumber)
			if err != nil {
				t.Fatal(err)
			}
			tail = &variable
		}
		row, err := NewRowType(fields, tail)
		if err != nil {
			t.Fatal(err)
		}
		return NewRecordRowType(row)
	}
	translate := func(expression Expr) EtcExpr {
		translated, err := NewToEtcExpr(nil, nil, NewTypeVarPool(1000), true).TranslateExpression(expression)
		if err != nil {
			t.Fatal(err)
		}
		return translated
	}

	recordSet := tlaOper("RECORD_SET", tlaString("x"), tlaName("S"), tlaString("y"), tlaName("T"))
	got := translate(recordSet)
	want := NewUniqueEtcApp([]Type{mustOperator(
		[]Type{mustSet(fresh(1000)), mustSet(fresh(1001))},
		mustSet(rowType(map[string]Type{"x": fresh(1000), "y": fresh(1001)}, -1)))},
		NewUniqueEtcName("S"), NewUniqueEtcName("T"))
	if !EqualEtcExpr(got, want) {
		t.Fatalf("row record set = %v, want %v", got, want)
	}

	record := tlaOper("RECORD", tlaString("f1"), tlaBool(true), tlaString("f2"), tlaInt(1))
	got = translate(record)
	want = NewUniqueEtcApp([]Type{mustOperator(
		[]Type{fresh(1000), fresh(1001)}, rowType(map[string]Type{"f1": fresh(1000), "f2": fresh(1001)}, -1))},
		NewUniqueEtcConst(BoolT1), NewUniqueEtcConst(IntT1))
	if !EqualEtcExpr(got, want) {
		t.Fatalf("row record = %v, want %v", got, want)
	}

	access := tlaOper("FUN_APP", tlaName("f"), tlaString("foo"))
	got = translate(access)
	want = NewUniqueEtcApp([]Type{
		mustOperator([]Type{mustFun(StrT1, fresh(1000)), StrT1}, fresh(1000)),
		mustOperator([]Type{rowType(map[string]Type{"foo": fresh(1000)}, 1001), StrT1}, fresh(1000)),
	}, NewUniqueEtcName("f"), NewUniqueEtcConst(StrT1))
	if !EqualEtcExpr(got, want) {
		t.Fatalf("row field access = %v, want %v", got, want)
	}
	if _, explained := got.Explain(nil, nil); !explained {
		t.Fatal("row field access lost its custom type error explanation")
	}

	got = translate(tlaOper("DOMAIN", tlaName("f")))
	emptySparse, _ := NewSparseTupleType(nil)
	want = NewUniqueEtcApp([]Type{
		mustOperator([]Type{mustFun(fresh(1000), fresh(1001))}, mustSet(fresh(1000))),
		mustOperator([]Type{mustSeq(fresh(1002))}, mustSet(IntT1)),
		mustOperator([]Type{rowType(nil, 1003)}, mustSet(StrT1)),
		mustOperator([]Type{emptySparse}, mustSet(IntT1)),
	}, NewUniqueEtcName("f"))
	if !EqualEtcExpr(got, want) {
		t.Fatalf("row DOMAIN = %v, want %v", got, want)
	}

	except := tlaOper("EXCEPT", tlaName("f"), tlaOper("TUPLE", tlaString("foo")), tlaName("e2"))
	got = translate(except)
	update := NewUniqueEtcApp([]Type{
		mustOperator([]Type{StrT1, fresh(1000)}, mustFun(StrT1, fresh(1000))),
		mustOperator([]Type{StrT1, fresh(1000)}, rowType(map[string]Type{"foo": fresh(1000)}, 1001)),
	}, NewUniqueEtcConst(StrT1), NewUniqueEtcName("e2"))
	want = NewUniqueEtcApp([]Type{mustOperator([]Type{fresh(1002), fresh(1002)}, fresh(1002))},
		NewUniqueEtcName("f"), update)
	if !EqualEtcExpr(got, want) {
		t.Fatalf("row EXCEPT = %v, want %v", got, want)
	}

	invalid := tlaOper("RECORD_SET", tlaName("invalidName"), tlaString("x"))
	if _, err := NewToEtcExpr(nil, nil, NewTypeVarPool(1000), true).TranslateExpression(invalid); err == nil ||
		err.Error() != "record key is *apalache.NameExpr" {
		t.Fatalf("invalid row record-set key error = %v", err)
	}
}

func TestToEtcExprDeclarationAnnotationContracts(t *testing.T) {
	tests := []struct {
		name        string
		declaration func() Decl
		annotation  Type
		want        Type
	}{
		{"constant", func() Decl { return NewConstDecl("C", Untyped()) }, IntT1, IntT1},
		{"variable", func() Decl { return NewVarDecl("x", Untyped()) }, BoolT1, BoolT1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			declaration := test.declaration()
			translator := NewToEtcExpr(map[UID]Type{declaration.ID(): test.annotation}, nil, NewTypeVarPool(1000), false)
			scoped, err := translator.TranslateExpression(tlaName(declaration.Name()))
			if err != nil {
				t.Fatal(err)
			}
			translated, err := translator.TranslateDeclaration(declaration, scoped)
			if err != nil {
				t.Fatal(err)
			}
			got, ok, err := NewEtcTypeChecker(translator.pool, true).Compute(&collectingTypeListener{}, EmptyTypeContext(), translated)
			if err != nil || !ok || !EqualTypes(got, test.want) {
				t.Fatalf("Compute() = %v, %v, %v", got, ok, err)
			}
		})
	}

	for _, declaration := range []Decl{NewConstDecl("C", Untyped()), NewVarDecl("x", Untyped())} {
		_, err := newTranslator().TranslateDeclaration(declaration, NewUniqueEtcConst(BoolT1))
		var inputErr *TypingInputException
		if !errors.As(err, &inputErr) || inputErr.CauseExprID() != declaration.ID() ||
			err.Error() != "Expected a type annotation for "+map[bool]string{true: "CONSTANT", false: "VARIABLE"}[declaration.Name() == "C"]+" "+declaration.Name() {
			t.Fatalf("missing annotation error for %T = %v", declaration, err)
		}
	}
}

func TestToEtcExprScalaDeclarationShapes(t *testing.T) {
	terminal := EtcExpr(NewUniqueEtcConst(BoolT1))
	positiveType := mustOperator([]Type{IntT1}, BoolT1)
	positive := NewOperDecl("Positive", []OperatorParam{ValueParam("x")},
		MustOperExpr(Op("LT"), Untyped(), NewNameExpr("x", Untyped()), tlaInt(0)), Untyped())
	translator := NewToEtcExpr(map[UID]Type{positive.ID(): positiveType}, nil, NewTypeVarPool(1000), false)
	translated, err := translator.TranslateDeclaration(positive, terminal)
	if err != nil {
		t.Fatal(err)
	}
	typeDeclaration, ok := translated.(*EtcTypeDecl)
	if !ok || typeDeclaration.Name() != "Positive" || !EqualTypes(typeDeclaration.DeclaredType(), positiveType) {
		t.Fatalf("annotated operator declaration = %v", translated)
	}
	let, ok := typeDeclaration.Scoped().(*EtcLet)
	if !ok || let.Name() != "Positive" || !EqualEtcExpr(let.Body(), terminal) {
		t.Fatalf("annotated operator LET = %v", typeDeclaration.Scoped())
	}
	abstraction, ok := let.Bound().(*EtcAbs)
	if !ok || len(abstraction.Bindings()) != 1 || abstraction.Bindings()[0].Name.Name() != "x" {
		t.Fatalf("annotated operator abstraction = %v", let.Bound())
	}
	variableType, _ := NewVarType(1000)
	if domain, ok := abstraction.Bindings()[0].Domain.(*EtcConst); !ok || !EqualTypes(domain.Polytype(), mustSet(variableType)) {
		t.Fatalf("annotated operator parameter domain = %v", abstraction.Bindings()[0].Domain)
	}
	application, ok := abstraction.Body().(*EtcApp)
	if !ok || len(application.OperatorTypes()) != 1 ||
		!EqualTypes(application.OperatorTypes()[0], mustOperator([]Type{IntT1, IntT1}, BoolT1)) || len(application.Arguments()) != 2 {
		t.Fatalf("annotated operator body = %v", abstraction.Body())
	}
	if application.Arguments()[0].(*EtcName).Name() != "x" ||
		!EqualTypes(application.Arguments()[1].(*EtcConst).Polytype(), IntT1) {
		t.Fatalf("annotated operator arguments = %v", application.Arguments())
	}

	entry := NewVarDecl("entry", Untyped())
	set := NewVarDecl("set", Untyped())
	entryAlias, _ := NewConstType("ENTRY")
	aliases := NewTypeAliasSubstitution(map[string]Type{"ENTRY": IntT1})
	translator = NewToEtcExpr(map[UID]Type{
		entry.ID(): entryAlias,
		set.ID():   mustSet(entryAlias),
	}, aliases, NewTypeVarPool(1000), false)
	translated, err = translator.TranslateDeclaration(set, terminal)
	if err == nil {
		translated, err = translator.TranslateDeclaration(entry, translated)
	}
	wantAliases := NewUniqueEtcTypeDecl("entry", IntT1,
		NewUniqueEtcTypeDecl("set", mustSet(IntT1), terminal))
	if err != nil || !EqualEtcExpr(translated, wantAliases) {
		t.Fatalf("aliased declarations = %v, want %v, error = %v", translated, wantAliases, err)
	}

	assumption := NewUnnamedAssumeDecl(NewNameExpr("x", Untyped()), Untyped())
	translated, err = newTranslator().TranslateDeclaration(assumption, terminal)
	if err != nil {
		t.Fatal(err)
	}
	assumeLet, ok := translated.(*EtcLet)
	if !ok || assumeLet.Name() != assumption.Name() || !EqualEtcExpr(assumeLet.Body(), terminal) {
		t.Fatalf("assumption LET = %v", translated)
	}
	assumeAbs, ok := assumeLet.Bound().(*EtcAbs)
	if !ok || len(assumeAbs.Bindings()) != 0 {
		t.Fatalf("assumption abstraction = %v", assumeLet.Bound())
	}
	assumeApp, ok := assumeAbs.Body().(*EtcApp)
	if !ok || len(assumeApp.OperatorTypes()) != 1 ||
		!EqualTypes(assumeApp.OperatorTypes()[0], mustOperator([]Type{BoolT1}, BoolT1)) ||
		len(assumeApp.Arguments()) != 1 || assumeApp.Arguments()[0].(*EtcName).Name() != "x" {
		t.Fatalf("assumption Boolean wrapper = %v", assumeAbs.Body())
	}
}

func TestToEtcExprTagPrecedenceAliasesAndFreshAnnotationVariables(t *testing.T) {
	declaration := NewConstDecl("C", Typed(BoolT1))
	translator := NewToEtcExpr(map[UID]Type{declaration.ID(): IntT1}, nil, NewTypeVarPool(1000), false)
	translated, err := translator.TranslateDeclaration(declaration, NewUniqueEtcName("C"))
	if err != nil {
		t.Fatal(err)
	}
	typeDeclaration := translated.(*EtcTypeDecl)
	if !EqualTypes(typeDeclaration.DeclaredType(), BoolT1) {
		t.Fatalf("typed tag did not take precedence: %s", typeDeclaration.DeclaredType())
	}

	alias, _ := NewConstType("A")
	aliased := NewConstDecl("Aliased", Untyped())
	translator = NewToEtcExpr(map[UID]Type{aliased.ID(): alias}, NewTypeAliasSubstitution(map[string]Type{"A": IntT1}), NewTypeVarPool(1000), false)
	translated, err = translator.TranslateDeclaration(aliased, NewUniqueEtcConst(BoolT1))
	if err != nil || !EqualTypes(translated.(*EtcTypeDecl).DeclaredType(), IntT1) {
		t.Fatalf("alias translation = %v, %v", translated, err)
	}

	variable, _ := NewVarType(0)
	polymorphic := NewConstDecl("P", Untyped())
	translator = NewToEtcExpr(map[UID]Type{polymorphic.ID(): variable}, nil, NewTypeVarPool(1000), false)
	translated, err = translator.TranslateDeclaration(polymorphic, NewUniqueEtcConst(BoolT1))
	if err != nil {
		t.Fatal(err)
	}
	declared := translated.(*EtcTypeDecl).DeclaredType()
	if names := declared.UsedNames(); len(names) != 1 || names[0] < 1000 {
		t.Fatalf("annotation variables were not freshened: %s (%v)", declared, names)
	}
}

func TestToEtcExprOperatorAssumptionAndTheoremDeclarations(t *testing.T) {
	higherOrder := NewOperDecl("Foo", []OperatorParam{{Name: "Bar", Arity: 1}}, tlaInt(1), Untyped())
	assumption := NewNamedAssumeDecl("A", tlaBool(true), Untyped())
	theorem := NewTheoremDecl("T", tlaBool(true), Untyped())
	translator := newTranslator()
	scoped := EtcExpr(NewUniqueEtcConst(BoolT1))
	var err error
	for _, declaration := range []Decl{theorem, assumption, higherOrder} {
		scoped, err = translator.TranslateDeclaration(declaration, scoped)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := NewEtcTypeChecker(translator.pool, true).Compute(&collectingTypeListener{}, EmptyTypeContext(), scoped)
	if err != nil || !ok || !EqualTypes(got, BoolT1) {
		t.Fatalf("Compute() = %v, %v, %v", got, ok, err)
	}

	bad := NewUnnamedAssumeDecl(tlaInt(1), Untyped())
	translated, err := newTranslator().TranslateDeclaration(bad, NewUniqueEtcConst(BoolT1))
	if err != nil {
		t.Fatal(err)
	}
	listener := &collectingTypeListener{}
	if _, ok, err = newEtcChecker().Compute(listener, EmptyTypeContext(), translated); err != nil || ok {
		t.Fatalf("bad assumption Compute() = %v, %v", ok, err)
	}
	if listener.count("error") == 0 {
		t.Fatal("non-Boolean assumption produced no type error")
	}
}

func TestToEtcExprMalformedAndOutdatedBoundaries(t *testing.T) {
	translator := newTranslator()
	if _, err := translator.TranslateExpression(nil); err == nil {
		t.Fatal("nil expression accepted")
	}
	if _, err := translator.TranslateDeclaration(nil, NewUniqueEtcConst(BoolT1)); err == nil {
		t.Fatal("nil declaration accepted")
	}
	if _, err := translator.TranslateDeclaration(NewConstDecl("C", Typed(IntT1)), nil); err == nil {
		t.Fatal("nil scoped expression accepted")
	}

	old := tlaOper("Apalache!<:", tlaName("e"), tlaOper("SET_ENUM", tlaOper("SET_ENUM", tlaInt(1))))
	_, err := translator.TranslateExpression(old)
	var outdated *OutdatedAnnotationsError
	if !errors.As(err, &outdated) || outdated.CauseExpr() != old ||
		err.Error() != "Old Apalache type annotations (predating 0.12.0) are no longer supported" {
		t.Fatalf("outdated annotation error = %T %v", err, err)
	}
}

func TestToEtcExprRemainingScalaOperatorContracts(t *testing.T) {
	realValue := func() Expr { return NewValueExpr(Real(), Untyped()) }
	intSeq := func() Expr { return tlaOper("TUPLE", tlaInt(1), tlaInt(2)).WithTag(Typed(mustSeq(IntT1))) }
	setAsFunInput := func() Expr {
		return tlaOper("SET_ENUM",
			tlaOper("TUPLE", tlaInt(1), tlaBool(true)),
			tlaOper("TUPLE", tlaInt(2), tlaBool(false)))
	}
	tests := []struct {
		name       string
		expression func() Expr
		want       Type
	}{
		{"real division", func() Expr { return tlaOper("REAL_DIV", realValue(), realValue()) }, RealT1},
		{"label", func() Expr { return tlaOper("LABEL", tlaInt(1), tlaString("lab"), tlaString("arg")) }, IntT1},
		{"skolem", func() Expr { return tlaOper("Apalache!Skolem", tlaBool(true)) }, BoolT1},
		{"constant cardinality", func() Expr { return tlaOper("Apalache!ConstCardinality", tlaBool(true)) }, BoolT1},
		{"set as function", func() Expr { return tlaOper("Apalache!SetAsFun", setAsFunInput()) }, mustFun(IntT1, BoolT1)},
		{"sequence capacity", func() Expr { return tlaOper("ApalacheInternal!__ApalacheSeqCapacity", intSeq()) }, IntT1},
		{"generated value constrained by old tag", func() Expr {
			return tlaOper("Apalache!Gen", tlaInt(1)).WithTag(Typed(IntT1))
		}, IntT1},
		{"unsupported marker constrained by old tag", func() Expr {
			return tlaOper("ApalacheInternal!__NotSupportedByModelChecker", tlaString("why")).WithTag(Typed(BoolT1))
		}, BoolT1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, listener, ok := inferIRExpression(t, test.expression())
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, listener.events)
			}
		})
	}

	constructor := NewOperDecl("Ctor", []OperatorParam{ValueParam("i")}, tlaName("i"), Untyped())
	mkSeq := tlaOper("Apalache!MkSeq", tlaInt(2), tlaName("Ctor"))
	let := NewLetInExpr(mkSeq, Untyped(), constructor)
	got, listener, ok := inferIRExpression(t, let)
	if !ok || !EqualTypes(got, mustSeq(IntT1)) {
		t.Fatalf("MkSeq inferred %v; events %+v", got, listener.events)
	}

	modelSet := tlaOper("SET_ENUM", tlaString("1_OF_A"), tlaString("2_OF_A"))
	modelFunction := tlaOper("FUN_CTOR", tlaBool(true), tlaName("x"), modelSet)
	modelApplication := tlaOper("FUN_APP", modelFunction, tlaString("1_OF_A"))
	got, listener, ok = inferIRExpression(t, modelApplication)
	if !ok || !EqualTypes(got, BoolT1) {
		t.Fatalf("model-value application inferred %v; events %+v", got, listener.events)
	}
}

func TestToEtcExprPreservesCustomExplanationContracts(t *testing.T) {
	equality := tlaOper("EQ", tlaInt(1), tlaBool(true))
	translated, err := newTranslator().TranslateExpression(equality)
	if err != nil {
		t.Fatal(err)
	}
	application := translated.(*EtcApp)
	message, found := application.Explain(nil, []Type{IntT1, BoolT1})
	if !found || message != "Arguments to = should have the same type. For arguments 1, TRUE with types Int, Bool, in expression 1 = TRUE" {
		t.Fatalf("equality explanation = %q, %v", message, found)
	}
	emptyRecord, err := NewRecordType(map[string]Type{})
	if err != nil {
		t.Fatal(err)
	}
	booleanRecord, err := NewRecordType(map[string]Type{"foo": BoolT1})
	if err != nil {
		t.Fatal(err)
	}
	record := tlaOper("RECORD", tlaString("foo"), tlaBool(true))
	equality = tlaOper("EQ", tlaName("m"), record)
	translated, err = newTranslator().TranslateExpression(equality)
	if err != nil {
		t.Fatal(err)
	}
	application = translated.(*EtcApp)
	message, found = application.Explain(nil, []Type{emptyRecord, booleanRecord})
	wantRecord := `Arguments to = should have the same type. For arguments m, ["foo" ↦ TRUE] with types {  }, { foo: Bool }, in expression m = (["foo" ↦ TRUE])`
	if !found || message != wantRecord {
		t.Fatalf("record equality explanation = %q, %v", message, found)
	}

	call := tlaOper("OPER_APP", tlaName("F"), tlaInt(1))
	translated, err = newTranslator().TranslateExpression(call)
	if err != nil {
		t.Fatal(err)
	}
	byName := translated.(*EtcAppByName)
	message, found = byName.Explain([]Type{parsedType(t, "Bool => Bool")}, []Type{IntT1})
	want := "The operator F of type ((Bool) => Bool) is applied to arguments of incompatible types in F(1):\nArgument 1 should have type Bool but has type Int."
	if !found || message != want {
		t.Fatalf("operator-application explanation = %q, %v", message, found)
	}
}

func TestToEtcExprScalaEmptyRecursiveAndLetShapes(t *testing.T) {
	varType := func(number int) Type {
		variable, err := NewVarType(number)
		if err != nil {
			t.Fatal(err)
		}
		return variable
	}
	translate := func(expression Expr, annotations map[UID]Type) EtcExpr {
		translated, err := NewToEtcExpr(annotations, nil, NewTypeVarPool(1000), false).TranslateExpression(expression)
		if err != nil {
			t.Fatal(err)
		}
		return translated
	}

	if got, want := translate(tlaOper("SET_ENUM"), nil), EtcExpr(NewUniqueEtcConst(mustSet(varType(1000)))); !EqualEtcExpr(got, want) {
		t.Fatalf("empty set = %v, want %v", got, want)
	}
	if got, want := translate(tlaOper("TUPLE"), nil), EtcExpr(NewUniqueEtcConst(mustSeq(varType(1000)))); !EqualEtcExpr(got, want) {
		t.Fatalf("empty tuple = %v, want %v", got, want)
	}
	if got := translate(tlaOper("FUN_REC_REF"), nil); !EqualEtcExpr(got, NewUniqueEtcName("$RecursiveFunction")) {
		t.Fatalf("recursive function reference = %v", got)
	}

	for _, test := range []struct {
		name        string
		declaration func() *OperDecl
		annotation  func(*OperDecl) map[UID]Type
		want        func(*OperDecl) EtcExpr
	}{
		{
			"simple",
			func() *OperDecl {
				return NewOperDecl("Foo", []OperatorParam{ValueParam("x")}, tlaName("x"), Untyped())
			},
			func(*OperDecl) map[UID]Type { return nil },
			func(*OperDecl) EtcExpr {
				abstraction := NewUniqueEtcAbs(NewUniqueEtcName("x"), EtcBinding{
					Name: NewUniqueEtcName("x"), Domain: NewUniqueEtcConst(mustSet(varType(1000))),
				})
				let := NewUniqueEtcLet("Foo", abstraction, NewUniqueEtcConst(BoolT1))
				return NewUniqueEtcApp([]Type{mustOperator([]Type{varType(1001)}, varType(1001))}, let)
			},
		},
		{
			"annotated",
			func() *OperDecl {
				return NewOperDecl("Foo", []OperatorParam{ValueParam("x")}, tlaName("x"), Untyped())
			},
			func(declaration *OperDecl) map[UID]Type {
				return map[UID]Type{declaration.ID(): mustOperator([]Type{IntT1}, IntT1)}
			},
			func(*OperDecl) EtcExpr {
				abstraction := NewUniqueEtcAbs(NewUniqueEtcName("x"), EtcBinding{
					Name: NewUniqueEtcName("x"), Domain: NewUniqueEtcConst(mustSet(varType(1000))),
				})
				let := NewUniqueEtcLet("Foo", abstraction, NewUniqueEtcConst(BoolT1))
				declared := NewUniqueEtcTypeDecl("Foo", mustOperator([]Type{IntT1}, IntT1), let)
				return NewUniqueEtcApp([]Type{mustOperator([]Type{varType(1001)}, varType(1001))}, declared)
			},
		},
		{
			"higher order",
			func() *OperDecl {
				return NewOperDecl("Foo", []OperatorParam{{Name: "Bar", Arity: 1}}, tlaInt(1), Untyped())
			},
			func(*OperDecl) map[UID]Type { return nil },
			func(*OperDecl) EtcExpr {
				parameterType := mustOperator([]Type{varType(1000)}, varType(1001))
				abstraction := NewUniqueEtcAbs(NewUniqueEtcConst(IntT1), EtcBinding{
					Name: NewUniqueEtcName("Bar"), Domain: NewUniqueEtcConst(mustSet(parameterType)),
				})
				let := NewUniqueEtcLet("Foo", abstraction, NewUniqueEtcConst(BoolT1))
				return NewUniqueEtcApp([]Type{mustOperator([]Type{varType(1002)}, varType(1002))}, let)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			declaration := test.declaration()
			input := NewLetInExpr(tlaBool(true), Untyped(), declaration)
			got := translate(input, test.annotation(declaration))
			want := test.want(declaration)
			if !EqualEtcExpr(got, want) {
				t.Fatalf("LET translation = %v, want %v", got, want)
			}
		})
	}
}
