package apalache

import "testing"

func TestTypedFunBuilderRecordsTuplesAndSequences(t *testing.T) {
	builder := NewTypedBuilder()
	for count := 0; count <= 8; count++ {
		fields := make([]RecordField, count)
		mixed := make([]Expr, 0, 2*count)
		wantFields := make(map[string]Type, count)
		for index := range fields {
			name := string(rune('a' + index))
			typ := concreteActionBuilderTypes(t)[index%len(concreteActionBuilderTypes(t))]
			value := builder.Name("v"+name, typ)
			fields[index] = RecordField{Name: name, Value: value}
			key, err := builder.Str(name)
			if err != nil {
				t.Fatal(err)
			}
			mixed = append(mixed, key, value)
			wantFields[name] = typ
		}
		wantRecord, err := NewRecordType(wantFields)
		if err != nil {
			t.Fatal(err)
		}
		for _, build := range []func() (*OperExpr, error){
			func() (*OperExpr, error) { return builder.Rec(fields...) },
			func() (*OperExpr, error) { return builder.RecMixed(mixed...) },
		} {
			record, buildErr := build()
			if buildErr != nil {
				t.Fatal(buildErr)
			}
			requireExprType(t, record, wantRecord)
			got := record.Arguments()
			if record.Operator().Name() != "RECORD" || len(got) != len(mixed) {
				t.Fatalf("record = %s/%d, want RECORD/%d", record.Operator(), len(got), len(mixed))
			}
			for index := range fields {
				key, ok := got[2*index].(*ValueExpr)
				if !ok || key.Value() != StringValue(fields[index].Name) || got[2*index+1] != fields[index].Value {
					t.Fatalf("record field %d was not preserved", index)
				}
			}
		}
	}

	key, _ := builder.Str("k")
	if _, err := builder.RecMixed(key); err == nil {
		t.Fatal("RecMixed accepted odd arity")
	}
	if _, err := builder.RecMixed(key, builder.Int64(1), key, builder.Int64(2)); err == nil {
		t.Fatal("RecMixed accepted a duplicate key")
	}
	if _, err := builder.RecMixed(builder.Name("k", StrT1), builder.Int64(1)); err == nil {
		t.Fatal("RecMixed accepted a non-literal key")
	}
	if _, err := builder.Rec(RecordField{Name: "k", Value: builder.Int64(1)}, RecordField{Name: "k", Value: builder.Int64(2)}); err == nil {
		t.Fatal("Rec accepted a duplicate key")
	}

	for count := 0; count <= 5; count++ {
		arguments := make([]Expr, count)
		types := make([]Type, count)
		for index := range arguments {
			types[index] = concreteActionBuilderTypes(t)[index%len(concreteActionBuilderTypes(t))]
			arguments[index] = builder.Name("x", types[index])
		}
		tuple, err := builder.Tuple(arguments...)
		if err != nil {
			t.Fatal(err)
		}
		requireActionOperation(t, tuple, "TUPLE", mustTupleType(t, types...), arguments...)
		if count > 0 {
			elements := make([]Expr, count)
			for index := range elements {
				elements[index] = builder.Name("e", types[0])
			}
			sequence, buildErr := builder.Seq(elements...)
			if buildErr != nil {
				t.Fatal(buildErr)
			}
			requireActionOperation(t, sequence, "TUPLE", mustSeqType(t, types[0]), elements...)
		}
	}
	if _, err := builder.Seq(); err == nil {
		t.Fatal("Seq accepted empty arity")
	}
	if _, err := builder.Seq(builder.Int64(1), builder.Name("s", StrT1)); err == nil {
		t.Fatal("Seq accepted heterogeneous elements")
	}
	empty := builder.EmptySeq(IntT1)
	requireActionOperation(t, empty, "TUPLE", mustSeqType(t, IntT1))

	tail := mustNumberedVar(t, 9)
	row, err := builder.RowRecMixed(&tail, key, builder.Int64(1))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, row, NewRecordRowType(mustRowType(t, map[string]Type{"k": IntT1}, &tail)))
}

