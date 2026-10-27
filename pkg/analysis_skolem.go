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

// RemoveSkolemizationMarkers restores the formula that existed before the
// polarity-sensitive analysis pass. Consumers that reverse a formula's
// polarity must analyze the reversed formula again.
func RemoveSkolemizationMarkers(expression Expr, tracker TransformationTracker) Expr {
	return RewriteExpressions(tracker, func(current Expr) (Expr, bool) {
		operation, ok := current.(*OperExpr)
		if !ok || operation.Operator().Name() != "Apalache!Skolem" {
			return nil, false
		}
		arguments := operation.Arguments()
		if len(arguments) != 1 {
			return nil, false
		}
		return arguments[0], true
	})(expression)
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

// MarkSetExpansionsExpression propagates whether a consumer needs explicit
// arena elements and wraps compact powersets/function sets at that boundary.
func MarkSetExpansionsExpression(expression Expr, tracker TransformationTracker) Expr {
	if expression == nil {
		panic("cannot mark expansions in a nil expression")
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var transform func(Expr, bool) Expr
	transform = func(current Expr, expand bool) Expr {
		tracked := tracker.TrackExpression(func(input Expr) Expr {
			switch typed := input.(type) {
			case *OperExpr:
				arguments := typed.Arguments()
				rebuild := func(args []Expr) Expr { return MustOperExpr(typed.Operator(), typed.Tag(), args...) }
				switch typed.Operator().Name() {
				case "SET_POWERSET":
					if len(arguments) != 1 {
						return input
					}
					constructor := rebuild([]Expr{transform(arguments[0], true)})
					if expand {
						return MustOperExpr(Op("Apalache!Expand"), typed.Tag(), constructor)
					}
					return constructor
				case "FUN_SET":
					if len(arguments) != 2 {
						return input
					}
					constructor := rebuild([]Expr{transform(arguments[0], true), transform(arguments[1], false)})
					if expand {
						return MustOperExpr(Op("Apalache!Expand"), typed.Tag(), constructor)
					}
					return constructor
				case "Apalache!Skolem":
					if len(arguments) == 1 {
						if exists, ok := arguments[0].(*OperExpr); ok && exists.Operator().Name() == "EXISTS3" && len(exists.Arguments()) == 3 {
							existsArgs := exists.Arguments()
							marked := MustOperExpr(exists.Operator(), exists.Tag(), existsArgs[0],
								transform(existsArgs[1], false), transform(existsArgs[2], false))
							return rebuild([]Expr{marked})
						}
					}
				case "CHOOSE3":
					if len(arguments) == 3 {
						return rebuild([]Expr{arguments[0], transform(arguments[1], false), transform(arguments[2], expand)})
					}
				case "Apalache!Guess":
					if len(arguments) == 1 {
						return rebuild([]Expr{transform(arguments[0], false)})
					}
				case "EXISTS3", "FORALL3":
					if len(arguments) == 3 {
						return rebuild([]Expr{arguments[0], transform(arguments[1], true), transform(arguments[2], expand)})
					}
				case "SET_IN", "SET_NOT_IN", "Apalache!SelectInSet", "Apalache!SelectInFun", "Apalache!StoreInSet", "Apalache!storeNotInSet", "Apalache!storeNotInFun":
					if len(arguments) == 2 {
						return rebuild([]Expr{transform(arguments[0], true), transform(arguments[1], false)})
					}
				case "SET_UNION2", "SET_UNARY_UNION":
					marked := make([]Expr, len(arguments))
					for index, argument := range arguments {
						marked[index] = transform(argument, true)
					}
					return rebuild(marked)
				case "Apalache!ApaFoldSet":
					if len(arguments) == 3 {
						return rebuild([]Expr{transform(arguments[0], expand), transform(arguments[1], expand), transform(arguments[2], true)})
					}
				case "SET_FILTER":
					if len(arguments) == 3 {
						return rebuild([]Expr{arguments[0], transform(arguments[1], true), transform(arguments[2], expand)})
					}
				case "SET_MAP", "FUN_CTOR", "FUN_REC_CTOR":
					if len(arguments) != 0 {
						marked := make([]Expr, len(arguments))
						marked[0] = transform(arguments[0], expand)
						for index := 1; index < len(arguments); index++ {
							marked[index] = transform(arguments[index], true)
						}
						return rebuild(marked)
					}
				}
				marked := make([]Expr, len(arguments))
				for index, argument := range arguments {
					marked[index] = transform(argument, expand)
				}
				return rebuild(marked)
			case *LetInExpr:
				declarations := typed.Declarations()
				for index, declaration := range declarations {
					copy := declaration.copyWithTag(declaration.Tag())
					copy.SetBody(transform(declaration.Body(), expand))
					declarations[index] = copy
				}
				return NewLetInExpr(transform(typed.Body(), expand), typed.Tag(), declarations...)
			default:
				return input
			}
		})
		return tracked(current)
	}
	return transform(expression, false)
}

func MarkSetExpansionsModule(module *Module, tracker TransformationTracker) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot mark expansions in a nil module")
	}
	return transformModuleExpressionsWithError(module, "set expansion analysis", nil,
		func(expression Expr) (Expr, error) { return MarkSetExpansionsExpression(expression, tracker), nil })
}
