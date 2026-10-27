package apalache

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnknownOperatorSignature = errors.New("unknown operator signature")

type OperatorTypeError struct {
	Operator Operator
	Types    []Type
	Reason   string
}

func (err *OperatorTypeError) Error() string {
	return fmt.Sprintf("cannot type %s(%s): %s", err.Operator.Name(), joinTypes(err.Types, ", "), err.Reason)
}

// InferOperatorType computes the result type for operators whose Scala
// typecomp layer defines a pure signature. Operators whose result depends on
// literal values (records, tuple/sequence overloads, function application,
// variants, and several internal forms) report ErrUnknownOperatorSignature.
func InferOperatorType(operator Operator, arguments []Type) (Type, error) {
	if !operator.IsCorrectArity(len(arguments)) {
		return nil, signatureError(operator, arguments, "unexpected arity")
	}
	all := func(expected Type) bool {
		for _, argument := range arguments {
			if !EqualTypes(argument, expected) {
				return false
			}
		}
		return true
	}
	binary := func(left, right Type) bool {
		return len(arguments) == 2 && EqualTypes(arguments[0], left) && EqualTypes(arguments[1], right)
	}

	switch operator.Name() {
	case "EQ", "NE", "Apalache!:=":
		if len(arguments) == 2 && CompatibleTypes(arguments[0], arguments[1]) {
			return BoolT1, nil
		}
	case "OPER_APP":
		if len(arguments) >= 1 {
			if operType, ok := arguments[0].(OperatorType); ok && len(operType.arguments) == len(arguments)-1 {
				for index := range operType.arguments {
					if !CompatibleTypes(operType.arguments[index], arguments[index+1]) {
						return nil, signatureError(operator, arguments, "operator argument type mismatch")
					}
				}
				return operType.result, nil
			}
		}
	case "CHOOSE3":
		if len(arguments) == 3 && EqualTypes(arguments[2], BoolT1) {
			if setType, ok := arguments[1].(SetType); ok {
				if common, compatible, err := CommonSupertype(arguments[0], setType.element); err == nil && compatible {
					return common, nil
				}
			}
		}
	case "CHOOSE2":
		if len(arguments) == 2 && EqualTypes(arguments[1], BoolT1) {
			return arguments[0], nil
		}
	case "LABEL":
		if len(arguments) >= 2 {
			return arguments[0], nil
		}
	case "AND", "OR":
		if all(BoolT1) {
			return BoolT1, nil
		}
	case "NOT":
		if len(arguments) == 1 && EqualTypes(arguments[0], BoolT1) {
			return BoolT1, nil
		}
	case "IMPLIES", "EQUIV":
		if binary(BoolT1, BoolT1) {
			return BoolT1, nil
		}
	case "FORALL3", "EXISTS3":
		if len(arguments) == 3 && EqualTypes(arguments[2], BoolT1) {
			if setType, ok := arguments[1].(SetType); ok && EqualTypes(arguments[0], setType.element) {
				return BoolT1, nil
			}
		}
	case "FORALL2", "EXISTS2":
		if len(arguments) == 2 && EqualTypes(arguments[1], BoolT1) {
			return BoolT1, nil
		}
	case "PLUS", "MINUS", "MULT", "DIV", "MOD", "POW":
		if binary(IntT1, IntT1) {
			return IntT1, nil
		}
	case "UNARY_MINUS":
		if len(arguments) == 1 && EqualTypes(arguments[0], IntT1) {
			return IntT1, nil
		}
	case "INT_RANGE":
		if binary(IntT1, IntT1) {
			return newSetTypeUnchecked(IntT1), nil
		}
	case "LT", "GT", "LE", "GE":
		if binary(IntT1, IntT1) {
			return BoolT1, nil
		}
	case "IF_THEN_ELSE":
		if len(arguments) == 3 && EqualTypes(arguments[0], BoolT1) {
			return compatibleResult(arguments[1], arguments[2], operator, arguments)
		}
	case "CASE":
		return inferCaseType(operator, arguments, false)
	case "CASE_OTHER":
		return inferCaseType(operator, arguments, true)
	case "SET_ENUM":
		if len(arguments) > 0 {
			if common, ok, err := CommonSequenceSupertype(arguments); err == nil && ok {
				return newSetTypeUnchecked(common), nil
			}
		}
	case "SET_UNION2", "SET_INTERSECT", "SET_MINUS":
		if len(arguments) == 2 {
			left, leftOK := arguments[0].(SetType)
			right, rightOK := arguments[1].(SetType)
			if leftOK && rightOK {
				common, ok, err := CommonSupertype(left.element, right.element)
				if err == nil && ok {
					return newSetTypeUnchecked(common), nil
				}
			}
		}
	case "SequencesExt!ToSet":
		if len(arguments) == 1 {
			if sequence, ok := arguments[0].(SeqType); ok {
				return newSetTypeUnchecked(sequence.element), nil
			}
		}
	case "SET_IN", "SET_NOT_IN", "SET_SUBSET_EQ":
		if len(arguments) == 2 {
			if operator.Name() == "SET_SUBSET_EQ" {
				left, leftOK := arguments[0].(SetType)
				right, rightOK := arguments[1].(SetType)
				if leftOK && rightOK && CompatibleTypes(left.element, right.element) {
					return BoolT1, nil
				}
			} else if setType, ok := arguments[1].(SetType); ok && CompatibleTypes(arguments[0], setType.element) {
				return BoolT1, nil
			}
		}
	case "SET_FILTER":
		if len(arguments) == 3 && EqualTypes(arguments[2], BoolT1) {
			if setType, ok := arguments[1].(SetType); ok {
				if common, compatible, err := CommonSupertype(arguments[0], setType.element); err == nil && compatible {
					return newSetTypeUnchecked(common), nil
				}
			}
		}
	case "SET_MAP":
		if len(arguments) >= 3 && len(arguments)%2 == 1 {
			for index := 1; index < len(arguments); index += 2 {
				setType, ok := arguments[index+1].(SetType)
				if !ok || !CompatibleTypes(arguments[index], setType.element) {
					return nil, signatureError(operator, arguments, "bound variable and set element types differ")
				}
			}
			return newSetTypeUnchecked(arguments[0]), nil
		}
	case "Sequences!Seq":
		if len(arguments) == 1 {
			if setType, ok := arguments[0].(SetType); ok {
				return newSetTypeUnchecked(newSeqTypeUnchecked(setType.element)), nil
			}
		}
	case "SET_POWERSET":
		if len(arguments) == 1 {
			if setType, ok := arguments[0].(SetType); ok {
				return newSetTypeUnchecked(setType), nil
			}
		}
	case "SET_UNARY_UNION":
		if len(arguments) == 1 {
			if outer, ok := arguments[0].(SetType); ok {
				if inner, ok := outer.element.(SetType); ok {
					return inner, nil
				}
			}
		}
	case "SET_TIMES":
		if len(arguments) >= 2 {
			elements := make([]Type, len(arguments))
			for index, argument := range arguments {
				setType, ok := argument.(SetType)
				if !ok {
					return nil, signatureError(operator, arguments, "cartesian-product argument is not a set")
				}
				elements[index] = setType.element
			}
			return newSetTypeUnchecked(newTupleTypeUnchecked(elements...)), nil
		}
	case "FUN_SET":
		if len(arguments) == 2 {
			domain, domainOK := arguments[0].(SetType)
			codomain, codomainOK := arguments[1].(SetType)
			if domainOK && codomainOK {
				return newSetTypeUnchecked(newFunTypeUnchecked(domain.element, codomain.element)), nil
			}
		}
	case "FUN_CTOR":
		return inferFunctionConstructorType(operator, arguments)
	case "Sequences!Head":
		if len(arguments) == 1 {
			if sequence, ok := arguments[0].(SeqType); ok {
				return sequence.element, nil
			}
		}
	case "Sequences!Tail":
		if len(arguments) == 1 {
			if sequence, ok := arguments[0].(SeqType); ok {
				return sequence, nil
			}
		}
	case "Sequences!Append":
		if len(arguments) == 2 {
			if sequence, ok := arguments[0].(SeqType); ok {
				if common, compatible, err := CommonSupertype(sequence.element, arguments[1]); err == nil && compatible {
					return newSeqTypeUnchecked(common), nil
				}
			}
		}
	case "Sequences!Concat":
		if len(arguments) == 2 {
			left, leftOK := arguments[0].(SeqType)
			right, rightOK := arguments[1].(SeqType)
			if leftOK && rightOK {
				if common, compatible, err := CommonSupertype(left.element, right.element); err == nil && compatible {
					return newSeqTypeUnchecked(common), nil
				}
			}
		}
	case "Sequences!Len":
		if len(arguments) == 1 {
			if _, ok := arguments[0].(SeqType); ok {
				return IntT1, nil
			}
		}
	case "Sequences!SubSeq":
		if len(arguments) == 3 && EqualTypes(arguments[1], IntT1) && EqualTypes(arguments[2], IntT1) {
			if sequence, ok := arguments[0].(SeqType); ok {
				return sequence, nil
			}
		}
	case "FiniteSets!IsFiniteSet":
		if len(arguments) == 1 {
			if _, ok := arguments[0].(SetType); ok {
				return BoolT1, nil
			}
		}
	case "FiniteSets!Cardinality":
		if len(arguments) == 1 {
			if _, ok := arguments[0].(SetType); ok {
				return IntT1, nil
			}
		}
	case "PRIME":
		if len(arguments) == 1 {
			return arguments[0], nil
		}
	case "STUTTER", "NO_STUTTER":
		if len(arguments) == 2 && EqualTypes(arguments[0], BoolT1) {
			return BoolT1, nil
		}
	case "ENABLED":
		if len(arguments) == 1 && EqualTypes(arguments[0], BoolT1) {
			return BoolT1, nil
		}
	case "UNCHANGED":
		if len(arguments) == 1 {
			return BoolT1, nil
		}
	case "COMPOSE", "LEADS_TO", "GUARANTEES":
		if binary(BoolT1, BoolT1) {
			return BoolT1, nil
		}
	case "GLOBALLY", "EVENTUALLY":
		if len(arguments) == 1 && EqualTypes(arguments[0], BoolT1) {
			return BoolT1, nil
		}
	case "WEAK_FAIRNESS", "STRONG_FAIRNESS", "TEMPORAL_EXISTS", "TEMPORAL_FORALL":
		if len(arguments) == 2 && EqualTypes(arguments[1], BoolT1) {
			return BoolT1, nil
		}
	case "Variants!VariantTag":
		if len(arguments) == 1 {
			if _, ok := arguments[0].(VariantType); ok {
				return StrT1, nil
			}
		}
	case "Apalache!Skolem", "Apalache!ConstCardinality":
		if len(arguments) == 1 && EqualTypes(arguments[0], BoolT1) {
			return BoolT1, nil
		}
	case "Apalache!Guess":
		if len(arguments) == 1 {
			if setType, ok := arguments[0].(SetType); ok {
				return setType.element, nil
			}
		}
	case "Apalache!Expand":
		if len(arguments) == 1 {
			if setType, ok := arguments[0].(SetType); ok {
				switch setType.element.(type) {
				case SetType, FunType:
					return setType, nil
				}
			}
		}
	case "Apalache!MkSeq":
		if len(arguments) == 2 && EqualTypes(arguments[0], IntT1) {
			if constructor, ok := arguments[1].(OperatorType); ok && len(constructor.arguments) == 1 && EqualTypes(constructor.arguments[0], IntT1) {
				return newSeqTypeUnchecked(constructor.result), nil
			}
		}
	case "Apalache!Repeat":
		return inferRepeatType(operator, arguments)
	case "Apalache!ApaFoldSet", "Apalache!ApaFoldSeqLeft":
		return inferFoldType(operator, arguments)
	case "Apalache!SetAsFun":
		if len(arguments) == 1 {
			if setType, ok := arguments[0].(SetType); ok {
				if tuple, ok := setType.element.(TupleType); ok && len(tuple.elements) == 2 {
					return newFunTypeUnchecked(tuple.elements[0], tuple.elements[1]), nil
				}
			}
		}
	case "Apalache!Distinct":
		if len(arguments) == 0 {
			return BoolT1, nil
		}
		if _, ok, err := CommonSequenceSupertype(arguments); err == nil && ok {
			return BoolT1, nil
		}
	case "TLC!ToString":
		if len(arguments) == 1 {
			return StrT1, nil
		}
	case "ApalacheInternal!__ApalacheSeqCapacity":
		if len(arguments) == 1 {
			if _, ok := arguments[0].(SeqType); ok {
				return IntT1, nil
			}
		}
	case "Apalache!SelectInSet", "Apalache!storeNotInSet":
		if len(arguments) == 2 {
			if setType, ok := arguments[1].(SetType); ok && CompatibleTypes(arguments[0], setType.element) {
				return BoolT1, nil
			}
		}
	case "Apalache!SelectInFun":
		if len(arguments) == 2 {
			if function, ok := arguments[1].(FunType); ok && CompatibleTypes(arguments[0], function.arg) {
				return function.result, nil
			}
		}
	case "Apalache!StoreInSet":
		return inferStoreInSetType(operator, arguments)
	case "Apalache!storeNotInFun":
		if len(arguments) == 2 {
			if function, ok := arguments[1].(FunType); ok && CompatibleTypes(arguments[0], function.arg) {
				return BoolT1, nil
			}
		}
	case "Apalache!UnconstrainArray":
		if len(arguments) == 1 {
			if _, ok := arguments[0].(SetType); ok {
				return BoolT1, nil
			}
		}
	default:
		if operator.Name() == "Apalache!SmtMap_AND" || operator.Name() == "Apalache!SmtMap_OR" {
			if len(arguments) == 2 {
				left, leftOK := arguments[0].(SetType)
				right, rightOK := arguments[1].(SetType)
				if leftOK && rightOK {
					if common, compatible, err := CommonSupertype(left.element, right.element); err == nil && compatible {
						return newSetTypeUnchecked(common), nil
					}
				}
			}
		} else if strings.HasPrefix(operator.Name(), "Apalache!SmtMap_") {
			return nil, fmt.Errorf("%w for %s", ErrUnknownOperatorSignature, operator.Name())
		} else {
			return nil, fmt.Errorf("%w for %s", ErrUnknownOperatorSignature, operator.Name())
		}
	}
	return nil, signatureError(operator, arguments, "argument types do not match the signature")
}