func TestTypedFunBuilderFunctionConstructor(t *testing.T) {
	builder := NewTypedBuilder()
	for count := 1; count <= 6; count++ {
		domains := make([]FunctionDomain, count)
		mixed := make([]Expr, 0, 2*count)
		domainTypes := make([]Type, count)
		for index := range domains {
			domainTypes[index] = concreteActionBuilderTypes(t)[index%len(concreteActionBuilderTypes(t))]
			domains[index] = FunctionDomain{
				Variable: builder.Name(string(rune('a'+index)), domainTypes[index]),
				Set:      builder.Name("S", mustSetType(t, domainTypes[index])),
			}
			mixed = append(mixed, domains[index].Variable, domains[index].Set)
		}
		body := builder.Name("body", StrT1)
		wantDomain := domainTypes[0]
		if count > 1 {
			wantDomain = mustTupleType(t, domainTypes...)
		}
		want := mustFunType(t, wantDomain, StrT1)
		for _, build := range []func() (*OperExpr, error){
			func() (*OperExpr, error) { return builder.FunDef(body, domains...) },
			func() (*OperExpr, error) { return builder.FunDefMixed(body, mixed...) },
		} {
			result, err := build()
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, result, "FUN_CTOR", want, append([]Expr{body}, mixed...)...)
		}
		for index := range domains {
			badVariable := append([]Expr(nil), mixed...)
			badVariable[2*index] = builder.Name("bad", differentConcreteType(domainTypes[index]))
			if _, err := builder.FunDefMixed(body, badVariable...); err == nil {
				t.Fatalf("FunDef accepted mismatched variable %d", index)
			}
			badSet := append([]Expr(nil), mixed...)
			badSet[2*index+1] = builder.Name("bad", IntT1)
			if _, err := builder.FunDefMixed(body, badSet...); err == nil {
				t.Fatalf("FunDef accepted non-set domain %d", index)
			}
		}
	}
	if _, err := builder.FunDef(builder.Int64(1)); err == nil {
		t.Fatal("FunDef accepted no domains")
	}
	if _, err := builder.FunDefMixed(builder.Int64(1), builder.Name("x", IntT1)); err == nil {
		t.Fatal("FunDefMixed accepted odd arity")
	}
	set := builder.Name("S", mustSetType(t, IntT1))
	if _, err := builder.FunDefMixed(builder.Int64(1), builder.Int64(0), set); err == nil {
		t.Fatal("FunDefMixed accepted a non-name variable")
	}
	if _, err := builder.FunDef(builder.Int64(1),
		FunctionDomain{builder.Name("x", IntT1), set}, FunctionDomain{builder.Name("x", IntT1), set}); err == nil {
		t.Fatal("FunDef accepted duplicate variables")
	}
}

