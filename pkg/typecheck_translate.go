package apalache

import (
	"fmt"
	"strings"
)

type ToEtcExpr struct {
	annotations map[UID]Type
	aliases     *TypeAliasSubstitution
	pool        *TypeVarPool
	useRows     bool
}

func NewToEtcExpr(annotations map[UID]Type, aliases *TypeAliasSubstitution, pool *TypeVarPool, useRows bool) *ToEtcExpr {
	if pool == nil {
		panic("NewToEtcExpr requires a type variable pool")
	}
	if aliases == nil {
		aliases = EmptyTypeAliasSubstitution()
	}
	copy := make(map[UID]Type, len(annotations))
	for id, typ := range annotations {
		copy[id] = typ
	}
	return &ToEtcExpr{annotations: copy, aliases: aliases, pool: pool, useRows: useRows}
}

func (translator *ToEtcExpr) TranslateDeclaration(declaration Decl, scoped EtcExpr) (EtcExpr, error) {
	if declaration == nil || scoped == nil {
		return nil, fmt.Errorf("declaration translation requires declaration and scoped expression")
	}
	switch typed := declaration.(type) {
	case *ConstDecl:
		return translator.translateTypedNameDeclaration(typed, "CONSTANT", scoped)
	case *VarDecl:
		return translator.translateTypedNameDeclaration(typed, "VARIABLE", scoped)
	case *AssumeDecl:
		body, err := translator.TranslateExpression(typed.Body())
		if err != nil {
			return nil, err
		}
		application := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{mustOperator([]Type{BoolT1}, BoolT1)}, body)
		abstraction := NewEtcAbs(ExactRef{SourceID: typed.ID()}, application)
		return NewEtcLet(BlameRef{SourceID: typed.ID()}, typed.Name(), abstraction, scoped), nil
	case *TheoremDecl:
		body, err := translator.TranslateExpression(typed.Body())
		if err != nil {
			return nil, err
		}
		application := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{mustOperator([]Type{BoolT1}, BoolT1)}, body)
		abstraction := NewEtcAbs(ExactRef{SourceID: typed.ID()}, application)
		return NewEtcLet(BlameRef{SourceID: typed.ID()}, typed.Name(), abstraction, scoped), nil
	case *OperDecl:
		return translator.translateOperatorDeclaration(typed, scoped)
	default:
		return nil, NewTypingInputException(fmt.Sprintf("Unexpected declaration: %T", declaration), declaration.ID())
	}
}

func (translator *ToEtcExpr) translateTypedNameDeclaration(declaration Decl, kind string, scoped EtcExpr) (EtcExpr, error) {
	typ, found, err := translator.declarationType(declaration)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, NewTypingInputException(
			fmt.Sprintf("Expected a type annotation for %s %s", kind, declaration.Name()), declaration.ID())
	}
	return NewEtcTypeDecl(ExactRef{SourceID: declaration.ID()}, declaration.Name(), typ, scoped), nil
}

func (translator *ToEtcExpr) translateOperatorDeclaration(declaration *OperDecl, scoped EtcExpr) (EtcExpr, error) {
	body, err := translator.TranslateExpression(declaration.Body())
	if err != nil {
		return nil, err
	}
	bindings := make([]EtcBinding, len(declaration.FormalParams()))
	for i, parameter := range declaration.FormalParams() {
		var parameterType Type
		if parameter.Arity <= 0 {
			parameterType = translator.pool.Fresh()
		} else {
			arguments := make([]Type, parameter.Arity)
			for j := range arguments {
				arguments[j] = translator.pool.Fresh()
			}
			parameterType = mustOperator(arguments, translator.pool.Fresh())
		}
		bindings[i] = EtcBinding{
			Name:   NewEtcName(BlameRef{SourceID: declaration.ID()}, parameter.Name),
			Domain: NewEtcConst(BlameRef{SourceID: declaration.ID()}, mustSet(parameterType)),
		}
	}
	abstraction := NewEtcAbs(ExactRef{SourceID: declaration.ID()}, body, bindings...)
	let := NewEtcLet(BlameRef{SourceID: declaration.ID()}, declaration.Name(), abstraction, scoped)
	typ, found, err := translator.declarationType(declaration)
	if err != nil {
		return nil, err
	}
	if !found {
		return let, nil
	}
	if len(declaration.FormalParams()) == 0 {
		if _, operator := typ.(OperatorType); !operator {
			typ = mustOperator(nil, typ)
		}
	}
	return NewEtcTypeDecl(ExactRef{SourceID: declaration.ID()}, declaration.Name(), typ, let), nil
}

func (translator *ToEtcExpr) declarationType(declaration Decl) (Type, bool, error) {
	if typ, found := declaration.Tag().Type(); found {
		return typ, true, nil
	}
	typ, found := translator.annotations[declaration.ID()]
	if !found {
		return nil, false, nil
	}
	replaced, _, err := translator.aliases.Apply(typ)
	if err != nil {
		return nil, false, err
	}
	renamed, err := translator.renameVariables(replaced)
	return renamed, true, err
}

