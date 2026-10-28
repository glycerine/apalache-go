package apalache

// Typed action builders mirror UnsafeActionBuilder and validate each
// application through the shared operator signatures.
func (builder *TypedBuilder) Prime(expression Expr) (*OperExpr, error) {
	return builder.Apply(Op("PRIME"), expression)
}

func (builder *TypedBuilder) Stutt(action, expression Expr) (*OperExpr, error) {
	return builder.Apply(Op("STUTTER"), action, expression)
}

func (builder *TypedBuilder) NoStutt(action, expression Expr) (*OperExpr, error) {
	return builder.Apply(Op("NO_STUTTER"), action, expression)
}

func (builder *TypedBuilder) Enabled(action Expr) (*OperExpr, error) {
	return builder.Apply(Op("ENABLED"), action)
}

func (builder *TypedBuilder) Unchanged(expression Expr) (*OperExpr, error) {
	return builder.Apply(Op("UNCHANGED"), expression)
}

func (builder *TypedBuilder) Comp(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("COMPOSE"), left, right)
}

// Scoped action builders preserve the delayed free-name checks of
// ActionBuilder while delegating type inference to TypedBuilder.
func (builder *ScopedBuilder) Prime(expression ExprInstruction) ExprInstruction {
	return builder.Apply(Op("PRIME"), expression)
}

func (builder *ScopedBuilder) Stutt(action, expression ExprInstruction) ExprInstruction {
	return builder.Apply(Op("STUTTER"), action, expression)
}

func (builder *ScopedBuilder) NoStutt(action, expression ExprInstruction) ExprInstruction {
	return builder.Apply(Op("NO_STUTTER"), action, expression)
}

func (builder *ScopedBuilder) Enabled(action ExprInstruction) ExprInstruction {
	return builder.Apply(Op("ENABLED"), action)
}

func (builder *ScopedBuilder) Unchanged(expression ExprInstruction) ExprInstruction {
	return builder.Apply(Op("UNCHANGED"), expression)
}

func (builder *ScopedBuilder) Comp(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("COMPOSE"), left, right)
}
