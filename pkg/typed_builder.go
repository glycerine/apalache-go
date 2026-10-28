package apalache

import (
	"fmt"
	"math/big"
)

type TypedBuilder struct{}

func NewTypedBuilder() *TypedBuilder { return &TypedBuilder{} }

type TypedParameter struct {
	Parameter OperatorParam
	Type      Type
}

func NewTypedParameter(name string, typ Type) (TypedParameter, error) {
	if typ == nil {
		return TypedParameter{}, fmt.Errorf("parameter %q requires a type", name)
	}
	arity := 0
	if operatorType, ok := typ.(OperatorType); ok {
		if len(operatorType.arguments) == 0 {
			return TypedParameter{}, fmt.Errorf("parameter %q may not have a nullary operator type", name)
		}
		arity = len(operatorType.arguments)
	}
	parameter := TypedParameter{Parameter: OperatorParam{Name: name, Arity: arity}, Type: typ}
	if err := validateTypedParameter(parameter); err != nil {
		return TypedParameter{}, err
	}
	return parameter, nil
}

func validateTypedParameter(parameter TypedParameter) error {
	_, isOperator := parameter.Type.(OperatorType)
	if isOperator != (parameter.Parameter.Arity > 0) || !acceptableParameterType(parameter.Type, parameter.Parameter.Arity > 0) {
		return fmt.Errorf("parameter %s type %v and arity %d are inconsistent", parameter.Parameter.Name, parameter.Type, parameter.Parameter.Arity)
	}
	return nil
}

func acceptableParameterType(typ Type, allowRootOperator bool) bool {
	switch typed := typ.(type) {
	case primitiveType, ConstType, VarType:
		return true
	case FunType:
		return acceptableParameterType(typed.arg, false) && acceptableParameterType(typed.result, false)
	case SetType:
		return acceptableParameterType(typed.element, false)
	case SeqType:
		return acceptableParameterType(typed.element, false)
	case TupleType:
		return acceptableParameterTypes(typed.elements)
	case SparseTupleType:
		return acceptableParameterTypes(mapIntValues(typed.fields))
	case RecordType:
		return acceptableParameterTypes(mapStringValues(typed.fields))
	case OperatorType:
		return allowRootOperator && len(typed.arguments) > 0 && acceptableParameterTypes(typed.arguments) && acceptableParameterType(typed.result, false)
	case RowType:
		return acceptableParameterTypes(mapStringValues(typed.fields))
	case RecordRowType:
		return acceptableParameterType(typed.row, false)
	case VariantType:
		return acceptableParameterType(typed.row, false)
	default:
		return false
	}
}

func acceptableParameterTypes(types []Type) bool {
	for _, typ := range types {
		if !acceptableParameterType(typ, false) {
			return false
		}
	}
	return true
}

func ExpressionType(expr Expr) (Type, error) {
	if expr == nil {
		return nil, fmt.Errorf("expected an expression, found nil")
	}
	typ, ok := expr.Tag().Type()
	if !ok {
		return nil, fmt.Errorf("expression %s is untyped", expr.ID())
	}
	return typ, nil
}

func (builder *TypedBuilder) Apply(operator Operator, arguments ...Expr) (*OperExpr, error) {
	types := make([]Type, len(arguments))
	for index, argument := range arguments {
		var err error
		types[index], err = ExpressionType(argument)
		if err != nil {
			return nil, fmt.Errorf("argument %d to %s: %w", index, operator.Name(), err)
		}
	}
	resultType, err := InferOperatorType(operator, types)
	if err != nil {
		return nil, err
	}
	return NewOperExpr(operator, Typed(resultType), arguments...)
}

func (builder *TypedBuilder) Name(name string, typ Type) *NameExpr {
	return NewNameExpr(name, Typed(typ))
}

