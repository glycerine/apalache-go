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
		result = expression
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
		case "EXCEPT":
			transformed := make([]Expr, len(arguments)-1)
			for index, argument := range arguments[1:] {
				var err error
				transformed[index], err = desugarer.Expression(argument)
				if err != nil {
					return nil, err
				}
			}
			accessors, values := deinterleaveExpressions(transformed)
			multidimensional := false
			for _, accessor := range accessors {
				tuple, ok := accessor.(*OperExpr)
				if !ok || tuple.Operator().Name() != "TUPLE" {
					return nil, NewLirError(fmt.Sprintf("Expected a tuple of keys as an accessor in EXCEPT. Found: %v", accessor))
				}
				multidimensional = multidimensional || len(tuple.Arguments()) > 1
			}
			function, err := desugarer.Expression(arguments[0])
			if err != nil {
				return nil, err
			}
			if len(accessors) < 2 && !multidimensional {
				result = MustOperExpr(Op("EXCEPT"), typed.Tag(), append([]Expr{function}, transformed...)...)
			} else {
				expanded, err := desugarer.expandExcept(function, accessors, values)
				if err != nil {
					return nil, err
				}
				result = expanded
			}
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
			unchanged := MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), arguments[1])
			var expanded Expr
			if typed.Operator().Name() == "STUTTER" {
				expanded = MustOperExpr(Op("OR"), Typed(BoolT1), arguments[0], unchanged)
			} else {
				expanded = MustOperExpr(Op("AND"), Typed(BoolT1), arguments[0], MustOperExpr(Op("NOT"), Typed(BoolT1), unchanged))
			}
			var err error
			result, err = desugarer.Expression(expanded)
			if err != nil {
				return nil, err
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
			result = MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), MustOperExpr(Op("IMPLIES"), Typed(BoolT1), arguments[0], MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1), arguments[1])))
		case "SET_FILTER", "EXISTS3", "FORALL3":
			bound, err := desugarer.Expression(arguments[0])
			if err != nil {
				return nil, err
			}
			set, err := desugarer.Expression(arguments[1])
			if err != nil {
				return nil, err
			}
			predicate, err := desugarer.Expression(arguments[2])
			if err != nil {
				return nil, err
			}
			collapsed, err := collapseTupleFilter(bound, set, predicate)
			if err != nil {
				return nil, err
			}
			result = MustOperExpr(typed.Operator(), typed.Tag(), collapsed...)
		case "SET_MAP":
			transformed := make([]Expr, len(arguments))
			for index, argument := range arguments {
				var err error
				transformed[index], err = desugarer.Expression(argument)
				if err != nil {
					return nil, err
				}
			}
			collapsed, err := collapseTupleMap(transformed[0], transformed[1:])
			if err != nil {
				return nil, err
			}
			result = MustOperExpr(typed.Operator(), typed.Tag(), collapsed...)
		case "FUN_CTOR", "FUN_REC_CTOR":
			transformed := make([]Expr, len(arguments))
			for index, argument := range arguments {
				var err error
				transformed[index], err = desugarer.Expression(argument)
				if err != nil {
					return nil, err
				}
			}
			body := transformed[0]
			bindings := transformed[1:]
			if len(bindings) > 2 {
				variables, sets := deinterleaveExpressions(bindings)
				elements := make([]Type, len(variables))
				for index, variable := range variables {
					typ, ok := variable.Tag().Type()
					if !ok {
						return nil, fmt.Errorf("untyped function parameter")
					}
					elements[index] = typ
				}
				tupleType := mustTuple(elements...)
				tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), variables...)
				product := MustOperExpr(Op("SET_TIMES"), Typed(mustSet(tupleType)), sets...)
				tuple = desugarer.tracker.Hold(variables[0], tuple).(*OperExpr)
				product = desugarer.tracker.Hold(sets[0], product).(*OperExpr)
				bindings = []Expr{tuple, product}
			}
			collapsed, err := collapseTupleMap(body, bindings)
			if err != nil {
				return nil, err
			}
			result = MustOperExpr(typed.Operator(), typed.Tag(), collapsed...)
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

func (desugarer *Desugarer) expandExcept(function Expr, accessors, values []Expr) (Expr, error) {
	if len(accessors) != len(values) {
		return nil, fmt.Errorf("EXCEPT accessors and values have different lengths")
	}
	functionType, ok := function.Tag().Type()
	if !ok {
		return nil, fmt.Errorf("untyped EXCEPT function")
	}
	firstName := desugarer.names.NewName()
	definitions := []*OperDecl{NewOperDecl(firstName, nil, function, Typed(mustOperator(nil, functionType)))}
	for index, accessor := range accessors {
		latest := definitions[len(definitions)-1]
		current := MustOperExpr(Op("OPER_APP"), Typed(functionType), NewNameExpr(latest.Name(), latest.Tag()))
		one, err := desugarer.expandExceptOne(current, accessor, values[index])
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, one...)
	}
	last := definitions[len(definitions)-1]
	body := MustOperExpr(Op("OPER_APP"), Typed(functionType), NewNameExpr(last.Name(), last.Tag()))
	return NewLetInExpr(body, Typed(functionType), definitions...), nil
}

