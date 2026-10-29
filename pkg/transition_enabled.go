package apalache

import "math/big"

func projectedTransitionEnabledness(
	transitions []Expr,
	actionConstraints, stateConstraints []namedExpression,
	variables []*VarDecl,
) []Expr {
	variableNames := make(map[string]struct{}, len(variables))
	for _, variable := range variables {
		variableNames[variable.Name()] = struct{}{}
	}
	constraints := make([]Expr, 0, len(actionConstraints)+len(stateConstraints))
	for _, constraint := range actionConstraints {
		constraints = append(constraints, constraint.expression)
	}
	for _, constraint := range stateConstraints {
		primed := MustOperExpr(Op("PRIME"), constraint.expression.Tag(), constraint.expression)
		constraints = append(constraints, PropagatePrimes(nil, variableNames)(primed))
	}
	projected := make([]Expr, len(transitions))
	for index, transition := range transitions {
		combined := transition
		if len(constraints) != 0 {
			arguments := make([]Expr, 0, len(constraints)+1)
			arguments = append(arguments, transition)
			arguments = append(arguments, constraints...)
			combined = booleanAnd(arguments...)
		}
		var exact bool
		projected[index], exact = transitionEnabledProjection(combined)
		if !exact {
			return nil
		}
	}
	return projected
}

// transitionEnabledProjection removes deterministic assignments from a
// normalized transition while retaining the state predicate under which the
// assignment can fire. The Boolean result is exact only for the deliberately
// small fragment recognized here; callers must fall back to ordinary
// transition satisfiability when exact is false.
func transitionEnabledProjection(expression Expr) (Expr, bool) {
	projected, exact := projectEnabled(expression, make(map[string]Expr))
	if !exact || expressionContainsPrime(projected) {
		return nil, false
	}
	return projected, true
}

func projectEnabled(expression Expr, primed map[string]Expr) (Expr, bool) {
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		name := typed.Operator().Name()
		if name == "Apalache!Skolem" && len(arguments) == 1 {
			return projectEnabled(arguments[0], primed)
		}
		if name == "Apalache!:=" && len(arguments) == 2 && isPrimedName(arguments[0]) {
			variable := arguments[0].(*OperExpr).Arguments()[0].(*NameExpr).Name()
			right, exact := replaceProjectedPrimes(arguments[1], primed)
			if !exact {
				return nil, false
			}
			primed[variable] = right
			return boolExpr(true), true
		}
		switch name {
		case "AND":
			// An assignment inside a disjunction produces a different primed
			// environment per branch. Distribute the surrounding conjunction so
			// later guards and target constraints are substituted in that branch.
			for disjunctionIndex, argument := range arguments {
				disjunction, ok := argument.(*OperExpr)
				if !ok || disjunction.Operator().Name() != "OR" || !expressionContainsAssignment(argument) {
					continue
				}
				branches := make([]Expr, len(disjunction.Arguments()))
				for branchIndex, branch := range disjunction.Arguments() {
					terms := append([]Expr(nil), arguments...)
					terms[disjunctionIndex] = branch
					var exact bool
					branches[branchIndex], exact = projectEnabled(booleanAnd(terms...), cloneMap(primed))
					if !exact {
						return nil, false
					}
				}
				return booleanOr(branches...), true
			}
			projected := make([]Expr, 0, len(arguments))
			for _, argument := range arguments {
				item, exact := projectEnabled(argument, primed)
				if !exact {
					return nil, false
				}
				if value, constant := boolConstant(item); constant {
					if !value {
						return boolExpr(false), true
					}
					continue
				}
				projected = append(projected, item)
			}
			return booleanAnd(projected...), true
		case "OR":
			projected := make([]Expr, len(arguments))
			for index, argument := range arguments {
				var exact bool
				projected[index], exact = projectEnabled(argument, cloneMap(primed))
				if !exact {
					return nil, false
				}
			}
			return booleanOr(projected...), true
		case "LABEL":
			if len(arguments) == 0 {
				return nil, false
			}
			return projectEnabled(arguments[0], primed)
		case "EXISTS3":
			if len(arguments) != 3 {
				return nil, false
			}
			inside := cloneMap(primed)
			body, exact := projectEnabled(arguments[2], inside)
			if !exact {
				return nil, false
			}
			if domain, integerRange := arguments[1].(*OperExpr); integerRange &&
				domain.Operator().Name() == "INT_RANGE" && len(domain.Arguments()) == 2 {
				binder, named := arguments[0].(*NameExpr)
				if !named {
					return nil, false
				}
				return projectIntegerRangeExistential(binder.Name(), domain.Arguments()[0], domain.Arguments()[1], body)
			}
			if value, constant := boolConstant(body); constant && value {
				return projectedSetNonempty(arguments[1])
			}
			// Keeping a general quantified predicate would require eliminating
			// its witness before negating enabledness for the deadlock query.
			return nil, false
		case "IF_THEN_ELSE", "LET_IN":
			// Transition slicing normally removes these assignment branches.
			// Refuse any survivors rather than approximate their bindings.
			return nil, false
		default:
			return replaceProjectedPrimes(expression, primed)
		}
	case *LetInExpr:
		return nil, false
	default:
		return expression, true
	}
}

