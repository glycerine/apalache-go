package apalache

import (
	"fmt"
	"math/big"
)

// ExprOptimizer applies the inexpensive KerA expression rewrites used by the
// Scala optimization pass.
type ExprOptimizer struct {
	names   *UniqueNameGenerator
	tracker TransformationTracker
}

func NewExprOptimizer(names *UniqueNameGenerator, tracker TransformationTracker) *ExprOptimizer {
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &ExprOptimizer{names: names, tracker: tracker}
}

func (optimizer *ExprOptimizer) Expression(expression Expr) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot optimize a nil expression")
	}
	if err := WatchExpression(FlatLanguagePredicate{}, expression); err != nil {
		return nil, err
	}
	return optimizer.transform(expression)
}

func (optimizer *ExprOptimizer) transform(expression Expr) (Expr, error) {
	var rebuilt Expr = expression
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		for index, argument := range arguments {
			var err error
			arguments[index], err = optimizer.transform(argument)
			if err != nil {
				return nil, err
			}
		}
		rebuilt = MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
	case *LetInExpr:
		declarations := typed.Declarations()
		for index, declaration := range declarations {
			body, err := optimizer.transform(declaration.Body())
			if err != nil {
				return nil, err
			}
			copy := declaration.copyWithTag(declaration.Tag())
			copy.SetBody(body)
			declarations[index] = copy
		}
		body, err := optimizer.transform(typed.Body())
		if err != nil {
			return nil, err
		}
		rebuilt = NewLetInExpr(body, typed.Tag(), declarations...)
	}
	result, err := optimizer.oneLevel(rebuilt)
	if err != nil {
		return nil, err
	}
	return optimizer.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
}