func (builder *TypedBuilder) Declaration(name string, body Expr, parameters ...TypedParameter) (*OperDecl, error) {
	bodyType, err := ExpressionType(body)
	if err != nil {
		return nil, err
	}
	formalParameters := make([]OperatorParam, len(parameters))
	parameterTypes := make([]Type, len(parameters))
	for index, parameter := range parameters {
		if err := validateTypedParameter(parameter); err != nil {
			return nil, err
		}
		formalParameters[index] = parameter.Parameter
		parameterTypes[index] = parameter.Type
	}
	declarationType, _ := NewOperatorType(parameterTypes, bodyType)
	return NewOperDecl(name, formalParameters, body, Typed(declarationType)), nil
}

func (builder *TypedBuilder) ApplyDeclaration(declaration *OperDecl, arguments ...Expr) (*OperExpr, error) {
	if declaration == nil {
		return nil, fmt.Errorf("cannot apply a nil declaration")
	}
	if len(arguments) != len(declaration.formalParams) {
		return nil, fmt.Errorf("operator %s of %d parameters is applied to %d arguments", declaration.Name(), len(declaration.formalParams), len(arguments))
	}
	declarationType, ok := declaration.Tag().Type()
	if !ok {
		return nil, fmt.Errorf("operator declaration %s is untyped", declaration.Name())
	}
	name := builder.Name(declaration.Name(), declarationType)
	return builder.Apply(Op("OPER_APP"), append([]Expr{name}, arguments...)...)
}

func (builder *TypedBuilder) LetIn(body Expr, declarations ...*OperDecl) (*LetInExpr, error) {
	if len(declarations) == 0 {
		return nil, fmt.Errorf("LetIn requires at least one declaration")
	}
	bodyType, err := ExpressionType(body)
	if err != nil {
		return nil, err
	}
	result := body
	for index := len(declarations) - 1; index >= 0; index-- {
		if declarations[index] == nil || !declarations[index].Tag().IsTyped() {
			return nil, fmt.Errorf("LET declaration %d is nil or untyped", index)
		}
		result = NewLetInExpr(result, Typed(bodyType), declarations[index])
	}
	return result.(*LetInExpr), nil
}

func (builder *TypedBuilder) Lambda(uniqueName string, body Expr, parameters ...TypedParameter) (*LetInExpr, error) {
	declaration, err := builder.Declaration(uniqueName, body, parameters...)
	if err != nil {
		return nil, err
	}
	typ, _ := declaration.Tag().Type()
	return builder.LetIn(builder.Name(uniqueName, typ), declaration)
}
func (builder *TypedBuilder) Int(value *big.Int) *ValueExpr {
	return NewValueExpr(NewIntValue(value), Typed(IntT1))
}
func (builder *TypedBuilder) Int64(value int64) *ValueExpr {
	return NewValueExpr(Int64Value(value), Typed(IntT1))
}
func (builder *TypedBuilder) Bool(value bool) *ValueExpr {
	return NewValueExpr(BoolValue(value), Typed(BoolT1))
}
func (builder *TypedBuilder) String(value string) (*ValueExpr, error) {
	if IsModelValue(value) {
		return nil, fmt.Errorf("%q is an uninterpreted model value, not a string", value)
	}
	return NewValueExpr(StringValue(value), Typed(StrT1)), nil
}
func (builder *TypedBuilder) ModelValue(root string, typ ConstType) (*ValueExpr, error) {
	if IsModelValue(root) {
		return nil, fmt.Errorf("model-value root %q already contains an _OF_ suffix", root)
	}
	return NewValueExpr(StringValue(ConstructModelValue(typ.Name(), root)), Typed(typ)), nil
}
func (builder *TypedBuilder) ParsedModelValue(value string) (*ValueExpr, error) {
	typ, _, ok := ParseModelValue(value)
	if !ok {
		return nil, fmt.Errorf("%q is a string, not an uninterpreted model value", value)
	}
	return NewValueExpr(StringValue(value), Typed(typ)), nil
}
func (builder *TypedBuilder) PredefinedSet(value PredefinedSet) *ValueExpr {
	var element Type
	switch value {
	case BoolSet:
		element = BoolT1
	case StringSet:
		element = StrT1
	case IntSet, NatSet:
		element = IntT1
	case RealSet:
		element = RealT1
	default:
		panic("unknown predefined set")
	}
	return NewValueExpr(value, Typed(newSetTypeUnchecked(element)))
}

