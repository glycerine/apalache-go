package apalache

import (
	"errors"
	"fmt"
)

// ValidatedIRJSONDecoder is the typed/scoped JSON-builder boundary. The plain
// IRJSONDecoder intentionally accepts legacy untyped IR; this decoder matches
// JsonToTlaViaBuilder by rejecting untyped expressions, inconsistent name
// types, bad operator signatures, and result-tag mismatches.
type ValidatedIRJSONDecoder struct{ SourceStore *SourceStore }

func (decoder ValidatedIRJSONDecoder) Expression(value JSONRepresentation) (Expr, error) {
	expression, err := (IRJSONDecoder{SourceStore: decoder.SourceStore}).Expression(value)
	if err != nil {
		return nil, err
	}
	if err = ValidateTypedIRExpression(expression); err != nil {
		return nil, NewJSONDeserializationError("JSON contains an invalid typed expression", err)
	}
	return expression, nil
}

func (decoder ValidatedIRJSONDecoder) Declaration(value JSONRepresentation) (Decl, error) {
	declaration, err := (IRJSONDecoder{SourceStore: decoder.SourceStore}).Declaration(value)
	if err != nil {
		return nil, err
	}
	if err = ValidateTypedIRDeclaration(declaration); err != nil {
		return nil, NewJSONDeserializationError("JSON contains an invalid typed declaration", err)
	}
	return declaration, nil
}

func (decoder ValidatedIRJSONDecoder) Module(value JSONRepresentation) (*Module, error) {
	module, err := (IRJSONDecoder{SourceStore: decoder.SourceStore}).Module(value)
	if err != nil {
		return nil, err
	}
	if err = ValidateTypedIRModule(module); err != nil {
		return nil, NewJSONDeserializationError("JSON contains an invalid typed module", err)
	}
	return module, nil
}

func (decoder ValidatedIRJSONDecoder) Root(value JSONRepresentation) ([]*Module, error) {
	modules, err := (IRJSONDecoder{SourceStore: decoder.SourceStore}).Root(value)
	if err != nil {
		return nil, err
	}
	for index, module := range modules {
		if err = ValidateTypedIRModule(module); err != nil {
			return nil, NewJSONDeserializationError(fmt.Sprintf("JSON module %d is invalid", index), err)
		}
	}
	return modules, nil
}

func (decoder ValidatedIRJSONDecoder) SingleModule(value JSONRepresentation) (*Module, error) {
	modules, err := decoder.Root(value)
	if err != nil {
		return nil, err
	}
	if len(modules) != 1 {
		return nil, NewJSONDeserializationError(fmt.Sprintf("JSON included %d modules, expected one", len(modules)), nil)
	}
	return modules[0], nil
}

// ValidateTypedIRExpression checks the invariants enforced while Scala's
// ScopedBuilder executes a JSON expression.
func ValidateTypedIRExpression(expression Expr) error {
	return validateTypedIRExpression(expression, make(map[string]Type))
}

func validateTypedIRExpression(expression Expr, names map[string]Type) error {
	if expression == nil || expression == NullExpr() {
		return fmt.Errorf("NullEx is not supported")
	}
	typ, err := ExpressionType(expression)
	if err != nil {
		return err
	}
	switch typed := expression.(type) {
	case *NameExpr:
		if previous, found := names[typed.Name()]; found && !EqualTypes(previous, typ) {
			return fmt.Errorf("name %s has inconsistent types %s and %s", typed.Name(), previous, typ)
		}
		names[typed.Name()] = typ
		return nil
	case *ValueExpr:
		actual, valueErr := typeOfTLAValue(typed.Value())
		if valueErr != nil {
			return valueErr
		}
		if !EqualTypes(actual, typ) {
			return fmt.Errorf("value %v is tagged with %s, expected %s", typed.Value(), typ, actual)
		}
		return nil
	case *OperExpr:
		arguments := typed.Arguments()
		argumentTypes := make([]Type, len(arguments))
		for index, argument := range arguments {
			if err = validateTypedIRExpression(argument, names); err != nil {
				return fmt.Errorf("argument %d to %s: %w", index, typed.Operator().Name(), err)
			}
			argumentTypes[index], _ = argument.Tag().Type()
		}
		inferred, inferErr := InferOperatorType(typed.Operator(), argumentTypes)
		if inferErr != nil {
			if errors.Is(inferErr, ErrUnknownOperatorSignature) {
				return validateValueDependentIRType(typed, typ, argumentTypes)
			}
			return inferErr
		}
		if !EqualTypes(inferred, typ) {
			return fmt.Errorf("%s is tagged with %s, expected %s", typed.Operator().Name(), typ, inferred)
		}
		return nil
	case *LetInExpr:
		for _, declaration := range typed.Declarations() {
			if err = validateTypedIRDeclaration(declaration, names); err != nil {
				return err
			}
		}
		if err = validateTypedIRExpression(typed.Body(), names); err != nil {
			return err
		}
		bodyType, _ := typed.Body().Tag().Type()
		if !EqualTypes(bodyType, typ) {
			return fmt.Errorf("LET-IN is tagged with %s, expected %s", typ, bodyType)
		}
		return nil
	default:
		return fmt.Errorf("unsupported expression %T", expression)
	}
}

