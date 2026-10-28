package apalache

import (
	"context"
	"testing"
)

func exceptExpression(targetType Type, target, key, value Expr) Expr {
	accessorType, _ := NewTupleType(mustTagType(key.Tag()))
	accessor := MustOperExpr(Op("TUPLE"), Typed(accessorType), key)
	return MustOperExpr(Op("EXCEPT"), Typed(targetType), target, accessor, value)
}

func TestStratifiedExceptUpdatesTuplesRecordsAndSequences(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	key := func(value string) Expr { return NewValueExpr(StringValue(value), Typed(StrT1)) }
	tupleType, _ := NewTupleType(IntT1, BoolT1)
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), intValue(1), NewValueExpr(BoolValue(false), Typed(BoolT1)))
	updatedTuple := exceptExpression(tupleType, tuple, intValue(2), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	expectedTuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), intValue(1), NewValueExpr(BoolValue(true), Typed(BoolT1)))

	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	record := MustOperExpr(Op("RECORD"), Typed(recordType), key("a"), intValue(1), key("b"), NewValueExpr(BoolValue(false), Typed(BoolT1)))
	updatedRecord := exceptExpression(recordType, record, key("a"), intValue(8))
	expectedRecord := MustOperExpr(Op("RECORD"), Typed(recordType), key("a"), intValue(8), key("b"), NewValueExpr(BoolValue(false), Typed(BoolT1)))

	sequenceType, _ := NewSeqType(IntT1)
	sequence := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1), intValue(2), intValue(3))
	symbolicIndex := MustOperExpr(Op("PLUS"), Typed(IntT1), intValue(1), intValue(1))
	updatedSequence := exceptExpression(sequenceType, sequence, symbolicIndex, intValue(9))
	expectedSequence := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1), intValue(9), intValue(3))

	tests := []struct {
		name         string
		actual, want Expr
	}{
		{"tuple", updatedTuple, expectedTuple},
		{"record", updatedRecord, expectedRecord},
		{"sequence symbolic index", updatedSequence, expectedSequence},
		{"sequence outside capacity", exceptExpression(sequenceType, sequence, intValue(5), intValue(9)), sequence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), test.actual, test.want)
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

func TestStratifiedFunctionExceptThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	functionType, _ := NewFunType(IntT1, IntT1)
	domainType := mustSet(IntT1)
	domain := MustOperExpr(Op("SET_ENUM"), Typed(domainType), intValue(1), intValue(2))
	binder := NewNameExpr("x", Typed(IntT1))
	body := MustOperExpr(Op("MULT"), Typed(IntT1), binder, intValue(10))
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, binder, domain)
	updated := exceptExpression(functionType, function, intValue(1), intValue(99))
	apply := func(function Expr, argument int64) Expr {
		return MustOperExpr(Op("FUN_APP"), Typed(IntT1), function, intValue(argument))
	}
	claim := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), apply(updated, 1), intValue(99)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), apply(updated, 2), intValue(20)),
		MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("DOMAIN"), Typed(domainType), updated), domain),
	)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(claim, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedFunctionExceptUsesNativeArrays(t *testing.T) {
	functionType, _ := NewFunType(IntT1, IntT1)
	domain := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1), intLiteral(2))
	binder := NewNameExpr("x", Typed(IntT1))
	body := MustOperExpr(Op("MULT"), Typed(IntT1), binder, intLiteral(10))
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, binder, domain)
	updated := exceptExpression(functionType, function, intLiteral(1), intLiteral(99))
	apply := func(argument int64) Expr {
		return MustOperExpr(Op("FUN_APP"), Typed(IntT1), updated, intLiteral(argument))
	}
	claim := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), apply(1), intLiteral(99)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), apply(2), intLiteral(20)))
	for _, encoding := range []SMTEncoding{SMTEncodingArrays, SMTEncodingFunArrays} {
		t.Run(string(encoding), func(t *testing.T) {
			rewriter := NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{SMTEncoding: encoding})
			scope, result, err := rewriter.Rewrite(claim, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScopeWithEncoding(solver, scope, rewriter.Constraints(), encoding)
			if err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
				t.Fatalf("native function EXCEPT scope = %v, %v", status, checkErr)
			}
			if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
				t.Fatalf("native function EXCEPT = %v, %v", status, checkErr)
			}
		})
	}
}