func signatureError(operator Operator, arguments []Type, reason string) error {
	return &OperatorTypeError{Operator: operator, Types: append([]Type(nil), arguments...), Reason: reason}
}

func compatibleResult(left, right Type, operator Operator, arguments []Type) (Type, error) {
	result, ok, err := CommonSupertype(left, right)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, signatureError(operator, arguments, "branches have incompatible types")
	}
	return result, nil
}

func inferCaseType(operator Operator, arguments []Type, hasOther bool) (Type, error) {
	bodies := make([]Type, 0)
	start := 0
	if hasOther {
		if len(arguments) == 0 {
			return nil, signatureError(operator, arguments, "missing OTHER body")
		}
		bodies = append(bodies, arguments[0])
		start = 1
	}
	if len(arguments)-start < 2 || (len(arguments)-start)%2 != 0 {
		return nil, signatureError(operator, arguments, "expected condition/body pairs")
	}
	for index := start; index < len(arguments); index += 2 {
		if !EqualTypes(arguments[index], BoolT1) {
			return nil, signatureError(operator, arguments, "CASE condition is not Bool")
		}
		bodies = append(bodies, arguments[index+1])
	}
	common, ok, err := CommonSequenceSupertype(bodies)
	if err != nil {
		return nil, err
	}
	if !ok || hasOther && !EqualTypes(common, arguments[0]) {
		return nil, signatureError(operator, arguments, "CASE bodies have incompatible types")
	}
	return common, nil
}