func (optimizer *ExprOptimizer) oneLevel(expression Expr) (Expr, error) {
	op, ok := expression.(*OperExpr)
	if !ok {
		return expression, nil
	}
	args := op.Arguments()
	name := op.Operator().Name()
	if name == "INT_RANGE" {
		lower, constant := constantInteger(args[0])
		length, isLength := operNamed(args[1], "Sequences!Len")
		if constant && lower.Cmp(big.NewInt(1)) == 0 && isLength {
			return MustOperExpr(Op("DOMAIN"), op.Tag(), length.Arguments()[0]), nil
		}
	}

	// A field access into a record literal can be selected statically.
	if name == "FUN_APP" {
		if record, ok := operNamed(args[0], "RECORD"); ok {
			if key, ok := stringLiteral(args[1]); ok {
				fields := record.Arguments()
				for index := 0; index+1 < len(fields); index += 2 {
					field, isString := stringLiteral(fields[index])
					if isString && field == key {
						return fields[index+1], nil
					}
				}
				return nil, fmt.Errorf("access to non-existent record field %s", key)
			}
		}
	}

	if name == "SET_IN" {
		if integerRange, ok := operNamed(args[1], "INT_RANGE"); ok {
			bounds := integerRange.Arguments()
			return MustOperExpr(Op("AND"), Typed(BoolT1),
				MustOperExpr(Op("LE"), Typed(BoolT1), bounds[0], args[0]),
				MustOperExpr(Op("LE"), Typed(BoolT1), args[0], bounds[1])), nil
		}
		if filter, ok := operNamed(args[1], "SET_FILTER"); ok {
			filterArgs := filter.Arguments()
			if bound, named := filterArgs[0].(*NameExpr); named {
				predicate := ReplaceEqual(optimizer.tracker, bound, func() Expr {
					return NewDeepCopier(optimizer.tracker).Expression(args[0])
				})(filterArgs[2])
				return MustOperExpr(Op("AND"), Typed(BoolT1),
					MustOperExpr(Op("SET_IN"), Typed(BoolT1), args[0], filterArgs[1]), predicate), nil
			}
		}
		if powerset, ok := operNamed(args[1], "SET_POWERSET"); ok {
			if mapped, mapOK := operNamed(powerset.Arguments()[0], "SET_MAP"); mapOK {
				mapArgs := mapped.Arguments()
				if record, recordOK := operNamed(mapArgs[0], "RECORD"); recordOK && len(record.Arguments()) == len(mapArgs)-1 {
					result, rewritten, err := optimizer.recordMapMembership(args[0], mapped, true)
					if err != nil || rewritten {
						return result, err
					}
					return expression, nil
				}
			}
			elementType, err := setElementType(args[0])
			if err != nil {
				return nil, err
			}
			bound := NewNameExpr(optimizer.names.NewName(), Typed(elementType))
			return optimizer.transform(MustOperExpr(Op("FORALL3"), Typed(BoolT1), bound, args[0],
				MustOperExpr(Op("SET_IN"), Typed(BoolT1), bound, powerset.Arguments()[0])))
		}
		if mapped, ok := operNamed(args[1], "SET_MAP"); ok {
			mapArgs := mapped.Arguments()
			if record, recordOK := operNamed(mapArgs[0], "RECORD"); recordOK && len(record.Arguments()) == len(mapArgs)-1 {
				result, rewritten, err := optimizer.recordMapMembership(args[0], mapped, false)
				if err != nil || rewritten {
					return result, err
				}
				return expression, nil
			}
			if result, rewritten, err := optimizer.tupleMapMembership(args[0], mapped); err != nil || rewritten {
				return result, err
			}
		}
	}

	if name == "FiniteSets!Cardinality" {
		if integerRange, ok := operNamed(args[0], "INT_RANGE"); ok {
			bounds := integerRange.Arguments()
			return MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
				MustOperExpr(Op("LE"), Typed(BoolT1), bounds[0], bounds[1]),
				MustOperExpr(Op("PLUS"), Typed(IntT1), MustOperExpr(Op("MINUS"), Typed(IntT1), bounds[1], bounds[0]), intLiteral(1)),
				intLiteral(0)), nil
		}
		if powerset, ok := operNamed(args[0], "SET_POWERSET"); ok {
			return MustOperExpr(Op("POW"), Typed(IntT1), intLiteral(2),
				MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), powerset.Arguments()[0])), nil
		}
	}

	if result, rewritten, err := optimizer.cardinalityComparison(op); err != nil || rewritten {
		return result, err
	}

	if name == "EXISTS3" {
		if filter, ok := operNamed(args[1], "SET_FILTER"); ok {
			filterArgs := filter.Arguments()
			x, xOK := args[0].(*NameExpr)
			y, yOK := filterArgs[0].(*NameExpr)
			if xOK && yOK {
				predicate := MustOperExpr(Op("AND"), Typed(BoolT1), filterArgs[2], args[2])
				predicate = ReplaceEqual(optimizer.tracker, x, func() Expr { return NewNameExpr(y.Name(), y.Tag()) })(predicate).(*OperExpr)
				result := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), NewNameExpr(y.Name(), y.Tag()), filterArgs[1], predicate)
				return optimizer.optimizeExists(result)
			}
		}
		if mapped, ok := operNamed(args[1], "SET_MAP"); ok {
			x, xOK := args[0].(*NameExpr)
			mapArgs := mapped.Arguments()
			if xOK {
				predicate := ReplaceEqual(optimizer.tracker, x, func() Expr {
					return NewDeepCopier(optimizer.tracker).Expression(mapArgs[0])
				})(args[2])
				for index := 1; index+1 < len(mapArgs); index += 2 {
					bound, named := mapArgs[index].(*NameExpr)
					if !named {
						continue
					}
					predicate = MustOperExpr(Op("EXISTS3"), Typed(BoolT1),
						NewNameExpr(bound.Name(), bound.Tag()), mapArgs[index+1], predicate)
					var err error
					predicate, err = optimizer.optimizeExists(predicate)
					if err != nil {
						return nil, err
					}
				}
				return predicate, nil
			}
		}
	}

	return expression, nil
}