func TestStratifiedFunctionExceptUsesNestedNativeArrays(t *testing.T) {
	setType := mustSet(IntT1)
	functionType, _ := NewFunType(IntT1, setType)
	domain := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2))
	binder := NewNameExpr("x", Typed(IntT1))
	body := MustOperExpr(Op("SET_ENUM"), Typed(setType), binder)
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, binder, domain)
	replacement := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(9), intLiteral(10))
	updated := exceptExpression(functionType, function, intLiteral(1), replacement)
	apply := func(argument int64) Expr {
		return MustOperExpr(Op("FUN_APP"), Typed(setType), updated, intLiteral(argument))
	}
	member := func(element int64, set Expr) Expr {
		return MustOperExpr(Op("SET_IN"), Typed(BoolT1), intLiteral(element), set)
	}
	claim := MustOperExpr(Op("AND"), Typed(BoolT1),
		member(10, apply(1)),
		MustOperExpr(Op("NOT"), Typed(BoolT1), member(1, apply(1))),
		member(2, apply(2)))
	for _, encoding := range []SMTEncoding{SMTEncodingArrays, SMTEncodingFunArrays} {
		t.Run(string(encoding), func(t *testing.T) {
			rewriter := NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{SMTEncoding: encoding})
			scope, result, err := rewriter.Rewrite(claim, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScopeWithEncoding(solver, scope, rewriter.Constraints(), encoding)
			if err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
				t.Fatalf("nested native function EXCEPT scope = %v, %v", status, checkErr)
			}
			if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
				t.Fatalf("nested native function EXCEPT = %v, %v", status, checkErr)
			}
		})
	}
}

func TestStratifiedFunctionArraysSupportSetValuedKeys(t *testing.T) {
	keyType := mustSet(IntT1)
	domainType := mustSet(keyType)
	functionType := mustFun(keyType, IntT1)
	key := func(value int64) Expr {
		return MustOperExpr(Op("SET_ENUM"), Typed(keyType), intLiteral(value))
	}
	domain := MustOperExpr(Op("SET_ENUM"), Typed(domainType), key(1), key(2))
	binder := NewNameExpr("s", Typed(keyType))
	body := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
		MustOperExpr(Op("SET_IN"), Typed(BoolT1), intLiteral(1), binder), intLiteral(11), intLiteral(22))
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, binder, domain)
	updated := exceptExpression(functionType, function, key(2), intLiteral(99))
	apply := func(argument Expr) Expr {
		return MustOperExpr(Op("FUN_APP"), Typed(IntT1), updated, argument)
	}
	claim := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), apply(key(1)), intLiteral(11)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), apply(key(2)), intLiteral(99)))
	for _, encoding := range []SMTEncoding{SMTEncodingArrays, SMTEncodingFunArrays} {
		t.Run(string(encoding), func(t *testing.T) {
			rewriter := NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{SMTEncoding: encoding})
			scope, result, err := rewriter.Rewrite(claim, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScopeWithEncoding(solver, scope, rewriter.Constraints(), encoding)
			if err != nil {
				t.Fatal(err)
			}
			if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
				t.Fatalf("set-keyed native function under %s = %v, %v", encoding, status, checkErr)
			}
		})
	}
}

