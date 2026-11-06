package lp

// Source: src/util/lp/lar_solver.h:1203-1240.

import "github.com/glycerine/g3/pkg/util/numeral"

// HasLowerBound publishes the witness before copying the exact rational bound,
// then publishes strictness. Missing variables leave every output unchanged;
// missing witnesses publish InvalidLARIndex alone. External term IDs are not
// translated to columns, matching the source's deferred term-bound support.
// value must remain alive through solver cleanup after an interrupted copy.
func (s *LARSolver) HasLowerBound(j uint32, witness *uint32, value *numeral.Rational, strict *bool) (found bool, err error) {
	return s.queryBoundWitness(j, witness, value, strict, false)
}
func (s *LARSolver) HasUpperBound(j uint32, witness *uint32, value *numeral.Rational, strict *bool) (found bool, err error) {
	return s.queryBoundWitness(j, witness, value, strict, true)
}
func (s *LARSolver) queryBoundWitness(j uint32, witness *uint32, value *numeral.Rational, strict *bool, upper bool) (found bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if j >= s.Bounds.Witnesses.Size() {
		return false, nil
	}
	if witness == nil {
		return false, ErrBounds
	}
	w, e := s.Bounds.Witnesses.At(j)
	if e != nil {
		return false, e
	}
	*witness = w.LowBoundWitness
	bounds := s.Bounds.Low
	if upper {
		*witness = w.UpperBoundWitness
		bounds = s.Bounds.Upper
	}
	if *witness == InvalidLARIndex {
		return false, nil
	}
	bound, e := bounds.At(j)
	if e != nil {
		return false, e
	}
	if value == nil {
		return false, ErrBounds
	}
	b, e := s.calls.begin(s.manager)
	if e != nil {
		return false, e
	}
	e = s.manager.Copy(value, &bound.X)
	if err = s.calls.after(s.manager, b, e); err != nil {
		return false, err
	}
	if strict == nil {
		return false, ErrBounds
	}
	var isStrict bool
	if upper {
		isStrict, err = s.manager.IsNeg(&bound.Y)
	} else {
		isStrict, err = s.manager.IsPos(&bound.Y)
	}
	if err != nil {
		return false, err
	}
	*strict = isStrict
	return true, nil
}