func (translator *ToEtcExpr) TranslateExpression(expression Expr) (EtcExpr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot translate a nil expression")
	}
	translated, err := translator.transform(expression)
	if err != nil {
		return nil, err
	}
	if typ, typed := expression.Tag().Type(); typed {
		identity := mustOperator([]Type{typ}, typ)
		return NewEtcApp(BlameRef{SourceID: expression.ID()}, []Type{identity}, translated), nil
	}
	return translated, nil
}

func (translator *ToEtcExpr) transform(expression Expr) (EtcExpr, error) {
	ref := ExactRef{SourceID: expression.ID()}
	switch typed := expression.(type) {
	case *nullExpr:
		return NewEtcConst(ref, StrT1), nil
	case *NameExpr:
		return NewEtcName(ref, typed.Name()), nil
	case *ValueExpr:
		typ, err := typeOfTLAValue(typed.Value())
		if err != nil {
			return nil, err
		}
		return NewEtcConst(ref, typ), nil
	case *LetInExpr:
		result, err := translator.TranslateExpression(typed.Body())
		if err != nil {
			return nil, err
		}
		declarations := typed.Declarations()
		for i := len(declarations) - 1; i >= 0; i-- {
			result, err = translator.TranslateDeclaration(declarations[i], result)
			if err != nil {
				return nil, err
			}
		}
		a := translator.pool.Fresh()
		return NewEtcApp(ref, []Type{mustOperator([]Type{a}, a)}, result), nil
	case *OperExpr:
		return translator.translateOperator(typed)
	default:
		return nil, fmt.Errorf("unsupported expression %T", expression)
	}
}

