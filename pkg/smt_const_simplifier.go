package apalache

// SMTConstantSimplifier is the SMT-aware, non-recursive specialization of the
// Scala constant simplifier. In particular, membership operands are preserved
// verbatim because the ground encoders recover their sorts from cell names.
type SMTConstantSimplifier struct{}

func (SMTConstantSimplifier) IsFalseConstant(expression Expr) bool {
	if value, ok := boolConstant(expression); ok {
		return !value
	}
	name, ok := expression.(*NameExpr)
	return ok && name.Name() == ArenaFalseName
}

func (SMTConstantSimplifier) IsTrueConstant(expression Expr) bool {
	if value, ok := boolConstant(expression); ok {
		return value
	}
	name, ok := expression.(*NameExpr)
	return ok && name.Name() == ArenaTrueName
}

func (simplifier SMTConstantSimplifier) IsBoolConstant(expression Expr) bool {
	return simplifier.IsFalseConstant(expression) || simplifier.IsTrueConstant(expression)
}

func (simplifier SMTConstantSimplifier) SimplifyShallow(expression Expr) (Expr, error) {
	if simplifier.IsFalseConstant(expression) {
		return boolExpr(false), nil
	}
	if simplifier.IsTrueConstant(expression) {
		return boolExpr(true), nil
	}
	operator, ok := expression.(*OperExpr)
	if !ok {
		return expression, nil
	}
	if _, membership := membershipPolarity(operator.Operator().Name()); membership {
		return expression, nil
	}
	if operator.Operator().Name() == "IF_THEN_ELSE" {
		arguments := operator.Arguments()
		if simplifier.IsTrueConstant(arguments[0]) {
			return arguments[1], nil
		}
		if simplifier.IsFalseConstant(arguments[0]) {
			return arguments[2], nil
		}
	}
	return simplifyConstantShallow(expression)
}
