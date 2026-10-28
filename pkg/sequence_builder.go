package apalache

// Typed sequence builders mirror UnsafeSeqBuilder through the shared
// signature inference engine.
func (builder *TypedBuilder) Append(sequence, element Expr) (*OperExpr, error) {
	return builder.Apply(Op("Sequences!Append"), sequence, element)
}

func (builder *TypedBuilder) Concat(left, right Expr) (*OperExpr, error) {
	return builder.Apply(Op("Sequences!Concat"), left, right)
}

func (builder *TypedBuilder) Head(sequence Expr) (*OperExpr, error) {
	return builder.Apply(Op("Sequences!Head"), sequence)
}

func (builder *TypedBuilder) Tail(sequence Expr) (*OperExpr, error) {
	return builder.Apply(Op("Sequences!Tail"), sequence)
}

func (builder *TypedBuilder) Len(sequence Expr) (*OperExpr, error) {
	return builder.Apply(Op("Sequences!Len"), sequence)
}

func (builder *TypedBuilder) SubSeq(sequence, fromIndex, toIndex Expr) (*OperExpr, error) {
	return builder.Apply(Op("Sequences!SubSeq"), sequence, fromIndex, toIndex)
}

func (builder *ScopedBuilder) Append(sequence, element ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Sequences!Append"), sequence, element)
}

func (builder *ScopedBuilder) Concat(left, right ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Sequences!Concat"), left, right)
}

func (builder *ScopedBuilder) Head(sequence ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Sequences!Head"), sequence)
}

func (builder *ScopedBuilder) Tail(sequence ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Sequences!Tail"), sequence)
}

func (builder *ScopedBuilder) Len(sequence ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Sequences!Len"), sequence)
}

func (builder *ScopedBuilder) SubSeq(sequence, fromIndex, toIndex ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Sequences!SubSeq"), sequence, fromIndex, toIndex)
}