func (translator *ToEtcExpr) translateOperator(expression *OperExpr) (EtcExpr, error) {
	name := expression.Operator().Name()
	args := expression.Arguments()
	ref := ExactRef{SourceID: expression.ID()}
	app := func(signature Type, selected ...Expr) (EtcExpr, error) {
		translated, err := translator.translateExpressions(selected)
		if err != nil {
			return nil, err
		}
		return NewEtcApp(ref, []Type{signature}, translated...), nil
	}
	appAll := func(signature Type) (EtcExpr, error) { return app(signature, args...) }
	fresh := translator.pool.Fresh

	switch name {
	case "EQ", "NE":
		a := fresh()
		result, err := appAll(mustOperator([]Type{a, a}, BoolT1))
		if err == nil {
			result.SetTypeErrorExplanation(func(_, actual []Type) (string, bool) {
				operation := "="
				if name == "NE" {
					operation = "/="
				}
				return fmt.Sprintf("Arguments to %s should have the same type. For arguments %s with types %s, in expression %s",
					operation, formatExpressionList(args), joinTypeList(actual, ", "), formatExpression(expression)), true
			})
		}
		return result, err
	case "OPER_APP":
		callee, ok := args[0].(*NameExpr)
		if !ok {
			return nil, fmt.Errorf("expected operator name in application, found %T", args[0])
		}
		translated, err := translator.translateExpressions(args[1:])
		if err != nil {
			return nil, err
		}
		result := NewEtcAppByName(ref, NewEtcName(ExactRef{SourceID: callee.ID()}, callee.Name()), translated...)
		result.SetTypeErrorExplanation(func(expected, actual []Type) (string, bool) {
			if len(expected) != 1 {
				return "", false
			}
			operator, ok := expected[0].(OperatorType)
			if !ok {
				return "", false
			}
			expectedArguments := operator.Arguments()
			differences := make([]string, 0)
			for i := 0; i < len(args)-1 && i < len(expectedArguments) && i < len(actual); i++ {
				if !EqualTypes(expectedArguments[i], actual[i]) {
					differences = append(differences, fmt.Sprintf("Argument %s should have type %s but has type %s.",
						formatExpression(args[i+1]), expectedArguments[i], actual[i]))
				}
			}
			return fmt.Sprintf("The operator %s of type %s is applied to arguments of incompatible types in %s:\n%s",
				formatExpression(callee), operator, formatExpression(expression), strings.Join(differences, "\n")), true
		})
		return result, nil
	case "CHOOSE3":
		a := fresh()
		return translator.translateBoundedBinder(expression, mustOperator([]Type{mustOperator([]Type{a}, BoolT1)}, a))
	case "CHOOSE2":
		if _, ok := args[0].(*NameExpr); !ok {
			return nil, fmt.Errorf("unexpected unbounded CHOOSE binding %T", args[0])
		}
		a, b := fresh(), fresh()
		lambda, err := translator.unboundedLambda(expression.ID(), args[0], args[1], b)
		if err != nil {
			return nil, err
		}
		return NewEtcApp(ref, []Type{mustOperator([]Type{mustOperator([]Type{a}, BoolT1)}, a)}, lambda), nil
	case "AND", "OR", "IMPLIES", "EQUIV":
		types := make([]Type, len(args))
		for i := range types {
			types[i] = BoolT1
		}
		return appAll(mustOperator(types, BoolT1))
	case "NOT", "ENABLED", "GLOBALLY", "EVENTUALLY", "Apalache!Skolem", "Apalache!ConstCardinality":
		return appAll(mustOperator([]Type{BoolT1}, BoolT1))
	case "EXISTS3", "FORALL3":
		bindings, err := translator.translateBindings([]bindingPair{{args[0], args[1]}})
		if err != nil {
			return nil, err
		}
		variables := make([]Type, len(bindings))
		for i := range variables {
			variables[i] = fresh()
		}
		lambdaBody, err := translator.TranslateExpression(args[2])
		if err != nil {
			return nil, err
		}
		lambda := NewEtcAbs(BlameRef{SourceID: expression.ID()}, lambdaBody, bindings...)
		return NewEtcApp(ref, []Type{mustOperator([]Type{mustOperator(variables, BoolT1)}, BoolT1)}, lambda), nil
	case "EXISTS2", "FORALL2", "TEMPORAL_EXISTS", "TEMPORAL_FORALL":
		a, b := fresh(), fresh()
		lambda, err := translator.unboundedLambda(expression.ID(), args[0], args[1], b)
		if err != nil {
			return nil, err
		}
		return NewEtcApp(ref, []Type{mustOperator([]Type{mustOperator([]Type{a}, BoolT1)}, BoolT1)}, lambda), nil
	case "SET_ENUM":
		a := fresh()
		if len(args) == 0 {
			return NewEtcConst(ref, mustSet(a)), nil
		}
		types := make([]Type, len(args))
		for i := range types {
			types[i] = a
		}
		return appAll(mustOperator(types, mustSet(a)))
	case "FUN_SET":
		a, b := fresh(), fresh()
		return appAll(mustOperator([]Type{mustSet(a), mustSet(b)}, mustSet(mustFun(a, b))))
	case "RECORD_SET":
		return translator.translateRecord(expression, true)
	case "Sequences!Seq":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(a)}, mustSet(mustSeq(a))))
	case "SequencesExt!ToSet":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a)}, mustSet(a)))
	case "SET_IN", "SET_NOT_IN":
		a := fresh()
		return appAll(mustOperator([]Type{a, mustSet(a)}, BoolT1))
	case "SET_SUBSET_EQ":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(a), mustSet(a)}, BoolT1))
	case "SET_POWERSET":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(a)}, mustSet(mustSet(a))))
	case "SET_UNARY_UNION":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(mustSet(a))}, mustSet(a)))
	case "SET_UNION2", "SET_INTERSECT", "SET_MINUS":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(a), mustSet(a)}, mustSet(a)))
	case "SET_TIMES":
		variables := make([]Type, len(args))
		inputs := make([]Type, len(args))
		for i := range args {
			variables[i] = fresh()
			inputs[i] = mustSet(variables[i])
		}
		return appAll(mustOperator(inputs, mustSet(mustTuple(variables...))))
	case "SET_FILTER":
		return translator.translateSetFilter(expression)
	case "SET_MAP":
		return translator.translateSetMap(expression)
	case "RECORD":
		return translator.translateRecord(expression, false)
	case "TUPLE":
		if len(args) == 0 {
			return NewEtcConst(ref, mustSeq(fresh())), nil
		}
		variables := make([]Type, len(args))
		for i := range variables {
			variables[i] = fresh()
		}
		translated, err := translator.translateExpressions(args)
		if err != nil {
			return nil, err
		}
		tupleSig := mustOperator(variables, mustTuple(variables...))
		same := fresh()
		seqInputs := make([]Type, len(args))
		for i := range seqInputs {
			seqInputs[i] = same
		}
		return NewEtcApp(ref, []Type{tupleSig, mustOperator(seqInputs, mustSeq(same))}, translated...), nil
	case "FUN_APP":
		return translator.translateFunctionApplication(expression)
	case "DOMAIN":
		return translator.translateDomain(expression)
	case "FUN_CTOR":
		return translator.translateFunctionConstructor(expression, false)
	case "FUN_REC_CTOR":
		return translator.translateFunctionConstructor(expression, true)
	case "FUN_REC_REF":
		return NewEtcName(ref, "$RecursiveFunction"), nil
	case "EXCEPT":
		return translator.translateExcept(expression)
	case "IF_THEN_ELSE":
		a := fresh()
		return appAll(mustOperator([]Type{BoolT1, a, a}, a))
	case "CASE", "CASE_OTHER":
		a := fresh()
		inputs := make([]Type, len(args))
		start := 0
		if name == "CASE_OTHER" {
			inputs[0] = a
			start = 1
		}
		for i := start; i < len(args); i++ {
			if (i-start)%2 == 0 {
				inputs[i] = BoolT1
			} else {
				inputs[i] = a
			}
		}
		return appAll(mustOperator(inputs, a))
	case "PRIME":
		a := fresh()
		return appAll(mustOperator([]Type{a}, a))
	case "STUTTER", "NO_STUTTER":
		a := fresh()
		return appAll(mustOperator([]Type{BoolT1, a}, BoolT1))
	case "UNCHANGED":
		if len(args) == 1 {
			if tuple, ok := args[0].(*OperExpr); ok && tuple.Operator().Name() == "TUPLE" {
				elements := tuple.Arguments()
				elementTypes := make([]Type, len(elements))
				for i := range elementTypes {
					elementTypes[i] = fresh()
				}
				translated, err := translator.translateExpressions(elements)
				if err != nil {
					return nil, err
				}
				tupleType := mustTuple(elementTypes...)
				tupleExpression := NewEtcApp(ExactRef{SourceID: tuple.ID()},
					[]Type{mustOperator(elementTypes, tupleType)}, translated...)
				return NewEtcApp(ref, []Type{mustOperator([]Type{tupleType}, BoolT1)}, tupleExpression), nil
			}
		}
		inputs := make([]Type, len(args))
		for i := range inputs {
			inputs[i] = fresh()
		}
		return appAll(mustOperator(inputs, BoolT1))
	case "COMPOSE", "LEADS_TO", "GUARANTEES":
		return appAll(mustOperator([]Type{BoolT1, BoolT1}, BoolT1))
	case "WEAK_FAIRNESS", "STRONG_FAIRNESS":
		a := fresh()
		return appAll(mustOperator([]Type{a, BoolT1}, BoolT1))
	case "Sequences!Head":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a)}, a))
	case "Sequences!Tail":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a)}, mustSeq(a)))
	case "Sequences!Append":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a), a}, mustSeq(a)))
	case "Sequences!Concat":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a), mustSeq(a)}, mustSeq(a)))
	case "Sequences!Len":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a)}, IntT1))
	case "Sequences!SubSeq":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a), IntT1, IntT1}, mustSeq(a)))
	case "UNARY_MINUS":
		return appAll(mustOperator([]Type{IntT1}, IntT1))
	case "PLUS", "MINUS", "MULT", "DIV", "MOD", "POW":
		return appAll(mustOperator([]Type{IntT1, IntT1}, IntT1))
	case "LT", "LE", "GT", "GE":
		return appAll(mustOperator([]Type{IntT1, IntT1}, BoolT1))
	case "INT_RANGE":
		return appAll(mustOperator([]Type{IntT1, IntT1}, mustSet(IntT1)))
	case "REAL_DIV":
		return appAll(mustOperator([]Type{RealT1, RealT1}, RealT1))
	case "FiniteSets!IsFiniteSet":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(a)}, BoolT1))
	case "FiniteSets!Cardinality":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(a)}, IntT1))
	case "Variants!Variant":
		return translator.translateVariant(expression, "construct")
	case "Variants!VariantFilter":
		return translator.translateVariant(expression, "filter")
	case "Variants!VariantTag":
		tail := fresh()
		row, _ := NewRowType(nil, &tail)
		return appAll(mustOperator([]Type{NewVariantType(row)}, StrT1))
	case "Variants!VariantGetUnsafe":
		return translator.translateVariant(expression, "unsafe")
	case "Variants!VariantGetOrElse":
		return translator.translateVariant(expression, "default")
	case "LABEL":
		a := fresh()
		inputs := make([]Type, len(args))
		for i := range inputs {
			if i == 0 {
				inputs[i] = a
			} else {
				inputs[i] = StrT1
			}
		}
		// Scala orders label metadata before the labelled expression, but both
		// encodings impose the same type constraints.
		return appAll(mustOperator(inputs, a))
	case "Apalache!<:":
		return nil, NewOutdatedAnnotationsError(
			"Old Apalache type annotations (predating 0.12.0) are no longer supported", expression)
	case "Apalache!:=":
		a := fresh()
		return appAll(mustOperator([]Type{a, a}, BoolT1))
	case "Apalache!Guess":
		a := fresh()
		return appAll(mustOperator([]Type{mustSet(a)}, a))
	case "Apalache!Gen":
		a := fresh()
		return appAll(mustOperator([]Type{IntT1}, a))
	case "Apalache!Expand":
		a := fresh()
		return appAll(mustOperator([]Type{a}, a))
	case "Apalache!MkSeq":
		a := fresh()
		return appAll(mustOperator([]Type{IntT1, mustOperator([]Type{IntT1}, a)}, mustSeq(a)))
	case "Apalache!SetAsFun":
		a, b := fresh(), fresh()
		return appAll(mustOperator([]Type{mustSet(mustTuple(a, b))}, mustFun(a, b)))
	case "Apalache!Repeat":
		a := fresh()
		return appAll(mustOperator([]Type{mustOperator([]Type{a, IntT1}, a), IntT1, a}, a))
	case "Apalache!ApaFoldSet":
		a, b := fresh(), fresh()
		return appAll(mustOperator([]Type{mustOperator([]Type{a, b}, a), a, mustSet(b)}, a))
	case "Apalache!ApaFoldSeqLeft":
		a, b := fresh(), fresh()
		return appAll(mustOperator([]Type{mustOperator([]Type{a, b}, a), a, mustSeq(b)}, a))
	case "Apalache!Distinct":
		a := fresh()
		inputs := make([]Type, len(args))
		for i := range inputs {
			inputs[i] = a
		}
		return appAll(mustOperator(inputs, BoolT1))
	case "ApalacheInternal!__ApalacheSeqCapacity":
		a := fresh()
		return appAll(mustOperator([]Type{mustSeq(a)}, IntT1))
	case "ApalacheInternal!__NotSupportedByModelChecker":
		a := fresh()
		return appAll(mustOperator([]Type{StrT1}, a))
	case "TLC!TLCGet":
		key, value := fresh(), fresh()
		return appAll(mustOperator([]Type{key}, value))
	case "TLC!ToString":
		a := fresh()
		return appAll(mustOperator([]Type{a}, StrT1))
	default:
		return nil, fmt.Errorf("unsupported operator %s", name)
	}
}

