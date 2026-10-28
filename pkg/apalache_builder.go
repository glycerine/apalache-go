package apalache

import (
	"fmt"
	"math/big"
)

func (builder *TypedBuilder) Assign(left, right Expr) (*OperExpr, error) {
	if builder.strict && !isPrimedNameExpression(left) {
		return nil, fmt.Errorf("assignment left-hand side is not a primed variable name")
	}
	return builder.Apply(Op("Apalache!:="), left, right)
}

func isPrimedNameExpression(expression Expr) bool {
	operation, ok := expression.(*OperExpr)
	return ok && operation.Operator().Name() == "PRIME" && len(operation.Arguments()) == 1 && isNameExpr(operation.Arguments()[0])
}

func isNameExpr(expression Expr) bool {
	_, ok := expression.(*NameExpr)
	return ok
}

func (builder *TypedBuilder) Gen(bound Expr, returnType Type) (*OperExpr, error) {
	if _, err := ExpressionType(bound); err != nil {
		return nil, err
	}
	if returnType == nil {
		return nil, fmt.Errorf("Gen requires a return type")
	}
	return NewOperExpr(Op("Apalache!Gen"), Typed(returnType), bound)
}

func (builder *TypedBuilder) Repeat(function Expr, count *big.Int, value Expr) (*OperExpr, error) {
	if count == nil {
		return nil, fmt.Errorf("Repeat requires a count")
	}
	if builder.strict && count.Sign() <= 0 {
		return nil, fmt.Errorf("Repeat count must be positive")
	}
	if builder.strict && !isNaryPassByName(function, 2) {
		return nil, fmt.Errorf("Repeat requires a binary operator passed by name")
	}
	return builder.Apply(Op("Apalache!Repeat"), function, builder.Int(count), value)
}

func (builder *TypedBuilder) Skolem(expression Expr) (*OperExpr, error) {
	if builder.strict && !isOperatorExpression(expression, "EXISTS3") {
		return nil, fmt.Errorf("Skolem requires a bounded existential")
	}
	return builder.Apply(Op("Apalache!Skolem"), expression)
}

func (builder *TypedBuilder) Guess(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!Guess"), set)
}

func (builder *TypedBuilder) Expand(expression Expr) (*OperExpr, error) {
	if builder.strict && !isOperatorExpression(expression, "SET_POWERSET", "FUN_SET") {
		return nil, fmt.Errorf("Expand requires a powerset or function set")
	}
	return builder.Apply(Op("Apalache!Expand"), expression)
}

func (builder *TypedBuilder) ConstCard(expression Expr) (*OperExpr, error) {
	if builder.strict && !isConstCardinalityExpression(expression) {
		return nil, fmt.Errorf("ConstCard requires Cardinality(S) >= integer")
	}
	return builder.Apply(Op("Apalache!ConstCardinality"), expression)
}

func isOperatorExpression(expression Expr, names ...string) bool {
	operation, ok := expression.(*OperExpr)
	if !ok {
		return false
	}
	for _, name := range names {
		if operation.Operator().Name() == name {
			return true
		}
	}
	return false
}

func isConstCardinalityExpression(expression Expr) bool {
	comparison, ok := expression.(*OperExpr)
	if !ok || comparison.Operator().Name() != "GE" || len(comparison.Arguments()) != 2 {
		return false
	}
	cardinality, ok := comparison.Arguments()[0].(*OperExpr)
	if !ok || cardinality.Operator().Name() != "FiniteSets!Cardinality" {
		return false
	}
	literal, ok := comparison.Arguments()[1].(*ValueExpr)
	if !ok {
		return false
	}
	_, ok = literal.Value().(IntValue)
	return ok
}

func isNaryPassByName(expression Expr, arity int) bool {
	let, ok := expression.(*LetInExpr)
	if !ok {
		return false
	}
	declarations := let.Declarations()
	body, ok := let.Body().(*NameExpr)
	return ok && len(declarations) == 1 && body.Name() == declarations[0].Name() && len(declarations[0].FormalParams()) == arity
}

