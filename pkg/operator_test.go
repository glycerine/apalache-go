package apalache

import (
	"strconv"
	"strings"
	"testing"
)

func TestArityCombinatorsMatchScala(t *testing.T) {
	tests := []struct {
		name   string
		arity  Arity
		accept []int
		reject []int
	}{
		{"fixed", FixedArity(2), []int{2}, []int{-1, 0, 1, 3}},
		{"minimum", MinimalArity(2), []int{2, 3, 10}, []int{-1, 0, 1}},
		{"any", AnyArity(), []int{0, 1, 10}, []int{-1}},
		{"odd", AnyOddArity(), []int{1, 3}, []int{-1, 0, 2}},
		{"even", AnyEvenArity(), []int{0, 2}, []int{-2, -1, 1}},
		{"positive", AnyPositiveArity(), []int{1, 2}, []int{-1, 0}},
		{"and", AnyOddArity().And(MinimalArity(3)), []int{3, 5}, []int{-1, 0, 1, 2, 4}},
		{"or", FixedArity(2).Or(FixedArity(3)), []int{2, 3}, []int{-1, 0, 1, 4}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, count := range test.accept {
				if !test.arity.Accepts(count) {
					t.Errorf("Accepts(%d) = false", count)
				}
			}
			for _, count := range test.reject {
				if test.arity.Accepts(count) {
					t.Errorf("Accepts(%d) = true", count)
				}
			}
		})
	}
}

func TestOperatorRegistry(t *testing.T) {
	operators := AllOperators()
	if len(operators) != 108 {
		t.Fatalf("registered %d operators, want 108", len(operators))
	}
	seen := make(map[string]bool)
	for index, operator := range operators {
		if seen[operator.Name()] {
			t.Fatalf("duplicate operator %q", operator.Name())
		}
		seen[operator.Name()] = true
		if index > 0 && operators[index-1].Name() >= operator.Name() {
			t.Fatalf("operators are not strictly sorted at %q, %q", operators[index-1].Name(), operator.Name())
		}
	}
	if operators[0].Name() == "" {
		t.Fatal("empty operator name")
	}
}