func (translator *ToEtcExpr) translateVariant(expression *OperExpr, operation string) (EtcExpr, error) {
	args := expression.Arguments()
	if len(args) == 0 {
		return nil, fmt.Errorf("variant operation requires arguments")
	}
	tagExpr, ok := args[0].(*ValueExpr)
	if !ok {
		return nil, NewTypingInputException(
			fmt.Sprintf("The first argument of a variant operation must be a string, found: %s", formatExpression(args[0])),
			expression.ID())
	}
	tag, ok := tagExpr.Value().(StringValue)
	if !ok {
		return nil, NewTypingInputException(
			fmt.Sprintf("The first argument of a variant operation must be a string, found: %s", formatExpression(args[0])),
			expression.ID())
	}
	valueType, tail := translator.pool.Fresh(), translator.pool.Fresh()
	row, _ := NewRowType(map[string]Type{tag.Value(): valueType}, &tail)
	variant := NewVariantType(row)
	var signature Type
	switch operation {
	case "construct":
		signature = mustOperator([]Type{StrT1, valueType}, variant)
	case "filter":
		signature = mustOperator([]Type{StrT1, mustSet(variant)}, mustSet(valueType))
	case "unsafe":
		signature = mustOperator([]Type{StrT1, variant}, valueType)
	case "default":
		signature = mustOperator([]Type{StrT1, variant, valueType}, valueType)
	default:
		return nil, fmt.Errorf("unknown variant translation %q", operation)
	}
	translated, err := translator.translateExpressions(args)
	if err != nil {
		return nil, err
	}
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{signature}, translated...), nil
}

