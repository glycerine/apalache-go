package apalache

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
