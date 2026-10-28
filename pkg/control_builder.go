package apalache

import "fmt"

// ControlCase is a condition/body branch of a CASE expression.
type ControlCase struct {
	Condition Expr
	Body      Expr
}

// ScopedControlCase is the scope-safe counterpart of ControlCase.
type ScopedControlCase struct {
	Condition ExprInstruction
	Body      ExprInstruction
}

func (builder *TypedBuilder) Ite(condition, thenBranch, elseBranch Expr) (*OperExpr, error) {
	return builder.Apply(Op("IF_THEN_ELSE"), condition, thenBranch, elseBranch)
}

func (builder *TypedBuilder) CaseSplit(pairs ...ControlCase) (*OperExpr, error) {
	arguments := make([]Expr, 0, 2*len(pairs))
	for _, pair := range pairs {
		arguments = append(arguments, pair.Condition, pair.Body)
	}
	return builder.CaseSplitMixed(arguments...)
}

func (builder *TypedBuilder) CaseSplitMixed(arguments ...Expr) (*OperExpr, error) {
	if len(arguments) < 2 || len(arguments)%2 != 0 {
		return nil, fmt.Errorf("CASE pairs must have even, positive arity: got %d arguments", len(arguments))
	}
	return builder.Apply(Op("CASE"), arguments...)
}

func (builder *TypedBuilder) CaseOther(other Expr, pairs ...ControlCase) (*OperExpr, error) {
	arguments := make([]Expr, 0, 2*len(pairs))
	for _, pair := range pairs {
		arguments = append(arguments, pair.Condition, pair.Body)
	}
	return builder.CaseOtherMixed(other, arguments...)
}

func (builder *TypedBuilder) CaseOtherMixed(other Expr, arguments ...Expr) (*OperExpr, error) {
	if len(arguments) < 2 || len(arguments)%2 != 0 {
		return nil, fmt.Errorf("CASE OTHER pairs must have even, positive arity: got %d arguments", len(arguments))
	}
	operands := make([]Expr, 0, 1+len(arguments))
	operands = append(operands, other)
	operands = append(operands, arguments...)
	return builder.Apply(Op("CASE_OTHER"), operands...)
}

func (builder *ScopedBuilder) Ite(condition, thenBranch, elseBranch ExprInstruction) ExprInstruction {
	return builder.Apply(Op("IF_THEN_ELSE"), condition, thenBranch, elseBranch)
}

func (builder *ScopedBuilder) CaseSplit(pairs ...ScopedControlCase) ExprInstruction {
	arguments := make([]ExprInstruction, 0, 2*len(pairs))
	for _, pair := range pairs {
		arguments = append(arguments, pair.Condition, pair.Body)
	}
	return builder.CaseSplitMixed(arguments...)
}

func (builder *ScopedBuilder) CaseSplitMixed(arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		if len(arguments) < 2 || len(arguments)%2 != 0 {
			return nil, fmt.Errorf("CASE pairs must have even, positive arity: got %d arguments", len(arguments))
		}
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.CaseSplitMixed(built...)
	}}
}

func (builder *ScopedBuilder) CaseOther(other ExprInstruction, pairs ...ScopedControlCase) ExprInstruction {
	arguments := make([]ExprInstruction, 0, 2*len(pairs))
	for _, pair := range pairs {
		arguments = append(arguments, pair.Condition, pair.Body)
	}
	return builder.CaseOtherMixed(other, arguments...)
}

func (builder *ScopedBuilder) CaseOtherMixed(other ExprInstruction, arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		if len(arguments) < 2 || len(arguments)%2 != 0 {
			return nil, fmt.Errorf("CASE OTHER pairs must have even, positive arity: got %d arguments", len(arguments))
		}
		otherExpr, err := other.run(context)
		if err != nil {
			return nil, err
		}
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.CaseOtherMixed(otherExpr, built...)
	}}
}
