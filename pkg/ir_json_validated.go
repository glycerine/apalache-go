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
	if err := validateIRBinderScopes(expression, make(map[string]struct{})); err != nil {
		return err
	}
	return validateTypedIRExpression(expression, make(map[string]Type))
}

func validateIRBinderScopes(expression Expr, bound map[string]struct{}) error {
	if expression == nil || expression == NullExpr() {
		return nil
	}
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		switch typed.Operator().Name() {
		case "CHOOSE3", "FORALL3", "EXISTS3", "SET_FILTER":
			return validateIRBoundOperator(arguments[0], []Expr{arguments[1]}, arguments[2], bound)
		case "CHOOSE2", "FORALL2", "EXISTS2", "TEMPORAL_EXISTS", "TEMPORAL_FORALL":
			return validateIRBoundOperator(arguments[0], nil, arguments[1], bound)
		case "SET_MAP", "FUN_CTOR":
			variables := make([]Expr, 0, (len(arguments)-1)/2)
			domains := make([]Expr, 0, (len(arguments)-1)/2)
			for index := 1; index < len(arguments); index += 2 {
				variables = append(variables, arguments[index])
				domains = append(domains, arguments[index+1])
			}
			return validateIRVariadicBinder(variables, domains, arguments[0], bound)
		default:
			for _, argument := range arguments {
				if err := validateIRBinderScopes(argument, bound); err != nil {
					return err
				}
			}
		}
	case *LetInExpr:
		for _, declaration := range typed.Declarations() {
			if err := validateIRBinderScopes(declaration.Body(), bound); err != nil {
				return err
			}
		}
		return validateIRBinderScopes(typed.Body(), bound)
	}
	return nil
}

func validateIRBoundOperator(variable Expr, domains []Expr, body Expr, bound map[string]struct{}) error {
	names, err := boundVariableNames(variable)
	if err != nil {
		return err
	}
	for name := range names {
		if _, shadowed := bound[name]; shadowed {
			return &ScopeError{Message: fmt.Sprintf("variable shadowing: %s", name)}
		}
	}
	for _, domain := range domains {
		occurrences := irExpressionNames(domain)
		for name := range names {
			if _, used := occurrences[name]; used {
				return &ScopeError{Message: fmt.Sprintf("variable %s is used in its own domain", name)}
			}
		}
		if err = validateIRBinderScopes(domain, bound); err != nil {
			return err
		}
	}
	inner := cloneNameSet(bound)
	for name := range names {
		inner[name] = struct{}{}
	}
	return validateIRBinderScopes(body, inner)
}

func validateIRVariadicBinder(variables, domains []Expr, body Expr, bound map[string]struct{}) error {
	allNames := make(map[string]struct{})
	for _, variable := range variables {
		names, err := boundVariableNames(variable)
		if err != nil {
			return err
		}
		for name := range names {
			if _, duplicate := allNames[name]; duplicate {
				return &ScopeError{Message: fmt.Sprintf("duplicate bound variable %s", name)}
			}
			if _, shadowed := bound[name]; shadowed {
				return &ScopeError{Message: fmt.Sprintf("variable shadowing: %s", name)}
			}
			allNames[name] = struct{}{}
		}
	}
	for _, domain := range domains {
		occurrences := irExpressionNames(domain)
		for name := range allNames {
			if _, used := occurrences[name]; used {
				return &ScopeError{Message: fmt.Sprintf("variable %s is used in a binder domain", name)}
			}
		}
		if err := validateIRBinderScopes(domain, bound); err != nil {
			return err
		}
	}
	inner := cloneNameSet(bound)
	for name := range allNames {
		inner[name] = struct{}{}
	}
	return validateIRBinderScopes(body, inner)
}

