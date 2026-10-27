package apalache

import "fmt"

const WrappedCallNamePrefix = "CALL"

type OperAppToLetInDef struct {
	names   *UniqueNameGenerator
	tracker TransformationTracker
}

func NewOperAppToLetInDef(names *UniqueNameGenerator, tracker TransformationTracker) *OperAppToLetInDef {
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &OperAppToLetInDef{names: names, tracker: tracker}
}

func (wrapper *OperAppToLetInDef) Wrap(expression Expr, wrappable map[string]struct{}) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot wrap a nil expression")
	}
	if wrappable == nil {
		wrappable = map[string]struct{}{}
	}
	var wrap func(Expr, map[string]struct{}) (Expr, error)
	wrap = func(current Expr, names map[string]struct{}) (Expr, error) {
		var result Expr = current
		switch typed := current.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			if typed.Operator().Name() == "OPER_APP" && len(arguments) > 1 {
				operatorName, named := arguments[0].(*NameExpr)
				_, selected := names[operatorName.Name()]
				if named && selected {
					resultType, ok := typed.Tag().Type()
					if !ok {
						return nil, fmt.Errorf("expected typed operator application %s", typed.ID())
					}
					newArguments := append([]Expr(nil), arguments...)
					changed := false
					for index := 1; index < len(newArguments); index++ {
						wrapped, err := wrap(newArguments[index], names)
						if err != nil {
							return nil, err
						}
						changed = changed || wrapped.ID() != newArguments[index].ID()
						newArguments[index] = wrapped
					}
					application := Expr(typed)
					if changed {
						application = MustOperExpr(Op("OPER_APP"), typed.Tag(), append([]Expr{NewNameExpr(operatorName.Name(), operatorName.Tag())}, newArguments[1:]...)...)
						application = wrapper.tracker.TrackExpression(func(Expr) Expr { return application })(typed)
					}
					name := WrappedCallNamePrefix + "_" + RenamedNameBase(operatorName.Name()) + "_" + wrapper.names.NewName()
					definition := NewOperDecl(name, nil, application, Typed(mustOperator(nil, resultType)))
					body := MustOperExpr(Op("OPER_APP"), typed.Tag(), NewNameExpr(name, operatorName.Tag()))
					result = NewLetInExpr(body, typed.Tag(), definition)
					return wrapper.tracker.TrackExpression(func(Expr) Expr { return result })(current), nil
				}
			}
			changed := false
			for index, argument := range arguments {
				wrapped, err := wrap(argument, names)
				if err != nil {
					return nil, err
				}
				changed = changed || wrapped.ID() != argument.ID()
				arguments[index] = wrapped
			}
			if changed {
				result = MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			}
		case *LetInExpr:
			inside := cloneMap(names)
			declarations := typed.Declarations()
			changed := false
			for index, declaration := range declarations {
				body, err := wrap(declaration.Body(), inside)
				if err != nil {
					return nil, err
				}
				copy := declaration.copyWithTag(declaration.Tag())
				copy.SetBody(body)
				declarations[index] = copy
				changed = changed || body.ID() != declaration.Body().ID()
				inside[declaration.Name()] = struct{}{}
			}
			body, err := wrap(typed.Body(), inside)
			if err != nil {
				return nil, err
			}
			changed = changed || body.ID() != typed.Body().ID()
			if changed {
				result = NewLetInExpr(body, typed.Tag(), declarations...)
			}
		}
		if result.ID() == current.ID() {
			return current, nil
		}
		return wrapper.tracker.TrackExpression(func(Expr) Expr { return result })(current), nil
	}
	return wrap(expression, wrappable)
}

func (wrapper *OperAppToLetInDef) Module(module *Module, wrappable map[string]struct{}) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot wrap applications in a nil module")
	}
	declarations := module.Declarations()
	for index, declaration := range declarations {
		operator, ok := declaration.(*OperDecl)
		if !ok {
			continue
		}
		body, err := wrapper.Wrap(operator.Body(), wrappable)
		if err != nil {
			return nil, err
		}
		copy := operator.copyWithTag(operator.Tag())
		copy.SetBody(body)
		declarations[index] = copy
	}
	result := NewModule(module.Name(), declarations)
	result.properties = cloneMap(module.properties)
	return result, nil
}
