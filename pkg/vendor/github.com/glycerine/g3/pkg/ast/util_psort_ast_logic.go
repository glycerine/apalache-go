package ast

// Source: src/util/sorting_network.h mk_or/mk_and simplification helpers.

func (s *PsortASTNetwork) logicOr(values ...ASTHandle) (ASTHandle, error) {
	filtered := make([]ASTHandle, 0, len(values))
	for _, h := range values {
		if s.context.manager.IsTrue(h) {
			return h, nil
		}
		if !s.context.manager.IsFalse(h) {
			filtered = append(filtered, h)
		}
	}
	if len(filtered) == 0 {
		return s.context.False()
	}
	if len(filtered) == 1 {
		return filtered[0], nil
	}
	h, e := s.context.manager.MkOrArray(filtered)
	if e != nil {
		return ASTHandle{}, e
	}
	return h, s.context.push(&s.context.trail, h)
}

func (s *PsortASTNetwork) logicAnd(values ...ASTHandle) (ASTHandle, error) {
	filtered := make([]ASTHandle, 0, len(values))
	for _, h := range values {
		if s.context.manager.IsFalse(h) {
			return h, nil
		}
		if !s.context.manager.IsTrue(h) {
			filtered = append(filtered, h)
		}
	}
	if len(filtered) == 0 {
		return s.context.True()
	}
	if len(filtered) == 1 {
		return filtered[0], nil
	}
	h, e := s.context.manager.MkAndArray(filtered)
	if e != nil {
		return ASTHandle{}, e
	}
	return h, s.context.push(&s.context.trail, h)
}