func irExpressionNames(expression Expr) map[string]struct{} {
	result := make(map[string]struct{})
	var visit func(Expr)
	visit = func(current Expr) {
		switch typed := current.(type) {
		case *NameExpr:
			result[typed.Name()] = struct{}{}
		case *OperExpr:
			for _, argument := range typed.Arguments() {
				visit(argument)
			}
		case *LetInExpr:
			for _, declaration := range typed.Declarations() {
				visit(declaration.Body())
			}
			visit(typed.Body())
		}
	}
	visit(expression)
	return result
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
		if _, unsupported := typed.Value().(DecimalValue); unsupported {
			return fmt.Errorf("TlaDecimal is not supported by the typed JSON builder")
		}
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
		if err = validateTypedIRLiteralRequirements(typed, typ, argumentTypes); err != nil {
			return err
		}
		inferred, inferErr := InferOperatorType(typed.Operator(), argumentTypes)
		if inferErr != nil {
			if errors.Is(inferErr, ErrUnknownOperatorSignature) || typed.Operator().Name() == "SET_ENUM" && len(arguments) == 0 {
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

func validateTypedIRLiteralRequirements(expression *OperExpr, result Type, arguments []Type) error {
	name := expression.Operator().Name()
	switch name {
	case "REAL_DIV", "FUN_REC_CTOR", "FUN_REC_REF", "Apalache!<:", "TLC!TLCGet", "TLC!ToString",
		"SequencesExt!ToSet", "ApalacheInternal!__ApalacheSeqCapacity",
		"ApalacheInternal!__CompactFunSetExpand", "ApalacheInternal!__NotSupportedByModelChecker":
		return fmt.Errorf("%s is not supported by the typed JSON builder", name)
	case "LABEL":
		for index, argument := range expression.Arguments()[1:] {
			if _, ok := stringIRLiteral(argument); !ok {
				return fmt.Errorf("LABEL argument %d must be a string literal", index+1)
			}
		}
	case "Apalache!MkSeq":
		if _, ok := integerIRLiteral(expression.Arguments()[0]); !ok {
			return fmt.Errorf("Apalache!MkSeq requires an integer literal")
		}
	case "Apalache!Repeat":
		if _, ok := integerIRLiteral(expression.Arguments()[1]); !ok {
			return fmt.Errorf("Apalache!Repeat requires an integer literal")
		}
	case "Apalache!Gen":
		if len(arguments) != 1 || !EqualTypes(arguments[0], IntT1) || result == nil {
			return fmt.Errorf("Apalache!Gen requires an integer bound and typed result")
		}
	case "Variants!Variant", "Variants!VariantFilter", "Variants!VariantGetUnsafe", "Variants!VariantGetOrElse":
		if _, ok := stringIRLiteral(expression.Arguments()[0]); !ok {
			return fmt.Errorf("%s tag must be a string literal", name)
		}
	}
	return nil
}

func stringIRLiteral(expression Expr) (string, bool) {
	value, ok := expression.(*ValueExpr)
	if !ok {
		return "", false
	}
	stringValue, ok := value.Value().(StringValue)
	if !ok {
		return "", false
	}
	return stringValue.Value(), true
}

func integerIRLiteral(expression Expr) (IntValue, bool) {
	value, ok := expression.(*ValueExpr)
	if !ok {
		return IntValue{}, false
	}
	integer, ok := value.Value().(IntValue)
	return integer, ok
}

func validateValueDependentIRType(expression *OperExpr, result Type, arguments []Type) error {
	switch expression.Operator().Name() {
	case "SET_ENUM":
		if len(arguments) == 0 {
			if _, ok := result.(SetType); ok {
				return nil
			}
		}
	case "TUPLE":
		if tuple, ok := result.(TupleType); ok && equalTypeSlices(tuple.Elements(), arguments) {
			return nil
		}
		if sequence, ok := result.(SeqType); ok {
			for _, argument := range arguments {
				if !CompatibleTypes(argument, sequence.Element()) {
					return fmt.Errorf("sequence element %s does not match result %s", argument, result)
				}
			}
			return nil
		}
		return fmt.Errorf("tuple arguments %v do not match result %s", arguments, result)
	case "FUN_APP":
		if len(arguments) == 2 {
			from, to, ok := ApplicativeType(arguments[0], expression.Arguments()[1])
			if ok && EqualTypes(from, arguments[1]) && EqualTypes(to, result) {
				return nil
			}
		}
	case "DOMAIN":
		if len(arguments) == 1 {
			if domain, ok := ApplicativeDomain(arguments[0]); ok && EqualTypes(result, newSetTypeUnchecked(domain)) {
				return nil
			}
		}
	case "EXCEPT":
		if validateExceptIRType(expression, result, arguments) == nil {
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
		if len(arguments) == 2 && EqualTypes(arguments[0], StrT1) {
			variant, ok := result.(VariantType)
			tag, literal := stringIRLiteral(expression.Arguments()[0])
			if ok && literal {
				if valueType, found := variant.Row().Fields()[tag]; found && CompatibleTypes(arguments[1], valueType) {
					return nil
				}
			}
		}
	case "Variants!VariantFilter":
		if len(arguments) == 2 && EqualTypes(arguments[0], StrT1) {
			set, setOK := arguments[1].(SetType)
			variant, ok := set.Element().(VariantType)
			tag, literal := stringIRLiteral(expression.Arguments()[0])
			if setOK && ok && literal {
				if valueType, found := variant.Row().Fields()[tag]; found && EqualTypes(result, newSetTypeUnchecked(valueType)) {
					return nil
				}
			}
		}
	case "Variants!VariantGetUnsafe":
		if len(arguments) == 2 && EqualTypes(arguments[0], StrT1) {
			variant, ok := arguments[1].(VariantType)
			tag, literal := stringIRLiteral(expression.Arguments()[0])
			if ok && literal {
				if valueType, found := variant.Row().Fields()[tag]; found && EqualTypes(result, valueType) {
					return nil
				}
			}
		}
	case "Variants!VariantGetOrElse":
		if len(arguments) == 3 && EqualTypes(arguments[0], StrT1) {
			variant, ok := arguments[1].(VariantType)
			tag, literal := stringIRLiteral(expression.Arguments()[0])
			if ok && literal {
				if valueType, found := variant.Row().Fields()[tag]; found && EqualTypes(valueType, arguments[2]) && EqualTypes(result, valueType) {
					return nil
				}
			}
		}
	case "Apalache!Gen":
		if len(arguments) == 1 && EqualTypes(arguments[0], IntT1) {
			return nil
		}
	}
	return fmt.Errorf("cannot validate %s(%v) with result tag %s", expression.Operator().Name(), arguments, result)
}

func validateExceptIRType(expression *OperExpr, result Type, arguments []Type) error {
	if len(arguments) < 3 || (len(arguments)-1)%2 != 0 || !EqualTypes(result, arguments[0]) {
		return fmt.Errorf("EXCEPT has an invalid result type or update count")
	}
	expressions := expression.Arguments()
	for index := 1; index < len(expressions); index += 2 {
		path, ok := expressions[index].(*OperExpr)
		if !ok || path.Operator().Name() != "TUPLE" || len(path.Arguments()) == 0 {
			return fmt.Errorf("EXCEPT update path %d is not a nonempty tuple", (index+1)/2)
		}
		current := arguments[0]
		for _, component := range path.Arguments() {
			componentType, err := ExpressionType(component)
			if err != nil {
				return err
			}
			from, to, applicable := ApplicativeType(current, component)
			if !applicable || !EqualTypes(from, componentType) {
				return fmt.Errorf("EXCEPT path component cannot index %s", current)
			}
			current = to
		}
		if replacement := arguments[index+1]; !EqualTypes(current, replacement) {
			return fmt.Errorf("EXCEPT replacement has type %s, expected %s", replacement, current)
		}
	}
	return nil
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
	if set {
		setResult, ok := result.(SetType)
		if !ok {
			return fmt.Errorf("record-set result is not a set")
		}
		return requireRecordFields(setResult.Element(), fields)
	}
	return requireRecordFields(result, fields)
}

func requireRecordFields(actual Type, expected map[string]Type) error {
	var fields map[string]Type
	switch typed := actual.(type) {
	case RecordType:
		fields = typed.Fields()
	case RecordRowType:
		if _, open := typed.Row().Other(); open {
			return fmt.Errorf("record result %s is open", actual)
		}
		fields = typed.Row().Fields()
	default:
		return fmt.Errorf("result %s is not a record", actual)
	}
	if !equalStringTypeMaps(fields, expected) {
		return fmt.Errorf("record fields %v do not match %v", fields, expected)
	}
	return nil
}

func requireEqualIRType(actual, expected Type) error {
	if !EqualTypes(actual, expected) {
		return fmt.Errorf("type %s does not match %s", actual, expected)
	}
	return nil
}

func ValidateTypedIRDeclaration(declaration Decl) error {
	if operator, ok := declaration.(*OperDecl); ok {
		if err := validateIRBinderScopes(operator.Body(), make(map[string]struct{})); err != nil {
			return err
		}
	}
	return validateTypedIRDeclaration(declaration, make(map[string]Type))
}

func validateTypedIRDeclaration(declaration Decl, names map[string]Type) error {
	operator, ok := declaration.(*OperDecl)
	if !ok {
		if assumption, ok := declaration.(*AssumeDecl); ok {
			return validateTypedIRExpression(assumption.Body(), names)
		}
		if theorem, ok := declaration.(*TheoremDecl); ok {
			if err := validateTypedIRExpression(theorem.Body(), names); err != nil {
				return err
			}
			declarationType, typed := theorem.Tag().Type()
			if !typed {
				return fmt.Errorf("theorem declaration %s is untyped", theorem.Name())
			}
			bodyType, _ := theorem.Body().Tag().Type()
			return requireEqualIRType(bodyType, declarationType)
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