type bindingPair struct{ target, set Expr }

func (translator *ToEtcExpr) translateBoundedBinder(expression *OperExpr, principal Type) (EtcExpr, error) {
	args := expression.Arguments()
	bindings, err := translator.translateBindings([]bindingPair{{args[0], args[1]}})
	if err != nil {
		return nil, err
	}
	body, err := translator.TranslateExpression(args[2])
	if err != nil {
		return nil, err
	}
	lambda := NewEtcAbs(BlameRef{SourceID: expression.ID()}, body, bindings...)
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{principal}, lambda), nil
}
func (translator *ToEtcExpr) unboundedLambda(id UID, target, body Expr, element Type) (EtcExpr, error) {
	name, ok := target.(*NameExpr)
	if !ok {
		return nil, NewTypingInputException(fmt.Sprintf("Unexpected binding %T", target), target.ID())
	}
	translated, err := translator.TranslateExpression(body)
	if err != nil {
		return nil, err
	}
	return NewEtcAbs(BlameRef{SourceID: id}, translated, EtcBinding{Name: NewEtcName(ExactRef{SourceID: name.ID()}, name.Name()), Domain: NewEtcConst(BlameRef{SourceID: id}, mustSet(element))}), nil
}

func (translator *ToEtcExpr) translateBindings(pairs []bindingPair) ([]EtcBinding, error) {
	result := make([]EtcBinding, 0)
	for _, pair := range pairs {
		set, err := translator.TranslateExpression(pair.set)
		if err != nil {
			return nil, err
		}
		bindings, err := translator.unpackBinding(pair.set.ID(), pair.target, set)
		if err != nil {
			return nil, err
		}
		result = append(result, bindings...)
	}
	return result, nil
}
func (translator *ToEtcExpr) unpackBinding(id UID, target Expr, set EtcExpr) ([]EtcBinding, error) {
	switch typed := target.(type) {
	case *NameExpr:
		return []EtcBinding{{Name: NewEtcName(ExactRef{SourceID: typed.ID()}, typed.Name()), Domain: set}}, nil
	case *OperExpr:
		if typed.Operator().Name() != "TUPLE" {
			return nil, NewTypingInputException(
				fmt.Sprintf("Unexpected binding %s", typed.Operator().Name()), typed.ID())
		}
		result := make([]EtcBinding, 0)
		elements := typed.Arguments()
		for i, element := range elements {
			variables := translator.pool.FreshN(len(elements))
			tuple := mustTuple(varTypes(variables)...)
			projection := NewEtcApp(BlameRef{SourceID: id}, []Type{mustOperator([]Type{tuple, mustSet(tuple)}, mustSet(variables[i]))}, NewEtcConst(ExactRef{SourceID: typed.ID()}, tuple), set)
			bindings, err := translator.unpackBinding(id, element, projection)
			if err != nil {
				return nil, err
			}
			result = append(result, bindings...)
		}
		return result, nil
	default:
		return nil, NewTypingInputException(fmt.Sprintf("Unexpected binding %T", target), target.ID())
	}
}