// scalaOperatorOracle is transcribed from every object in lir/oper. Each row
// is: name category interpretation arity low high constructible. The three
// synthetic internal operators used by the importer/rewriter are included too.
const scalaOperatorOracle = `
EQ base P f2 50 50 true
NE base P f2 50 50 true
OPER_APP base P positive 160 160 true
CHOOSE3 base P f3 0 0 true
CHOOSE2 base P f2 0 0 true
LABEL control P min2 160 160 true
AND boolean P any 30 30 true
OR boolean P any 30 30 true
NOT boolean P f1 40 40 true
IMPLIES boolean P f2 10 10 true
EQUIV boolean P f2 20 20 true
FORALL3 boolean P f3 0 0 true
FORALL2 boolean P f2 0 0 true
EXISTS3 boolean P f3 0 0 true
EXISTS2 boolean P f2 0 0 true
PLUS arithmetic L f2 100 100 true
UNARY_MINUS arithmetic L f1 120 120 true
MINUS arithmetic L f2 110 110 true
MULT arithmetic L f2 130 130 true
DIV arithmetic L f2 130 130 true
MOD arithmetic L f2 100 110 true
REAL_DIV arithmetic L f2 130 130 true
POW arithmetic L f2 140 140 true
INT_RANGE arithmetic L f2 90 90 true
LT arithmetic L f2 50 50 true
GT arithmetic L f2 50 50 true
LE arithmetic L f2 50 50 true
GE arithmetic L f2 50 50 true
CASE control P even2 0 0 true
CASE_OTHER control P odd3 0 0 true
IF_THEN_ELSE control P f3 0 0 true
SET_ENUM set P any 160 160 true
FUN_SET set P f2 160 160 true
RECORD_SET set P even2 160 160 true
Sequences!Seq set P f1 160 160 true
SET_IN set P f2 50 50 true
SET_NOT_IN set P f2 50 50 true
SET_UNION2 set P f2 80 80 true
SET_INTERSECT set P f2 80 80 true
SET_SUBSET_EQ set P f2 50 50 true
SET_MINUS set P f2 80 80 true
SET_FILTER set P f3 160 160 true
SET_MAP set P odd3 160 160 true
SET_POWERSET set P f1 100 130 true
SET_UNARY_UNION set P f1 100 130 true
SET_TIMES set P min2 100 130 true
RECORD function P even 160 160 true
TUPLE function P any 160 160 true
FUN_APP function P f2 160 160 true
DOMAIN function P f1 100 130 true
FUN_CTOR function P odd3 160 160 true
FUN_REC_CTOR function P min3 160 160 true
FUN_REC_REF function P f0 160 160 true
EXCEPT function P odd3 160 160 true
Sequences!Head sequence L f1 160 160 true
Sequences!Tail sequence L f1 160 160 true
Sequences!Append sequence L f2 160 160 true
Sequences!Concat sequence L f2 130 130 true
Sequences!Len sequence L f1 160 160 true
Sequences!SubSeq sequence L f3 160 160 true
SequencesExt!ToSet sequence L f1 160 160 true
Sequences!SelectSeq sequence L f2 160 160 false
FiniteSets!IsFiniteSet finite-set L f1 160 160 true
FiniteSets!Cardinality finite-set L f1 160 160 true
PRIME action P f1 150 150 true
STUTTER action P f2 40 150 true
NO_STUTTER action P f2 40 150 true
ENABLED action P f1 40 150 true
UNCHANGED action P f1 40 150 true
COMPOSE action P f2 50 140 true
GLOBALLY temporal P f1 40 150 true
EVENTUALLY temporal P f1 40 150 true
LEADS_TO temporal P f2 20 20 true
GUARANTEES temporal P f2 20 20 true
WEAK_FAIRNESS temporal P f2 40 150 true
STRONG_FAIRNESS temporal P f2 40 150 true
TEMPORAL_EXISTS temporal P f2 0 0 true
TEMPORAL_FORALL temporal P f2 0 0 true
Variants!Variant variant L f2 160 160 true
Variants!VariantFilter variant L f2 160 160 true
Variants!VariantTag variant L f1 160 160 true
Variants!VariantGetOrElse variant L f3 160 160 true
Variants!VariantGetUnsafe variant L f2 160 160 true
Apalache!<: apalache L f2 70 70 true
Apalache!:= apalache L f2 50 50 true
Apalache!Gen apalache L f1 160 160 true
Apalache!Skolem apalache L f1 160 160 true
Apalache!Guess apalache P f1 160 160 true
Apalache!Expand apalache L f1 160 160 true
Apalache!ConstCardinality apalache L f1 160 160 true
Apalache!MkSeq apalache L f2 160 160 true
Apalache!ApaFoldSet apalache L f3 160 160 true
Apalache!ApaFoldSeqLeft apalache L f3 160 160 true
Apalache!Repeat apalache L f3 160 160 true
Apalache!SetAsFun apalache L f1 160 160 true
Apalache!FunAsSeq apalache L f2 160 160 false
ApalacheInternal!__NotSupportedByModelChecker apalache-internal L f1 160 160 true
Apalache!Distinct apalache-internal L any 160 160 true
TLC!TLCGet apalache-internal L f1 160 160 true
TLC!ToString apalache-internal L f1 160 160 true
ApalacheInternal!__ApalacheSeqCapacity apalache-internal L f1 160 160 true
ApalacheInternal!__CompactFunSetExpand apalache-internal L f1 160 160 true
Apalache!SelectInSet apalache-internal L f2 160 160 true
Apalache!SelectInFun apalache-internal L f2 160 160 true
Apalache!StoreInSet apalache-internal L f2or3 160 160 true
Apalache!storeNotInSet apalache-internal L f2 160 160 true
Apalache!storeNotInFun apalache-internal L f2 160 160 true
Apalache!UnconstrainArray apalache-internal L f1 160 160 true
`

func scalaArityAccepts(pattern string, count int) bool {
	switch pattern {
	case "any":
		return count >= 0
	case "positive":
		return count > 0
	case "even":
		return count >= 0 && count%2 == 0
	case "even2":
		return count >= 2 && count%2 == 0
	case "odd3":
		return count >= 3 && count%2 == 1
	case "min2":
		return count >= 2
	case "min3":
		return count >= 3
	case "f2or3":
		return count == 2 || count == 3
	default:
		if strings.HasPrefix(pattern, "f") {
			fixed, err := strconv.Atoi(strings.TrimPrefix(pattern, "f"))
			return err == nil && count == fixed
		}
		panic("unknown Scala arity pattern " + pattern)
	}
}

