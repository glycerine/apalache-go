package apalache

import "fmt"

// Typed temporal builders mirror UnsafeTemporalBuilder and validate
// applications through the shared operator signatures.
func (builder *TypedBuilder) Box(predicate Expr) (*OperExpr, error) {
	return builder.Apply(Op("GLOBALLY"), predicate)
}

func (builder *TypedBuilder) Diamond(predicate Expr) (*OperExpr, error) {
	return builder.Apply(Op("EVENTUALLY"), predicate)
}

func (builder *TypedBuilder) LeadsTo(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("LEADS_TO"), left, right)
}

func (builder *TypedBuilder) Guarantees(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("GUARANTEES"), left, right)
}

func (builder *TypedBuilder) WF(expression, action Expr) (*OperExpr, error) {
	return builder.Apply(Op("WEAK_FAIRNESS"), expression, action)
}

func (builder *TypedBuilder) SF(expression, action Expr) (*OperExpr, error) {
	return builder.Apply(Op("STRONG_FAIRNESS"), expression, action)
}

func (builder *TypedBuilder) EE(variable, predicate Expr) (*OperExpr, error) {
	if _, ok := variable.(*NameExpr); !ok {
		return nil, fmt.Errorf("temporal existential variable must be a name")
	}
	return builder.Apply(Op("TEMPORAL_EXISTS"), variable, predicate)
}

func (builder *TypedBuilder) AA(variable, predicate Expr) (*OperExpr, error) {
	if _, ok := variable.(*NameExpr); !ok {
		return nil, fmt.Errorf("temporal universal variable must be a name")
	}
	return builder.Apply(Op("TEMPORAL_FORALL"), variable, predicate)
}

func (builder *ScopedBuilder) Box(predicate ExprInstruction) ExprInstruction {
	return builder.Apply(Op("GLOBALLY"), predicate)
}

func (builder *ScopedBuilder) Diamond(predicate ExprInstruction) ExprInstruction {
	return builder.Apply(Op("EVENTUALLY"), predicate)
}

func (builder *ScopedBuilder) LeadsTo(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("LEADS_TO"), left, right)
}

func (builder *ScopedBuilder) Guarantees(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("GUARANTEES"), left, right)
}

func (builder *ScopedBuilder) WF(expression, action ExprInstruction) ExprInstruction {
	return builder.Apply(Op("WEAK_FAIRNESS"), expression, action)
}

func (builder *ScopedBuilder) SF(expression, action ExprInstruction) ExprInstruction {
	return builder.Apply(Op("STRONG_FAIRNESS"), expression, action)
}

func (builder *ScopedBuilder) EE(variable, predicate ExprInstruction) ExprInstruction {
	return builder.bindTemporal(Op("TEMPORAL_EXISTS"), variable, predicate)
}

func (builder *ScopedBuilder) AA(variable, predicate ExprInstruction) ExprInstruction {
	return builder.bindTemporal(Op("TEMPORAL_FORALL"), variable, predicate)
}

func (builder *ScopedBuilder) bindTemporal(operator Operator, variable, predicate ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		boundBefore := boundNames(context)
		predicateExpr, err := predicate.run(context)
		if err != nil {
			return nil, err
		}
		boundAfterPredicate := boundNames(context)
		variableExpr, err := variable.run(context)
		if err != nil {
			return nil, err
		}
		name, ok := variableExpr.(*NameExpr)
		if !ok {
			return nil, fmt.Errorf("temporal quantified variable must be a name")
		}
		if _, err = markBound(context, variableExpr); err != nil {
			return nil, err
		}
		if _, shadowed := differenceNameSets(boundAfterPredicate, boundBefore)[name.Name()]; shadowed {
			return nil, &ScopeError{Message: fmt.Sprintf("variable shadowing: %s", name.Name())}
		}
		return builder.typed.Apply(operator, variableExpr, predicateExpr)
	}}
}