func expressionContainsAssignment(expression Expr) bool {
	found := false
	RewriteExpressions(nil, func(current Expr) (Expr, bool) {
		operation, ok := current.(*OperExpr)
		if ok && operation.Operator().Name() == "Apalache!:=" {
			found = true
		}
		return nil, false
	})(expression)
	return found
}

// projectIntegerRangeExistential eliminates a bounded integer witness when
// its predicate is a conjunction of state-only conditions and lower/upper
// bounds on the witness. Over integers, strict bounds are shifted by one.
func projectIntegerRangeExistential(binder string, domainLower, domainUpper, predicate Expr) (Expr, bool) {
	if disjunction, ok := predicate.(*OperExpr); ok && disjunction.Operator().Name() == "OR" {
		branches := make([]Expr, len(disjunction.Arguments()))
		for index, branch := range disjunction.Arguments() {
			var exact bool
			branches[index], exact = projectIntegerRangeExistential(binder, domainLower, domainUpper, branch)
			if !exact {
				return nil, false
			}
		}
		return booleanOr(branches...), true
	}
	lowers := []Expr{domainLower}
	uppers := []Expr{domainUpper}
	conditions := make([]Expr, 0)
	terms := []Expr{predicate}
	if conjunction, ok := predicate.(*OperExpr); ok && conjunction.Operator().Name() == "AND" {
		terms = conjunction.Arguments()
	}
	for _, term := range terms {
		if value, constant := boolConstant(term); constant {
			if !value {
				return boolExpr(false), true
			}
			continue
		}
		operation, comparison := term.(*OperExpr)
		if !comparison || len(operation.Arguments()) != 2 {
			if expressionContainsName(term, binder) {
				return nil, false
			}
			conditions = append(conditions, term)
			continue
		}
		arguments := operation.Arguments()
		leftCoefficient, leftOffset, leftBinder := projectedBinderAffine(arguments[0], binder)
		rightCoefficient, rightOffset, rightBinder := projectedBinderAffine(arguments[1], binder)
		if !leftBinder && !rightBinder {
			if expressionContainsName(term, binder) {
				return nil, false
			}
			conditions = append(conditions, term)
			continue
		}
		if leftBinder == rightBinder || expressionContainsName(arguments[1], binder) && leftBinder ||
			expressionContainsName(arguments[0], binder) && rightBinder {
			return nil, false
		}
		other := arguments[1]
		coefficient := leftCoefficient
		offset := leftOffset
		comparisonName := operation.Operator().Name()
		if rightBinder {
			other = arguments[0]
			coefficient = rightCoefficient
			offset = rightOffset
			comparisonName = reverseIntegerComparison(comparisonName)
		}
		rightHandSide := shiftIntegerBoundBig(other, new(big.Int).Neg(offset))
		if coefficient.Sign() < 0 {
			coefficient = new(big.Int).Neg(coefficient)
			comparisonName = reverseIntegerComparison(comparisonName)
			rightHandSide = negateIntegerBound(rightHandSide)
		}
		if coefficient.Sign() == 0 {
			return nil, false
		}
		floor := func(value Expr) Expr { return divideIntegerBound(value, coefficient) }
		ceil := func(value Expr) Expr {
			return divideIntegerBound(shiftIntegerBoundBig(value, new(big.Int).Sub(coefficient, big.NewInt(1))), coefficient)
		}
		switch comparisonName {
		case "EQ":
			quotient := floor(rightHandSide)
			lowers = append(lowers, quotient)
			uppers = append(uppers, quotient)
			if coefficient.Cmp(big.NewInt(1)) != 0 {
				remainder := MustOperExpr(Op("MOD"), Typed(IntT1), rightHandSide, integerBoundLiteral(coefficient))
				conditions = append(conditions, MustOperExpr(Op("EQ"), Typed(BoolT1), remainder, intLiteral(0)))
			}
			if coefficient.Cmp(big.NewInt(1)) != 0 {
				conditions = append(conditions, MustOperExpr(Op("EQ"), Typed(BoolT1),
					MustOperExpr(Op("MOD"), Typed(IntT1), rightHandSide, integerBoundLiteral(coefficient)), intLiteral(0)))
			}
		case "GE":
			lowers = append(lowers, ceil(rightHandSide))
		case "GT":
			lowers = append(lowers, ceil(shiftIntegerBound(rightHandSide, 1)))
		case "LE":
			uppers = append(uppers, floor(rightHandSide))
		case "LT":
			uppers = append(uppers, floor(shiftIntegerBound(rightHandSide, -1)))
		default:
			return nil, false
		}
	}
	for _, lower := range lowers {
		for _, upper := range uppers {
			conditions = append(conditions, MustOperExpr(Op("LE"), Typed(BoolT1), lower, upper))
		}
	}
	return booleanAnd(conditions...), true
}

