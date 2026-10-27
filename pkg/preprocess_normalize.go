package apalache

import "fmt"

// NormalizeNNF converts Boolean structure to negation-normal form.
func NormalizeNNF(expression Expr, tracker TransformationTracker) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot normalize a nil expression")
	}
	if err := WatchExpression(FlatLanguagePredicate{}, expression); err != nil {
		return nil, err
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var nnf func(Expr, bool) (Expr, error)
	nnf = func(current Expr, negated bool) (Expr, error) {
		var result Expr
		switch typed := current.(type) {
		case *ValueExpr:
			if boolean, ok := typed.Value().(BoolValue); ok {
				result = NewValueExpr(BoolValue(bool(boolean) != negated), typed.Tag())
			} else {
				result = current
			}
		case *NameExpr:
			if negated {
				result = MustOperExpr(Op("NOT"), typed.Tag(), typed)
			} else {
				result = typed
			}
		case *nullExpr:
			if negated {
				result = MustOperExpr(Op("NOT"), typed.Tag(), typed)
			} else {
				result = typed
			}
		case *LetInExpr:
			declarations := typed.Declarations()
			for index, declaration := range declarations {
				body, err := nnf(declaration.Body(), false)
				if err != nil {
					return nil, err
				}
				copy := declaration.copyWithTag(declaration.Tag())
				copy.SetBody(body)
				declarations[index] = copy
			}
			body, err := nnf(typed.Body(), negated)
			if err != nil {
				return nil, err
			}
			if negated {
				body, declarations, err = normalizeNegatedLet(body, declarations, nnf)
				if err != nil {
					return nil, err
				}
			}
			result = NewLetInExpr(body, typed.Tag(), declarations...)
		case *OperExpr:
			arguments := typed.Arguments()
			name := typed.Operator().Name()
			switch name {
			case "NOT":
				return nnf(arguments[0], !negated)
			case "IF_THEN_ELSE":
				thenExpr, err := nnf(arguments[1], negated)
				if err != nil {
					return nil, err
				}
				elseExpr, err := nnf(arguments[2], negated)
				if err != nil {
					return nil, err
				}
				result = MustOperExpr(typed.Operator(), typed.Tag(), arguments[0], thenExpr, elseExpr)
			case "AND", "OR":
				operator := name
				if negated {
					if name == "AND" {
						operator = "OR"
					} else {
						operator = "AND"
					}
				}
				for index, argument := range arguments {
					var err error
					arguments[index], err = nnf(argument, negated)
					if err != nil {
						return nil, err
					}
				}
				result = MustOperExpr(Op(operator), typed.Tag(), arguments...)
			case "IMPLIES":
				left, err := nnf(arguments[0], !negated)
				if err != nil {
					return nil, err
				}
				right, err := nnf(arguments[1], negated)
				if err != nil {
					return nil, err
				}
				operator := "OR"
				if negated {
					operator = "AND"
					left, err = nnf(arguments[0], false)
					if err != nil {
						return nil, err
					}
					right, err = nnf(arguments[1], true)
					if err != nil {
						return nil, err
					}
				}
				result = MustOperExpr(Op(operator), typed.Tag(), left, right)
			case "EQUIV":
				left, err := nnf(arguments[0], false)
				if err != nil {
					return nil, err
				}
				right, err := nnf(arguments[1], false)
				if err != nil {
					return nil, err
				}
				operator := "EQ"
				if negated {
					operator = "NE"
				}
				result = MustOperExpr(Op(operator), typed.Tag(), left, right)
			case "EXISTS3", "FORALL3":
				set, err := nnf(arguments[1], false)
				if err != nil {
					return nil, err
				}
				predicate, err := nnf(arguments[2], negated)
				if err != nil {
					return nil, err
				}
				operator := name
				if negated {
					if name == "EXISTS3" {
						operator = "FORALL3"
					} else {
						operator = "EXISTS3"
					}
				}
				result = MustOperExpr(Op(operator), typed.Tag(), arguments[0], set, predicate)
			case "LT", "LE", "GT", "GE":
				if !negated {
					result = current
				} else {
					complement := map[string]string{"LT": "GE", "LE": "GT", "GT": "LE", "GE": "LT"}[name]
					result = MustOperExpr(Op(complement), typed.Tag(), arguments...)
				}
			case "EQ", "NE", "SET_IN", "SET_NOT_IN":
				left, err := nnf(arguments[0], false)
				if err != nil {
					return nil, err
				}
				right, err := nnf(arguments[1], false)
				if err != nil {
					return nil, err
				}
				base := name
				inherentlyNegated := false
				if name == "NE" {
					base, inherentlyNegated = "EQ", true
				}
				if name == "SET_NOT_IN" {
					base, inherentlyNegated = "SET_IN", true
				}
				baseExpr := MustOperExpr(Op(base), typed.Tag(), left, right)
				if negated != inherentlyNegated {
					result = MustOperExpr(Op("NOT"), typed.Tag(), baseExpr)
				} else {
					result = baseExpr
				}
			case "LABEL":
				first, err := nnf(arguments[0], negated)
				if err != nil {
					return nil, err
				}
				arguments[0] = first
				result = MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			case "EVENTUALLY", "GLOBALLY":
				operator := name
				childNegated := false
				if negated {
					childNegated = true
					if name == "EVENTUALLY" {
						operator = "GLOBALLY"
					} else {
						operator = "EVENTUALLY"
					}
				}
				for index, argument := range arguments {
					var err error
					arguments[index], err = nnf(argument, childNegated)
					if err != nil {
						return nil, err
					}
				}
				result = MustOperExpr(Op(operator), typed.Tag(), arguments...)
			case "LEADS_TO", "GUARANTEES", "WEAK_FAIRNESS", "STRONG_FAIRNESS":
				if negated {
					result = MustOperExpr(Op("NOT"), typed.Tag(), current)
				} else {
					for index, argument := range arguments {
						var err error
						arguments[index], err = nnf(argument, false)
						if err != nil {
							return nil, err
						}
					}
					result = MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
				}
			default:
				for index, argument := range arguments {
					var err error
					arguments[index], err = nnf(argument, false)
					if err != nil {
						return nil, err
					}
				}
				base := MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
				if negated {
					result = MustOperExpr(Op("NOT"), typed.Tag(), base)
				} else {
					result = base
				}
			}
		default:
			return nil, fmt.Errorf("unsupported expression %T", current)
		}
		return tracker.TrackExpression(func(Expr) Expr { return result })(current), nil
	}
	return nnf(expression, false)
}