func (translator *ToEtcExpr) translateSetFilter(expression *OperExpr) (EtcExpr, error) {
	args := expression.Arguments()
	bindings, err := translator.translateBindings([]bindingPair{{args[0], args[1]}})
	if err != nil {
		return nil, err
	}
	variables := make([]Type, len(bindings))
	for i := range variables {
		variables[i] = translator.pool.Fresh()
	}
	resultType := Type(variables[0])
	if len(variables) > 1 {
		resultType = mustTuple(variables...)
	}
	body, err := translator.TranslateExpression(args[2])
	if err != nil {
		return nil, err
	}
	lambda := NewEtcAbs(BlameRef{SourceID: expression.ID()}, body, bindings...)
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{mustOperator([]Type{mustOperator(variables, BoolT1)}, mustSet(resultType))}, lambda), nil
}
func (translator *ToEtcExpr) translateSetMap(expression *OperExpr) (EtcExpr, error) {
	args := expression.Arguments()
	pairs, err := deinterleaveBindings(args[1:])
	if err != nil {
		return nil, err
	}
	bindings, err := translator.translateBindings(pairs)
	if err != nil {
		return nil, err
	}
	result := translator.pool.Fresh()
	parameters := make([]Type, len(bindings))
	for i := range parameters {
		parameters[i] = translator.pool.Fresh()
	}
	body, err := translator.TranslateExpression(args[0])
	if err != nil {
		return nil, err
	}
	lambda := NewEtcAbs(BlameRef{SourceID: args[0].ID()}, body, bindings...)
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{mustOperator([]Type{mustOperator(parameters, result)}, mustSet(result))}, lambda), nil
}

func (translator *ToEtcExpr) translateRecord(expression *OperExpr, set bool) (EtcExpr, error) {
	args := expression.Arguments()
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("record requires key/value pairs")
	}
	fields := make([]string, len(args)/2)
	values := make([]Expr, len(fields))
	types := make([]Type, len(fields))
	for i := range fields {
		key, ok := args[2*i].(*ValueExpr)
		if !ok {
			return nil, fmt.Errorf("record key is %T", args[2*i])
		}
		text, ok := key.Value().(StringValue)
		if !ok {
			return nil, fmt.Errorf("record key is not a string")
		}
		fields[i] = text.Value()
		values[i] = args[2*i+1]
		types[i] = translator.pool.Fresh()
	}
	fieldTypes := make(map[string]Type, len(fields))
	for i, name := range fields {
		fieldTypes[name] = types[i]
	}
	var record Type
	if translator.useRows {
		row, _ := NewRowType(fieldTypes, nil)
		record = NewRecordRowType(row)
	} else {
		record, _ = NewRecordType(fieldTypes)
	}
	inputs := types
	if set {
		for i := range inputs {
			inputs[i] = mustSet(inputs[i])
		}
		record = mustSet(record)
	}
	translated, err := translator.translateExpressions(values)
	if err != nil {
		return nil, err
	}
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{mustOperator(inputs, record)}, translated...), nil
}

func (translator *ToEtcExpr) translateFunctionApplication(expression *OperExpr) (EtcExpr, error) {
	args := expression.Arguments()
	options, err := translator.functionLikeOptions(args[1])
	if err != nil {
		return nil, err
	}
	signatures := make([]Type, len(options))
	for i, option := range options {
		signatures[i] = mustOperator([]Type{option.function, option.argument}, option.result)
	}
	translated, err := translator.translateExpressions(args)
	if err != nil {
		return nil, err
	}
	result := NewEtcApp(ExactRef{SourceID: expression.ID()}, signatures, translated...)
	result.SetTypeErrorExplanation(func(_, _ []Type) (string, bool) {
		return "Cannot apply a function-like value to the supplied argument.", true
	})
	return result, nil
}