func inferFunctionConstructorType(operator Operator, arguments []Type) (Type, error) {
	if len(arguments) < 3 || len(arguments)%2 != 1 {
		return nil, signatureError(operator, arguments, "expected body and variable/set pairs")
	}
	domains := make([]Type, 0, (len(arguments)-1)/2)
	for index := 1; index < len(arguments); index += 2 {
		setType, ok := arguments[index+1].(SetType)
		if !ok || !CompatibleTypes(arguments[index], setType.element) {
			return nil, signatureError(operator, arguments, "bound variable and domain types differ")
		}
		domains = append(domains, arguments[index])
	}
	domain := domains[0]
	if len(domains) > 1 {
		domain = newTupleTypeUnchecked(domains...)
	}
	return newFunTypeUnchecked(domain, arguments[0]), nil
}

func inferRepeatType(operator Operator, arguments []Type) (Type, error) {
	if len(arguments) == 3 && EqualTypes(arguments[1], IntT1) {
		if operation, ok := arguments[0].(OperatorType); ok && len(operation.arguments) == 2 && EqualTypes(operation.arguments[0], IntT1) {
			common, compatible, err := CommonSequenceSupertype([]Type{operation.arguments[1], operation.result, arguments[2]})
			if err == nil && compatible {
				return common, nil
			}
		}
	}
	return nil, signatureError(operator, arguments, "argument types do not match repeat")
}