func TestStratifiedFunctionExceptSupportsStructuralKeys(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	keyType, _ := NewTupleType(IntT1, IntT1)
	key := func(left, right int64) Expr {
		return MustOperExpr(Op("TUPLE"), Typed(keyType), intValue(left), intValue(right))
	}
	functionType, _ := NewFunType(keyType, IntT1)
	domainType := mustSet(keyType)
	domain := MustOperExpr(Op("SET_ENUM"), Typed(domainType), key(1, 2), key(3, 4))
	binder := NewNameExpr("pair", Typed(keyType))
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), intValue(0), binder, domain)
	updated := exceptExpression(functionType, function, key(3, 4), intValue(7))
	application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), updated, key(3, 4))
	claim := MustOperExpr(Op("EQ"), Typed(BoolT1), application, intValue(7))
	for _, encoding := range []SMTEncoding{SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays} {
		t.Run(string(encoding), func(t *testing.T) {
			rewriter := NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{SMTEncoding: encoding})
			scope, result, err := rewriter.Rewrite(claim, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScopeWithEncoding(solver, scope, rewriter.Constraints(), encoding)
			if err != nil {
				t.Fatal(err)
			}
			if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
				t.Fatalf("tuple-keyed function EXCEPT under %s = %v, %v", encoding, status, checkErr)
			}
		})
	}
}

func TestStratifiedExceptRejectsInvalidLiteralSelectors(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	tupleType, _ := NewTupleType(IntT1)
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), intValue(1))
	if _, _, err := NewStratifiedRewriter().Rewrite(
		exceptExpression(tupleType, tuple, intValue(2), intValue(3)), InitialRewriterScope()); err == nil {
		t.Fatal("out-of-bounds tuple EXCEPT was accepted")
	}
}

func TestStratifiedRecordExceptPreservesDomainAndRejectsInvalidUpdates(t *testing.T) {
	recordType, err := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	if err != nil {
		t.Fatal(err)
	}
	key := func(name string) Expr { return NewValueExpr(StringValue(name), Typed(StrT1)) }
	record := MustOperExpr(Op("RECORD"), Typed(recordType), key("a"), intLiteral(1))
	updated := exceptExpression(recordType, record, key("a"), intLiteral(8))
	domain := MustOperExpr(Op("DOMAIN"), Typed(mustSet(StrT1)), updated)
	expected := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(StrT1)), key("a"))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), domain, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)

	for _, invalid := range []Expr{
		exceptExpression(recordType, record, NewNameExpr("field", Typed(StrT1)), intLiteral(2)),
		exceptExpression(recordType, record, key("missing"), intLiteral(2)),
		exceptExpression(recordType, record, key("a"), NewValueExpr(BoolValue(true), Typed(BoolT1))),
	} {
		if _, _, err := NewStratifiedRewriter().Rewrite(invalid, InitialRewriterScope()); err == nil {
			t.Fatalf("invalid record update was accepted: %v", invalid)
		}
	}
}

func TestStratifiedExceptTypeErrorsDoNotMutateArena(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	tupleType, _ := NewTupleType(IntT1)
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), intValue(1))
	sequenceType, _ := NewSeqType(IntT1)
	sequence := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1))
	functionType := mustFun(IntT1, IntT1)
	binder := NewNameExpr("x", Typed(IntT1))
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), binder, binder,
		MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intValue(1)))
	for _, invalid := range []Expr{
		exceptExpression(BoolT1, tuple, intValue(1), intValue(2)),
		exceptExpression(tupleType, tuple, intValue(1), boolExpr(true)),
		exceptExpression(sequenceType, sequence, boolExpr(true), intValue(2)),
		exceptExpression(sequenceType, sequence, intValue(1), boolExpr(true)),
		exceptExpression(functionType, function, boolExpr(true), intValue(2)),
	} {
		base := InitialRewriterScope()
		returned, _, err := NewStratifiedRewriter().Rewrite(invalid, base)
		if err == nil || returned.Arena != base.Arena {
			t.Fatalf("invalid EXCEPT returned scope=%v, err=%v", returned, err)
		}
	}
}
