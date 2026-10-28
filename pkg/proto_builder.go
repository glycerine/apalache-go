package apalache

import "fmt"

// TypeInference is a local operator signature computation. It supports
// extension operators without registering them in the built-in signature map.
type TypeInference func(arguments []Type) (Type, error)

func (builder *TypedBuilder) ApplyWithInference(operator Operator, inference TypeInference, arguments ...Expr) (*OperExpr, error) {
	if inference == nil {
		return nil, fmt.Errorf("operator %s requires a type inference function", operator.Name())
	}
	types, err := expressionTypes(arguments)
	if err != nil {
		return nil, err
	}
	result, err := inference(types)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("operator %s inference returned a nil type", operator.Name())
	}
	return NewOperExpr(operator, Typed(result), arguments...)
}

func (builder *ScopedBuilder) ApplyWithInference(operator Operator, inference TypeInference, arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.ApplyWithInference(operator, inference, built...)
	}}
}
