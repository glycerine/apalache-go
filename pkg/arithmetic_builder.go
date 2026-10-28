package apalache

// Typed arithmetic builders mirror UnsafeArithmeticBuilder while returning
// errors instead of throwing when an operand does not match the signature.
func (builder *TypedBuilder) Plus(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("PLUS"), left, right)
}
func (builder *TypedBuilder) Minus(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("MINUS"), left, right)
}
func (builder *TypedBuilder) UMinus(operand Expr) (*OperExpr, error) {
	return builder.Apply(Op("UNARY_MINUS"), operand)
}
func (builder *TypedBuilder) Mult(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("MULT"), left, right)
}
func (builder *TypedBuilder) Div(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("DIV"), left, right)
}
func (builder *TypedBuilder) Mod(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("MOD"), left, right)
}
func (builder *TypedBuilder) Exp(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("POW"), left, right)
}
func (builder *TypedBuilder) DotDot(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("INT_RANGE"), left, right)
}
func (builder *TypedBuilder) Lt(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("LT"), left, right)
}
func (builder *TypedBuilder) Gt(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("GT"), left, right)
}
func (builder *TypedBuilder) Le(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("LE"), left, right)
}
func (builder *TypedBuilder) Ge(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("GE"), left, right)
}

// Scoped arithmetic builders compose delayed instructions and retain the
// free-name consistency checks of ArithmeticBuilder.
func (builder *ScopedBuilder) Plus(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("PLUS"), left, right)
}
func (builder *ScopedBuilder) Minus(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("MINUS"), left, right)
}
func (builder *ScopedBuilder) UMinus(operand ExprInstruction) ExprInstruction {
	return builder.Apply(Op("UNARY_MINUS"), operand)
}
func (builder *ScopedBuilder) Mult(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("MULT"), left, right)
}
func (builder *ScopedBuilder) Div(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("DIV"), left, right)
}
func (builder *ScopedBuilder) Mod(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("MOD"), left, right)
}
func (builder *ScopedBuilder) Exp(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("POW"), left, right)
}
func (builder *ScopedBuilder) DotDot(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("INT_RANGE"), left, right)
}
func (builder *ScopedBuilder) Lt(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("LT"), left, right)
}
func (builder *ScopedBuilder) Gt(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("GT"), left, right)
}
func (builder *ScopedBuilder) Le(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("LE"), left, right)
}
func (builder *ScopedBuilder) Ge(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("GE"), left, right)
}