func normalizeNegatedLet(body Expr, declarations []*OperDecl, nnf func(Expr, bool) (Expr, error)) (Expr, []*OperDecl, error) {
	negatedNames := make(map[string]struct{})
	var find func(Expr)
	find = func(expression Expr) {
		switch typed := expression.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			if typed.Operator().Name() == "NOT" && len(arguments) == 1 {
				if app, ok := arguments[0].(*OperExpr); ok && app.Operator().Name() == "OPER_APP" && len(app.Arguments()) == 1 {
					if name, ok := app.Arguments()[0].(*NameExpr); ok {
						negatedNames[name.Name()] = struct{}{}
					}
				}
			}
			for _, argument := range arguments {
				find(argument)
			}
		case *LetInExpr:
			find(typed.Body())
			for _, declaration := range typed.Declarations() {
				find(declaration.Body())
			}
		}
	}
	find(body)
	for name := range negatedNames {
		oldName, newName := name, "Neg"+name+"$"
		pattern := MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr(oldName, Typed(mustOperator(nil, BoolT1)))))
		replacement := func() Expr {
			return MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr(newName, Typed(mustOperator(nil, BoolT1))))
		}
		body = ReplaceEqual(IdleTracker{}, pattern, replacement)(body)
	}
	additional := make([]*OperDecl, 0)
	for _, declaration := range declarations {
		if _, found := negatedNames[declaration.Name()]; found {
			negatedBody, err := nnf(declaration.Body(), true)
			if err != nil {
				return nil, nil, err
			}
			copy := NewOperDecl("Neg"+declaration.Name()+"$", declaration.FormalParams(), negatedBody, declaration.Tag())
			additional = append(additional, copy)
		}
	}
	return body, append(declarations, additional...), nil
}
