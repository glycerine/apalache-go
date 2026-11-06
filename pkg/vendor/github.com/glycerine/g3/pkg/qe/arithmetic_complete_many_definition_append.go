package qe

func (r *ArithmeticCompleteProjectionResult) appendDefinitions(target *DefinitionVector) error {
	n, e := r.Len()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		declaration, definition, cause := r.definitionEntry(i)
		if cause != nil {
			return cause
		}
		if cause = target.PushBack(declaration, definition); cause != nil {
			return cause
		}
	}
	return nil
}
