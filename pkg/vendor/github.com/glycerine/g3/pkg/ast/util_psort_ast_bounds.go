package ast

// Source: src/util/sorting_network.h:190-260,269-494,507-520.

func (s *PsortASTNetwork) ge(full bool, k, n uint32, p psortASTPointer) (result ASTHandle, err error) {
	if k > n {
		return s.context.False()
	}
	if k == 0 {
		return s.context.True()
	}
	on, e := s.assertions()
	if e != nil {
		return result, e
	}
	if on && !(0 < k && k <= n) {
		if e = s.assertion(197, "0 < k && k <= n"); e != nil {
			return result, e
		}
	}
	f, e := s.beginFrame(2)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	in, out := &f.vectors[0], &f.vectors[1]
	dual, e := s.dualize(&k, n, p, in)
	if e != nil {
		return result, e
	}
	if dual {
		p, e := psortVectorPointer(in)
		if e != nil {
			return result, e
		}
		sz, e := in.Size()
		if e != nil {
			return result, e
		}
		return s.le(full, k, sz, p)
	}
	on, e = s.assertions()
	if e != nil {
		return result, e
	}
	if on && !(2*k <= n) {
		if e = s.assertion(203, "2*k <= n"); e != nil {
			return result, e
		}
	}
	s.mode = psortGE
	if full {
		s.mode = psortGEFull
	}
	s.modeInitialized = true
	if e = s.card(k, n, p, out); e != nil {
		return result, e
	}
	return out.At(k - 1)
}
func (s *PsortASTNetwork) le(full bool, k, n uint32, p psortASTPointer) (result ASTHandle, err error) {
	if k >= n {
		return s.context.True()
	}
	on, e := s.assertions()
	if e != nil {
		return result, e
	}
	if on && !(k < n) {
		if e = s.assertion(215, "k < n"); e != nil {
			return result, e
		}
	}
	f, e := s.beginFrame(2)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	in, out := &f.vectors[0], &f.vectors[1]
	dual, e := s.dualize(&k, n, p, in)
	if e != nil {
		return result, e
	}
	if dual {
		p, e := psortVectorPointer(in)
		if e != nil {
			return result, e
		}
		return s.ge(full, k, n, p)
	}
	if k == 1 {
		g, e := s.beginFrame(1)
		if e != nil {
			return result, e
		}
		defer func() { err = s.finishFrame(g, err) }()
		if e = s.defaultVectors(g); e != nil {
			return result, e
		}
		return s.atMostOne(full, n, p, &g.vectors[0], false)
	}
	on, e = s.assertions()
	if e != nil {
		return result, e
	}
	if on && !(2*k <= n) {
		if e = s.assertion(226, "2*k <= n"); e != nil {
			return result, e
		}
	}
	s.mode = psortLE
	if full {
		s.mode = psortLEFull
	}
	s.modeInitialized = true
	if e = s.card(k+1, n, p, out); e != nil {
		return result, e
	}
	h, e := out.At(k)
	if e != nil {
		return result, e
	}
	return s.not(h)
}
func (s *PsortASTNetwork) eq(full bool, k, n uint32, p psortASTPointer) (result ASTHandle, err error) {
	if k > n {
		return s.context.False()
	}
	on, e := s.assertions()
	if e != nil {
		return result, e
	}
	if on && !(k <= n) {
		if e = s.assertion(238, "k <= n"); e != nil {
			return result, e
		}
	}
	f, e := s.beginFrame(2)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	in, out := &f.vectors[0], &f.vectors[1]
	dual, e := s.dualize(&k, n, p, in)
	if e != nil {
		return result, e
	}
	if dual {
		p, e := psortVectorPointer(in)
		if e != nil {
			return result, e
		}
		return s.eq(full, k, n, p)
	}
	if k == 1 {
		return s.exactlyOne(full, n, p)
	}
	on, e = s.assertions()
	if e != nil {
		return result, e
	}
	if on && !(2*k <= n) {
		if e = s.assertion(249, "2*k <= n"); e != nil {
			return result, e
		}
	}
	s.mode = psortEQ
	s.modeInitialized = true
	if e = s.card(k+1, n, p, out); e != nil {
		return result, e
	}
	on, e = s.assertions()
	if e != nil {
		return result, e
	}
	if on {
		sz, e := out.Size()
		if e != nil {
			return result, e
		}
		if !(sz >= k+1) {
			if e = s.assertion(252, "out.size() >= k+1"); e != nil {
				return result, e
			}
		}
	}
	if k == 0 {
		h, e := out.At(k)
		if e != nil {
			return result, e
		}
		return s.not(h)
	}
	a, e := out.At(k - 1)
	if e != nil {
		return result, e
	}
	b, e := out.At(k)
	if e != nil {
		return result, e
	}
	b, e = s.not(b)
	if e != nil {
		return result, e
	}
	return s.minimum(a, b)
}
func (s *PsortASTNetwork) dualize(k *uint32, n uint32, p psortASTPointer, in *ASTLiteralVector) (bool, error) {
	on, e := s.assertions()
	if e != nil {
		return false, e
	}
	if on && !(*k <= n) {
		if e = s.assertion(508, "0 <= k && k <= N"); e != nil {
			return false, e
		}
	}
	if 2*(*k) <= n {
		return false, nil
	}
	*k = n - *k
	for i := uint32(0); i < n; i++ {
		h, e := p.at(i)
		if e != nil {
			return false, e
		}
		h, e = s.not(h)
		if e != nil {
			return false, e
		}
		if e = in.Push(h); e != nil {
			return false, e
		}
	}
	return true, nil
}
func (s *PsortASTNetwork) impliesOr(l ASTHandle, n uint32, p psortASTPointer) (err error) {
	f, e := s.beginFrame(1)
	if e != nil {
		return e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.rangeVector(f, 0, n, p); e != nil {
		return e
	}
	v := &f.vectors[0]
	h, e := s.not(l)
	if e != nil {
		return e
	}
	if e = v.Push(h); e != nil {
		return e
	}
	return s.clauseVector(v)
}
func (s *PsortASTNetwork) orImplies(l ASTHandle, n uint32, p psortASTPointer) error {
	for j := uint32(0); j < n; j++ {
		h, e := p.at(j)
		if e != nil {
			return e
		}
		h, e = s.not(h)
		if e != nil {
			return e
		}
		if e = s.clause2(h, l); e != nil {
			return e
		}
	}
	return nil
}
func (s *PsortASTNetwork) makeOr(n uint32, p psortASTPointer) (ASTHandle, error) {
	if n == 1 {
		return p.at(0)
	}
	h, e := s.fresh()
	if e != nil {
		return h, e
	}
	if e = s.impliesOr(h, n, p); e != nil {
		return h, e
	}
	return h, s.orImplies(h, n, p)
}
func (s *PsortASTNetwork) makeOr2(a, b ASTHandle) (ASTHandle, error) {
	ls := [3]ASTHandle{a, b}
	return s.makeOr(2, psortASTPointer{inline: &ls})
}
func (s *PsortASTNetwork) makeOrVector(v *ASTLiteralVector) (ASTHandle, error) {
	n, e := v.Size()
	if e != nil {
		return ASTHandle{}, e
	}
	p, e := psortVectorPointer(v)
	if e != nil {
		return ASTHandle{}, e
	}
	return s.makeOr(n, p)
}
func (s *PsortASTNetwork) impliesAnd(l ASTHandle, v *ASTLiteralVector) error {
	for j := uint32(0); ; j++ {
		n, e := v.Size()
		if e != nil {
			return e
		}
		if j >= n {
			return nil
		}
		h, e := s.not(l)
		if e != nil {
			return e
		}
		a, e := v.At(j)
		if e != nil {
			return e
		}
		if e = s.clause2(h, a); e != nil {
			return e
		}
	}
}
func (s *PsortASTNetwork) andImplies(l ASTHandle, v *ASTLiteralVector) (err error) {
	f, e := s.beginFrame(1)
	if e != nil {
		return e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return e
	}
	lits := &f.vectors[0]
	for j := uint32(0); ; j++ {
		n, e := v.Size()
		if e != nil {
			return e
		}
		if j >= n {
			break
		}
		h, e := v.At(j)
		if e != nil {
			return e
		}
		h, e = s.not(h)
		if e != nil {
			return e
		}
		if e = lits.Push(h); e != nil {
			return e
		}
	}
	if e = lits.Push(l); e != nil {
		return e
	}
	return s.clauseVector(lits)
}
func (s *PsortASTNetwork) makeAnd(v *ASTLiteralVector) (ASTHandle, error) {
	n, e := v.Size()
	if e != nil {
		return ASTHandle{}, e
	}
	if n == 1 {
		return v.At(0)
	}
	h, e := s.fresh()
	if e != nil {
		return h, e
	}
	if e = s.impliesAnd(h, v); e != nil {
		return h, e
	}
	return h, s.andImplies(h, v)
}
func (s *PsortASTNetwork) makeAnd2(a, b ASTHandle) (result ASTHandle, err error) {
	f, e := s.beginFrame(1)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	v := &f.vectors[0]
	if e = v.Push(a); e != nil {
		return result, e
	}
	if e = v.Push(b); e != nil {
		return result, e
	}
	return s.makeAnd(v)
}
func (s *PsortASTNetwork) exactlyOne(full bool, n uint32, p psortASTPointer) (result ASTHandle, err error) {
	f, e := s.beginFrame(1)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	ors := &f.vectors[0]
	r, e := s.atMostOne(full, n, p, ors, true)
	if e != nil {
		return result, e
	}
	if full {
		h, e := s.makeOrVector(ors)
		if e != nil {
			return result, e
		}
		return s.makeAnd2(r, h)
	}
	sz, e := ors.Size()
	if e != nil {
		return result, e
	}
	ptr, e := psortVectorPointer(ors)
	if e != nil {
		return result, e
	}
	return r, s.impliesOr(r, sz, ptr)
}
func (s *PsortASTNetwork) atMostOne(full bool, n uint32, p psortASTPointer, ors *ASTLiteralVector, useOrs bool) (result ASTHandle, err error) {
	// Source n>=4 && false erases the entire bimander call on this actual path.
	f, e := s.beginFrame(2)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.rangeVector(f, 0, n, p); e != nil {
		return result, e
	}
	in, ands := &f.vectors[0], &f.vectors[1]
	r, e := s.fresh()
	if e != nil {
		return result, e
	}
	incSize := uint32(4)
	if e = s.defaultVector(f, 1); e != nil {
		return result, e
	}
	if e = ands.Push(r); e != nil {
		return result, e
	}
	for {
		sz, e := in.Size()
		if e != nil {
			return result, e
		}
		if sz == 0 {
			break
		}
		if e = ors.Reset(); e != nil {
			return result, e
		}
		n, e := in.Size()
		if e != nil {
			return result, e
		}
		if n+1 == incSize {
			incSize++
		}
		for i := uint32(0); i < n; i += incSize {
			inc := psortMin(n-i, incSize)
			ptr, e := psortVectorPointer(in)
			if e != nil {
				return result, e
			}
			if e = s.atMostOneSmall(full, inc, ptr.add(i), r, ands); e != nil {
				return result, e
			}
			if useOrs || n > incSize {
				ptr, e = psortVectorPointer(in)
				if e != nil {
					return result, e
				}
				h, e := s.makeOr(inc, ptr.add(i))
				if e != nil {
					return result, e
				}
				if e = ors.Push(h); e != nil {
					return result, e
				}
			}
		}
		if n <= incSize {
			break
		}
		if e = in.Reset(); e != nil {
			return result, e
		}
		if e = in.Append(ors); e != nil {
			return result, e
		}
	}
	if full {
		if e = s.clauseVector(ands); e != nil {
			return result, e
		}
	}
	return r, nil
}
func (s *PsortASTNetwork) atMostOneSmall(full bool, n uint32, p psortASTPointer, result ASTHandle, ands *ASTLiteralVector) error {
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on && !(n > 0) {
		if e = s.assertion(376, "n > 0"); e != nil {
			return e
		}
	}
	if n == 1 {
		return nil
	}
	for i := uint32(0); i < n; i++ {
		for j := i + 1; j < n; j++ {
			a, e := s.not(result)
			if e != nil {
				return e
			}
			b, e := p.at(i)
			if e != nil {
				return e
			}
			b, e = s.not(b)
			if e != nil {
				return e
			}
			c, e := p.at(j)
			if e != nil {
				return e
			}
			c, e = s.not(c)
			if e != nil {
				return e
			}
			if e = s.clause3(a, b, c); e != nil {
				return e
			}
		}
	}
	if full {
		a, e := s.fresh()
		if e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			if e = s.atMostOneFullClause(i, n, p, a); e != nil {
				return e
			}
		}
		a, e = s.not(a)
		if e != nil {
			return e
		}
		return ands.Push(a)
	}
	return nil
}
func (s *PsortASTNetwork) atMostOneFullClause(i, n uint32, p psortASTPointer, a ASTHandle) (err error) {
	f, e := s.beginFrame(1)
	if e != nil {
		return e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return e
	}
	v := &f.vectors[0]
	if e = v.Push(a); e != nil {
		return e
	}
	for j := uint32(0); j < n; j++ {
		if j != i {
			h, e := p.at(j)
			if e != nil {
				return e
			}
			if e = v.Push(h); e != nil {
				return e
			}
		}
	}
	return s.clauseVector(v)
}

