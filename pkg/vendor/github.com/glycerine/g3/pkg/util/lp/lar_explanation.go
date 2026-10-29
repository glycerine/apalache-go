package lp

// Source: src/util/lp/lar_solver.h:371-394, explain_implied_bound.

// explanationTermColumn preserves the source map's operator[] publication:
// an existing but unregistered term inserts a zero-valued column mapping.
func (s *LARSolver) explanationTermColumn(term uint32) uint32 {
	column, found := s.ExternalToColumns[term]
	if !found {
		s.ExternalToColumns[term] = 0
	}
	return column
}

// ExplainImpliedBound sends the original row coefficients and selected bound
// witnesses to the borrowed propagator's evidence consumer, in row order. It
// does not normalize coefficients or rebuild a row from the current LU basis.
// On consumer failure, completed evidence belongs to that consumer; its stopped
// numeric calls remain separately owned. No model numeric capture surrounds it.
func (s *LARSolver) ExplainImpliedBound(bound *ImpliedBound, bp *LARBoundPropagator) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if bound == nil || bp == nil {
		return ErrBounds
	}
	if bound.manager != s.manager || bp.manager != s.manager {
		return ErrExactVectorOwner
	}
	if bound.stopped || bound.calls.pending() {
		return ErrExactVectorPending
	}
	boundSign := int32(-1)
	if bound.IsLowBound {
		boundSign = 1
	}
	targetSign := -boundSign
	if bound.CoefficientIsPositive {
		targetSign = boundSign
	}
	target := bound.Column
	if s.IsTerm(target) {
		target = s.explanationTermColumn(target)
	}
	row := bound.RowOrTermIndex
	if uint64(row) >= uint64(len(s.Core.RA.Rows)) {
		return ErrBounds
	}
	for k := range s.Core.RA.Rows[row] {
		cell := &s.Core.RA.Rows[row][k]
		column := cell.Column
		if column == target {
			continue
		}
		if s.IsTerm(column) {
			column = s.explanationTermColumn(column)
		}
		sign, e := s.manager.Sign(&cell.Value)
		if e != nil {
			return e
		}
		coefficientSign := int32(-1)
		if sign > 0 {
			coefficientSign = 1
		}
		witnesses, e := s.Bounds.Witnesses.At(column)
		if e != nil {
			return e
		}
		witness := witnesses.LowBoundWitness
		if targetSign*coefficientSign > 0 {
			witness = witnesses.UpperBoundWitness
		}
		if sourceDebugEnabled && witness == InvalidLARIndex {
			return ErrAssertion
		}
		if err = bp.Consume(&cell.Value, witness); err != nil {
			return err
		}
	}
	return nil
}
