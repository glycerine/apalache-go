package apalache

import "fmt"

func (builder *TypedBuilder) NotSupportedByModelChecker(message string, resultType Type) (*OperExpr, error) {
	if resultType == nil {
		return nil, fmt.Errorf("not-supported expression requires a result type")
	}
	messageExpr, err := builder.Str(message)
	if err != nil {
		return nil, err
	}
	return NewOperExpr(Op("ApalacheInternal!__NotSupportedByModelChecker"), Typed(resultType), messageExpr)
}

func (builder *TypedBuilder) Distinct(arguments ...Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!Distinct"), arguments...)
}

func (builder *TypedBuilder) ApalacheSeqCapacity(sequence Expr) (*OperExpr, error) {
	return builder.Apply(Op("ApalacheInternal!__ApalacheSeqCapacity"), sequence)
}

func (builder *TypedBuilder) SelectInSet(element, set Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!SelectInSet"), element, set)
}

func (builder *TypedBuilder) SelectInFun(element, function Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!SelectInFun"), element, function)
}

func (builder *TypedBuilder) StoreNotInSet(element, set Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!storeNotInSet"), element, set)
}

func (builder *TypedBuilder) StoreNotInFun(element, function Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!storeNotInFun"), element, function)
}

func (builder *TypedBuilder) StoreInSet(element, set Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!StoreInSet"), element, set)
}

// StoreInFun is Scala's ternary StoreInSet overload: value, function, argument.
func (builder *TypedBuilder) StoreInFun(value, function, argument Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!StoreInSet"), value, function, argument)
}

func (builder *TypedBuilder) SMTMap(operator Operator, left, right Expr) (*OperExpr, error) {
	return builder.Apply(NewSMTMapOperator(operator), left, right)
}

func (builder *TypedBuilder) UnconstrainArray(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!UnconstrainArray"), set)
}

func (builder *ScopedBuilder) NotSupportedByModelChecker(message string, resultType Type) ExprInstruction {
	return ExprInstruction{run: func(*scopeContext) (Expr, error) {
		return builder.typed.NotSupportedByModelChecker(message, resultType)
	}}
}

func (builder *ScopedBuilder) Distinct(arguments ...ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!Distinct"), arguments...)
}

func (builder *ScopedBuilder) ApalacheSeqCapacity(sequence ExprInstruction) ExprInstruction {
	return builder.Apply(Op("ApalacheInternal!__ApalacheSeqCapacity"), sequence)
}

func (builder *ScopedBuilder) SelectInSet(element, set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!SelectInSet"), element, set)
}

func (builder *ScopedBuilder) SelectInFun(element, function ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!SelectInFun"), element, function)
}

func (builder *ScopedBuilder) StoreNotInSet(element, set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!storeNotInSet"), element, set)
}

func (builder *ScopedBuilder) StoreNotInFun(element, function ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!storeNotInFun"), element, function)
}

func (builder *ScopedBuilder) StoreInSet(element, set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!StoreInSet"), element, set)
}

func (builder *ScopedBuilder) StoreInFun(value, function, argument ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!StoreInSet"), value, function, argument)
}

func (builder *ScopedBuilder) SMTMap(operator Operator, left, right ExprInstruction) ExprInstruction {
	return builder.Apply(NewSMTMapOperator(operator), left, right)
}

func (builder *ScopedBuilder) UnconstrainArray(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!UnconstrainArray"), set)
}
