package apalache

import "fmt"

type Desugarer struct {
	names          *UniqueNameGenerator
	stateVariables map[string]struct{}
	tracker        TransformationTracker
	operatorDefs   map[string]Expr
}

func NewDesugarer(names *UniqueNameGenerator, stateVariables []string, tracker TransformationTracker, operatorDefs map[string]Expr) *Desugarer {
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	variables := make(map[string]struct{}, len(stateVariables))
	for _, name := range stateVariables {
		variables[name] = struct{}{}
	}
	return &Desugarer{names: names, stateVariables: variables, tracker: tracker, operatorDefs: cloneMap(operatorDefs)}
}

func (desugarer *Desugarer) Expression(expression Expr) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot desugar a nil expression")
	}
	var result Expr
	switch typed := expression.(type) {
	case *NameExpr, *ValueExpr, *nullExpr:
		return expression, nil
	case *LetInExpr:
		body, err := desugarer.Expression(typed.Body())
		if err != nil {
			return nil, err
		}
		declarations := typed.Declarations()
		for index, declaration := range declarations {
			newBody, bodyErr := desugarer.Expression(declaration.Body())
			if bodyErr != nil {
				return nil, bodyErr
			}
			copy := declaration.copyWithTag(declaration.Tag())
			copy.SetBody(newBody)
			declarations[index] = copy
		}
		result = NewLetInExpr(body, typed.Tag(), declarations...)
	case *OperExpr:
		arguments := typed.Arguments()
		switch typed.Operator().Name() {
		case "UNCHANGED":
			transformed := make([]Expr, len(arguments))
			for index, argument := range arguments {
				var err error
				transformed[index], err = desugarer.Expression(argument)
				if err != nil {
					return nil, err
				}
			}
			items := make([]Expr, 0)
			for _, argument := range transformed {
				items = append(items, desugarer.flattenUnchanged(argument)...)
			}
			equalities := make([]Expr, len(items))
			copier := NewDeepCopier(IdleTracker{})
			for index, item := range items {
				primedValue := copier.Expression(item)
				unprimedValue := copier.Expression(item)
				prime := MustOperExpr(Op("PRIME"), item.Tag(), primedValue)
				operator := "EQ"
				if name, ok := item.(*NameExpr); ok {
					if _, state := desugarer.stateVariables[name.Name()]; state {
						operator = "Apalache!:="
					}
				}
				equalities[index] = MustOperExpr(Op(operator), Typed(BoolT1), prime, unprimedValue)
			}
			if len(equalities) == 0 {
				result = NewValueExpr(BoolValue(true), Typed(BoolT1))
			} else if len(equalities) == 1 {
				result = equalities[0]
			} else {
				result = MustOperExpr(Op("AND"), Typed(BoolT1), equalities...)
			}
		case "STUTTER", "NO_STUTTER":
			body, err := desugarer.Expression(arguments[0])
			if err != nil {
				return nil, err
			}
			unchanged, err := desugarer.Expression(MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), arguments[1]))
			if err != nil {
				return nil, err
			}
			if typed.Operator().Name() == "STUTTER" {
				result = MustOperExpr(Op("OR"), Typed(BoolT1), body, unchanged)
			} else {
				result = MustOperExpr(Op("AND"), Typed(BoolT1), body, MustOperExpr(Op("NOT"), Typed(BoolT1), unchanged))
			}
		case "EQ", "NE":
			if len(arguments) == 2 {
				left, leftTuple := arguments[0].(*OperExpr)
				right, rightTuple := arguments[1].(*OperExpr)
				if leftTuple && rightTuple && left.Operator().Name() == "TUPLE" && right.Operator().Name() == "TUPLE" {
					leftArgs, rightArgs := left.Arguments(), right.Arguments()
					if len(leftArgs) != len(rightArgs) {
						result = NewValueExpr(BoolValue(typed.Operator().Name() == "NE"), Typed(BoolT1))
						break
					}
					comparisons := make([]Expr, len(leftArgs))
					comparisonName, connective := "EQ", "AND"
					if typed.Operator().Name() == "NE" {
						comparisonName, connective = "NE", "OR"
					}
					for index := range leftArgs {
						l, err := desugarer.Expression(leftArgs[index])
						if err != nil {
							return nil, err
						}
						r, err := desugarer.Expression(rightArgs[index])
						if err != nil {
							return nil, err
						}
						comparisons[index] = MustOperExpr(Op(comparisonName), Typed(BoolT1), l, r)
					}
					result = MustOperExpr(Op(connective), Typed(BoolT1), comparisons...)
					break
				}
			}
			var err error
			result, err = desugarer.rebuild(typed, arguments)
			if err != nil {
				return nil, err
			}
		case "LEADS_TO":
			antecedent, err := desugarer.Expression(arguments[0])
			if err != nil {
				return nil, err
			}
			consequent, err := desugarer.Expression(arguments[1])
			if err != nil {
				return nil, err
			}
			result = MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), MustOperExpr(Op("IMPLIES"), Typed(BoolT1), antecedent, MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1), consequent)))
		default:
			var err error
			result, err = desugarer.rebuild(typed, arguments)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported expression %T", expression)
	}
	return desugarer.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
}

func (desugarer *Desugarer) rebuild(original *OperExpr, arguments []Expr) (Expr, error) {
	for index, argument := range arguments {
		transformed, err := desugarer.Expression(argument)
		if err != nil {
			return nil, err
		}
		arguments[index] = transformed
	}
	return MustOperExpr(original.Operator(), original.Tag(), arguments...), nil
}

func (desugarer *Desugarer) flattenUnchanged(expression Expr) []Expr {
	if tuple, ok := expression.(*OperExpr); ok {
		if tuple.Operator().Name() == "TUPLE" {
			result := make([]Expr, 0)
			for _, argument := range tuple.Arguments() {
				result = append(result, desugarer.flattenUnchanged(argument)...)
			}
			return result
		}
		if tuple.Operator().Name() == "OPER_APP" {
			arguments := tuple.Arguments()
			if len(arguments) == 1 {
				if name, ok := arguments[0].(*NameExpr); ok {
					if definition, found := desugarer.operatorDefs[name.Name()]; found {
						return desugarer.flattenUnchanged(definition)
					}
				}
			}
		}
	}
	if _, literal := expression.(*ValueExpr); literal {
		return nil
	}
	return []Expr{expression}
}
