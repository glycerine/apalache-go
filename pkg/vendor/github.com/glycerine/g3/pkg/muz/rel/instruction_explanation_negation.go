package rel

// Source: dl_finite_product_relation.cpp negation filtering against finite
// columns; explanation rows remain outside the equality keys.

type explanationNegationInstruction struct{ *tableInstruction }

func newExplanationNegationInstruction(target, negative uint32, targetColumns, negativeColumns []int) Instruction {
	return &explanationNegationInstruction{NewFilterByNegationInstruction(target, negative, targetColumns, negativeColumns).(*tableInstruction)}
}

func (i *explanationNegationInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	source, err := context.ExplanationRegister(i.first)
	if err != nil || source == nil {
		return err == nil, err
	}
	negative, err := context.ExplanationRegister(i.second)
	if err != nil || negative == nil {
		return err == nil, err
	}
	context.stats.FilterByNegation++
	if err := source.FilterFiniteNegation(negative, i.left, i.right); err != nil {
		return false, err
	}
	count, err := source.Len()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, context.MakeEmpty(i.first)
	}
	return true, nil
}
