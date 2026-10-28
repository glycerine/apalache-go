package apalache

import "fmt"

type SetMapDomain struct {
	Variable Expr
	Set      Expr
}

func (builder *TypedBuilder) EnumSet(arguments ...Expr) (*OperExpr, error) {
	if len(arguments) == 0 {
		return nil, fmt.Errorf("EnumSet requires at least one argument; use EmptySet")
	}
	return builder.Apply(Op("SET_ENUM"), arguments...)
}

func (builder *TypedBuilder) In(element, set Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_IN"), element, set)
}

func (builder *TypedBuilder) NotIn(element, set Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_NOT_IN"), element, set)
}

func (builder *TypedBuilder) Cap(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_INTERSECT"), left, right)
}

func (builder *TypedBuilder) Cup(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_UNION2"), left, right)
}

func (builder *TypedBuilder) Union(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_UNARY_UNION"), set)
}

func (builder *TypedBuilder) Filter(variable, set, predicate Expr) (*OperExpr, error) {
	if _, err := boundVariableNames(variable); err != nil {
		return nil, err
	}
	return builder.Apply(Op("SET_FILTER"), variable, set, predicate)
}

func (builder *TypedBuilder) Map(expression Expr, domains ...SetMapDomain) (*OperExpr, error) {
	arguments := make([]Expr, 0, 2*len(domains))
	for _, domain := range domains {
		arguments = append(arguments, domain.Variable, domain.Set)
	}
	return builder.MapMixed(expression, arguments...)
}

func (builder *TypedBuilder) MapMixed(expression Expr, arguments ...Expr) (*OperExpr, error) {
	if len(arguments) < 2 || len(arguments)%2 != 0 {
		return nil, fmt.Errorf("set-map domains require even, positive arity: got %d arguments", len(arguments))
	}
	seen := make(map[string]struct{})
	for index := 0; index < len(arguments); index += 2 {
		names, err := boundVariableNames(arguments[index])
		if err != nil {
			return nil, err
		}
		for name := range names {
			if _, duplicate := seen[name]; duplicate {
				return nil, fmt.Errorf("duplicate set-map variable %q", name)
			}
			seen[name] = struct{}{}
		}
	}
	operands := make([]Expr, 0, 1+len(arguments))
	operands = append(operands, expression)
	operands = append(operands, arguments...)
	return builder.Apply(Op("SET_MAP"), operands...)
}

func (builder *TypedBuilder) FunSet(fromSet, toSet Expr) (*OperExpr, error) {
	return builder.Apply(Op("FUN_SET"), fromSet, toSet)
}

func (builder *TypedBuilder) RecSet(fields ...RecordField) (*OperExpr, error) {
	return builder.RecordSet(fields...)
}

func (builder *TypedBuilder) RecSetMixed(arguments ...Expr) (*OperExpr, error) {
	if len(arguments) < 2 || len(arguments)%2 != 0 {
		return nil, fmt.Errorf("record-set fields require even, positive arity: got %d arguments", len(arguments))
	}
	fieldTypes := make(map[string]Type, len(arguments)/2)
	for index := 0; index < len(arguments); index += 2 {
		key, ok := arguments[index].(*ValueExpr)
		if !ok {
			return nil, fmt.Errorf("record-set key %d is not a string literal", index/2)
		}
		value, ok := key.Value().(StringValue)
		if !ok {
			return nil, fmt.Errorf("record-set key %d is not a string literal", index/2)
		}
		name := string(value)
		if _, duplicate := fieldTypes[name]; duplicate {
			return nil, fmt.Errorf("duplicate record-set field %q", name)
		}
		typ, err := ExpressionType(arguments[index+1])
		if err != nil {
			return nil, err
		}
		setType, ok := typ.(SetType)
		if !ok {
			return nil, fmt.Errorf("record-set field %q is not a set", name)
		}
		fieldTypes[name] = setType.element
	}
	record, err := NewRecordType(fieldTypes)
	if err != nil {
		return nil, err
	}
	return NewOperExpr(Op("RECORD_SET"), Typed(newSetTypeUnchecked(record)), arguments...)
}

