package apalache

import "fmt"

type Keramelizer struct {
	names   *UniqueNameGenerator
	tracker TransformationTracker
}

func NewKeramelizer(names *UniqueNameGenerator, tracker TransformationTracker) *Keramelizer {
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &Keramelizer{names: names, tracker: tracker}
}

func (keramelizer *Keramelizer) Expression(expression Expr) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot keramelize a nil expression")
	}
	if err := WatchExpression(FlatLanguagePredicate{}, expression); err != nil {
		return nil, err
	}
	return keramelizer.transform(expression)
}

func (keramelizer *Keramelizer) transform(expression Expr) (Expr, error) {
	var shallow Expr = expression
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		changed := false
		for index, argument := range arguments {
			var err error
			arguments[index], err = keramelizer.transform(argument)
			if err != nil {
				return nil, err
			}
			changed = changed || arguments[index].ID() != argument.ID()
		}
		if changed {
			shallow = keramelizer.tracker.Hold(typed, MustOperExpr(typed.Operator(), typed.Tag(), arguments...))
		}
	case *LetInExpr:
		body, err := keramelizer.transform(typed.Body())
		if err != nil {
			return nil, err
		}
		declarations := typed.Declarations()
		for index, declaration := range declarations {
			newBody, bodyErr := keramelizer.transform(declaration.Body())
			if bodyErr != nil {
				return nil, bodyErr
			}
			copy := declaration.copyWithTag(declaration.Tag())
			copy.SetBody(newBody)
			declarations[index] = copy
		}
		result := NewLetInExpr(body, typed.Tag(), declarations...)
		return keramelizer.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
	}
	result, err := keramelizer.oneLevel(shallow)
	if err != nil {
		return nil, err
	}
	result = keramelizer.tracker.TrackExpression(func(Expr) Expr { return result })(shallow)
	return keramelizer.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
}

func (keramelizer *Keramelizer) oneLevel(expression Expr) (Expr, error) {
	op, ok := expression.(*OperExpr)
	if !ok {
		return expression, nil
	}
	args := op.Arguments()
	name := op.Operator().Name()
	setElement := func(set Expr) (Type, error) { return keramelizerSetElementType(set) }
	forallMembership := func(left, right Expr) (Expr, error) {
		typ, err := setElement(left)
		if err != nil {
			return nil, err
		}
		variable := NewNameExpr(keramelizer.names.NewName(), Typed(typ))
		return MustOperExpr(Op("FORALL3"), Typed(BoolT1), variable, left, MustOperExpr(Op("SET_IN"), Typed(BoolT1), variable, right)), nil
	}

	if name == "SET_IN" {
		if prime, ok := args[0].(*OperExpr); ok && prime.Operator().Name() == "PRIME" {
			if _, ok := prime.Arguments()[0].(*NameExpr); ok {
				typ, err := setElement(args[1])
				if err != nil {
					return nil, err
				}
				variable := NewNameExpr(keramelizer.names.NewName(), Typed(typ))
				return MustOperExpr(Op("EXISTS3"), Typed(BoolT1), variable, args[1], MustOperExpr(Op("EQ"), Typed(BoolT1), prime, variable)), nil
			}
		}
		if powerset, ok := args[1].(*OperExpr); ok && powerset.Operator().Name() == "SET_POWERSET" {
			return forallMembership(args[0], powerset.Arguments()[0])
		}
		if functionSet, ok := args[1].(*OperExpr); ok && functionSet.Operator().Name() == "FUN_SET" {
			domain, codomain := functionSet.Arguments()[0], functionSet.Arguments()[1]
			domainType, err := setElement(domain)
			if err != nil {
				return nil, err
			}
			domainVariable := NewNameExpr(keramelizer.names.NewName(), Typed(domainType))
			domainEquality := MustOperExpr(Op("EQ"), Typed(BoolT1), MustOperExpr(Op("DOMAIN"), domain.Tag(), args[0]), domain)
			if powerset, ok := codomain.(*OperExpr); ok && powerset.Operator().Name() == "SET_POWERSET" {
				rangeType, err := setElement(powerset.Arguments()[0])
				if err != nil {
					return nil, err
				}
				rangeVariable := NewNameExpr(keramelizer.names.NewName(), Typed(rangeType))
				app := MustOperExpr(Op("FUN_APP"), Typed(mustSet(rangeType)), args[0], domainVariable)
				inner := MustOperExpr(Op("FORALL3"), Typed(BoolT1), rangeVariable, app, MustOperExpr(Op("SET_IN"), Typed(BoolT1), rangeVariable, powerset.Arguments()[0]))
				outer := MustOperExpr(Op("FORALL3"), Typed(BoolT1), domainVariable, domain, inner)
				return MustOperExpr(Op("AND"), Typed(BoolT1), domainEquality, outer), nil
			}
			codomainType, err := setElement(codomain)
			if err != nil {
				return nil, err
			}
			app := MustOperExpr(Op("FUN_APP"), Typed(codomainType), args[0], domainVariable)
			constraint := MustOperExpr(Op("FORALL3"), Typed(BoolT1), domainVariable, domain, MustOperExpr(Op("SET_IN"), Typed(BoolT1), app, codomain))
			return MustOperExpr(Op("AND"), Typed(BoolT1), domainEquality, constraint), nil
		}
	}
	switch name {
	case "SET_INTERSECT", "SET_MINUS":
		typ, err := setElement(args[0])
		if err != nil {
			return nil, err
		}
		variable := NewNameExpr(keramelizer.names.NewName(), Typed(typ))
		predicate := MustOperExpr(Op("SET_IN"), Typed(BoolT1), variable, args[1])
		if name == "SET_MINUS" {
			predicate = MustOperExpr(Op("NOT"), Typed(BoolT1), predicate)
		}
		return MustOperExpr(Op("SET_FILTER"), op.Tag(), variable, args[0], predicate), nil
	case "SET_NOT_IN":
		return MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("SET_IN"), Typed(BoolT1), args...)), nil
	case "SET_SUBSET_EQ":
		leftPower, leftIsPower := args[0].(*OperExpr)
		rightPower, rightIsPower := args[1].(*OperExpr)
		if leftIsPower && rightIsPower && leftPower.Operator().Name() == "SET_POWERSET" && rightPower.Operator().Name() == "SET_POWERSET" {
			return keramelizer.oneLevel(MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), leftPower.Arguments()[0], rightPower.Arguments()[0]))
		}
		if rightIsPower && rightPower.Operator().Name() == "SET_POWERSET" {
			typ, err := setElement(rightPower.Arguments()[0])
			if err != nil {
				return nil, err
			}
			variable := NewNameExpr(keramelizer.names.NewName(), Typed(mustSet(typ)))
			var nested Expr = MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), variable, rightPower.Arguments()[0])
			nested, err = keramelizer.oneLevel(nested)
			if err != nil {
				return nil, err
			}
			return MustOperExpr(Op("FORALL3"), Typed(BoolT1), variable, args[0], nested), nil
		}
		return forallMembership(args[0], args[1])
	case "RECORD_SET":
		keys, sets := deinterleaveExpressions(args)
		names := make([]Expr, len(sets))
		for index, set := range sets {
			typ, err := setElement(set)
			if err != nil {
				return nil, err
			}
			names[index] = NewNameExpr(keramelizer.names.NewName(), Typed(typ))
		}
		recordArgs := make([]Expr, 0, len(args))
		bindings := make([]Expr, 0, len(args))
		for index := range keys {
			recordArgs = append(recordArgs, keys[index], names[index])
			bindings = append(bindings, names[index], sets[index])
		}
		recordType, err := setElement(expression)
		if err != nil {
			return nil, err
		}
		record := MustOperExpr(Op("RECORD"), Typed(recordType), recordArgs...)
		return MustOperExpr(Op("SET_MAP"), op.Tag(), append([]Expr{record}, bindings...)...), nil
	case "SET_TIMES":
		types := make([]Type, len(args))
		names := make([]Expr, len(args))
		bindings := make([]Expr, 0, 2*len(args))
		for index, set := range args {
			typ, err := setElement(set)
			if err != nil {
				return nil, err
			}
			types[index] = typ
			names[index] = NewNameExpr(keramelizer.names.NewName(), Typed(typ))
			bindings = append(bindings, names[index], set)
		}
		tuple := MustOperExpr(Op("TUPLE"), Typed(mustTuple(types...)), names...)
		return MustOperExpr(Op("SET_MAP"), op.Tag(), append([]Expr{tuple}, bindings...)...), nil
	case "EQUIV":
		return MustOperExpr(Op("EQ"), Typed(BoolT1), args...), nil
	case "IMPLIES":
		return MustOperExpr(Op("OR"), Typed(BoolT1), MustOperExpr(Op("NOT"), Typed(BoolT1), args[0]), args[1]), nil
	case "NE":
		return MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQ"), Typed(BoolT1), args...)), nil
	case "CASE", "CASE_OTHER":
		return keramelizeCase(op, args)
	}
	return expression, nil
}