func inferFoldType(operator Operator, arguments []Type) (Type, error) {
	if len(arguments) == 3 {
		operation, ok := arguments[0].(OperatorType)
		if !ok || len(operation.arguments) != 2 {
			return nil, signatureError(operator, arguments, "fold operator has wrong type")
		}
		var element Type
		if operator.Name() == "Apalache!ApaFoldSet" {
			container, ok := arguments[2].(SetType)
			if !ok {
				return nil, signatureError(operator, arguments, "fold input is not a set")
			}
			element = container.element
		} else {
			container, ok := arguments[2].(SeqType)
			if !ok {
				return nil, signatureError(operator, arguments, "fold input is not a sequence")
			}
			element = container.element
		}
		if CompatibleTypes(operation.arguments[1], element) {
			common, compatible, err := CommonSequenceSupertype([]Type{operation.arguments[0], operation.result, arguments[1]})
			if err == nil && compatible {
				return common, nil
			}
		}
	}
	return nil, signatureError(operator, arguments, "argument types do not match fold")
}

func inferStoreInSetType(operator Operator, arguments []Type) (Type, error) {
	if len(arguments) == 2 {
		if setType, ok := arguments[1].(SetType); ok && CompatibleTypes(arguments[0], setType.element) {
			return BoolT1, nil
		}
	}
	if len(arguments) == 3 {
		if function, ok := arguments[1].(FunType); ok && CompatibleTypes(arguments[2], function.arg) && CompatibleTypes(arguments[0], function.result) {
			return BoolT1, nil
		}
	}
	return nil, signatureError(operator, arguments, "argument types do not match store-in-set")
}

func newSetTypeUnchecked(element Type) SetType {
	result, _ := NewSetType(element)
	return result
}
func newSeqTypeUnchecked(element Type) SeqType {
	result, _ := NewSeqType(element)
	return result
}
func newFunTypeUnchecked(argument, result Type) FunType {
	typ, _ := NewFunType(argument, result)
	return typ
}
func newTupleTypeUnchecked(elements ...Type) TupleType {
	result, _ := NewTupleType(elements...)
	return result
}