func (desugarer *Desugarer) expandExceptOne(function, accessor, value Expr) ([]*OperDecl, error) {
	tuple, ok := accessor.(*OperExpr)
	if !ok || tuple.Operator().Name() != "TUPLE" {
		return nil, NewLirError(fmt.Sprintf("Expected a tuple of keys as an accessor in EXCEPT. Found: %v", accessor))
	}
	keys := tuple.Arguments()
	if len(keys) == 0 {
		return nil, NewLirError("Expected at least one key in EXCEPT, found none")
	}
	var rewrite func(Expr, []Expr) ([]*OperDecl, error)
	rewrite = func(current Expr, remaining []Expr) ([]*OperDecl, error) {
		currentType, ok := current.Tag().Type()
		if !ok {
			return nil, fmt.Errorf("untyped function in EXCEPT")
		}
		key := remaining[0]
		if len(remaining) == 1 {
			name := desugarer.names.NewName()
			path := MustOperExpr(Op("TUPLE"), Typed(mustTuple(mustTagType(key.Tag()))), key)
			body := MustOperExpr(Op("EXCEPT"), Typed(currentType), current, path, value)
			return []*OperDecl{NewOperDecl(name, nil, body, Typed(mustOperator(nil, currentType)))}, nil
		}
		resultType, err := exceptAccessResultType(currentType, key)
		if err != nil {
			return nil, err
		}
		nested := MustOperExpr(Op("FUN_APP"), Typed(resultType), current, key)
		definitions, err := rewrite(nested, remaining[1:])
		if err != nil {
			return nil, err
		}
		last := definitions[len(definitions)-1]
		nestedValue := MustOperExpr(Op("OPER_APP"), Typed(resultType), NewNameExpr(last.Name(), last.Tag()))
		path := MustOperExpr(Op("TUPLE"), Typed(mustTuple(mustTagType(key.Tag()))), key)
		body := MustOperExpr(Op("EXCEPT"), Typed(currentType), current, path, nestedValue)
		name := desugarer.names.NewName()
		definitions = append(definitions, NewOperDecl(name, nil, body, Typed(mustOperator(nil, currentType))))
		return definitions, nil
	}
	return rewrite(function, keys)
}

func exceptAccessResultType(functionType Type, key Expr) (Type, error) {
	keyType, typed := key.Tag().Type()
	if !typed {
		return nil, fmt.Errorf("untyped EXCEPT key")
	}
	switch function := functionType.(type) {
	case FunType:
		if !EqualTypes(function.Argument(), keyType) {
			return nil, NewTypingException(fmt.Sprintf("Expected a function argument of type %s, found %s", function.Argument(), keyType), key.ID())
		}
		return function.Result(), nil
	case SeqType:
		if !EqualTypes(keyType, IntT1) {
			return nil, NewTypingException(fmt.Sprintf("Expected a sequence argument to be an integer, found %s", keyType), key.ID())
		}
		return function.Element(), nil
	case TupleType:
		value, ok := key.(*ValueExpr)
		if !ok {
			return nil, fmt.Errorf("expected literal tuple index")
		}
		integer, ok := value.Value().(IntValue)
		if !ok || !integer.BigInt().IsInt64() {
			return nil, fmt.Errorf("expected integer tuple index")
		}
		index := integer.BigInt().Int64()
		elements := function.Elements()
		if index <= 0 || index > int64(len(elements)) {
			return nil, fmt.Errorf("tuple index %d is out of bounds", index)
		}
		return elements[index-1], nil
	case RecordType:
		value, ok := key.(*ValueExpr)
		if !ok {
			return nil, fmt.Errorf("expected literal record key")
		}
		text, ok := value.Value().(StringValue)
		if !ok {
			return nil, fmt.Errorf("expected string record key")
		}
		field, found := function.Fields()[text.Value()]
		if !found {
			return nil, fmt.Errorf("record has no field %s", text.Value())
		}
		return field, nil
	case RecordRowType:
		value, ok := key.(*ValueExpr)
		if !ok {
			return nil, fmt.Errorf("expected literal record key")
		}
		text, ok := value.Value().(StringValue)
		if !ok {
			return nil, fmt.Errorf("expected string record key")
		}
		field, found := function.Row().Fields()[text.Value()]
		if !found {
			return nil, fmt.Errorf("row record has no field %s", text.Value())
		}
		return field, nil
	default:
		return nil, NewTypingException(fmt.Sprintf("Unexpected type in function application: %s", key), key.ID())
	}
}

