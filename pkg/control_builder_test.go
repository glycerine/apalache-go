package apalache

import "testing"

func typedControlPairs(builder *TypedBuilder, count int, typ Type) ([]ControlCase, []Expr) {
	pairs := make([]ControlCase, count)
	arguments := make([]Expr, 0, 2*count)
	for index := range pairs {
		pairs[index] = ControlCase{
			Condition: builder.Name("p", BoolT1),
			Body:      builder.Name("e", typ),
		}
		arguments = append(arguments, pairs[index].Condition, pairs[index].Body)
	}
	return pairs, arguments
}

func TestTypedControlBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			condition := builder.Name("p", BoolT1)
			thenBranch, elseBranch := builder.Name("A", typ), builder.Name("B", typ)
			result, err := builder.Ite(condition, thenBranch, elseBranch)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, result, "IF_THEN_ELSE", typ, condition, thenBranch, elseBranch)
			if _, err = builder.Ite(builder.Name("bad", StrT1), thenBranch, elseBranch); err == nil {
				t.Fatal("Ite accepted a non-Boolean condition")
			}
			if _, err = builder.Ite(condition, builder.Name("bad", differentConcreteType(typ)), elseBranch); err == nil {
				t.Fatal("Ite accepted an incompatible then branch")
			}
			if _, err = builder.Ite(condition, thenBranch, builder.Name("bad", differentConcreteType(typ))); err == nil {
				t.Fatal("Ite accepted an incompatible else branch")
			}

			for count := 1; count <= 11; count++ {
				pairs, arguments := typedControlPairs(builder, count, typ)
				for _, test := range []struct {
					name  string
					build func() (*OperExpr, error)
				}{
					{"pairs", func() (*OperExpr, error) { return builder.CaseSplit(pairs...) }},
					{"mixed", func() (*OperExpr, error) { return builder.CaseSplitMixed(arguments...) }},
				} {
					got, buildErr := test.build()
					if buildErr != nil {
						t.Fatalf("CASE %s/%d: %v", test.name, count, buildErr)
					}
					requireActionOperation(t, got, "CASE", typ, arguments...)
				}

				other := builder.Name("other", typ)
				otherArguments := append([]Expr{other}, arguments...)
				for _, test := range []struct {
					name  string
					build func() (*OperExpr, error)
				}{
					{"pairs", func() (*OperExpr, error) { return builder.CaseOther(other, pairs...) }},
					{"mixed", func() (*OperExpr, error) { return builder.CaseOtherMixed(other, arguments...) }},
				} {
					got, buildErr := test.build()
					if buildErr != nil {
						t.Fatalf("CASE OTHER %s/%d: %v", test.name, count, buildErr)
					}
					requireActionOperation(t, got, "CASE_OTHER", typ, otherArguments...)
				}

				for index := range pairs {
					badCondition := append([]Expr(nil), arguments...)
					badCondition[2*index] = builder.Name("bad", StrT1)
					badConditionPairs := append([]ControlCase(nil), pairs...)
					badConditionPairs[index].Condition = badCondition[2*index]
					if _, err = builder.CaseSplit(badConditionPairs...); err == nil {
						t.Fatalf("CASE pair API accepted bad condition %d/%d", index, count)
					}
					if _, err = builder.CaseSplitMixed(badCondition...); err == nil {
						t.Fatalf("CASE accepted bad condition %d/%d", index, count)
					}
					if _, err = builder.CaseOther(other, badConditionPairs...); err == nil {
						t.Fatalf("CASE OTHER pair API accepted bad condition %d/%d", index, count)
					}
					if _, err = builder.CaseOtherMixed(other, badCondition...); err == nil {
						t.Fatalf("CASE OTHER accepted bad condition %d/%d", index, count)
					}
					if count > 1 {
						badBody := append([]Expr(nil), arguments...)
						badBody[2*index+1] = builder.Name("bad", differentConcreteType(typ))
						badBodyPairs := append([]ControlCase(nil), pairs...)
						badBodyPairs[index].Body = badBody[2*index+1]
						if _, err = builder.CaseSplit(badBodyPairs...); err == nil {
							t.Fatalf("CASE pair API accepted bad body %d/%d", index, count)
						}
						if _, err = builder.CaseSplitMixed(badBody...); err == nil {
							t.Fatalf("CASE accepted bad body %d/%d", index, count)
						}
						if _, err = builder.CaseOther(other, badBodyPairs...); err == nil {
							t.Fatalf("CASE OTHER pair API accepted bad body %d/%d", index, count)
						}
						if _, err = builder.CaseOtherMixed(other, badBody...); err == nil {
							t.Fatalf("CASE OTHER accepted bad body %d/%d", index, count)
						}
					}
				}
				if _, err = builder.CaseOther(builder.Name("bad", differentConcreteType(typ)), pairs...); err == nil {
					t.Fatal("CASE OTHER pair API accepted an incompatible OTHER body")
				}
				if _, err = builder.CaseOtherMixed(builder.Name("bad", differentConcreteType(typ)), arguments...); err == nil {
					t.Fatal("CASE OTHER accepted an incompatible OTHER body")
				}
			}
		})
	}

	if _, err := builder.CaseSplit(); err == nil {
		t.Fatal("CaseSplit accepted no pairs")
	}
	if _, err := builder.CaseSplitMixed(); err == nil {
		t.Fatal("CaseSplitMixed accepted no arguments")
	}
	if _, err := builder.CaseSplitMixed(builder.Name("x", BoolT1)); err == nil {
		t.Fatal("CaseSplitMixed accepted odd arity")
	}
	if _, err := builder.CaseOther(builder.Name("e", IntT1)); err == nil {
		t.Fatal("CaseOther accepted no pairs")
	}
	if _, err := builder.CaseOtherMixed(builder.Name("e", IntT1)); err == nil {
		t.Fatal("CaseOtherMixed accepted no pairs")
	}
	if _, err := builder.CaseOtherMixed(builder.Name("e", IntT1), builder.Name("x", BoolT1)); err == nil {
		t.Fatal("CaseOtherMixed accepted odd pair arity")
	}
}

func TestScopedControlBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			result, err := builder.Ite(builder.Name("p", BoolT1), builder.Name("A", typ), builder.Name("B", typ)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, result, typ)
			if result.(*OperExpr).Operator().Name() != "IF_THEN_ELSE" {
				t.Fatalf("ITE operator = %s", result.(*OperExpr).Operator())
			}
			if _, err = builder.Ite(builder.Name("p", StrT1), builder.Name("A", typ), builder.Name("B", typ)).Build(); err == nil {
				t.Fatal("Ite accepted a non-Boolean condition")
			}
			if _, err = builder.Ite(builder.Name("p", BoolT1), builder.Name("bad", differentConcreteType(typ)), builder.Name("B", typ)).Build(); err == nil {
				t.Fatal("Ite accepted an incompatible then branch")
			}
			if _, err = builder.Ite(builder.Name("p", BoolT1), builder.Name("A", typ), builder.Name("bad", differentConcreteType(typ))).Build(); err == nil {
				t.Fatal("Ite accepted an incompatible else branch")
			}

			for count := 1; count <= 11; count++ {
				pairs := make([]ScopedControlCase, count)
				arguments := make([]ExprInstruction, 0, 2*count)
				for index := range pairs {
					pairs[index] = ScopedControlCase{builder.Name("p", BoolT1), builder.Name("e", typ)}
					arguments = append(arguments, pairs[index].Condition, pairs[index].Body)
				}
				for _, instruction := range []ExprInstruction{
					builder.CaseSplit(pairs...),
					builder.CaseSplitMixed(arguments...),
				} {
					got, buildErr := instruction.Build()
					if buildErr != nil {
						t.Fatal(buildErr)
					}
					requireExprType(t, got, typ)
					if got.(*OperExpr).Operator().Name() != "CASE" {
						t.Fatalf("CASE operator = %s", got.(*OperExpr).Operator())
					}
				}
				for _, instruction := range []ExprInstruction{
					builder.CaseOther(builder.Name("other", typ), pairs...),
					builder.CaseOtherMixed(builder.Name("other", typ), arguments...),
				} {
					got, buildErr := instruction.Build()
					if buildErr != nil {
						t.Fatal(buildErr)
					}
					requireExprType(t, got, typ)
					if got.(*OperExpr).Operator().Name() != "CASE_OTHER" {
						t.Fatalf("CASE OTHER operator = %s", got.(*OperExpr).Operator())
					}
				}
				for index := range pairs {
					badConditionPairs := append([]ScopedControlCase(nil), pairs...)
					badConditionPairs[index].Condition = builder.Name("bad", StrT1)
					badConditionArguments := append([]ExprInstruction(nil), arguments...)
					badConditionArguments[2*index] = builder.Name("bad", StrT1)
					for name, instruction := range map[string]ExprInstruction{
						"CASE pairs":       builder.CaseSplit(badConditionPairs...),
						"CASE mixed":       builder.CaseSplitMixed(badConditionArguments...),
						"CASE OTHER pairs": builder.CaseOther(builder.Name("other", typ), badConditionPairs...),
						"CASE OTHER mixed": builder.CaseOtherMixed(builder.Name("other", typ), badConditionArguments...),
					} {
						if _, buildErr := instruction.Build(); buildErr == nil {
							t.Errorf("%s accepted bad condition %d/%d", name, index, count)
						}
					}
					if count > 1 {
						badType := differentConcreteType(typ)
						badBodyPairs := append([]ScopedControlCase(nil), pairs...)
						badBodyPairs[index].Body = builder.Name("bad", badType)
						badBodyArguments := append([]ExprInstruction(nil), arguments...)
						badBodyArguments[2*index+1] = builder.Name("bad", badType)
						for name, instruction := range map[string]ExprInstruction{
							"CASE pairs":       builder.CaseSplit(badBodyPairs...),
							"CASE mixed":       builder.CaseSplitMixed(badBodyArguments...),
							"CASE OTHER pairs": builder.CaseOther(builder.Name("other", typ), badBodyPairs...),
							"CASE OTHER mixed": builder.CaseOtherMixed(builder.Name("other", typ), badBodyArguments...),
						} {
							if _, buildErr := instruction.Build(); buildErr == nil {
								t.Errorf("%s accepted bad body %d/%d", name, index, count)
							}
						}
					}
				}
				badOther := builder.Name("bad", differentConcreteType(typ))
				if _, err = builder.CaseOther(badOther, pairs...).Build(); err == nil {
					t.Fatal("CASE OTHER pair API accepted an incompatible OTHER body")
				}
				if _, err = builder.CaseOtherMixed(badOther, arguments...).Build(); err == nil {
					t.Fatal("CASE OTHER mixed API accepted an incompatible OTHER body")
				}
			}
		})
	}

	for name, instruction := range map[string]ExprInstruction{
		"case pairs":        builder.CaseSplit(),
		"case mixed empty":  builder.CaseSplitMixed(),
		"case mixed odd":    builder.CaseSplitMixed(builder.Name("x", BoolT1)),
		"other pairs":       builder.CaseOther(builder.Name("e", IntT1)),
		"other mixed empty": builder.CaseOtherMixed(builder.Name("e", IntT1)),
		"other mixed odd":   builder.CaseOtherMixed(builder.Name("e", IntT1), builder.Name("x", BoolT1)),
	} {
		if _, err := instruction.Build(); err == nil {
			t.Errorf("%s accepted invalid arity", name)
		}
	}
}