func TestAllOperatorMetadataMatchesScala(t *testing.T) {
	rows := strings.Split(strings.TrimSpace(scalaOperatorOracle), "\n")
	if len(rows) != 108 {
		t.Fatalf("oracle has %d rows, want 108", len(rows))
	}
	wantNames := make(map[string]bool, len(rows))
	for _, row := range rows {
		fields := strings.Fields(row)
		if len(fields) != 7 {
			t.Fatalf("malformed oracle row %q", row)
		}
		name, category, interpretation, arity := fields[0], OperatorCategory(fields[1]), fields[2], fields[3]
		low, lowErr := strconv.Atoi(fields[4])
		high, highErr := strconv.Atoi(fields[5])
		constructible, boolErr := strconv.ParseBool(fields[6])
		if lowErr != nil || highErr != nil || boolErr != nil {
			t.Fatalf("malformed oracle values %q", row)
		}
		wantInterpretation := InterpretationPredefined
		if interpretation == "L" {
			wantInterpretation = InterpretationStandardLibrary
		} else if interpretation != "P" {
			t.Fatalf("unknown interpretation %q", interpretation)
		}
		wantNames[name] = true
		t.Run(name, func(t *testing.T) {
			operator, ok := LookupOperator(name)
			if !ok {
				t.Fatal("operator not found")
			}
			if operator.Category() != category || operator.Interpretation() != wantInterpretation ||
				operator.Precedence() != (Precedence{low, high}) || operator.Constructible() != constructible {
				t.Fatalf("metadata = category %s interpretation %v precedence %v constructible %v",
					operator.Category(), operator.Interpretation(), operator.Precedence(), operator.Constructible())
			}
			for count := -2; count <= 12; count++ {
				if got, want := operator.IsCorrectArity(count), scalaArityAccepts(arity, count); got != want {
					t.Errorf("IsCorrectArity(%d) = %v, want %v (%s)", count, got, want, arity)
				}
			}
		})
	}
	for _, operator := range AllOperators() {
		if !wantNames[operator.Name()] {
			t.Errorf("registered operator %q is absent from oracle", operator.Name())
		}
	}
}

func TestProhibitedAndParameterizedOperators(t *testing.T) {
	if Op("Sequences!SelectSeq").Constructible() || Op("Apalache!FunAsSeq").Constructible() {
		t.Fatal("rewired Scala operators are constructible")
	}
	mapped := NewSMTMapOperator(Op("AND"))
	if mapped.Name() != "Apalache!SmtMap_AND" || mapped.Category() != CategoryApalacheInternal ||
		mapped.Interpretation() != InterpretationStandardLibrary || mapped.Precedence() != (Precedence{160, 160}) ||
		!mapped.Constructible() || !mapped.IsCorrectArity(2) || mapped.IsCorrectArity(1) {
		t.Fatalf("SMT map metadata = %#v", mapped)
	}
	if _, registered := LookupOperator(mapped.Name()); registered {
		t.Fatal("parameterized SMT map operator was inserted into the fixed registry")
	}
}

func TestOperatorArgumentInvariantMatchesScala(t *testing.T) {
	expression := NewNameExpr("x", Untyped())
	stringLiteral := NewValueExpr(StringValue("label"), Untyped())
	integerLiteral := NewValueExpr(Int64Value(1), Untyped())
	label := Op("LABEL")

	if !label.PermitsArgs([]Expr{expression, stringLiteral}) ||
		!label.PermitsArgs([]Expr{expression, stringLiteral, NewValueExpr(StringValue("arg"), Untyped())}) {
		t.Fatal("LABEL rejected Scala-valid string label arguments")
	}
	for _, args := range [][]Expr{{expression, integerLiteral}, {expression, NewNameExpr("not-a-string", Untyped())}} {
		if label.PermitsArgs(args) {
			t.Fatalf("LABEL accepted invalid arguments %v", args)
		}
		if _, err := NewOperExpr(label, Untyped(), args...); err == nil {
			t.Fatalf("constructor accepted invalid LABEL arguments %v", args)
		}
	}
	if !Op("EQ").PermitsArgs([]Expr{integerLiteral, expression}) {
		t.Fatal("default Scala operator invariant rejected arguments")
	}
}

func TestInterleaveAndDeinterleaveMatchScala(t *testing.T) {
	values := []Expr{
		NewNameExpr("a", Untyped()), NewNameExpr("b", Untyped()),
		NewNameExpr("c", Untyped()), NewNameExpr("d", Untyped()),
	}
	even, odd := DeinterleaveExpressions(values)
	if len(even) != 2 || len(odd) != 2 || even[0] != values[0] || even[1] != values[2] ||
		odd[0] != values[1] || odd[1] != values[3] {
		t.Fatalf("deinterleave = %v, %v", even, odd)
	}
	joined := InterleaveExpressions(even, odd)
	for index := range values {
		if joined[index] != values[index] {
			t.Fatalf("interleave[%d] = %v, want %v", index, joined[index], values[index])
		}
	}
	emptyEven, emptyOdd := DeinterleaveExpressions(nil)
	if len(emptyEven) != 0 || len(emptyOdd) != 0 || len(InterleaveExpressions(nil, nil)) != 0 {
		t.Fatal("empty argument helpers did not return empty lists")
	}

	assertPanics(t, func() { DeinterleaveExpressions(values[:3]) })
	assertPanics(t, func() { InterleaveExpressions(even, odd[:1]) })
}

func assertPanics(t *testing.T, body func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("operation did not panic like the Scala assertion")
		}
	}()
	body()
}
