package nlsat

// Source: src/nlsat/nlsat_explain.cpp:313-348.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// lc and reduct are the two actual lexical locals, in declaration order.
// The caller's p is borrowed; this frame never closes that reference.
type explanationVanishingScope struct {
	refs              [2]*polynomial.PolynomialRef
	coefficient       polynomial.PolynomialCoefficientCall
	predicate         polynomial.PolynomialNonzeroConstantCall
	active, executing bool
	cursor            int
	sourceCause       error
}

func (n *explanationAssumptions) vanishingAssert(line int32, expression string, p polynomial.Polynomial) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	constant, err := n.search.store.atoms.ineq.core.IsConstant(p)
	if err != nil {
		return err
	}
	return atomAssert("nlsat_explain.cpp", line, expression, !constant)
}
func (n *explanationAssumptions) finishVanishing(owning bool) error {
	return n.finishVanishingPhase(owning, owning)
}

// A caller's older locals die before any unjoined child allocation is aborted.
func (n *explanationAssumptions) finishVanishingPhase(owning, abort bool) (err error) {
	s := &n.vanishing
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
	if err = s.coefficient.FinishSource(); err != nil {
		return err
	}
	// The nested add_zero_assumption f local is younger than reduct and lc.
	if n.localFactor != nil {
		if !owning {
			return ErrSearchPending
		}
		if err = n.localFactor.Close(); err != nil {
			return err
		}
		n.localFactor = nil
	}
	for s.cursor < 2 {
		i := 1 - s.cursor
		if s.refs[i] != nil {
			if owning {
				err = s.refs[i].Close()
			} else {
				_, err = s.refs[i].SourceFinalize()
			}
			if err != nil {
				return err
			}
			if owning {
				s.refs[i] = nil
			}
		}
		s.cursor++
	}
	if owning {
		for i := 1; i >= 0; i-- {
			if s.refs[i] != nil {
				if err = s.refs[i].Close(); err != nil {
					return err
				}
				s.refs[i] = nil
			}
		}
	}
	if abort {
		if err = s.predicate.CleanupRetained(); err != nil {
			return err
		}
		if err = s.coefficient.CleanupRetained(); err != nil {
			return err
		}
		*s = explanationVanishingScope{}
	}
	return nil
}
func (n *explanationAssumptions) elimVanishing(p *polynomial.PolynomialRef) (err error) {
	s := &n.vanishing
	if s.active {
		return ErrSearchPending
	}
	value, err := p.Get()
	if err != nil {
		return err
	}
	if err = n.vanishingAssert(314, "!is_const(p)", value); err != nil {
		return err
	}
	c := n.search.store.atoms.ineq.core
	x, err := c.MaxVariable(value)
	if err != nil {
		return err
	}
	k, err := c.Degree(value, x)
	if err != nil {
		return err
	}
	if err = atomAssert("nlsat_explain.cpp", 317, "k > 0", k > 0); err != nil {
		return err
	}
	*s = explanationVanishingScope{active: true, executing: true}
	defer func() {
		s.executing = false
		err = errors.Join(err, n.finishVanishing(false))
		// On a successful body all references and factory handoffs completed.
		// Explicit exact child acknowledgment also permits the next vector element.
		if err == nil {
			err = n.finishVanishing(true)
		}
	}()
	for i := range s.refs {
		s.refs[i], err = polynomial.NewPolynomialRef(c)
		if err != nil {
			return err
		}
	}
	for {
		value, err = p.Get()
		if err != nil {
			return err
		}
		constant, e := c.IsConstant(value)
		if e != nil {
			return e
		}
		if constant {
			return nil
		}
		if k == 0 {
			x, err = c.MaxVariable(value)
			if err != nil {
				return err
			}
			if err = atomAssert("nlsat_explain.cpp", 326, "x != null_var", x != NullVariable); err != nil {
				return err
			}
			k, err = c.Degree(value, x)
			if err != nil {
				return err
			}
		}
		nonzero, e := c.NonzeroConstantCoefficientWithCall(value, x, k, &s.predicate)
		if e != nil {
			return e
		}
		if nonzero {
			return nil
		}
		if err = c.CoeffWithReductInto(value, x, k, s.refs[0], s.refs[1], &s.coefficient); err != nil {
			return err
		}
		lc, e := s.refs[0].Get()
		if e != nil {
			return e
		}
		zero, e := c.IsZero(lc)
		if e != nil {
			return e
		}
		if !zero {
			sign, e := n.sign(lc)
			if e != nil {
				return e
			}
			if sign != 0 {
				return nil
			}
			if err = n.addZeroAssumption(lc); err != nil {
				return err
			}
		}
		if k == 0 {
			zero, e := c.Zero()
			if e != nil {
				return e
			}
			return p.Assign(zero)
		}
		k--
		reduct, e := s.refs[1].Get()
		if e != nil {
			return e
		}
		if err = p.Assign(reduct); err != nil {
			return err
		}
	}
}

// collectVanishing binds the same actual result scope as other explanation
// primitives; the caller retains p and observes source in-place reductions.
func (n *explanationAssumptions) collectVanishing(p *polynomial.PolynomialRef) (err error) {
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.elimVanishing(p)
}