func (optimizer *ExprOptimizer) recordMapMembership(member Expr, mapped *OperExpr, powerset bool) (Expr, bool, error) {
	mapArgs := mapped.Arguments()
	record, ok := operNamed(mapArgs[0], "RECORD")
	if !ok {
		return nil, false, nil
	}
	fieldsAndValues := record.Arguments()
	bindings := mapArgs[1:]
	if len(fieldsAndValues) != len(bindings) || len(fieldsAndValues)%2 != 0 {
		return nil, false, nil
	}
	for index := 1; index < len(fieldsAndValues); index += 2 {
		if !EqualExpr(fieldsAndValues[index], bindings[index-1]) {
			return nil, false, nil
		}
	}
	recordValue := member
	if powerset {
		elementType, err := setElementType(member)
		if err != nil {
			return nil, false, err
		}
		recordValue = NewNameExpr(optimizer.names.NewName(), Typed(elementType))
	}
	fieldNames := make([]Expr, 0, len(fieldsAndValues)/2)
	conjuncts := make([]Expr, 0, len(fieldsAndValues)/2+1)
	for index := 0; index < len(fieldsAndValues); index += 2 {
		fieldNames = append(fieldNames, fieldsAndValues[index])
	}
	stringSet := Typed(mustSet(StrT1))
	conjuncts = append(conjuncts, MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("DOMAIN"), stringSet, recordValue), MustOperExpr(Op("SET_ENUM"), stringSet, fieldNames...)))
	for index := 0; index < len(fieldsAndValues); index += 2 {
		valueType, ok := fieldsAndValues[index+1].Tag().Type()
		if !ok {
			return nil, false, fmt.Errorf("record map value is untyped")
		}
		access := MustOperExpr(Op("FUN_APP"), Typed(valueType), recordValue, fieldsAndValues[index])
		conjuncts = append(conjuncts, MustOperExpr(Op("SET_IN"), Typed(BoolT1), access, bindings[index+1]))
	}
	result := Expr(MustOperExpr(Op("AND"), Typed(BoolT1), conjuncts...))
	if powerset {
		result = MustOperExpr(Op("FORALL3"), Typed(BoolT1), recordValue, member, result)
	}
	optimized, err := optimizer.transform(result)
	return optimized, true, err
}

func (optimizer *ExprOptimizer) tupleMapMembership(member Expr, mapped *OperExpr) (Expr, bool, error) {
	mapArgs := mapped.Arguments()
	tuple, ok := operNamed(mapArgs[0], "TUPLE")
	if !ok {
		return nil, false, nil
	}
	elements := tuple.Arguments()
	bindings := mapArgs[1:]
	if len(bindings) != 2*len(elements) {
		return nil, false, nil
	}
	tupleTag, typed := tuple.Tag().Type()
	if !typed {
		return nil, false, fmt.Errorf("tuple map result is untyped")
	}
	if _, tupleType := tupleTag.(TupleType); !tupleType {
		return nil, false, nil
	}
	for index, element := range elements {
		if !EqualExpr(element, bindings[2*index]) {
			return nil, false, nil
		}
	}
	memberships := make([]Expr, len(elements))
	for index, element := range elements {
		elementType, ok := element.Tag().Type()
		if !ok {
			return nil, false, fmt.Errorf("tuple element is untyped")
		}
		access := MustOperExpr(Op("FUN_APP"), Typed(elementType), member, intLiteral(int64(index+1)))
		memberships[index] = MustOperExpr(Op("SET_IN"), Typed(BoolT1), access, bindings[2*index+1])
	}
	result, err := optimizer.transform(MustOperExpr(Op("AND"), Typed(BoolT1), memberships...))
	return result, true, err
}