func (builder *TypedBuilder) EmptySet(element Type) *OperExpr {
	return MustOperExpr(Op("SET_ENUM"), Typed(newSetTypeUnchecked(element)))
}
func (builder *TypedBuilder) EmptySequence(element Type) *OperExpr {
	return MustOperExpr(Op("TUPLE"), Typed(newSeqTypeUnchecked(element)))
}
func (builder *TypedBuilder) IsFiniteSet(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("FiniteSets!IsFiniteSet"), set)
}
func (builder *TypedBuilder) Cardinality(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("FiniteSets!Cardinality"), set)
}
func (builder *TypedBuilder) Tuple(arguments ...Expr) (*OperExpr, error) {
	types, err := expressionTypes(arguments)
	if err != nil {
		return nil, err
	}
	return NewOperExpr(Op("TUPLE"), Typed(newTupleTypeUnchecked(types...)), arguments...)
}
func (builder *TypedBuilder) Sequence(arguments ...Expr) (*OperExpr, error) {
	if len(arguments) == 0 {
		return nil, fmt.Errorf("Sequence requires at least one argument; use EmptySequence")
	}
	types, err := expressionTypes(arguments)
	if err != nil {
		return nil, err
	}
	for _, typ := range types[1:] {
		if !EqualTypes(types[0], typ) {
			return nil, fmt.Errorf("sequence elements must have identical types")
		}
	}
	return NewOperExpr(Op("TUPLE"), Typed(newSeqTypeUnchecked(types[0])), arguments...)
}

type RecordField struct {
	Name  string
	Value Expr
}

func (builder *TypedBuilder) Record(fields ...RecordField) (*OperExpr, error) {
	return builder.record(nil, false, fields...)
}
func (builder *TypedBuilder) RowRecord(tail *VarType, fields ...RecordField) (*OperExpr, error) {
	return builder.record(tail, true, fields...)
}
func (builder *TypedBuilder) record(tail *VarType, precise bool, fields ...RecordField) (*OperExpr, error) {
	fieldTypes := make(map[string]Type, len(fields))
	arguments := make([]Expr, 0, len(fields)*2)
	for _, field := range fields {
		if _, exists := fieldTypes[field.Name]; exists {
			return nil, fmt.Errorf("duplicate record field %q", field.Name)
		}
		typ, err := ExpressionType(field.Value)
		if err != nil {
			return nil, fmt.Errorf("record field %q: %w", field.Name, err)
		}
		fieldTypes[field.Name] = typ
		arguments = append(arguments, NewValueExpr(StringValue(field.Name), Typed(StrT1)), field.Value)
	}
	var resultType Type
	if precise {
		row, _ := NewRowType(fieldTypes, tail)
		resultType = NewRecordRowType(row)
	} else {
		legacy, _ := NewRecordType(fieldTypes)
		resultType = legacy
	}
	return NewOperExpr(Op("RECORD"), Typed(resultType), arguments...)
}