func validateValueDependentIRType(expression *OperExpr, result Type, arguments []Type) error {
	switch expression.Operator().Name() {
	case "TUPLE":
		tuple, ok := result.(TupleType)
		if !ok || !equalTypeSlices(tuple.Elements(), arguments) {
			return fmt.Errorf("tuple arguments %v do not match result %s", arguments, result)
		}
		return nil
	case "FUN_APP":
		if len(arguments) == 2 {
			if function, ok := arguments[0].(FunType); ok && CompatibleTypes(function.Argument(), arguments[1]) && EqualTypes(function.Result(), result) {
				return nil
			}
		}
	case "DOMAIN":
		if len(arguments) == 1 {
			if function, ok := arguments[0].(FunType); ok && EqualTypes(result, newSetTypeUnchecked(function.Argument())) {
				return nil
			}
		}
	case "EXCEPT":
		if len(arguments) >= 3 && EqualTypes(result, arguments[0]) {
			return nil
		}
	case "RECORD":
		if validateRecordIRType(expression.Arguments(), result, false) == nil {
			return nil
		}
	case "RECORD_SET":
		if validateRecordIRType(expression.Arguments(), result, true) == nil {
			return nil
		}
	case "Variants!Variant":
		if len(arguments) == 2 {
			if _, ok := result.(VariantType); ok && EqualTypes(arguments[0], StrT1) {
				return nil
			}
		}
	}
	return fmt.Errorf("cannot validate %s(%v) with result tag %s", expression.Operator().Name(), arguments, result)
}

func validateRecordIRType(arguments []Expr, result Type, set bool) error {
	fields := make(map[string]Type)
	if len(arguments)%2 != 0 {
		return fmt.Errorf("record has odd argument count")
	}
	for index := 0; index < len(arguments); index += 2 {
		literal, ok := arguments[index].(*ValueExpr)
		if !ok {
			return fmt.Errorf("record key is not a literal")
		}
		key, ok := literal.Value().(StringValue)
		if !ok {
			return fmt.Errorf("record key is not a string")
		}
		fieldType, _ := arguments[index+1].Tag().Type()
		if set {
			setType, ok := fieldType.(SetType)
			if !ok {
				return fmt.Errorf("record-set field is not a set")
			}
			fieldType = setType.Element()
		}
		fields[key.Value()] = fieldType
	}
	expected, _ := NewRecordType(fields)
	if set {
		return requireEqualIRType(result, newSetTypeUnchecked(expected))
	}
	return requireEqualIRType(result, expected)
}

func requireEqualIRType(actual, expected Type) error {
	if !EqualTypes(actual, expected) {
		return fmt.Errorf("type %s does not match %s", actual, expected)
	}
	return nil
}

func ValidateTypedIRDeclaration(declaration Decl) error {
	return validateTypedIRDeclaration(declaration, make(map[string]Type))
}

func validateTypedIRDeclaration(declaration Decl, names map[string]Type) error {
	operator, ok := declaration.(*OperDecl)
	if !ok {
		if assumption, ok := declaration.(*AssumeDecl); ok {
			return validateTypedIRExpression(assumption.Body(), names)
		}
		if theorem, ok := declaration.(*TheoremDecl); ok {
			return validateTypedIRExpression(theorem.Body(), names)
		}
		return nil
	}
	typ, ok := operator.Tag().Type()
	if !ok {
		return fmt.Errorf("operator declaration %s is untyped", operator.Name())
	}
	operatorType, ok := typ.(OperatorType)
	if !ok || len(operatorType.Arguments()) != len(operator.FormalParams()) {
		return fmt.Errorf("operator declaration %s has an inconsistent type", operator.Name())
	}
	localNames := cloneMap(names)
	for index, parameter := range operator.FormalParams() {
		parameterType := operatorType.Arguments()[index]
		_, higherOrder := parameterType.(OperatorType)
		if higherOrder != (parameter.Arity > 0) {
			return fmt.Errorf("parameter %s arity and type are inconsistent", parameter.Name)
		}
		localNames[parameter.Name] = parameterType
	}
	if err := validateTypedIRExpression(operator.Body(), localNames); err != nil {
		return fmt.Errorf("operator declaration %s: %w", operator.Name(), err)
	}
	bodyType, _ := operator.Body().Tag().Type()
	return requireEqualIRType(bodyType, operatorType.Result())
}

func ValidateTypedIRModule(module *Module) error {
	if module == nil {
		return fmt.Errorf("module is nil")
	}
	names := make(map[string]Type)
	for _, declaration := range module.Declarations() {
		if err := validateTypedIRDeclaration(declaration, names); err != nil {
			return err
		}
		if typ, ok := declaration.Tag().Type(); ok {
			names[declaration.Name()] = typ
		}
	}
	return nil
}