func (optimizer *ExprOptimizer) cardinalityComparison(op *OperExpr) (Expr, bool, error) {
	args := op.Arguments()
	if len(args) != 2 {
		return nil, false, nil
	}
	cardinality, ok := operNamed(args[0], "FiniteSets!Cardinality")
	if !ok {
		return nil, false, nil
	}
	integer, ok := integerLiteral(args[1])
	if !ok {
		return nil, false, nil
	}
	set := cardinality.Arguments()[0]
	empty := func() Expr { return MustOperExpr(Op("SET_ENUM"), set.Tag()) }
	notEmpty := func() Expr {
		return MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQ"), Typed(BoolT1), set, empty()))
	}
	switch op.Operator().Name() {
	case "EQ":
		if integer.Sign() == 0 {
			return MustOperExpr(Op("EQ"), Typed(BoolT1), set, empty()), true, nil
		}
	case "NE":
		if integer.Sign() == 0 {
			return notEmpty(), true, nil
		}
	case "GT":
		if integer.Sign() == 0 {
			return notEmpty(), true, nil
		}
		plusOne := new(big.Int).Add(integer, big.NewInt(1))
		if plusOne.IsInt64() && plusOne.Cmp(big.NewInt(-2147483648)) >= 0 && plusOne.Cmp(big.NewInt(2147483647)) <= 0 {
			generated := MustOperExpr(Op("GE"), Typed(BoolT1), args[0], NewValueExpr(NewIntValue(plusOne), Typed(IntT1)))
			result, rewritten, err := optimizer.cardinalityComparison(generated)
			if err != nil {
				return nil, true, err
			}
			if rewritten {
				return result, true, nil
			}
			return generated, true, nil
		}
	case "GE":
		if integer.Cmp(big.NewInt(1)) == 0 {
			return notEmpty(), true, nil
		}
		if integer.Cmp(big.NewInt(2)) == 0 {
			elementType, err := setElementType(set)
			if err != nil {
				return nil, true, err
			}
			firstName, secondName, definitionName := optimizer.names.NewName(), optimizer.names.NewName(), optimizer.names.NewName()
			operatorType := mustOperator(nil, mustSet(elementType))
			definition := NewOperDecl(definitionName, nil, set, Typed(operatorType))
			applyDefinition := func() Expr {
				return MustOperExpr(Op("OPER_APP"), set.Tag(), NewNameExpr(definitionName, Typed(operatorType)))
			}
			first := NewNameExpr(firstName, Typed(elementType))
			second := NewNameExpr(secondName, Typed(elementType))
			test := MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQ"), Typed(BoolT1), first, second))
			body := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), first, applyDefinition(),
				MustOperExpr(Op("EXISTS3"), Typed(BoolT1), second, applyDefinition(), test))
			return NewLetInExpr(body, Typed(BoolT1), definition), true, nil
		}
	}
	return nil, false, nil
}

func (optimizer *ExprOptimizer) optimizeExists(expression Expr) (Expr, error) {
	if op, ok := expression.(*OperExpr); ok && op.Operator().Name() == "EXISTS3" {
		return optimizer.oneLevel(op)
	}
	return expression, nil
}

func operNamed(expression Expr, name string) (*OperExpr, bool) {
	op, ok := expression.(*OperExpr)
	return op, ok && op.Operator().Name() == name
}

func stringLiteral(expression Expr) (string, bool) {
	value, ok := expression.(*ValueExpr)
	if !ok {
		return "", false
	}
	stringValue, ok := value.Value().(StringValue)
	return string(stringValue), ok
}

func integerLiteral(expression Expr) (*big.Int, bool) {
	value, ok := expression.(*ValueExpr)
	if !ok {
		return nil, false
	}
	integer, ok := value.Value().(IntValue)
	if !ok {
		return nil, false
	}
	return integer.BigInt(), true
}

func intLiteral(value int64) Expr {
	return NewValueExpr(Int64Value(value), Typed(IntT1))
}
