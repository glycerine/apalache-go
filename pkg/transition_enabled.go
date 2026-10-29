package apalache

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
		case "OR", "IF_THEN_ELSE", "LET_IN":
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

// projectIntegerRangeExistential eliminates a bounded integer witness when
// its predicate is a conjunction of state-only conditions and lower/upper
// bounds on the witness. Over integers, strict bounds are shifted by one.
func projectIntegerRangeExistential(binder string, domainLower, domainUpper, predicate Expr) (Expr, bool) {
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
		leftBinder := isProjectedBinder(arguments[0], binder)
		rightBinder := isProjectedBinder(arguments[1], binder)
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
		if rightBinder {
			other = arguments[0]
		}
		switch operation.Operator().Name() {
		case "EQ":
			lowers = append(lowers, other)
			uppers = append(uppers, other)
		case "GE":
			if leftBinder {
				lowers = append(lowers, other)
			} else {
				uppers = append(uppers, other)
			}
		case "GT":
			if leftBinder {
				lowers = append(lowers, shiftIntegerBound(other, 1))
			} else {
				uppers = append(uppers, shiftIntegerBound(other, -1))
			}
		case "LE":
			if leftBinder {
				uppers = append(uppers, other)
			} else {
				lowers = append(lowers, other)
			}
		case "LT":
			if leftBinder {
				uppers = append(uppers, shiftIntegerBound(other, -1))
			} else {
				lowers = append(lowers, shiftIntegerBound(other, 1))
			}
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

func isProjectedBinder(expression Expr, binder string) bool {
	name, ok := expression.(*NameExpr)
	return ok && name.Name() == binder
}

func shiftIntegerBound(expression Expr, amount int64) Expr {
	if amount == 0 {
		return expression
	}
	operator := "PLUS"
	value := amount
	if amount < 0 {
		operator = "MINUS"
		value = -amount
	}
	return MustOperExpr(Op(operator), Typed(IntT1), expression, NewValueExpr(Int64Value(value), Typed(IntT1)))
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
