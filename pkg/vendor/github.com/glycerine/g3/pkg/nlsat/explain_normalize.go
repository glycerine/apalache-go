package nlsat

// Source: src/nlsat/nlsat_explain.cpp:355-370,395-501.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The vector overload declares p. Literal normalization declares ps, is_even,
// then p. Only one of these actual caller scopes can execute at a time.
type explanationNormalizationScope struct {
	p                     *polynomial.PolynomialRef
	ps                    *polynomial.PolynomialRefVector
	isEven                []bool
	active, executing     bool
	pFinished, psFinished bool
	sourceCause           error
}

func (n *explanationAssumptions) finishNormalization(owning bool) (err error) {
	s := &n.normalization
	if !s.active {
		return nil
	}
	if s.executing {
		return ErrSearchPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	defer func() {
		if err != nil && s.sourceCause == nil {
			s.sourceCause = err
		}
	}()
	if err = n.finishVanishingPhase(owning, false); err != nil {
		return err
	}
	if s.p != nil && (owning || !s.pFinished) {
		if owning {
			err = s.p.Close()
		} else {
			_, err = s.p.SourceFinalize()
		}
		if err != nil {
			return err
		}
		s.pFinished = true
	}
	if s.ps != nil && (owning || !s.psFinished) {
		if owning {
			err = s.ps.Close()
		} else {
			err = s.ps.SourceFinalize()
		}
		if err != nil {
			return err
		}
		s.psFinished = true
	}
	if owning {
		if err = n.finishVanishingPhase(true, true); err != nil {
			return err
		}
		*s = explanationNormalizationScope{}
	}
	return nil
}
func (n *explanationAssumptions) leaveNormalization(cause error) error {
	n.normalization.executing = false
	err := errors.Join(cause, n.finishNormalization(false))
	if err == nil {
		err = n.finishNormalization(true)
	}
	return err
}
func (n *explanationAssumptions) elimVanishingVector(ps *polynomial.PolynomialRefVector) (err error) {
	if n.normalization.active {
		return ErrSearchPending
	}
	size, err := ps.Size()
	if err != nil {
		return err
	}
	s := &n.normalization
	*s = explanationNormalizationScope{active: true, executing: true}
	defer func() { err = n.leaveNormalization(err) }()
	c := n.search.store.atoms.ineq.core
	s.p, err = polynomial.NewPolynomialRef(c)
	if err != nil {
		return err
	}
	j := uint32(0)
	for i := uint32(0); i < size; i++ {
		p, e := ps.Get(i)
		if e != nil {
			return e
		}
		if err = s.p.Assign(p); err != nil {
			return err
		}
		if err = n.elimVanishing(s.p); err != nil {
			return err
		}
		p, err = s.p.Get()
		if err != nil {
			return err
		}
		constant, e := c.IsConstant(p)
		if e != nil {
			return e
		}
		if !constant {
			if err = ps.Set(j, p); err != nil {
				return err
			}
			j++
		}
	}
	return ps.Shrink(j)
}
func (n *explanationAssumptions) assertLowerStage(p polynomial.Polynomial, max uint32) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	c := n.search.store.atoms.ineq.core
	x, err := c.MaxVariable(p)
	if err != nil {
		return err
	}
	if err = atomAssert("nlsat_explain.cpp", 415, "max_var(p) != null_var", x != NullVariable); err != nil {
		return err
	}
	x, err = c.MaxVariable(p)
	if err != nil {
		return err
	}
	return atomAssert("nlsat_explain.cpp", 416, "max_var(p) < max", x < max)
}
func (n *explanationAssumptions) assertChangedFactor(p, original polynomial.Polynomial) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	equal, err := n.search.store.atoms.ineq.core.Equal(p, original)
	if err != nil {
		return err
	}
	return atomAssert("nlsat_explain.cpp", 439, "!m_pm.eq(p, a->p(i))", !equal)
}
func (n *explanationAssumptions) normalizeLiteral(l sat.Literal, max uint32) (result sat.Literal, err error) {
	result = sat.DefaultLiteral()
	b := l.Variable()
	if b == TrueBooleanVariable {
		return l, nil
	}
	f := n.search.store.atoms
	var a ArithmeticAtom
	if uint64(b) < uint64(len(f.bools.atoms)) {
		a = f.bools.atoms[b]
	}
	if err = atomAssert("nlsat_explain.cpp", 399, "m_atoms[b] != 0", a != nil); err != nil {
		return result, err
	}
	if uint64(b) >= uint64(len(f.bools.atoms)) {
		return result, arena.ErrBounds
	}
	if a == nil {
		return result, ErrAtomCacheOwner
	}
	inequality, ok := a.(*InequalityAtom)
	if !ok {
		return l, nil
	}
	if n.normalization.active {
		return result, ErrSearchPending
	}
	s := &n.normalization
	*s = explanationNormalizationScope{active: true, executing: true}
	defer func() { err = n.leaveNormalization(err) }()
	c := f.ineq.core
	s.ps, err = polynomial.NewPolynomialRefVector(c)
	if err != nil {
		return result, err
	}
	s.p, err = polynomial.NewPolynomialRef(c)
	if err != nil {
		return result, err
	}
	atomSign := 1
	normalized := false
	for i := range inequality.factors {
		original := inequality.factors[i].Polynomial
		if err = s.p.Assign(original); err != nil {
			return result, err
		}
		p, e := s.p.Get()
		if e != nil {
			return result, e
		}
		x, e := c.MaxVariable(p)
		if e != nil {
			return result, e
		}
		if x == max {
			if err = n.elimVanishing(s.p); err != nil {
				return result, err
			}
		}
		p, err = s.p.Get()
		if err != nil {
			return result, err
		}
		constant, e := c.IsConstant(p)
		if e != nil {
			return result, e
		}
		lower := constant
		if !lower {
			x, e = c.MaxVariable(p)
			if e != nil {
				return result, e
			}
			lower = x < max
		}
		if lower {
			sign, e := n.sign(p)
			if e != nil {
				return result, e
			}
			constant, e = c.IsConstant(p)
			if e != nil {
				return result, e
			}
			if !constant {
				if err = n.assertLowerStage(p, max); err != nil {
					return result, err
				}
				switch {
				case sign == 0:
					err = n.addSimpleAssumption(AtomEQ, p, false)
				case inequality.factors[i].Even:
					err = n.addSimpleAssumption(AtomEQ, p, true)
				case sign < 0:
					err = n.addSimpleAssumption(AtomLT, p, false)
				default:
					err = n.addSimpleAssumption(AtomGT, p, false)
				}
				if err != nil {
					return result, err
				}
			}
			if sign == 0 {
				return explanationTruthLiteral((inequality.kind == AtomEQ) != l.Sign()), nil
			}
			// Source cpp432 tests -1, but rational signs can be -2, -3, etc.
			// Compare by sign to preserve odd-factor polarity (C++ oracle regression).
			if sign < 0 && !inequality.factors[i].Even {
				atomSign = -atomSign
			}
			normalized = true
		} else {
			if p != original {
				if err = n.assertChangedFactor(p, original); err != nil {
					return result, err
				}
				normalized = true
			}
			s.isEven = append(s.isEven, inequality.factors[i].Even)
			if err = s.ps.Push(p); err != nil {
				return result, err
			}
		}
	}
	size, e := s.ps.Size()
	if e != nil {
		return result, e
	}
	if size == 0 {
		if err = atomAssert("nlsat_explain.cpp", 447, "atom_sign != 0", atomSign != 0); err != nil {
			return result, err
		}
		value := false
		switch inequality.kind {
		case AtomLT:
			value = atomSign < 0
		case AtomGT:
			value = atomSign > 0
		}
		return explanationTruthLiteral(value != l.Sign()), nil
	}
	if normalized {
		kind := inequality.kind
		if atomSign < 0 {
			kind, err = kind.Flip()
			if err != nil {
				return result, err
			}
		}
		ps := make([]polynomial.Polynomial, size)
		for i := range ps {
			ps[i], err = s.ps.Get(uint32(i))
			if err != nil {
				return result, err
			}
		}
		result, err = f.mkInequalityLiteral(kind, ps, s.isEven)
		if err != nil {
			return result, err
		}
		if l.Sign() {
			result = result.Negated()
		}
		return result, nil
	}
	if err = atomAssert("nlsat_explain.cpp", 469, "atom_sign > 0", atomSign > 0); err != nil {
		return result, err
	}
	return l, nil
}
func explanationTruthLiteral(value bool) sat.Literal {
	if value {
		return TrueLiteral
	}
	return FalseLiteral
}
func (n *explanationAssumptions) normalizeCore(core *scopedLiteralVector, max uint32) error {
	if core == nil || core.factory != n.search.store.atoms {
		return ErrSearchOwner
	}
	if core.executing || core.resetting || core.setActive {
		return ErrSearchPending
	}
	size := len(core.result)
	j := uint32(0)
	for i := 0; i < size; i++ {
		l, err := n.normalizeLiteral(core.result[i], max)
		if err != nil {
			return err
		}
		if l == TrueLiteral {
			continue
		}
		if l == FalseLiteral {
			return core.reset()
		}
		if err = core.set(j, l); err != nil {
			return err
		}
		j++
	}
	return core.shrink(j)
}
func (n *explanationAssumptions) collectNormalize(l sat.Literal, max uint32) (result sat.Literal, err error) {
	if err = n.start(); err != nil {
		return sat.DefaultLiteral(), err
	}
	defer func() { err = n.finish(err) }()
	return n.normalizeLiteral(l, max)
}
func (n *explanationAssumptions) collectNormalizeCore(core *scopedLiteralVector, max uint32) (err error) {
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.normalizeCore(core, max)
}
func (n *explanationAssumptions) collectVanishingVector(ps *polynomial.PolynomialRefVector) (err error) {
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.elimVanishingVector(ps)
}
