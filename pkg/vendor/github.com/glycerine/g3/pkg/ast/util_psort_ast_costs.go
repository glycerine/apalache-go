package ast

// Source: src/util/sorting_network.h:125-147,523-528,593-639,680-702,754-797,888-916,968-978,1003-1013.

// These are source unsigned costs, including each intermediate wrap. They
// select actual algorithms; no host sorting or mathematical oracle is used.
type psortCost struct{ variables, clauses uint32 }

func (v psortCost) value() uint32 { return 5*v.variables + v.clauses }
func (v psortCost) plus(w psortCost) psortCost {
	return psortCost{v.variables + w.variables, v.clauses + w.clauses}
}
func (v psortCost) times(n uint32) psortCost { return psortCost{v.variables * n, v.clauses * n} }
func psortMin(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
func psortCeil(n uint32) uint32 { return n/2 + n%2 }
func (s *PsortASTNetwork) costCompare() (psortCost, error) {
	m, e := s.comparison()
	if e != nil {
		return psortCost{}, e
	}
	c := uint32(3)
	if m == psortEQ {
		c = 6
	}
	return psortCost{2, c}, nil
}
func (s *PsortASTNetwork) costInterleave(a, b uint32) (psortCost, error) {
	v, e := s.costCompare()
	return v.times(psortMin(a-1, b)), e
}
func (s *PsortASTNetwork) power2(n uint32) (uint32, error) {
	on, e := s.assertions()
	if e != nil {
		return 0, e
	}
	if on && !(n < 10) {
		if e = s.assertion(527, "n < 10"); e != nil {
			return 0, e
		}
	}
	if n >= 32 {
		return 0, ErrASTDomain
	}
	return uint32(1) << n, nil
}
func (s *PsortASTNetwork) costDirectSort(m, n uint32) (psortCost, error) {
	on, e := s.assertions()
	if e != nil {
		return psortCost{}, e
	}
	if on && !(m <= n && n < 10) {
		if e = s.assertion(1004, "m <= n && n < 10"); e != nil {
			return psortCost{}, e
		}
	}
	mode, e := s.comparison()
	if e != nil {
		return psortCost{}, e
	}
	v := psortCost{m, 0}
	if mode != psortGE {
		c, e := s.power2(n - 1)
		if e != nil {
			return v, e
		}
		v = v.plus(psortCost{0, c})
	}
	if mode != psortLE {
		c, e := s.power2(n - 1)
		if e != nil {
			return v, e
		}
		v = v.plus(psortCost{0, c})
	}
	return v, nil
}
func (s *PsortASTNetwork) costDirectMerge(a, b, c uint32) (psortCost, error) {
	mode, e := s.comparison()
	if e != nil {
		return psortCost{}, e
	}
	v := psortCost{c, 0}
	if mode != psortGE {
		v = v.plus(psortCost{0, a + b + psortMin(a, c)*psortMin(b, c)/2})
	}
	if mode != psortLE {
		v = v.plus(psortCost{0, psortMin(a, c) * psortMin(b, c) / 2})
	}
	return v, nil
}
func (s *PsortASTNetwork) costSort(n uint32) (psortCost, error) {
	switch n {
	case 0, 1:
		return psortCost{}, nil
	case 2:
		return s.costMerge(1, 1)
	}
	direct, e := s.useDirectSort(n)
	if e != nil {
		return psortCost{}, e
	}
	if direct {
		return s.costDirectSort(n, n)
	}
	return s.costSortRecursive(n)
}
func (s *PsortASTNetwork) costSortRecursive(n uint32) (psortCost, error) {
	on, e := s.assertions()
	if e != nil {
		return psortCost{}, e
	}
	if on && !(n > 2) {
		if e = s.assertion(790, "n > 2"); e != nil {
			return psortCost{}, e
		}
	}
	l := n / 2
	a, e := s.costSort(l)
	if e != nil {
		return a, e
	}
	b, e := s.costSort(n - l)
	if e != nil {
		return b, e
	}
	c, e := s.costMerge(l, n-l)
	return a.plus(b).plus(c), e
}
func (s *PsortASTNetwork) useDirectSort(n uint32) (bool, error) {
	on, e := s.assertions()
	if e != nil {
		return false, e
	}
	if on && !(n > 2) {
		if e = s.assertion(796, "n > 2"); e != nil {
			return false, e
		}
	}
	if n >= 10 {
		return false, nil
	}
	a, e := s.costDirectSort(n, n)
	if e != nil {
		return false, e
	}
	b, e := s.costSortRecursive(n)
	return a.value() < b.value(), e
}
func (s *PsortASTNetwork) costCard(k, n uint32) (psortCost, error) {
	if n <= k {
		return s.costSort(n)
	}
	direct, e := s.useDirectCard(k, n)
	if e != nil {
		return psortCost{}, e
	}
	if direct {
		return s.costDirectSort(k, n)
	}
	return s.costCardRecursive(k, n)
}
func (s *PsortASTNetwork) costCardRecursive(k, n uint32) (psortCost, error) {
	l := n / 2
	a, e := s.costCard(k, l)
	if e != nil {
		return a, e
	}
	b, e := s.costCard(k, n-l)
	if e != nil {
		return b, e
	}
	// vc_smerge takes (a,b,c); executable smerge takes (c,a,b).
	// Preserve the source cost call's literal order, even where its estimate
	// differs from the executable merge that card subsequently creates.
	c, e := s.costMergeSmall(n-l, k, l)
	return a.plus(b).plus(c), e
}
func (s *PsortASTNetwork) useDirectCard(k, n uint32) (bool, error) {
	if n >= 10 {
		return false, nil
	}
	a, e := s.costDirectSort(k, n)
	if e != nil {
		return false, e
	}
	b, e := s.costCardRecursive(k, n)
	return a.value() < b.value(), e
}
func (s *PsortASTNetwork) costMerge(a, b uint32) (psortCost, error) {
	if a == 1 && b == 1 {
		return s.costCompare()
	}
	if a == 0 || b == 0 {
		return psortCost{}, nil
	}
	direct, e := s.useDirectMerge(a, b, a+b)
	if e != nil {
		return psortCost{}, e
	}
	if direct {
		return s.costDirectMerge(a, b, a+b)
	}
	return s.costMergeRecursive(a, b)
}
func (s *PsortASTNetwork) costMergeRecursive(a, b uint32) (psortCost, error) {
	x, e := s.costMerge(psortCeil(a), psortCeil(b))
	if e != nil {
		return x, e
	}
	y, e := s.costMerge(a/2, b/2)
	if e != nil {
		return y, e
	}
	z, e := s.costInterleave(psortCeil(a)+psortCeil(b), a/2+b/2)
	return x.plus(y).plus(z), e
}
func (s *PsortASTNetwork) costMergeSmall(c, a, b uint32) (psortCost, error) {
	if a == 1 && b == 1 && c == 1 {
		mode, e := s.comparison()
		if e != nil {
			return psortCost{}, e
		}
		v := psortCost{1, 0}
		if mode != psortGE {
			v = v.plus(psortCost{0, 2})
		}
		if mode != psortLE {
			v = v.plus(psortCost{0, 1})
		}
		return v, nil
	}
	if a == 0 || b == 0 {
		return psortCost{}, nil
	}
	if a > c {
		return s.costMergeSmall(c, c, b)
	}
	if b > c {
		return s.costMergeSmall(c, a, c)
	}
	if a+b <= c {
		return s.costMerge(a, b)
	}
	direct, e := s.useDirectMerge(a, b, c)
	if e != nil {
		return psortCost{}, e
	}
	if direct {
		return s.costDirectMerge(a, b, c)
	}
	return s.costMergeSmallRecursive(a, b, c)
}
func (s *PsortASTNetwork) costMergeSmallRecursive(a, b, c uint32) (psortCost, error) {
	c1, c2 := (c+1)/2, (c-1)/2
	if c%2 == 0 {
		c1 = 1 + c/2
		c2 = c / 2
	}
	x, e := s.costMergeSmall(c1, psortCeil(a), psortCeil(b))
	if e != nil {
		return x, e
	}
	y, e := s.costMergeSmall(c2, a/2, b/2)
	if e != nil {
		return y, e
	}
	z, e := s.costInterleave(psortCeil(a)+psortCeil(b), a/2+b/2)
	if e != nil {
		return z, e
	}
	mode, e := s.comparison()
	if e != nil {
		return z, e
	}
	v := x.plus(y).plus(z).plus(psortCost{1, 0})
	if mode != psortGE {
		v = v.plus(psortCost{0, 2})
	}
	if mode != psortLE {
		v = v.plus(psortCost{0, 1})
	}
	return v, nil
}
func (s *PsortASTNetwork) useDirectMerge(a, b, c uint32) (bool, error) {
	if a >= 1<<15 || b >= 1<<15 {
		return false, nil
	}
	x, e := s.costDirectMerge(a, b, a+b)
	if e != nil {
		return false, e
	}
	y, e := s.costMergeSmallRecursive(a, b, c)
	return x.value() < y.value(), e
}

func psortCostMinimum(a, b psortCost) psortCost {
	if a.value() < b.value() {
		return a
	}
	return b
}