// The source active #if1 helper is retained as defined even though the main
// bounded policies call its result/ands overload instead.
func (s *PsortASTNetwork) atMostOnePairwise(n uint32, p psortASTPointer) (result ASTHandle, err error) {
	on, e := s.assertions()
	if e != nil {
		return result, e
	}
	if on && !(n > 0) {
		if e = s.assertion(404, "n > 0"); e != nil {
			return result, e
		}
	}
	if n == 1 {
		return s.context.True()
	}
	f, e := s.beginFrame(1)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	ands := &f.vectors[0]
	for i := uint32(0); i < n; i++ {
		for j := i + 1; j < n; j++ {
			a, e := p.at(i)
			if e != nil {
				return result, e
			}
			a, e = s.not(a)
			if e != nil {
				return result, e
			}
			b, e := p.at(j)
			if e != nil {
				return result, e
			}
			b, e = s.not(b)
			if e != nil {
				return result, e
			}
			h, e := s.makeOr2(a, b)
			if e != nil {
				return result, e
			}
			if e = ands.Push(h); e != nil {
				return result, e
			}
		}
	}
	return s.makeAnd(ands)
}
func (s *PsortASTNetwork) atMostOneBimander(full bool, n uint32, p psortASTPointer, ors *ASTLiteralVector) (result ASTHandle, err error) {
	f, e := s.beginFrame(3)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.rangeVector(f, 0, n, p); e != nil {
		return result, e
	}
	in, ands, bits := &f.vectors[0], &f.vectors[1], &f.vectors[2]
	r, e := s.fresh()
	if e != nil {
		return result, e
	}
	incSize := uint32(2)
	if e = s.defaultVector(f, 1); e != nil {
		return result, e
	}
	for i := uint32(0); i < n; i += incSize {
		inc := psortMin(n-i, incSize)
		ptr, e := psortVectorPointer(in)
		if e != nil {
			return result, e
		}
		if e = s.atMostOneSmall(full, inc, ptr.add(i), r, ands); e != nil {
			return result, e
		}
		ptr, e = psortVectorPointer(in)
		if e != nil {
			return result, e
		}
		h, e := s.makeOr(inc, ptr.add(i))
		if e != nil {
			return result, e
		}
		if e = ors.Push(h); e != nil {
			return result, e
		}
	}
	nbits := uint32(0)
	for {
		sz, e := ors.Size()
		if e != nil {
			return result, e
		}
		if nbits >= 31 {
			return result, ErrASTDomain
		}
		if uint32(1)<<nbits >= sz {
			break
		}
		nbits++
	}
	if e = s.defaultVector(f, 2); e != nil {
		return result, e
	}
	for k := uint32(0); k < nbits; k++ {
		h, e := s.fresh()
		if e != nil {
			return result, e
		}
		if e = bits.Push(h); e != nil {
			return result, e
		}
	}
	for i := uint32(0); ; i++ {
		sz, e := ors.Size()
		if e != nil {
			return result, e
		}
		if i >= sz {
			break
		}
		for k := uint32(0); k < nbits; k++ {
			a, e := s.not(r)
			if e != nil {
				return result, e
			}
			b, e := ors.At(i)
			if e != nil {
				return result, e
			}
			b, e = s.not(b)
			if e != nil {
				return result, e
			}
			c, e := bits.At(k)
			if e != nil {
				return result, e
			}
			if i&(uint32(1)<<k) == 0 {
				c, e = s.not(c)
				if e != nil {
					return result, e
				}
			}
			if e = s.clause3(a, b, c); e != nil {
				return result, e
			}
		}
	}
	return r, nil
}
