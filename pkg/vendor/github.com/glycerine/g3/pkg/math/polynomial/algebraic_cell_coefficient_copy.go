package polynomial

// Source: algebraic_numbers.cpp:405-410,445-455,494.

// The coefficient SET already issues its exact allocation receipt. This borrow
// protects the enclosing placement loop only while that source call executes.
type algebraicCellCoefficientCopy struct {
	running bool
	borrow  upolyFactorTraceBorrow
}

func (s *algebraicCellCoefficientCopy) callback() bool {
	return s.running && s.borrow.executing && s.borrow.callback()
}
func (m *AlgebraicRootCore) cellCoefficientFactoryContinuation() bool {
	s := &m.cellCoefficientCopy
	p := m.config.Polynomials
	return s.running && s.borrow.executing && p.factorTrace == &s.borrow && !s.borrow.callback() && p.callbackDepth == 0
}
func (m *AlgebraicRootCore) copyCellPolynomial(l *algebraicLocation, r *algebraicRecord, p UPolynomialView, existing bool) error {
	s := &m.cellCoefficientCopy
	if s.running || m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.coefficientCopy) {
		return ErrAlgebraicPending
	}
	if e := s.borrow.bindManagers([3]*UPolynomialManager{m.config.Polynomials}, m.config.Rationals.Integers().BorrowTraceTags()); e != nil {
		return e
	}
	s.running = true
	defer func() { s.running = false; s.borrow.unbind() }()
	if existing {
		return m.copyPolynomial(l, r, p)
	}
	return m.copyPolynomialDataFromOwner(l, r, m.config.Polynomials, p)
}