func TestFunBuilderApplicativeOverloads(t *testing.T) {
	builder := NewTypedBuilder()
	field, _ := builder.Str("field")
	recordType, _ := NewRecordType(map[string]Type{"field": BoolT1})
	tupleType := mustTupleType(t, StrT1, BoolT1)
	for _, test := range []struct {
		name     string
		typ      Type
		argument Expr
		result   Type
		domain   Type
	}{
		{"function", mustFunType(t, IntT1, StrT1), builder.Int64(1), StrT1, IntT1},
		{"sequence", mustSeqType(t, BoolT1), builder.Int64(1), BoolT1, IntT1},
		{"tuple", tupleType, builder.Int64(2), BoolT1, IntT1},
		{"record", recordType, field, BoolT1, StrT1},
	} {
		t.Run(test.name, func(t *testing.T) {
			applicative := builder.Name("f", test.typ)
			application, err := builder.App(applicative, test.argument)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, application, "FUN_APP", test.result, applicative, test.argument)
			domain, err := builder.Dom(applicative)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, domain, "DOMAIN", mustSetType(t, test.domain), applicative)
			replacement := builder.Name("replacement", test.result)
			updated, err := builder.Except(applicative, test.argument, replacement)
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, updated, test.typ)
			operands := updated.Arguments()
			if updated.Operator().Name() != "EXCEPT" || len(operands) != 3 || operands[0] != applicative || operands[2] != replacement {
				t.Fatal("Except did not preserve its function and replacement operands")
			}
			wrapped, ok := operands[1].(*OperExpr)
			if !ok || wrapped.Operator().Name() != "TUPLE" || len(wrapped.Arguments()) != 1 || wrapped.Arguments()[0] != test.argument {
				t.Fatal("Except did not wrap its argument in a singleton tuple")
			}
		})
	}
	bad := builder.Name("bad", IntT1)
	if _, err := builder.App(bad, builder.Int64(1)); err == nil {
		t.Fatal("App accepted a non-applicative")
	}
	if _, err := builder.Dom(bad); err == nil {
		t.Fatal("Dom accepted a non-applicative")
	}
	tuple := builder.Name("tuple", tupleType)
	if _, err := builder.App(tuple, builder.Name("index", IntT1)); err == nil {
		t.Fatal("App accepted a non-literal tuple index")
	}
	if _, err := builder.Except(tuple, builder.Int64(2), builder.Int64(1)); err == nil {
		t.Fatal("Except accepted a wrong replacement type")
	}
}

func TestScopedFunBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	record, err := builder.Rec(
		ScopedRecordField{"a", builder.Int64(1)}, ScopedRecordField{"b", builder.Name("b", BoolT1)}).Build()
	if err != nil {
		t.Fatal(err)
	}
	wantRecord, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	requireExprType(t, record, wantRecord)
	if _, err = builder.Rec(ScopedRecordField{"a", builder.Int64(1)}, ScopedRecordField{"a", builder.Int64(2)}).Build(); err == nil {
		t.Fatal("scoped Rec accepted duplicate keys")
	}

	domain := builder.Name("S", mustSetType(t, IntT1))
	function := builder.FunDef(builder.Name("x", IntT1), BoundDomain{builder.Name("x", IntT1), domain})
	built, err := function.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, built, mustFunType(t, IntT1, IntT1))
	if _, err = builder.FunDef(builder.Int64(1)).Build(); err == nil {
		t.Fatal("scoped FunDef accepted no domains")
	}
	if _, err = builder.FunDefMixed(builder.Int64(1), builder.Name("x", IntT1)).Build(); err == nil {
		t.Fatal("scoped FunDefMixed accepted odd arity")
	}
	duplicate := builder.FunDef(builder.Int64(1),
		BoundDomain{builder.Name("x", IntT1), domain}, BoundDomain{builder.Name("x", IntT1), domain})
	if _, err = duplicate.Build(); err == nil {
		t.Fatal("scoped FunDef accepted duplicate variables")
	}
	tupleVariable := builder.Tuple(builder.Name("x", IntT1), builder.Name("y", IntT1))
	if _, err = builder.FunDef(builder.Int64(1),
		BoundDomain{tupleVariable, builder.Name("pairs", mustSetType(t, mustTupleType(t, IntT1, IntT1)))}).Build(); err == nil {
		t.Fatal("scoped FunDef accepted a tuple variable")
	}

	app := builder.App(function, builder.Int64(1))
	applied, err := app.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, applied, IntT1)
	dom, err := builder.Dom(function).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, dom, mustSetType(t, IntT1))
	updated, err := builder.Except(function, builder.Int64(1), builder.Int64(2)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, updated, mustFunType(t, IntT1, IntT1))

	// Regression #2551: independent function heads do not share binder state.
	set, err := builder.Apply(Op("SET_ENUM"), function, function).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, set, mustSetType(t, mustFunType(t, IntT1, IntT1)))
}
