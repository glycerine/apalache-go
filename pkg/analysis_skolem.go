package apalache

import "fmt"

// MarkSkolemizableExpression ports the positive-fragment portion of Scala's
// SkolemizationMarker. The input is expected to be in negation normal form.
func MarkSkolemizableExpression(expression Expr, tracker TransformationTracker) Expr {
	if expression == nil {
		panic("cannot mark a nil expression")
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var transform ExprTransform
	transform = tracker.TrackExpression(func(current Expr) Expr {
		operation, ok := current.(*OperExpr)
		if !ok {
			return current
		}
		arguments := operation.Arguments()
		switch operation.Operator().Name() {
		case "EXISTS3":
			if len(arguments) != 3 {
				return current
			}
			marked := MustOperExpr(operation.Operator(), operation.Tag(), arguments[0], arguments[1], transform(arguments[2]))
			return MustOperExpr(Op("Apalache!Skolem"), operation.Tag(), marked)
		case "FORALL3":
			if len(arguments) != 3 {
				return current
			}
			return MustOperExpr(operation.Operator(), operation.Tag(), arguments[0], arguments[1], transform(arguments[2]))
		case "NOT":
			return current
		case "AND", "OR":
			marked := make([]Expr, len(arguments))
			for index, argument := range arguments {
				marked[index] = transform(argument)
			}
			return MustOperExpr(operation.Operator(), operation.Tag(), marked...)
		case "IF_THEN_ELSE":
			if len(arguments) != 3 {
				return current
			}
			return MustOperExpr(operation.Operator(), operation.Tag(), arguments[0], transform(arguments[1]), transform(arguments[2]))
		case "LABEL":
			if len(arguments) == 0 {
				return current
			}
			marked := append([]Expr(nil), arguments...)
			marked[0] = transform(marked[0])
			return MustOperExpr(operation.Operator(), operation.Tag(), marked...)
		case "GE":
			if len(arguments) == 2 {
				if cardinality, ok := arguments[0].(*OperExpr); ok && cardinality.Operator().Name() == "FiniteSets!Cardinality" {
					if value, constant := constantInteger(arguments[1]); constant && value.IsInt64() {
						return MustOperExpr(Op("Apalache!ConstCardinality"), operation.Tag(), current)
					}
				}
			}
		}
		return current
	})
	return transform(expression)
}

func MarkSkolemizableModule(module *Module, tracker TransformationTracker) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot analyze a nil module")
	}
	result, err := transformModuleExpressionsWithError(module, "Skolemization analysis", nil,
		func(expression Expr) (Expr, error) { return MarkSkolemizableExpression(expression, tracker), nil })
	if err != nil {
		return nil, err
	}
	result.AddProperty(PropertyAnalyzed)
	return result, nil
}