func (builder *TypedBuilder) MkSeq(count *big.Int, function Expr) (*OperExpr, error) {
	if count == nil {
		return nil, fmt.Errorf("MkSeq requires a count")
	}
	if builder.strict && count.Sign() < 0 {
		return nil, fmt.Errorf("MkSeq count must be nonnegative")
	}
	if builder.strict && !isNaryPassByName(function, 1) {
		return nil, fmt.Errorf("MkSeq requires a unary operator passed by name")
	}
	return builder.Apply(Op("Apalache!MkSeq"), builder.Int(count), function)
}

func (builder *TypedBuilder) MkSeqConst(count, function Expr) (*OperExpr, error) {
	if builder.strict && !isNaryPassByName(function, 1) {
		return nil, fmt.Errorf("MkSeqConst requires a unary operator passed by name")
	}
	return builder.Apply(Op("Apalache!MkSeq"), count, function)
}

func (builder *TypedBuilder) FoldSet(function, initial, set Expr) (*OperExpr, error) {
	return builder.fold("Apalache!ApaFoldSet", function, initial, set)
}

func (builder *TypedBuilder) FoldSeq(function, initial, sequence Expr) (*OperExpr, error) {
	return builder.fold("Apalache!ApaFoldSeqLeft", function, initial, sequence)
}

func (builder *TypedBuilder) fold(operator string, function, initial, collection Expr) (*OperExpr, error) {
	if builder.strict && !isNaryPassByName(function, 2) {
		return nil, fmt.Errorf("%s requires a binary operator passed by name", operator)
	}
	return builder.Apply(Op(operator), function, initial, collection)
}

func (builder *TypedBuilder) SetAsFun(set Expr) (*OperExpr, error) {
	return builder.Apply(Op("Apalache!SetAsFun"), set)
}

func scopedApalacheCall(builder *ScopedBuilder, instructions []ExprInstruction, call func([]Expr) (Expr, error)) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, instructions)
		if err != nil {
			return nil, err
		}
		return call(built)
	}}
}

func (builder *ScopedBuilder) Assign(left, right ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{left, right}, func(args []Expr) (Expr, error) {
		return builder.typed.Assign(args[0], args[1])
	})
}

func (builder *ScopedBuilder) Gen(bound ExprInstruction, returnType Type) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{bound}, func(args []Expr) (Expr, error) {
		return builder.typed.Gen(args[0], returnType)
	})
}

func (builder *ScopedBuilder) Repeat(function ExprInstruction, count *big.Int, value ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{function, value}, func(args []Expr) (Expr, error) {
		return builder.typed.Repeat(args[0], count, args[1])
	})
}

func (builder *ScopedBuilder) Skolem(expression ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{expression}, func(args []Expr) (Expr, error) {
		return builder.typed.Skolem(args[0])
	})
}

func (builder *ScopedBuilder) Guess(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!Guess"), set)
}

func (builder *ScopedBuilder) Expand(expression ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{expression}, func(args []Expr) (Expr, error) {
		return builder.typed.Expand(args[0])
	})
}

func (builder *ScopedBuilder) ConstCard(expression ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{expression}, func(args []Expr) (Expr, error) {
		return builder.typed.ConstCard(args[0])
	})
}

func (builder *ScopedBuilder) MkSeq(count *big.Int, function ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{function}, func(args []Expr) (Expr, error) {
		return builder.typed.MkSeq(count, args[0])
	})
}

func (builder *ScopedBuilder) MkSeqConst(count, function ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{count, function}, func(args []Expr) (Expr, error) {
		return builder.typed.MkSeqConst(args[0], args[1])
	})
}

func (builder *ScopedBuilder) FoldSet(function, initial, set ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{function, initial, set}, func(args []Expr) (Expr, error) {
		return builder.typed.FoldSet(args[0], args[1], args[2])
	})
}

func (builder *ScopedBuilder) FoldSeq(function, initial, sequence ExprInstruction) ExprInstruction {
	return scopedApalacheCall(builder, []ExprInstruction{function, initial, sequence}, func(args []Expr) (Expr, error) {
		return builder.typed.FoldSeq(args[0], args[1], args[2])
	})
}

func (builder *ScopedBuilder) SetAsFun(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Apalache!SetAsFun"), set)
}