func (builder *TypedBuilder) RecordSet(fields ...RecordField) (*OperExpr, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("RecordSet requires at least one field")
	}
	fieldTypes := make(map[string]Type, len(fields))
	arguments := make([]Expr, 0, len(fields)*2)
	for _, field := range fields {
		if _, duplicate := fieldTypes[field.Name]; duplicate {
			return nil, fmt.Errorf("duplicate record-set field %q", field.Name)
		}
		typ, err := ExpressionType(field.Value)
		if err != nil {
			return nil, err
		}
		setType, ok := typ.(SetType)
		if !ok {
			return nil, fmt.Errorf("record-set field %q is not a set", field.Name)
		}
		fieldTypes[field.Name] = setType.element
		arguments = append(arguments, NewValueExpr(StringValue(field.Name), Typed(StrT1)), field.Value)
	}
	record, _ := NewRecordType(fieldTypes)
	return NewOperExpr(Op("RECORD_SET"), Typed(newSetTypeUnchecked(record)), arguments...)
}

func (builder *TypedBuilder) FunctionApplication(function, argument Expr) (*OperExpr, error) {
	functionType, err := ExpressionType(function)
	if err != nil {
		return nil, err
	}
	argumentType, err := ExpressionType(argument)
	if err != nil {
		return nil, err
	}
	from, to, ok := ApplicativeType(functionType, argument)
	if !ok || !EqualTypes(from, argumentType) {
		return nil, fmt.Errorf("%v cannot be applied to an argument of type %v", functionType, argumentType)
	}
	return NewOperExpr(Op("FUN_APP"), Typed(to), function, argument)
}

func (builder *TypedBuilder) Domain(function Expr) (*OperExpr, error) {
	functionType, err := ExpressionType(function)
	if err != nil {
		return nil, err
	}
	domain, ok := ApplicativeDomain(functionType)
	if !ok {
		return nil, fmt.Errorf("DOMAIN requires an applicative type, found %v", functionType)
	}
	return NewOperExpr(Op("DOMAIN"), Typed(newSetTypeUnchecked(domain)), function)
}

func (builder *TypedBuilder) Except(function, argument, replacement Expr) (*OperExpr, error) {
	functionType, err := ExpressionType(function)
	if err != nil {
		return nil, err
	}
	argumentType, err := ExpressionType(argument)
	if err != nil {
		return nil, err
	}
	replacementType, err := ExpressionType(replacement)
	if err != nil {
		return nil, err
	}
	from, to, ok := ApplicativeType(functionType, argument)
	if !ok || !CompatibleTypes(from, argumentType) || !CompatibleTypes(to, replacementType) {
		return nil, fmt.Errorf("EXCEPT argument/replacement types do not match %v", functionType)
	}
	wrapped, err := builder.Tuple(argument)
	if err != nil {
		return nil, err
	}
	return NewOperExpr(Op("EXCEPT"), Typed(functionType), function, wrapped, replacement)
}

func ApplicativeDomain(typ Type) (Type, bool) {
	switch typed := typ.(type) {
	case FunType:
		return typed.arg, true
	case RecordType, RowType, RecordRowType:
		return StrT1, true
	case SeqType, TupleType, SparseTupleType:
		return IntT1, true
	default:
		return nil, false
	}
}

func ApplicativeType(typ Type, argument Expr) (from, to Type, ok bool) {
	switch typed := typ.(type) {
	case FunType:
		return typed.arg, typed.result, true
	case SeqType:
		return IntT1, typed.element, true
	case RecordType:
		return applicativeStringField(typed.fields, argument)
	case RowType:
		return applicativeStringField(typed.fields, argument)
	case RecordRowType:
		return applicativeStringField(typed.row.fields, argument)
	case TupleType:
		index, indexOK := literalIntIndex(argument)
		if !indexOK || index < 1 || index > len(typed.elements) {
			return nil, nil, false
		}
		return IntT1, typed.elements[index-1], true
	case SparseTupleType:
		index, indexOK := literalIntIndex(argument)
		field, exists := typed.fields[index]
		if !indexOK || !exists {
			return nil, nil, false
		}
		return IntT1, field, true
	default:
		return nil, nil, false
	}
}

