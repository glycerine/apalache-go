package apalache

// Typed Boolean builders mirror UnsafeBoolBuilder through the shared
// signature inference engine.
func (builder *TypedBuilder) And(arguments ...Expr) (*OperExpr, error) {
	return builder.Apply(Op("AND"), arguments...)
}

func (builder *TypedBuilder) Or(arguments ...Expr) (*OperExpr, error) {
	return builder.Apply(Op("OR"), arguments...)
}

func (builder *TypedBuilder) Not(predicate Expr) (*OperExpr, error) {
	return builder.Apply(Op("NOT"), predicate)
}

func (builder *TypedBuilder) Impl(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("IMPLIES"), left, right)
}

func (builder *TypedBuilder) Equiv(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("EQUIV"), left, right)
}

func (builder *TypedBuilder) ForallIn(variable, set, predicate Expr) (*OperExpr, error) {
	if _, err := boundVariableNames(variable); err != nil {
		return nil, err
	}
	return builder.Apply(Op("FORALL3"), variable, set, predicate)
}

func (builder *TypedBuilder) Forall(variable, predicate Expr) (*OperExpr, error) {
	if _, err := boundVariableNames(variable); err != nil {
		return nil, err
	}
	return builder.Apply(Op("FORALL2"), variable, predicate)
}

func (builder *TypedBuilder) ExistsIn(variable, set, predicate Expr) (*OperExpr, error) {
	if _, err := boundVariableNames(variable); err != nil {
		return nil, err
	}
	return builder.Apply(Op("EXISTS3"), variable, set, predicate)
}

func (builder *TypedBuilder) Exists(variable, predicate Expr) (*OperExpr, error) {
	if _, err := boundVariableNames(variable); err != nil {
		return nil, err
	}
	return builder.Apply(Op("EXISTS2"), variable, predicate)
}

func (builder *ScopedBuilder) And(arguments ...ExprInstruction) ExprInstruction {
	return builder.Apply(Op("AND"), arguments...)
}

func (builder *ScopedBuilder) Or(arguments ...ExprInstruction) ExprInstruction {
	return builder.Apply(Op("OR"), arguments...)
}

func (builder *ScopedBuilder) Not(predicate ExprInstruction) ExprInstruction {
	return builder.Apply(Op("NOT"), predicate)
}

func (builder *ScopedBuilder) Impl(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("IMPLIES"), left, right)
}

func (builder *ScopedBuilder) Equiv(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("EQUIV"), left, right)
}

func (builder *ScopedBuilder) ForallIn(variable, set, predicate ExprInstruction) ExprInstruction {
	return builder.BindBounded(Op("FORALL3"), variable, set, predicate)
}

func (builder *ScopedBuilder) Forall(variable, predicate ExprInstruction) ExprInstruction {
	return builder.BindUnbounded(Op("FORALL2"), variable, predicate)
}

func (builder *ScopedBuilder) ExistsIn(variable, set, predicate ExprInstruction) ExprInstruction {
	return builder.BindBounded(Op("EXISTS3"), variable, set, predicate)
}

func (builder *ScopedBuilder) Exists(variable, predicate ExprInstruction) ExprInstruction {
	return builder.BindUnbounded(Op("EXISTS2"), variable, predicate)
}
