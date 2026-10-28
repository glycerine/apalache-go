package apalache

func (builder *TypedBuilder) VariantTag(variant Expr) (*OperExpr, error) {
	return builder.Apply(Op("Variants!VariantTag"), variant)
}

func (builder *ScopedBuilder) Variant(tag string, value ExprInstruction, target VariantType) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, []ExprInstruction{value})
		if err != nil {
			return nil, err
		}
		return builder.typed.Variant(tag, built[0], target)
	}}
}

func (builder *ScopedBuilder) VariantFilter(tag string, set ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, []ExprInstruction{set})
		if err != nil {
			return nil, err
		}
		return builder.typed.VariantFilter(tag, built[0])
	}}
}

func (builder *ScopedBuilder) VariantTag(variant ExprInstruction) ExprInstruction {
	return builder.Apply(Op("Variants!VariantTag"), variant)
}

func (builder *ScopedBuilder) VariantGetOrElse(tag string, variant, fallback ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, []ExprInstruction{variant, fallback})
		if err != nil {
			return nil, err
		}
		return builder.typed.VariantGetOrElse(tag, built[0], built[1])
	}}
}

func (builder *ScopedBuilder) VariantGetUnsafe(tag string, variant ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, []ExprInstruction{variant})
		if err != nil {
			return nil, err
		}
		return builder.typed.VariantGetUnsafe(tag, built[0])
	}}
}