func TestControlBuilderFlexibleBranchTypes(t *testing.T) {
	builder := NewTypedBuilder()
	closed := NewRecordRowType(mustRowType(t, map[string]Type{"a": IntT1}, nil))
	tail, err := NewVarType(700)
	if err != nil {
		t.Fatal(err)
	}
	open := NewRecordRowType(mustRowType(t, map[string]Type{"a": IntT1}, &tail))

	ite, err := builder.Ite(builder.Name("p", BoolT1), builder.Name("closed", closed), builder.Name("open", open))
	if err != nil {
		t.Fatal(err)
	}
	common, ok, err := CommonSupertype(closed, open)
	if err != nil || !ok {
		t.Fatalf("common record type: ok=%v err=%v", ok, err)
	}
	requireExprType(t, ite, common)

	caseExpr, err := builder.CaseSplit(
		ControlCase{builder.Name("p", BoolT1), builder.Name("closed", closed)},
		ControlCase{builder.Name("q", BoolT1), builder.Name("open", open)},
	)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, caseExpr, common)
	leftRecord, err := NewRecordType(map[string]Type{"left": IntT1})
	if err != nil {
		t.Fatal(err)
	}
	rightRecord, err := NewRecordType(map[string]Type{"right": BoolT1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = builder.CaseOther(builder.Name("other", leftRecord),
		ControlCase{builder.Name("p", BoolT1), builder.Name("branch", rightRecord)}); err == nil {
		t.Fatal("CASE OTHER accepted OTHER whose exact type differs from the common branch type")
	}
}