func applicativeStringField(fields map[string]Type, argument Expr) (Type, Type, bool) {
	valueExpr, ok := argument.(*ValueExpr)
	if !ok {
		return nil, nil, false
	}
	value, ok := valueExpr.Value().(StringValue)
	if !ok {
		return nil, nil, false
	}
	field, exists := fields[string(value)]
	return StrT1, field, exists
}

func literalIntIndex(argument Expr) (int, bool) {
	valueExpr, ok := argument.(*ValueExpr)
	if !ok {
		return 0, false
	}
	value, ok := valueExpr.Value().(IntValue)
	if !ok || !value.BigInt().IsInt64() {
		return 0, false
	}
	integer := value.BigInt().Int64()
	if int64(int(integer)) != integer {
		return 0, false
	}
	return int(integer), true
}

func (builder *TypedBuilder) Variant(tag string, value Expr, target VariantType) (*OperExpr, error) {
	expected, exists := target.row.fields[tag]
	if !exists {
		return nil, fmt.Errorf("variant type does not contain tag %q", tag)
	}
	actual, err := ExpressionType(value)
	if err != nil || !CompatibleTypes(actual, expected) {
		return nil, fmt.Errorf("variant value for %q has incompatible type", tag)
	}
	tagExpr := NewValueExpr(StringValue(tag), Typed(StrT1))
	return NewOperExpr(Op("Variants!Variant"), Typed(target), tagExpr, value)
}

func (builder *TypedBuilder) VariantFilter(tag string, set Expr) (*OperExpr, error) {
	typ, err := ExpressionType(set)
	if err != nil {
		return nil, err
	}
	setType, ok := typ.(SetType)
	if !ok {
		return nil, fmt.Errorf("variant filter requires a set")
	}
	variant, ok := setType.element.(VariantType)
	if !ok {
		return nil, fmt.Errorf("variant filter requires a set of variants")
	}
	field, exists := variant.row.fields[tag]
	if !exists {
		return nil, fmt.Errorf("variant type does not contain tag %q", tag)
	}
	tagExpr := NewValueExpr(StringValue(tag), Typed(StrT1))
	return NewOperExpr(Op("Variants!VariantFilter"), Typed(newSetTypeUnchecked(field)), tagExpr, set)
}

func (builder *TypedBuilder) VariantGetUnsafe(tag string, variant Expr) (*OperExpr, error) {
	return builder.variantGet(tag, variant, nil)
}
func (builder *TypedBuilder) VariantGetOrElse(tag string, variant, fallback Expr) (*OperExpr, error) {
	return builder.variantGet(tag, variant, fallback)
}
func (builder *TypedBuilder) variantGet(tag string, variantExpr, fallback Expr) (*OperExpr, error) {
	typ, err := ExpressionType(variantExpr)
	if err != nil {
		return nil, err
	}
	variant, ok := typ.(VariantType)
	if !ok {
		return nil, fmt.Errorf("variant get requires a variant")
	}
	field, exists := variant.row.fields[tag]
	if !exists {
		return nil, fmt.Errorf("variant type does not contain tag %q", tag)
	}
	tagExpr := NewValueExpr(StringValue(tag), Typed(StrT1))
	if fallback == nil {
		return NewOperExpr(Op("Variants!VariantGetUnsafe"), Typed(field), tagExpr, variantExpr)
	}
	fallbackType, err := ExpressionType(fallback)
	if err != nil || !EqualTypes(fallbackType, field) {
		return nil, fmt.Errorf("variant fallback has type %v, want %v", fallbackType, field)
	}
	return NewOperExpr(Op("Variants!VariantGetOrElse"), Typed(field), tagExpr, variantExpr, fallback)
}

func expressionTypes(expressions []Expr) ([]Type, error) {
	result := make([]Type, len(expressions))
	for index, expression := range expressions {
		var err error
		result[index], err = ExpressionType(expression)
		if err != nil {
			return nil, fmt.Errorf("expression %d: %w", index, err)
		}
	}
	return result, nil
}