func keramelizerSetElementType(expression Expr) (Type, error) {
	typ, ok := expression.Tag().Type()
	if !ok {
		return nil, NewMalformedTlaError("Expected a set, found: "+expression.Tag().String(), expression)
	}
	set, ok := typ.(SetType)
	if !ok {
		return nil, NewMalformedTlaError("Expected a set, found: "+expression.Tag().String(), expression)
	}
	return set.Element(), nil
}

func keramelizeCase(expression *OperExpr, arguments []Expr) (Expr, error) {
	resultType, typed := expression.Tag().Type()
	if !typed {
		return nil, fmt.Errorf("untyped CASE")
	}
	boolean := EqualTypes(resultType, BoolT1)
	pairs := arguments
	var other Expr
	if expression.Operator().Name() == "CASE_OTHER" {
		other, pairs = arguments[0], arguments[1:]
	}
	guards, actions := deinterleaveExpressions(pairs)
	if boolean {
		terms := make([]Expr, 0, len(guards)+1)
		for index := range guards {
			terms = append(terms, MustOperExpr(Op("AND"), Typed(BoolT1), guards[index], actions[index]))
		}
		if other != nil {
			otherArgs := make([]Expr, 0, len(guards)+1)
			for _, guard := range guards {
				otherArgs = append(otherArgs, MustOperExpr(Op("NOT"), Typed(BoolT1), guard))
			}
			otherArgs = append(otherArgs, other)
			terms = append(terms, MustOperExpr(Op("AND"), Typed(BoolT1), otherArgs...))
		}
		return MustOperExpr(Op("OR"), Typed(BoolT1), terms...), nil
	}
	if other == nil {
		if len(actions) == 0 {
			return NewValueExpr(BoolValue(false), Typed(BoolT1)), nil
		}
		other = actions[len(actions)-1]
		guards, actions = guards[:len(guards)-1], actions[:len(actions)-1]
	}
	for index := len(guards) - 1; index >= 0; index-- {
		other = MustOperExpr(Op("IF_THEN_ELSE"), Typed(resultType), guards[index], actions[index], other)
	}
	return other, nil
}
