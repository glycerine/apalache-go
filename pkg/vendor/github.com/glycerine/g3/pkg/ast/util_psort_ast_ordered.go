package ast

// Source: src/util/sorting_network.h mk_ordered_1.

func (s *PsortASTNetwork) orderedOne(full, equality bool, n uint32, p psortASTPointer) (ASTHandle, error) {
	if n <= 1 && !equality {
		return s.context.True()
	}
	if n == 0 {
		return s.context.False()
	}
	if n == 1 {
		return p.at(0)
	}
	r, e := s.fresh()
	if e != nil {
		return ASTHandle{}, e
	}
	ys := make([]ASTHandle, n-1)
	for i := range ys {
		ys[i], e = s.fresh()
		if e != nil {
			return ASTHandle{}, e
		}
	}
	for i := uint32(0); i+1 < uint32(len(ys)); i++ {
		not, e := s.not(ys[i])
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause2(not, ys[i+1]); e != nil {
			return ASTHandle{}, e
		}
	}
	for i := uint32(0); i+1 < n; i++ {
		x, e := p.at(i)
		if e != nil {
			return ASTHandle{}, e
		}
		notX, e := s.not(x)
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause2(notX, ys[i]); e != nil {
			return ASTHandle{}, e
		}
		notR, e := s.not(r)
		if e != nil {
			return ASTHandle{}, e
		}
		notY, e := s.not(ys[i])
		if e != nil {
			return ASTHandle{}, e
		}
		next, e := p.at(i + 1)
		if e != nil {
			return ASTHandle{}, e
		}
		notNext, e := s.not(next)
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause3(notR, notY, notNext); e != nil {
			return ASTHandle{}, e
		}
	}
	if equality {
		notR, e := s.not(r)
		if e != nil {
			return ASTHandle{}, e
		}
		last, e := p.at(n - 1)
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause3(notR, ys[n-2], last); e != nil {
			return ASTHandle{}, e
		}
	}
	for i := uint32(1); i < n-1; i++ {
		notY, e := s.not(ys[i])
		if e != nil {
			return ASTHandle{}, e
		}
		x, e := p.at(i)
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause3(notY, x, ys[i-1]); e != nil {
			return ASTHandle{}, e
		}
	}
	notFirstY, e := s.not(ys[0])
	if e != nil {
		return ASTHandle{}, e
	}
	first, e := p.at(0)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = s.clause2(notFirstY, first); e != nil {
		return ASTHandle{}, e
	}
	if !full {
		return r, nil
	}
	twos := make([]ASTHandle, n-1)
	for i := range twos {
		twos[i], e = s.fresh()
		if e != nil {
			return ASTHandle{}, e
		}
	}
	notTwo, e := s.not(twos[0])
	if e != nil {
		return ASTHandle{}, e
	}
	if e = s.clause2(notTwo, ys[0]); e != nil {
		return ASTHandle{}, e
	}
	second, e := p.at(1)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = s.clause2(notTwo, second); e != nil {
		return ASTHandle{}, e
	}
	for i := uint32(1); i < n-1; i++ {
		notTwo, e = s.not(twos[i])
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause3(notTwo, ys[i], twos[i-1]); e != nil {
			return ASTHandle{}, e
		}
		x, e := p.at(i + 1)
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause3(notTwo, x, twos[i-1]); e != nil {
			return ASTHandle{}, e
		}
	}
	if equality {
		zero, e := s.fresh()
		if e != nil {
			return ASTHandle{}, e
		}
		notZero, e := s.not(zero)
		if e != nil {
			return ASTHandle{}, e
		}
		last, e := p.at(n - 1)
		if e != nil {
			return ASTHandle{}, e
		}
		notLast, e := s.not(last)
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause2(notZero, notLast); e != nil {
			return ASTHandle{}, e
		}
		notY, e := s.not(ys[n-2])
		if e != nil {
			return ASTHandle{}, e
		}
		if e = s.clause2(notZero, notY); e != nil {
			return ASTHandle{}, e
		}
		return r, s.clause3(r, zero, twos[len(twos)-1])
	}
	return r, s.clause2(r, twos[len(twos)-1])
}
