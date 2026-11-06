package ast

// Source: src/util/sorting_network.h unate_cmp and wrappers.

func (s *PsortASTNetwork) unateCompare(mode uint8, k, n uint32, p psortASTPointer) (ASTHandle, error) {
	if mode == psortEQ && k == 0 {
		// Source reads carry[k-1] after constructing carry[0]. Preserve the
		// reached invalid unsigned index as a checked domain failure.
		return ASTHandle{}, ErrASTDomain
	}
	last := k
	if mode == psortLE || mode == psortLEFull || mode == psortEQ {
		last = k + 1
	}
	carry := make([]ASTHandle, last)
	for i := range carry {
		h, e := s.context.False()
		if e != nil {
			return ASTHandle{}, e
		}
		carry[i] = h
	}
	truth, e := s.context.True()
	if e != nil {
		return ASTHandle{}, e
	}
	for i := uint32(0); i < n; i++ {
		x, e := p.at(i)
		if e != nil {
			return ASTHandle{}, e
		}
		for j := last; j > 0; j-- {
			index := j - 1
			previous := truth
			if index > 0 {
				previous = carry[index-1]
			}
			term, e := s.logicAnd(x, previous)
			if e != nil {
				return ASTHandle{}, e
			}
			carry[index], e = s.logicOr(term, carry[index])
			if e != nil {
				return ASTHandle{}, e
			}
		}
	}
	switch mode {
	case psortLE, psortLEFull:
		return s.not(carry[k])
	case psortGE, psortGEFull:
		return carry[k-1], nil
	case psortEQ:
		n, e := s.not(carry[k])
		if e != nil {
			return ASTHandle{}, e
		}
		return s.logicAnd(n, carry[k-1])
	}
	return ASTHandle{}, ErrASTDomain
}
