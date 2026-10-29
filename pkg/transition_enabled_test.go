package apalache

import (
	"fmt"
	"math/big"
	"testing"
)

func TestTransitionEnabledProjectionEliminatesSymbolicRangeAssignment(t *testing.T) {
	i := NewNameExpr("i", Typed(IntT1))
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	assignment := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, i)
	domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), x, intLiteral(0))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, assignment)
	skolem := MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists)
	projected, exact := transitionEnabledProjection(skolem)
	if !exact {
		t.Fatal("symbolic range assignment was not projected")
	}
	want := MustOperExpr(Op("LE"), Typed(BoolT1), x, intLiteral(0))
	if !EqualTypedExpr(projected, want) {
		t.Fatalf("enabled projection = %v, want %v", projected, want)
	}
}

func TestTransitionEnabledProjectionSubstitutesAssignedPrimeInGuard(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	right := MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))
	assignment := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, right)
	guard := MustOperExpr(Op("LT"), Typed(BoolT1), prime, intLiteral(4))
	projected, exact := transitionEnabledProjection(MustOperExpr(Op("AND"), Typed(BoolT1), assignment, guard))
	if !exact || expressionContainsPrime(projected) || countOperator(projected, "PLUS") != 1 {
		t.Fatalf("guard projection = %v, exact=%t", projected, exact)
	}
}

func TestTransitionEnabledProjectionDistributesAssignedDisjunctions(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), x)
	assign := func(value int64) Expr {
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(value))
	}
	branch := func(guard Expr, value int64) Expr {
		return MustOperExpr(Op("AND"), Typed(BoolT1), guard, assign(value))
	}
	disjunction := MustOperExpr(Op("OR"), Typed(BoolT1),
		branch(MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)), 1),
		branch(MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(1)), 3))
	targetGuard := MustOperExpr(Op("LT"), Typed(BoolT1), prime, intLiteral(2))
	projected, exact := transitionEnabledProjection(MustOperExpr(Op("AND"), Typed(BoolT1), disjunction, targetGuard))
	if !exact || projected == nil || expressionContainsPrime(projected) || expressionContainsAssignment(projected) {
		t.Fatalf("assigned disjunction projection = %v, exact=%t", projected, exact)
	}
	formatted, err := FormatTLAExpression(projected)
	if err != nil {
		t.Fatal(err)
	}
	if formatted != "(x = 0 /\\ 1 < 2) \\/ (x = 1 /\\ 3 < 2)" {
		t.Fatalf("assigned disjunction projection = %q", formatted)
	}
}

func TestTransitionEnabledProjectionEliminatesBoundedNonlinearWitnessPredicate(t *testing.T) {
	i := NewNameExpr("i", Typed(IntT1))
	domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(-2), intLiteral(3))
	body := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("MULT"), Typed(IntT1), i, i), NewNameExpr("x", Typed(IntT1)))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, body)
	projected, exact := transitionEnabledProjection(exists)
	if !exact || projected == nil || expressionContainsName(projected, "i") {
		t.Fatalf("bounded nonlinear witness projection = %v, exact=%t", projected, exact)
	}
	for value := int64(-1); value <= 10; value++ {
		got, err := evaluateProjectedBoolean(projected, map[string]*big.Int{"x": big.NewInt(value)})
		want := value == 0 || value == 1 || value == 4 || value == 9
		if err != nil || got != want {
			t.Fatalf("x=%d: bounded nonlinear projection = %v, %v; want %v", value, got, err, want)
		}
	}
}

func TestProjectedTransitionEnablednessIncludesActionAndTargetConstraints(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	transition := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, x)
	action := namedExpression{expression: MustOperExpr(Op("LE"), Typed(BoolT1), x, intLiteral(1))}
	target := namedExpression{expression: MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))}
	projected := projectedTransitionEnabledness([]Expr{transition}, []namedExpression{action}, []namedExpression{target},
		[]*VarDecl{NewVarDecl("x", Typed(IntT1))})
	if len(projected) != 1 || expressionContainsPrime(projected[0]) ||
		countOperator(projected[0], "LE") != 1 || countOperator(projected[0], "EQ") != 1 {
		t.Fatalf("constrained enabledness = %v", projected)
	}
}

