package apalache

import "fmt"

type ParameterNormalizationDecision func(*OperDecl) bool

func NormalizeAllParameters(*OperDecl) bool            { return true }
func NormalizeRecursiveParameters(decl *OperDecl) bool { return decl.IsRecursive() }

type ParameterNormalizer struct {
	names    *UniqueNameGenerator
	tracker  TransformationTracker
	decision ParameterNormalizationDecision
}

func NewParameterNormalizer(names *UniqueNameGenerator, tracker TransformationTracker, decision ParameterNormalizationDecision) *ParameterNormalizer {
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	if decision == nil {
		decision = NormalizeRecursiveParameters
	}
	return &ParameterNormalizer{names: names, tracker: tracker, decision: decision}
}

func (normalizer *ParameterNormalizer) Declaration(declaration *OperDecl) (*OperDecl, error) {
	if declaration == nil {
		return nil, fmt.Errorf("cannot normalize a nil declaration")
	}
	body, err := normalizer.internalLets(declaration.Body())
	if err != nil {
		return nil, err
	}
	typ, typed := declaration.Tag().Type()
	operatorType, ok := typ.(OperatorType)
	if !typed || !ok {
		return nil, fmt.Errorf("operator %s has an invalid type tag: %s", declaration.Name(), declaration.Tag())
	}
	parameterTypes := operatorType.Arguments()
	parameters := declaration.FormalParams()
	if len(parameterTypes) != len(parameters) {
		return nil, fmt.Errorf("the signature of operator %s has %d parameters whereas the type tag has %d parameters", declaration.Name(), len(parameters), len(parameterTypes))
	}
	if normalizer.decision(declaration) {
		body, err = normalizer.parameters(body, parameters, parameterTypes)
		if err != nil {
			return nil, err
		}
	}
	result := NewOperDecl(declaration.Name(), parameters, body, declaration.Tag())
	result.SetRecursive(declaration.IsRecursive())
	return result, nil
}

func (normalizer *ParameterNormalizer) Module(module *Module) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot normalize a nil module")
	}
	declarations := module.Declarations()
	for index, declaration := range declarations {
		if operator, ok := declaration.(*OperDecl); ok {
			normalized, err := normalizer.Declaration(operator)
			if err != nil {
				return nil, err
			}
			declarations[index] = normalized
		}
	}
	result := NewModule(module.Name(), declarations)
	result.properties = cloneMap(module.properties)
	return result, nil
}

func (normalizer *ParameterNormalizer) internalLets(expression Expr) (Expr, error) {
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		changed := false
		for index, argument := range arguments {
			newArgument, err := normalizer.internalLets(argument)
			if err != nil {
				return nil, err
			}
			arguments[index] = newArgument
			changed = changed || newArgument.ID() != argument.ID()
		}
		if !changed {
			return expression, nil
		}
		result := MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
		return normalizer.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
	case *LetInExpr:
		declarations := typed.Declarations()
		changed := false
		for index, declaration := range declarations {
			normalized, err := normalizer.Declaration(declaration)
			if err != nil {
				return nil, err
			}
			declarations[index] = normalized
			changed = changed || !EqualTypedDecl(normalized, declaration) || normalized.IsRecursive() != declaration.IsRecursive()
		}
		body, err := normalizer.internalLets(typed.Body())
		if err != nil {
			return nil, err
		}
		changed = changed || body.ID() != typed.Body().ID()
		if !changed {
			return expression, nil
		}
		result := NewLetInExpr(body, typed.Tag(), declarations...)
		return normalizer.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
	default:
		return expression, nil
	}
}

func (normalizer *ParameterNormalizer) parameters(expression Expr, parameters []OperatorParam, types []Type) (Expr, error) {
	result := expression
	for index, parameter := range parameters {
		parameterType := types[index]
		newName := normalizer.names.NewName()
		if parameter.Arity <= 0 {
			operatorType := mustOperator(nil, parameterType)
			replacement := func() Expr {
				return MustOperExpr(Op("OPER_APP"), Typed(parameterType), NewNameExpr(newName, Typed(operatorType)))
			}
			result = ReplaceEqual(normalizer.tracker, NewNameExpr(parameter.Name, Typed(parameterType)), replacement)(result)
			definition := NewOperDecl(newName, nil, NewNameExpr(parameter.Name, Typed(parameterType)), Typed(operatorType))
			result = NewLetInExpr(result, result.Tag(), definition)
			continue
		}
		higherOrder, ok := parameterType.(OperatorType)
		if !ok || len(higherOrder.Arguments()) != parameter.Arity {
			return nil, fmt.Errorf("expected a higher-order parameter %s of arity %d, found type: %s", parameter.Name, parameter.Arity, parameterType)
		}
		result = ReplaceEqual(normalizer.tracker, NewNameExpr(parameter.Name, Typed(parameterType)), func() Expr {
			return NewNameExpr(newName, Typed(parameterType))
		})(result)
		argumentTypes := higherOrder.Arguments()
		freshParameters := make([]OperatorParam, parameter.Arity)
		applicationArguments := make([]Expr, 0, parameter.Arity+1)
		applicationArguments = append(applicationArguments, NewNameExpr(parameter.Name, Typed(parameterType)))
		for argumentIndex := range parameter.Arity {
			fresh := normalizer.names.NewName()
			freshParameters[argumentIndex] = ValueParam(fresh)
			applicationArguments = append(applicationArguments, NewNameExpr(fresh, Typed(argumentTypes[argumentIndex])))
		}
		definitionBody := MustOperExpr(Op("OPER_APP"), Typed(higherOrder.Result()), applicationArguments...)
		definition := NewOperDecl(newName, freshParameters, definitionBody, Typed(parameterType))
		result = NewLetInExpr(result, result.Tag(), definition)
	}
	if result.ID() == expression.ID() {
		return result, nil
	}
	return normalizer.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
}