type functionLike struct{ function, argument, result Type }

func (translator *ToEtcExpr) functionLikeOptions(argument Expr) ([]functionLike, error) {
	a := translator.pool.Fresh()
	if value, ok := argument.(*ValueExpr); ok {
		switch typed := value.Value().(type) {
		case IntValue:
			field := int(typed.BigInt().Int64())
			sparse, _ := NewSparseTupleType(map[int]Type{field: a})
			return []functionLike{{mustFun(IntT1, a), IntT1, a}, {mustSeq(a), IntT1, a}, {sparse, IntT1, a}}, nil
		case StringValue:
			text := typed.Value()
			if model, _, ok := ParseModelValue(text); ok {
				return []functionLike{{mustFun(model, a), model, a}}, nil
			}
			// Field selection is an open-row constraint even when record
			// constructors use legacy closed records. Cross-family unification
			// preserves the concrete representation of the selected value.
			tail := translator.pool.Fresh()
			row, _ := NewRowType(map[string]Type{text: a}, &tail)
			record := NewRecordRowType(row)
			return []functionLike{{mustFun(StrT1, a), StrT1, a}, {record, StrT1, a}}, nil
		}
	}
	b := translator.pool.Fresh()
	return []functionLike{{mustFun(a, b), a, b}, {mustSeq(a), IntT1, a}}, nil
}
func (translator *ToEtcExpr) translateDomain(expression *OperExpr) (EtcExpr, error) {
	a, b, c, d := translator.pool.Fresh(), translator.pool.Fresh(), translator.pool.Fresh(), translator.pool.Fresh()
	var record Type
	if translator.useRows {
		row, _ := NewRowType(nil, &d)
		record = NewRecordRowType(row)
	} else {
		record, _ = NewRecordType(nil)
	}
	sparse, _ := NewSparseTupleType(nil)
	sigs := []Type{mustOperator([]Type{mustFun(a, b)}, mustSet(a)), mustOperator([]Type{mustSeq(c)}, mustSet(IntT1)), mustOperator([]Type{record}, mustSet(StrT1)), mustOperator([]Type{sparse}, mustSet(IntT1))}
	translated, err := translator.translateExpressions(expression.Arguments())
	if err != nil {
		return nil, err
	}
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, sigs, translated...), nil
}

func (translator *ToEtcExpr) translateFunctionConstructor(expression *OperExpr, recursive bool) (EtcExpr, error) {
	args := expression.Arguments()
	pairs, err := deinterleaveBindings(args[1:])
	if err != nil {
		return nil, err
	}
	bindings, err := translator.translateBindings(pairs)
	if err != nil {
		return nil, err
	}
	parameterTypes := make([]Type, len(bindings))
	for i := range parameterTypes {
		parameterTypes[i] = translator.pool.Fresh()
	}
	result := translator.pool.Fresh()
	body, err := translator.TranslateExpression(args[0])
	if err != nil {
		return nil, err
	}
	inner := NewEtcAbs(BlameRef{SourceID: args[0].ID()}, body, bindings...)
	domain := functionDomainType(pairs, parameterTypes)
	function := mustFun(domain, result)
	principal := mustOperator([]Type{mustOperator(parameterTypes, result)}, function)
	if !recursive {
		return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{principal}, inner), nil
	}
	refName := NewEtcName(BlameRef{SourceID: expression.ID()}, "$RecursiveFunction")
	outer := NewEtcAbs(BlameRef{SourceID: expression.ID()}, inner, EtcBinding{Name: refName, Domain: NewEtcConst(BlameRef{SourceID: expression.ID()}, mustSet(function))})
	recPrincipal := mustOperator([]Type{mustOperator([]Type{function}, mustOperator(parameterTypes, result))}, function)
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{recPrincipal}, outer), nil
}
func functionDomainType(pairs []bindingPair, leaf []Type) Type {
	if len(pairs) == 1 {
		return binderShape(pairs[0].target, &leaf)
	}
	types := make([]Type, len(pairs))
	for i, pair := range pairs {
		types[i] = binderShape(pair.target, &leaf)
	}
	return mustTuple(types...)
}
func binderShape(target Expr, leaf *[]Type) Type {
	if _, ok := target.(*NameExpr); ok {
		result := (*leaf)[0]
		*leaf = (*leaf)[1:]
		return result
	}
	operation := target.(*OperExpr)
	elements := operation.Arguments()
	types := make([]Type, len(elements))
	for i, element := range elements {
		types[i] = binderShape(element, leaf)
	}
	return mustTuple(types...)
}