func TestTransitionEnabledProjectionEliminatesIntegerWitnessBounds(t *testing.T) {
	i := NewNameExpr("i", Typed(IntT1))
	x := NewNameExpr("x", Typed(IntT1))
	domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(0), intLiteral(3))
	predicate := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("GT"), Typed(BoolT1), i, x),
		MustOperExpr(Op("LE"), Typed(BoolT1), i, intLiteral(2)),
		MustOperExpr(Op("GE"), Typed(BoolT1), x, intLiteral(0)))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, predicate)
	projected, exact := transitionEnabledProjection(exists)
	if !exact || expressionContainsName(projected, "i") || countOperator(projected, "LE") != 4 {
		t.Fatalf("integer witness bounds projection = %v, exact=%t", projected, exact)
	}

	affine := MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), MustOperExpr(Op("PLUS"), Typed(IntT1), i, intLiteral(1)), x),
		MustOperExpr(Op("LT"), Typed(BoolT1), MustOperExpr(Op("MINUS"), Typed(IntT1), i, intLiteral(2)), x))
	if projected, exact = transitionEnabledProjection(MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, affine)); !exact || projected == nil || expressionContainsName(projected, "i") {
		t.Fatalf("affine disjunction projection = %v, exact=%t", projected, exact)
	}

	nonunit := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("MULT"), Typed(IntT1), i, intLiteral(2)), x)
	if projected, exact = transitionEnabledProjection(MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, nonunit)); !exact || projected == nil ||
		expressionContainsName(projected, "i") || countOperator(projected, "DIV") == 0 || countOperator(projected, "MOD") == 0 {
		t.Fatalf("non-unit affine binder projection = %v, exact=%t", projected, exact)
	}

	negative := MustOperExpr(Op("LT"), Typed(BoolT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1),
			MustOperExpr(Op("MULT"), Typed(IntT1), intLiteral(-3), i), intLiteral(2)), x)
	if projected, exact = transitionEnabledProjection(MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, negative)); !exact || projected == nil ||
		expressionContainsName(projected, "i") || countOperator(projected, "DIV") == 0 {
		t.Fatalf("negative affine binder projection = %v, exact=%t", projected, exact)
	}

	nonlinear := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("MULT"), Typed(IntT1), i, i), x)
	if projected, exact = transitionEnabledProjection(MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, nonlinear)); !exact || projected == nil || expressionContainsName(projected, "i") {
		t.Fatalf("nonlinear binder projection = %v, exact=%t", projected, exact)
	}

	symbolicDomain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)),
		NewNameExpr("a", Typed(IntT1)), NewNameExpr("b", Typed(IntT1)))
	if projected, exact = transitionEnabledProjection(MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, symbolicDomain, nonlinear)); !exact ||
		projected == nil || countOperator(projected, "ApalacheInternal!IntExists") != 1 {
		t.Fatalf("variable-width nonlinear binder projection = %v, exact=%t", projected, exact)
	}

	oversizedDomain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)),
		intLiteral(0), intLiteral(maxExpandedIntegerRange))
	if projected, exact = transitionEnabledProjection(MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, oversizedDomain, nonlinear)); !exact ||
		projected == nil || countOperator(projected, "ApalacheInternal!IntExists") != 1 {
		t.Fatalf("oversized nonlinear binder projection = %v, exact=%t", projected, exact)
	}
}

func TestVariableWidthNonlinearProjectionThroughG3(t *testing.T) {
	for _, test := range []struct {
		x    int64
		want bool
	}{{2, false}, {4, true}} {
		t.Run(fmt.Sprintf("x=%d", test.x), func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope := InitialRewriterScope()
			var err error
			scope, xCell, err := rewriter.Rewrite(intLiteral(test.x), scope)
			if err != nil {
				t.Fatal(err)
			}
			scope.Binding = scope.Binding.With("x", xCell)
			i := NewNameExpr("i", Typed(IntT1))
			predicate := MustOperExpr(Op("EQ"), Typed(BoolT1),
				MustOperExpr(Op("MULT"), Typed(IntT1), i, i), NewNameExpr("x", Typed(IntT1)))
			quantified := MustOperExpr(internalIntegerExistsOperator, Typed(BoolT1),
				i, intLiteral(0), NewNameExpr("x", Typed(IntT1)), predicate)
			scope, result, err := rewriter.Rewrite(quantified, scope)
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, test.want)
		})
	}
}

