package apalache

import "fmt"

func (builder *TypedBuilder) Eql(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("EQ"), left, right)
}

func (builder *TypedBuilder) Neql(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("NE"), left, right)
}

// ApplyOperator specializes a possibly polymorphic operator type against the
// actual argument types before constructing OPER_APP, matching appOp.
func (builder *TypedBuilder) ApplyOperator(operator Expr, arguments ...Expr) (Expr, error) {
	operatorTypeValue, err := ExpressionType(operator)
	if err != nil {
		return nil, err
	}
	operatorType, ok := operatorTypeValue.(OperatorType)
	if !ok {
		return nil, fmt.Errorf("operator application requires an operator type, found %v", operatorTypeValue)
	}
	argumentTypes, err := expressionTypes(arguments)
	if err != nil {
		return nil, err
	}
	mockType, err := NewOperatorType(argumentTypes, operatorType.Result())
	if err != nil {
		return nil, err
	}
	substitution, unified, compatible, err := NewTypeUnifier(NewTypeVarPool(0)).Unify(
		EmptySubstitution(), operatorType, mockType,
	)
	if err != nil {
		return nil, err
	}
	if !compatible {
		return nil, fmt.Errorf("operator application argument types %v do not unify with %v", argumentTypes, operatorType)
	}
	unifiedOperator, ok := unified.(OperatorType)
	if !ok {
		return nil, fmt.Errorf("operator application unified to non-operator type %v", unified)
	}
	retypedArguments := make([]Expr, len(arguments))
	for index, argument := range arguments {
		typ, applyErr := substitution.ApplyRecursive(argumentTypes[index])
		if applyErr != nil {
			return nil, applyErr
		}
		retypedArguments[index] = argument.WithTag(Typed(typ))
	}
	resultType, err := substitution.ApplyRecursive(operatorType.Result())
	if err != nil {
		return nil, err
	}

	if lambda, isLambda := operator.(*LetInExpr); isLambda {
		declarations := lambda.Declarations()
		name, isName := lambda.Body().(*NameExpr)
		if isName && len(declarations) == 1 && name.Name() == declarations[0].Name() {
			callee := name.WithTag(Typed(unifiedOperator))
			application, appErr := NewOperExpr(Op("OPER_APP"), Typed(resultType), append([]Expr{callee}, retypedArguments...)...)
			if appErr != nil {
				return nil, appErr
			}
			return NewLetInExpr(application, Typed(resultType), declarations...), nil
		}
	}
	retypedOperator := operator.WithTag(Typed(unifiedOperator))
	return NewOperExpr(Op("OPER_APP"), Typed(resultType), append([]Expr{retypedOperator}, retypedArguments...)...)
}

func (builder *TypedBuilder) ChooseIn(variable, set, predicate Expr) (*OperExpr, error) {
	if _, ok := variable.(*NameExpr); !ok {
		return nil, fmt.Errorf("bounded CHOOSE variable must be a name")
	}
	return builder.Apply(Op("CHOOSE3"), variable, set, predicate)
}

func (builder *TypedBuilder) Choose(variable, predicate Expr) (*OperExpr, error) {
	if _, ok := variable.(*NameExpr); !ok {
		return nil, fmt.Errorf("CHOOSE variable must be a name")
	}
	return builder.Apply(Op("CHOOSE2"), variable, predicate)
}

func (builder *TypedBuilder) Label(expression Expr, labels ...string) (*OperExpr, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("LABEL requires at least one label")
	}
	arguments := make([]Expr, 1, len(labels)+1)
	arguments[0] = expression
	for _, label := range labels {
		literal, err := builder.Str(label)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, literal)
	}
	return builder.Apply(Op("LABEL"), arguments...)
}

func (builder *ScopedBuilder) Eql(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("EQ"), left, right)
}

func (builder *ScopedBuilder) Neql(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("NE"), left, right)
}

func (builder *ScopedBuilder) ApplyOperator(operator ExprInstruction, arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		operatorExpr, err := operator.run(context)
		if err != nil {
			return nil, err
		}
		argumentExprs, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.ApplyOperator(operatorExpr, argumentExprs...)
	}}
}

func (builder *ScopedBuilder) ChooseIn(variable, set, predicate ExprInstruction) ExprInstruction {
	return builder.bindChoice(Op("CHOOSE3"), variable, set, predicate)
}

func (builder *ScopedBuilder) Choose(variable, predicate ExprInstruction) ExprInstruction {
	return builder.bindChoice(Op("CHOOSE2"), variable, ExprInstruction{}, predicate)
}

func (builder *ScopedBuilder) bindChoice(operator Operator, variable, set, predicate ExprInstruction) ExprInstruction {
	if operator.Name() == "CHOOSE2" {
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
				return nil, fmt.Errorf("CHOOSE variable must be a name")
			}
			if _, err = markBound(context, variableExpr); err != nil {
				return nil, err
			}
			if _, shadowed := differenceNameSets(boundAfterPredicate, boundBefore)[name.Name()]; shadowed {
				return nil, &ScopeError{Message: fmt.Sprintf("variable shadowing: %s", name.Name())}
			}
			return builder.typed.Choose(variableExpr, predicateExpr)
		}}
	}
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		usedBefore := cloneNameSet(context.used)
		boundBefore := boundNames(context)
		setExpr, err := set.run(context)
		if err != nil {
			return nil, err
		}
		usedAfterSet := cloneNameSet(context.used)
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
			return nil, fmt.Errorf("bounded CHOOSE variable must be a name")
		}
		if _, err = markBound(context, variableExpr); err != nil {
			return nil, err
		}
		usedInSet := differenceNameSets(usedAfterSet, usedBefore)
		newlyBoundInPredicate := differenceNameSets(boundAfterPredicate, boundBefore)
		for _, shadowed := range intersectionNameSets(map[string]struct{}{name.Name(): {}}, unionNameSets(usedInSet, newlyBoundInPredicate)) {
			return nil, &ScopeError{Message: fmt.Sprintf("variable shadowing: %s", shadowed)}
		}
		return builder.typed.ChooseIn(variableExpr, setExpr, predicateExpr)
	}}
}

func (builder *ScopedBuilder) Label(expression ExprInstruction, labels ...string) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		expressionValue, err := expression.run(context)
		if err != nil {
			return nil, err
		}
		return builder.typed.Label(expressionValue, labels...)
	}}
}
