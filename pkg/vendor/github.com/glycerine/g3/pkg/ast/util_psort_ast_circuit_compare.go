package ast

// Source: src/util/sorting_network.h circuit_cmp and mk_ge.

func (s *PsortASTNetwork) circuitVectorGE(left, right []ASTHandle) (ASTHandle, error) {
	equal, e := s.context.True()
	if e != nil {
		return ASTHandle{}, e
	}
	greater, e := s.context.False()
	if e != nil {
		return ASTHandle{}, e
	}
	for i := len(left); i > 0; i-- {
		index := i - 1
		notRight, e := s.not(right[index])
		if e != nil {
			return ASTHandle{}, e
		}
		term, e := s.logicAnd(equal, left[index], notRight)
		if e != nil {
			return ASTHandle{}, e
		}
		greater, e = s.logicOr(greater, term)
		if e != nil {
			return ASTHandle{}, e
		}
		compatible, e := s.logicOr(left[index], notRight)
		if e != nil {
			return ASTHandle{}, e
		}
		equal, e = s.logicAnd(equal, compatible)
		if e != nil {
			return ASTHandle{}, e
		}
		equal, e = s.logicOr(greater, equal)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	return equal, nil
}

func (s *PsortASTNetwork) circuitCompare(mode uint8, k, n uint32, p psortASTPointer) (ASTHandle, error) {
	threshold := k
	if mode == psortLE || mode == psortLEFull {
		threshold++
	}
	bits := uint32(0)
	for value := threshold; value != 0; value >>= 1 {
		bits++
	}
	constant := make([]ASTHandle, bits)
	for i := uint32(0); i < bits; i++ {
		var e error
		if threshold&(uint32(1)<<i) != 0 {
			constant[i], e = s.context.True()
		} else {
			constant[i], e = s.context.False()
		}
		if e != nil {
			return ASTHandle{}, e
		}
	}
	value, overflow, e := s.circuitAdd(bits, n, p)
	if e != nil {
		return ASTHandle{}, e
	}
	ge, e := s.circuitVectorGE(value, constant)
	if e != nil {
		return ASTHandle{}, e
	}
	switch mode {
	case psortLE, psortLEFull:
		h, e := s.logicOr(overflow, ge)
		if e != nil {
			return ASTHandle{}, e
		}
		return s.not(h)
	case psortGE, psortGEFull:
		return s.logicOr(overflow, ge)
	case psortEQ:
		terms := make([]ASTHandle, 0, 2*len(value)+1)
		for i := range value {
			nc, e := s.not(constant[i])
			if e != nil {
				return ASTHandle{}, e
			}
			nv, e := s.not(value[i])
			if e != nil {
				return ASTHandle{}, e
			}
			a, e := s.logicOr(nc, value[i])
			if e != nil {
				return ASTHandle{}, e
			}
			b, e := s.logicOr(constant[i], nv)
			if e != nil {
				return ASTHandle{}, e
			}
			terms = append(terms, a, b)
		}
		noOverflow, e := s.not(overflow)
		if e != nil {
			return ASTHandle{}, e
		}
		terms = append(terms, noOverflow)
		return s.logicAnd(terms...)
	}
	return ASTHandle{}, ErrASTDomain
}