func (builder *TypedBuilder) SeqSet(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("Sequences!Seq"), set)
}

func (builder *TypedBuilder) SubsetEq(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_SUBSET_EQ"), left, right)
}

func (builder *TypedBuilder) SetMinus(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_MINUS"), left, right)
}

func (builder *TypedBuilder) Times(sets ...Expr) (*OperExpr, error) {
	if len(sets) < 2 {
		return nil, fmt.Errorf("Times requires at least two sets")
	}
	return builder.Apply(Op("SET_TIMES"), sets...)
}

func (builder *TypedBuilder) PowSet(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("SET_POWERSET"), set)
}

func (builder *ScopedBuilder) EnumSet(arguments ...ExprInstruction) ExprInstruction {
	if len(arguments) == 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("EnumSet requires at least one argument; use EmptySet")
		}}
	}
	return builder.Apply(Op("SET_ENUM"), arguments...)
}

func (builder *ScopedBuilder) In(element, set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_IN"), element, set)
}

func (builder *ScopedBuilder) NotIn(element, set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_NOT_IN"), element, set)
}

func (builder *ScopedBuilder) Cap(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_INTERSECT"), left, right)
}

func (builder *ScopedBuilder) Cup(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_UNION2"), left, right)
}

func (builder *ScopedBuilder) Union(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_UNARY_UNION"), set)
}

func (builder *ScopedBuilder) Filter(variable, set, predicate ExprInstruction) ExprInstruction {
	return builder.BindBounded(Op("SET_FILTER"), variable, set, predicate)
}

func (builder *ScopedBuilder) Map(expression ExprInstruction, domains ...BoundDomain) ExprInstruction {
	return builder.BindVariadic(Op("SET_MAP"), expression, domains...)
}

func (builder *ScopedBuilder) MapMixed(expression ExprInstruction, arguments ...ExprInstruction) ExprInstruction {
	if len(arguments)%2 != 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("set-map domains require even arity: got %d arguments", len(arguments))
		}}
	}
	domains := make([]BoundDomain, len(arguments)/2)
	for index := range domains {
		domains[index] = BoundDomain{Variable: arguments[2*index], Set: arguments[2*index+1]}
	}
	return builder.Map(expression, domains...)
}

func (builder *ScopedBuilder) FunSet(fromSet, toSet ExprInstruction) ExprInstruction {
	return builder.Apply(Op("FUN_SET"), fromSet, toSet)
}

func (builder *ScopedBuilder) RecSet(fields ...ScopedRecordField) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		values := make([]ExprInstruction, len(fields))
		for index := range fields {
			values[index] = fields[index].Value
		}
		built, err := runInstructions(context, values)
		if err != nil {
			return nil, err
		}
		typedFields := make([]RecordField, len(fields))
		for index := range fields {
			typedFields[index] = RecordField{Name: fields[index].Name, Value: built[index]}
		}
		return builder.typed.RecSet(typedFields...)
	}}
}

func (builder *ScopedBuilder) RecSetMixed(arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.RecSetMixed(built...)
	}}
}

func (builder *ScopedBuilder) SeqSet(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Sequences!Seq"), set)
}

func (builder *ScopedBuilder) SubsetEq(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_SUBSET_EQ"), left, right)
}

func (builder *ScopedBuilder) SetMinus(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_MINUS"), left, right)
}

func (builder *ScopedBuilder) Times(sets ...ExprInstruction) ExprInstruction {
	if len(sets) < 2 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("Times requires at least two sets")
		}}
	}
	return builder.Apply(Op("SET_TIMES"), sets...)
}

func (builder *ScopedBuilder) PowSet(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("SET_POWERSET"), set)
}