func deinterleaveExpressions(arguments []Expr) ([]Expr, []Expr) {
	left, right := make([]Expr, 0, (len(arguments)+1)/2), make([]Expr, 0, len(arguments)/2)
	for index, argument := range arguments {
		if index%2 == 0 {
			left = append(left, argument)
		} else {
			right = append(right, argument)
		}
	}
	return left, right
}

func collapseTupleFilter(bound, set, predicate Expr) ([]Expr, error) {
	name, err := tuplePatternName(bound)
	if err != nil {
		return nil, err
	}
	substitutions, err := tuplePatternSubstitutions(bound)
	if err != nil {
		return nil, err
	}
	return []Expr{NewNameExpr(name, bound.Tag()), set, substitutePatternNames(predicate, substitutions)}, nil
}

func collapseTupleMap(body Expr, bindings []Expr) ([]Expr, error) {
	if len(bindings)%2 != 0 {
		return nil, fmt.Errorf("expected variable/set pairs")
	}
	variables, sets := deinterleaveExpressions(bindings)
	boundNames := make([]Expr, len(variables))
	substitutions := make(map[string]Expr)
	for index, variable := range variables {
		name, err := tuplePatternName(variable)
		if err != nil {
			return nil, err
		}
		boundNames[index] = NewNameExpr(name, variable.Tag())
		one, err := tuplePatternSubstitutions(variable)
		if err != nil {
			return nil, err
		}
		for key, value := range one {
			substitutions[key] = value
		}
	}
	result := []Expr{substitutePatternNames(body, substitutions)}
	for index := range boundNames {
		result = append(result, boundNames[index], sets[index])
	}
	return result, nil
}

func tuplePatternName(expression Expr) (string, error) {
	switch typed := expression.(type) {
	case *NameExpr:
		return typed.Name(), nil
	case *OperExpr:
		if typed.Operator().Name() != "TUPLE" {
			break
		}
		name := ""
		for index, argument := range typed.Arguments() {
			part, err := tuplePatternName(argument)
			if err != nil {
				return "", err
			}
			if index > 0 {
				name += "_"
			}
			name += part
		}
		return name, nil
	}
	return "", fmt.Errorf("unexpected %T among tuple-pattern parameters", expression)
}

func tuplePatternSubstitutions(expression Expr) (map[string]Expr, error) {
	name, err := tuplePatternName(expression)
	if err != nil {
		return nil, err
	}
	result := make(map[string]Expr)
	var visit func(Expr, Expr) error
	visit = func(pattern Expr, access Expr) error {
		switch typed := pattern.(type) {
		case *NameExpr:
			result[typed.Name()] = access
			return nil
		case *OperExpr:
			if typed.Operator().Name() != "TUPLE" {
				return fmt.Errorf("unexpected operator %s in tuple pattern", typed.Operator().Name())
			}
			tupleType, ok := access.Tag().Type()
			if !ok {
				return fmt.Errorf("untyped tuple pattern")
			}
			elements, ok := tupleType.(TupleType)
			if !ok {
				return fmt.Errorf("expected tuple type, found %s", tupleType)
			}
			types := elements.Elements()
			arguments := typed.Arguments()
			if len(types) != len(arguments) {
				return fmt.Errorf("tuple pattern arity does not match type")
			}
			for index, argument := range arguments {
				indexExpr := NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1))
				next := MustOperExpr(Op("FUN_APP"), Typed(types[index]), access, indexExpr)
				if err := visit(argument, next); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("unexpected %T in tuple pattern", pattern)
		}
	}
	if _, simple := expression.(*NameExpr); simple {
		return result, nil
	}
	if err := visit(expression, NewNameExpr(name, expression.Tag())); err != nil {
		return nil, err
	}
	return result, nil
}

func substitutePatternNames(expression Expr, substitutions map[string]Expr) Expr {
	switch typed := expression.(type) {
	case *NameExpr:
		if replacement, found := substitutions[typed.Name()]; found {
			return replacement
		}
		return expression
	case *OperExpr:
		arguments := typed.Arguments()
		for index, argument := range arguments {
			arguments[index] = substitutePatternNames(argument, substitutions)
		}
		return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
	case *LetInExpr:
		declarations := typed.Declarations()
		for index, declaration := range declarations {
			copy := declaration.copyWithTag(declaration.Tag())
			copy.SetBody(substitutePatternNames(declaration.Body(), substitutions))
			declarations[index] = copy
		}
		return NewLetInExpr(substitutePatternNames(typed.Body(), substitutions), typed.Tag(), declarations...)
	default:
		return expression
	}
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