func projectedBinderAffine(expression Expr, binder string) (*big.Int, *big.Int, bool) {
	if name, ok := expression.(*NameExpr); ok && name.Name() == binder {
		return big.NewInt(1), new(big.Int), true
	}
	operation, ok := expression.(*OperExpr)
	if !ok || len(operation.Arguments()) != 2 {
		return nil, nil, false
	}
	arguments := operation.Arguments()
	switch operation.Operator().Name() {
	case "PLUS":
		if offset, constant := constantInteger(arguments[1]); constant {
			if coefficient, base, binderExpression := projectedBinderAffine(arguments[0], binder); binderExpression {
				return coefficient, new(big.Int).Add(base, offset), true
			}
		}
		if offset, constant := constantInteger(arguments[0]); constant {
			if coefficient, base, binderExpression := projectedBinderAffine(arguments[1], binder); binderExpression {
				return coefficient, new(big.Int).Add(base, offset), true
			}
		}
	case "MINUS":
		if offset, constant := constantInteger(arguments[1]); constant {
			if coefficient, base, binderExpression := projectedBinderAffine(arguments[0], binder); binderExpression {
				return coefficient, new(big.Int).Sub(base, offset), true
			}
		}
		if offset, constant := constantInteger(arguments[0]); constant {
			if coefficient, base, binderExpression := projectedBinderAffine(arguments[1], binder); binderExpression {
				return new(big.Int).Neg(coefficient), new(big.Int).Sub(offset, base), true
			}
		}
	case "MULT":
		if factor, constant := constantInteger(arguments[1]); constant && factor.Sign() != 0 {
			if coefficient, base, binderExpression := projectedBinderAffine(arguments[0], binder); binderExpression {
				return new(big.Int).Mul(coefficient, factor), new(big.Int).Mul(base, factor), true
			}
		}
		if factor, constant := constantInteger(arguments[0]); constant && factor.Sign() != 0 {
			if coefficient, base, binderExpression := projectedBinderAffine(arguments[1], binder); binderExpression {
				return new(big.Int).Mul(coefficient, factor), new(big.Int).Mul(base, factor), true
			}
		}
	}
	return nil, nil, false
}

func reverseIntegerComparison(name string) string {
	switch name {
	case "LT":
		return "GT"
	case "LE":
		return "GE"
	case "GT":
		return "LT"
	case "GE":
		return "LE"
	default:
		return name
	}
}

func integerBoundLiteral(value *big.Int) Expr {
	return NewValueExpr(NewIntValue(new(big.Int).Set(value)), Typed(IntT1))
}

func divideIntegerBound(value Expr, divisor *big.Int) Expr {
	if divisor.Cmp(big.NewInt(1)) == 0 {
		return value
	}
	return MustOperExpr(Op("DIV"), Typed(IntT1), value, integerBoundLiteral(divisor))
}

func negateIntegerBound(value Expr) Expr {
	return MustOperExpr(Op("MINUS"), Typed(IntT1), intLiteral(0), value)
}

func shiftIntegerBound(expression Expr, amount int64) Expr {
	return shiftIntegerBoundBig(expression, big.NewInt(amount))
}

func shiftIntegerBoundBig(expression Expr, amount *big.Int) Expr {
	if amount.Sign() == 0 {
		return expression
	}
	operator := "PLUS"
	value := new(big.Int).Set(amount)
	if amount.Sign() < 0 {
		operator = "MINUS"
		value.Neg(value)
	}
	return MustOperExpr(Op(operator), Typed(IntT1), expression, NewValueExpr(NewIntValue(value), Typed(IntT1)))
}

func projectedSetNonempty(set Expr) (Expr, bool) {
	operation, ok := set.(*OperExpr)
	if !ok {
		return nil, false
	}
	switch operation.Operator().Name() {
	case "SET_ENUM":
		return boolExpr(len(operation.Arguments()) != 0), true
	case "INT_RANGE":
		arguments := operation.Arguments()
		if len(arguments) != 2 {
			return nil, false
		}
		return MustOperExpr(Op("LE"), Typed(BoolT1), arguments[0], arguments[1]), true
	default:
		return nil, false
	}
}

func replaceProjectedPrimes(expression Expr, primed map[string]Expr) (Expr, bool) {
	exact := true
	replaced := RewriteExpressions(nil, func(current Expr) (Expr, bool) {
		if !isPrimedName(current) {
			return nil, false
		}
		name := current.(*OperExpr).Arguments()[0].(*NameExpr).Name()
		value, found := primed[name]
		if !found {
			exact = false
			return nil, false
		}
		return value, true
	})(expression)
	return replaced, exact
}

func expressionContainsPrime(expression Expr) bool {
	found := false
	RewriteExpressions(nil, func(current Expr) (Expr, bool) {
		if isPrimedName(current) {
			found = true
		}
		return nil, false
	})(expression)
	return found
}
