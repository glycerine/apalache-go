package apalache

import "fmt"

// FunctionDomain is one bound variable and its domain in a function constructor.
type FunctionDomain struct {
	Variable Expr
	Set      Expr
}

func (builder *TypedBuilder) Rec(fields ...RecordField) (*OperExpr, error) {
	return builder.Record(fields...)
}

func (builder *TypedBuilder) RowRec(tail *VarType, fields ...RecordField) (*OperExpr, error) {
	return builder.RowRecord(tail, fields...)
}

func (builder *TypedBuilder) RecMixed(arguments ...Expr) (*OperExpr, error) {
	return builder.recordMixed(nil, false, arguments...)
}

func (builder *TypedBuilder) RowRecMixed(tail *VarType, arguments ...Expr) (*OperExpr, error) {
	return builder.recordMixed(tail, true, arguments...)
}

func (builder *TypedBuilder) recordMixed(tail *VarType, precise bool, arguments ...Expr) (*OperExpr, error) {
	if len(arguments)%2 != 0 {
		return nil, fmt.Errorf("record fields require even arity: got %d arguments", len(arguments))
	}
	fieldTypes := make(map[string]Type, len(arguments)/2)
	for index := 0; index < len(arguments); index += 2 {
		key, ok := arguments[index].(*ValueExpr)
		if !ok {
			return nil, fmt.Errorf("record key %d is not a string literal", index/2)
		}
		value, ok := key.Value().(StringValue)
		if !ok {
			return nil, fmt.Errorf("record key %d is not a string literal", index/2)
		}
		name := string(value)
		if _, duplicate := fieldTypes[name]; duplicate {
			return nil, fmt.Errorf("duplicate record field %q", name)
		}
		typ, err := ExpressionType(arguments[index+1])
		if err != nil {
			return nil, fmt.Errorf("record field %q: %w", name, err)
		}
		fieldTypes[name] = typ
	}
	var result Type
	if precise {
		row, err := NewRowType(fieldTypes, tail)
		if err != nil {
			return nil, err
		}
		result = NewRecordRowType(row)
	} else {
		record, err := NewRecordType(fieldTypes)
		if err != nil {
			return nil, err
		}
		result = record
	}
	return NewOperExpr(Op("RECORD"), Typed(result), arguments...)
}

func (builder *TypedBuilder) EmptySeq(element Type) *OperExpr {
	return builder.EmptySequence(element)
}

func (builder *TypedBuilder) Seq(arguments ...Expr) (*OperExpr, error) {
	return builder.Sequence(arguments...)
}

func (builder *TypedBuilder) FunDef(body Expr, domains ...FunctionDomain) (*OperExpr, error) {
	arguments := make([]Expr, 0, 2*len(domains))
	for _, domain := range domains {
		arguments = append(arguments, domain.Variable, domain.Set)
	}
	return builder.FunDefMixed(body, arguments...)
}

func (builder *TypedBuilder) FunDefMixed(body Expr, arguments ...Expr) (*OperExpr, error) {
	if len(arguments) < 2 || len(arguments)%2 != 0 {
		return nil, fmt.Errorf("function domains require even, positive arity: got %d arguments", len(arguments))
	}
	seen := make(map[string]struct{}, len(arguments)/2)
	for index := 0; index < len(arguments); index += 2 {
		variable, ok := arguments[index].(*NameExpr)
		if !ok {
			return nil, fmt.Errorf("function variable %d is not a name", index/2)
		}
		if _, duplicate := seen[variable.Name()]; duplicate {
			return nil, fmt.Errorf("duplicate function variable %q", variable.Name())
		}
		seen[variable.Name()] = struct{}{}
	}
	operands := make([]Expr, 0, 1+len(arguments))
	operands = append(operands, body)
	operands = append(operands, arguments...)
	return builder.Apply(Op("FUN_CTOR"), operands...)
}

func (builder *TypedBuilder) App(function, argument Expr) (*OperExpr, error) {
	return builder.FunctionApplication(function, argument)
}

func (builder *TypedBuilder) Dom(function Expr) (*OperExpr, error) {
	return builder.Domain(function)
}

type ScopedRecordField struct {
	Name  string
	Value ExprInstruction
}

func (builder *ScopedBuilder) Rec(fields ...ScopedRecordField) ExprInstruction {
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
		return builder.typed.Rec(typedFields...)
	}}
}

func (builder *ScopedBuilder) RowRec(tail *VarType, fields ...ScopedRecordField) ExprInstruction {
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
		return builder.typed.RowRec(tail, typedFields...)
	}}
}

func (builder *ScopedBuilder) RecMixed(arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.RecMixed(built...)
	}}
}

func (builder *ScopedBuilder) RowRecMixed(tail *VarType, arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.RowRecMixed(tail, built...)
	}}
}

func (builder *ScopedBuilder) EmptySeq(element Type) ExprInstruction {
	return builder.EmptySequence(element)
}

func (builder *ScopedBuilder) Seq(arguments ...ExprInstruction) ExprInstruction {
	return builder.Sequence(arguments...)
}

func (builder *ScopedBuilder) FunDef(body ExprInstruction, domains ...BoundDomain) ExprInstruction {
	bound := builder.BindVariadic(Op("FUN_CTOR"), body, domains...)
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		result, err := bound.run(context)
		if err != nil {
			return nil, err
		}
		arguments := result.(*OperExpr).Arguments()
		for index := 1; index < len(arguments); index += 2 {
			if _, ok := arguments[index].(*NameExpr); !ok {
				return nil, fmt.Errorf("function variable %d is not a name", (index-1)/2)
			}
		}
		return result, nil
	}}
}

func (builder *ScopedBuilder) FunDefMixed(body ExprInstruction, arguments ...ExprInstruction) ExprInstruction {
	if len(arguments)%2 != 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("function domains require even arity: got %d arguments", len(arguments))
		}}
	}
	domains := make([]BoundDomain, len(arguments)/2)
	for index := range domains {
		domains[index] = BoundDomain{Variable: arguments[2*index], Set: arguments[2*index+1]}
	}
	return builder.FunDef(body, domains...)
}

func (builder *ScopedBuilder) App(function, argument ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, []ExprInstruction{function, argument})
		if err != nil {
			return nil, err
		}
		return builder.typed.App(built[0], built[1])
	}}
}

func (builder *ScopedBuilder) Dom(function ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, []ExprInstruction{function})
		if err != nil {
			return nil, err
		}
		return builder.typed.Dom(built[0])
	}}
}

func (builder *ScopedBuilder) Except(function, argument, replacement ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, []ExprInstruction{function, argument, replacement})
		if err != nil {
			return nil, err
		}
		return builder.typed.Except(built[0], built[1], built[2])
	}}
}