func TestNonUnitAffineWitnessProjectionExhaustive(t *testing.T) {
	i := NewNameExpr("i", Typed(IntT1))
	x := NewNameExpr("x", Typed(IntT1))
	comparisons := []string{"EQ", "LT", "LE", "GT", "GE"}
	for _, coefficient := range []int64{-3, -2, -1, 1, 2, 3} {
		for offset := int64(-2); offset <= 2; offset++ {
			affine := Expr(MustOperExpr(Op("MULT"), Typed(IntT1), intLiteral(coefficient), i))
			if offset != 0 {
				affine = MustOperExpr(Op("PLUS"), Typed(IntT1), affine, intLiteral(offset))
			}
			for lower := int64(-2); lower <= 2; lower++ {
				for upper := int64(-2); upper <= 2; upper++ {
					domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(lower), intLiteral(upper))
					for _, comparison := range comparisons {
						for _, reversed := range []bool{false, true} {
							left, right := affine, Expr(x)
							if reversed {
								left, right = right, left
							}
							predicate := MustOperExpr(Op(comparison), Typed(BoolT1), left, right)
							exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, predicate)
							projected, exact := transitionEnabledProjection(exists)
							if !exact {
								t.Fatalf("projection was inexact: coefficient=%d offset=%d range=%d..%d comparison=%s reversed=%v",
									coefficient, offset, lower, upper, comparison, reversed)
							}
							for stateValue := int64(-6); stateValue <= 6; stateValue++ {
								want := false
								for witness := lower; witness <= upper; witness++ {
									affineValue := coefficient*witness + offset
									lhs, rhs := affineValue, stateValue
									if reversed {
										lhs, rhs = rhs, lhs
									}
									if testIntegerComparison(comparison, lhs, rhs) {
										want = true
										break
									}
								}
								got, err := evaluateProjectedBoolean(projected, map[string]*big.Int{"x": big.NewInt(stateValue)})
								if err != nil || got != want {
									t.Fatalf("coefficient=%d offset=%d range=%d..%d comparison=%s reversed=%v x=%d: got %v, %v; want %v; projection=%v",
										coefficient, offset, lower, upper, comparison, reversed, stateValue, got, err, want, projected)
								}
							}
						}
					}
				}
			}
		}
	}
}

func testIntegerComparison(name string, left, right int64) bool {
	switch name {
	case "EQ":
		return left == right
	case "LT":
		return left < right
	case "LE":
		return left <= right
	case "GT":
		return left > right
	case "GE":
		return left >= right
	default:
		panic("unknown comparison " + name)
	}
}

func evaluateProjectedBoolean(expression Expr, names map[string]*big.Int) (bool, error) {
	if value, constant := boolConstant(expression); constant {
		return value, nil
	}
	operation, ok := expression.(*OperExpr)
	if !ok {
		return false, fmt.Errorf("unsupported Boolean expression %T", expression)
	}
	arguments := operation.Arguments()
	switch operation.Operator().Name() {
	case "AND", "OR":
		result := operation.Operator().Name() == "AND"
		for _, argument := range arguments {
			value, err := evaluateProjectedBoolean(argument, names)
			if err != nil {
				return false, err
			}
			if operation.Operator().Name() == "AND" {
				result = result && value
			} else {
				result = result || value
			}
		}
		return result, nil
	case "EQ", "LT", "LE", "GT", "GE":
		left, err := evaluateProjectedInteger(arguments[0], names)
		if err != nil {
			return false, err
		}
		right, err := evaluateProjectedInteger(arguments[1], names)
		if err != nil {
			return false, err
		}
		comparison := left.Cmp(right)
		switch operation.Operator().Name() {
		case "EQ":
			return comparison == 0, nil
		case "LT":
			return comparison < 0, nil
		case "LE":
			return comparison <= 0, nil
		case "GT":
			return comparison > 0, nil
		default:
			return comparison >= 0, nil
		}
	default:
		return false, fmt.Errorf("unsupported Boolean operator %s", operation.Operator().Name())
	}
}

func evaluateProjectedInteger(expression Expr, names map[string]*big.Int) (*big.Int, error) {
	if value, constant := constantInteger(expression); constant {
		return new(big.Int).Set(value), nil
	}
	if name, ok := expression.(*NameExpr); ok {
		value, found := names[name.Name()]
		if !found {
			return nil, fmt.Errorf("unknown name %s", name.Name())
		}
		return new(big.Int).Set(value), nil
	}
	operation, ok := expression.(*OperExpr)
	if !ok || len(operation.Arguments()) != 2 {
		return nil, fmt.Errorf("unsupported integer expression %T", expression)
	}
	left, err := evaluateProjectedInteger(operation.Arguments()[0], names)
	if err != nil {
		return nil, err
	}
	right, err := evaluateProjectedInteger(operation.Arguments()[1], names)
	if err != nil {
		return nil, err
	}
	switch operation.Operator().Name() {
	case "PLUS":
		return new(big.Int).Add(left, right), nil
	case "MINUS":
		return new(big.Int).Sub(left, right), nil
	case "DIV":
		return new(big.Int).Div(left, right), nil
	case "MOD":
		return new(big.Int).Mod(left, right), nil
	default:
		return nil, fmt.Errorf("unsupported integer operator %s", operation.Operator().Name())
	}
}
