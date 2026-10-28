package apalache

import "math/big"

// Scala-name aliases keep the port directly traceable while the older
// descriptive TypedBuilder methods remain available to Go callers.
func (builder *TypedBuilder) Str(value string) (*ValueExpr, error) {
	return builder.String(value)
}

func (builder *TypedBuilder) Const(root string, typ ConstType) (*ValueExpr, error) {
	return builder.ModelValue(root, typ)
}

func (builder *TypedBuilder) ConstParsed(value string) (*ValueExpr, error) {
	return builder.ParsedModelValue(value)
}

func (builder *TypedBuilder) BooleanSet() *ValueExpr { return builder.PredefinedSet(BoolSet) }
func (builder *TypedBuilder) StringSet() *ValueExpr  { return builder.PredefinedSet(StringSet) }
func (builder *TypedBuilder) IntSet() *ValueExpr     { return builder.PredefinedSet(IntSet) }
func (builder *TypedBuilder) NatSet() *ValueExpr     { return builder.PredefinedSet(NatSet) }

func (builder *ScopedBuilder) Int(value *big.Int) ExprInstruction {
	return builder.Unchecked(builder.typed.Int(value))
}

func (builder *ScopedBuilder) Str(value string) ExprInstruction {
	return ExprInstruction{run: func(*scopeContext) (Expr, error) { return builder.typed.Str(value) }}
}

func (builder *ScopedBuilder) Const(root string, typ ConstType) ExprInstruction {
	return ExprInstruction{run: func(*scopeContext) (Expr, error) { return builder.typed.Const(root, typ) }}
}

func (builder *ScopedBuilder) ConstParsed(value string) ExprInstruction {
	return ExprInstruction{run: func(*scopeContext) (Expr, error) { return builder.typed.ConstParsed(value) }}
}

func (builder *ScopedBuilder) BooleanSet() ExprInstruction {
	return builder.Unchecked(builder.typed.BooleanSet())
}

func (builder *ScopedBuilder) StringSet() ExprInstruction {
	return builder.Unchecked(builder.typed.StringSet())
}

func (builder *ScopedBuilder) IntSet() ExprInstruction {
	return builder.Unchecked(builder.typed.IntSet())
}

func (builder *ScopedBuilder) NatSet() ExprInstruction {
	return builder.Unchecked(builder.typed.NatSet())
}