func (translator *ToEtcExpr) translateExcept(expression *OperExpr) (EtcExpr, error) {
	args := expression.Arguments()
	updates := make([]EtcExpr, 0, (len(args)-1)/2)
	for i := 1; i < len(args); i += 2 {
		path, ok := args[i].(*OperExpr)
		if !ok || path.Operator().Name() != "TUPLE" {
			return nil, fmt.Errorf("EXCEPT accessor must be tuple")
		}
		result, err := translator.TranslateExpression(args[i+1])
		if err != nil {
			return nil, err
		}
		indices := path.Arguments()
		for j := len(indices) - 1; j >= 0; j-- {
			options, _ := translator.functionLikeOptions(indices[j])
			signatures := make([]Type, len(options))
			for k, option := range options {
				signatures[k] = mustOperator([]Type{option.argument, option.result}, option.function)
			}
			index, err := translator.TranslateExpression(indices[j])
			if err != nil {
				return nil, err
			}
			result = NewEtcApp(BlameRef{SourceID: indices[j].ID()}, signatures, index, result)
		}
		updates = append(updates, result)
	}
	all := make([]EtcExpr, 0, len(updates)+1)
	base, err := translator.TranslateExpression(args[0])
	if err != nil {
		return nil, err
	}
	all = append(all, base)
	all = append(all, updates...)
	a := translator.pool.Fresh()
	inputs := make([]Type, len(all))
	for i := range inputs {
		inputs[i] = a
	}
	return NewEtcApp(ExactRef{SourceID: expression.ID()}, []Type{mustOperator(inputs, a)}, all...), nil
}

func (translator *ToEtcExpr) translateExpressions(expressions []Expr) ([]EtcExpr, error) {
	result := make([]EtcExpr, len(expressions))
	for i, expression := range expressions {
		translated, err := translator.TranslateExpression(expression)
		if err != nil {
			return nil, err
		}
		result[i] = translated
	}
	return result, nil
}
func deinterleaveBindings(expressions []Expr) ([]bindingPair, error) {
	if len(expressions)%2 != 0 {
		return nil, fmt.Errorf("expected binding pairs")
	}
	result := make([]bindingPair, len(expressions)/2)
	for i := range result {
		result[i] = bindingPair{expressions[2*i], expressions[2*i+1]}
	}
	return result, nil
}

func formatExpression(expression Expr) string {
	formatted, err := FormatTLAExpression(expression)
	if err != nil {
		return fmt.Sprintf("<%T>", expression)
	}
	return formatted
}

func formatExpressionList(expressions []Expr) string {
	formatted := make([]string, len(expressions))
	for i, expression := range expressions {
		formatted[i] = formatExpression(expression)
	}
	return strings.Join(formatted, ", ")
}

func typeOfTLAValue(value Value) (Type, error) {
	switch typed := value.(type) {
	case IntValue:
		return IntT1, nil
	case DecimalValue, RealValue:
		return RealT1, nil
	case BoolValue:
		return BoolT1, nil
	case StringValue:
		return ModelValueOrStringType(typed.Value()), nil
	case PredefinedSet:
		switch typed {
		case BoolSet:
			return mustSet(BoolT1), nil
		case StringSet:
			return mustSet(StrT1), nil
		case IntSet, NatSet:
			return mustSet(IntT1), nil
		case RealSet:
			return mustSet(RealT1), nil
		}
	}
	return nil, fmt.Errorf("unsupported literal %T", value)
}
func (translator *ToEtcExpr) renameVariables(typ Type) (Type, error) {
	entries := make([]SubstitutionEntry, len(typ.UsedNames()))
	for i, variable := range typ.UsedNames() {
		entries[i] = SubstitutionEntry{Class: NewEqClass(variable), Type: translator.pool.Fresh()}
	}
	substitution, err := NewSubstitution(entries...)
	if err != nil {
		return nil, err
	}
	return substitution.ApplyRecursive(typ)
}
func mustSet(element Type) Type {
	result, err := NewSetType(element)
	if err != nil {
		panic(err)
	}
	return result
}
func mustSeq(element Type) Type {
	result, err := NewSeqType(element)
	if err != nil {
		panic(err)
	}
	return result
}
func mustFun(argument, result Type) Type {
	value, err := NewFunType(argument, result)
	if err != nil {
		panic(err)
	}
	return value
}
func mustTuple(elements ...Type) Type {
	result, err := NewTupleType(elements...)
	if err != nil {
		panic(err)
	}
	return result
}
func mustOperator(arguments []Type, result Type) Type {
	value, err := NewOperatorType(arguments, result)
	if err != nil {
		panic(err)
	}
	return value
}
func varTypes(variables []VarType) []Type {
	result := make([]Type, len(variables))
	for i := range variables {
		result[i] = variables[i]
	}
	return result
}
