package nlsat

// Source: src/nlsat/nlsat_explain.cpp:704-725,864-949.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type explanationCellScope struct {
	roots                    *polynomial.AlgebraicVector
	lower, upper             polynomial.Algebraic
	lowerActive, upperActive bool
	active, executing        bool
}

func (n *explanationAssumptions) finishCell() error {
	s := &n.cell
	if s.executing {
		return ErrSearchPending
	}
	am := n.search.evaluator.am
	if s.roots != nil {
		if err := s.roots.Reset(); err != nil {
			return err
		}
	}
	if s.upperActive {
		if err := am.Del(&s.upper); err != nil {
			return err
		}
		s.upperActive = false
	}
	if s.lowerActive {
		if err := am.Del(&s.lower); err != nil {
			return err
		}
		s.lowerActive = false
	}
	s.active = false
	return nil
}

func (n *explanationAssumptions) addLinearRoot(k AtomKind, p polynomial.Polynomial, negativeCoefficient bool) error {
	sign := false
	switch k {
	case AtomRootEQ:
		k = AtomEQ
	case AtomRootLT:
		k = AtomLT
	case AtomRootGT:
		k = AtomGT
	case AtomRootLE:
		k, sign = AtomGT, true
	case AtomRootGE:
		k, sign = AtomLT, true
	default:
		return arena.ErrConfig
	}
	// The source negates p when its y coefficient is negative. Flipping the
	// strict comparison is the identical predicate and avoids constructing a
	// second polynomial; equality and literal polarity are unchanged.
	if negativeCoefficient {
		switch k {
		case AtomLT:
			k = AtomGT
		case AtomGT:
			k = AtomLT
		}
	}
	return n.addSimpleAssumption(k, p, sign)
}

func (n *explanationAssumptions) addLinearRootLiteral(k AtomKind, y uint32, p polynomial.Polynomial) (matched bool, err error) {
	core := n.search.store.atoms.ineq.core
	degree, err := core.Degree(p, y)
	if err != nil || degree != 1 {
		return false, err
	}
	integers, err := core.IntegerManager()
	if err != nil {
		return false, err
	}
	var coefficient unumeral.Int
	defer func() { err = errors.Join(err, integers.Del(&coefficient)) }()
	constant, err := core.ConstantCoefficient(p, y, 1, &coefficient)
	if err != nil || !constant {
		return false, err
	}
	zero, err := integers.IsZero(&coefficient)
	if err != nil {
		return false, err
	}
	if err = atomAssert("nlsat_explain.cpp", 743, "!m_pm.m().is_zero(c)", !zero); err != nil {
		return false, err
	}
	if zero {
		return false, arena.ErrConfig
	}
	negative, err := integers.IsNeg(&coefficient)
	if err != nil {
		return false, err
	}
	return true, n.addLinearRoot(k, p, negative)
}

func (n *explanationAssumptions) ensurePolynomialSign(p polynomial.Polynomial) (int, error) {
	sign, err := n.sign(p)
	if err != nil {
		return 0, err
	}
	constant, err := n.search.store.atoms.ineq.core.IsConstant(p)
	if err != nil || constant {
		return sign, err
	}
	k := AtomGT
	if sign == 0 {
		k = AtomEQ
	} else if sign < 0 {
		k = AtomLT
	}
	return sign, n.addSimpleAssumption(k, p, false)
}

type explanationQuadraticRefs struct {
	refs []*polynomial.PolynomialRef
}

func (s *explanationQuadraticRefs) close() (err error) {
	for i := len(s.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, s.refs[i].Close())
	}
	s.refs = nil
	return err
}

func (s *explanationQuadraticRefs) keep(core *polynomial.MultivariatePolynomialCore, value polynomial.Polynomial, cause error) (polynomial.Polynomial, error) {
	if cause != nil {
		return polynomial.Polynomial{}, cause
	}
	ref, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return polynomial.Polynomial{}, err
	}
	s.refs = append(s.refs, ref)
	if err = ref.Assign(value); err != nil {
		return polynomial.Polynomial{}, err
	}
	return ref.Get()
}

func (n *explanationAssumptions) addPseudoLinearRoot(k AtomKind, y uint32, p polynomial.Polynomial) (bool, error) {
	core := n.search.store.atoms.ineq.core
	degree, err := core.Degree(p, y)
	if err != nil || degree != 1 {
		return false, err
	}
	coefficient, err := core.Coeff(p, y, 1)
	if err != nil {
		return false, err
	}
	ref, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, ref.Close()) }()
	if err = ref.Assign(coefficient); err != nil {
		return false, err
	}
	coefficient, err = ref.Get()
	if err != nil {
		return false, err
	}
	sign, err := n.sign(coefficient)
	if err != nil || sign == 0 {
		return false, err
	}
	if _, err = n.ensurePolynomialSign(coefficient); err != nil {
		return false, err
	}
	return true, n.addLinearRoot(k, p, sign < 0)
}

func (n *explanationAssumptions) addQuadraticRootLiteral(k AtomKind, y, index uint32, p polynomial.Polynomial) (matched bool, err error) {
	core := n.search.store.atoms.ineq.core
	degree, err := core.Degree(p, y)
	if err != nil || degree != 2 || index < 1 || index > 2 {
		return false, err
	}
	assigned, err := n.search.assignment.Contains(y)
	if err != nil {
		return false, err
	}
	if err = atomAssert("nlsat_explain.cpp", 788, "m_assignment.is_assigned(y)", assigned); err != nil {
		return false, err
	}
	if !assigned {
		return false, arena.ErrConfig
	}
	var owned explanationQuadraticRefs
	defer func() { err = errors.Join(err, owned.close()) }()
	coefficient := func(power uint32) (polynomial.Polynomial, error) {
		value, e := core.Coeff(p, y, power)
		return owned.keep(core, value, e)
	}
	keep := func(value polynomial.Polynomial, cause error) (polynomial.Polynomial, error) {
		return owned.keep(core, value, cause)
	}
	a, err := coefficient(2)
	if err != nil {
		return false, err
	}
	b, err := coefficient(1)
	if err != nil {
		return false, err
	}
	c, err := coefficient(0)
	if err != nil {
		return false, err
	}
	bb, err := keep(core.Mul(b, b))
	if err != nil {
		return false, err
	}
	ac, err := keep(core.Mul(a, c))
	if err != nil {
		return false, err
	}
	twoAC, err := keep(core.Add(ac, ac))
	if err != nil {
		return false, err
	}
	fourAC, err := keep(core.Add(twoAC, twoAC))
	if err != nil {
		return false, err
	}
	discriminant, err := keep(core.Sub(bb, fourAC))
	if err != nil {
		return false, err
	}
	yy, err := keep(core.MkVariable(y, 1))
	if err != nil {
		return false, err
	}
	ay, err := keep(core.Mul(a, yy))
	if err != nil {
		return false, err
	}
	twoAY, err := keep(core.Add(ay, ay))
	if err != nil {
		return false, err
	}
	derivative, err := keep(core.Add(twoAY, b))
	if err != nil {
		return false, err
	}
	discriminantSign, err := n.ensurePolynomialSign(discriminant)
	if err != nil || discriminantSign < 0 {
		return false, err
	}
	aSign, err := n.ensurePolynomialSign(a)
	if err != nil {
		return false, err
	}
	if aSign == 0 {
		by, e := keep(core.Mul(b, yy))
		if e != nil {
			return false, e
		}
		linear, e := keep(core.Add(by, c))
		if e != nil {
			return false, e
		}
		return n.addPseudoLinearRoot(k, y, linear)
	}
	if _, err = n.ensurePolynomialSign(derivative); err != nil {
		return false, err
	}
	if discriminantSign > 0 {
		if _, err = n.ensurePolynomialSign(p); err != nil {
			return false, err
		}
	}
	return true, nil
}

// addRootLiteral uses the source's constant-coefficient linear rewrite before
// falling back to a root atom. The pseudo-linear and quadratic optimizations
// remain separate because their coefficient signs require projected guards.
func (n *explanationAssumptions) addRootLiteral(k AtomKind, y, index uint32, p polynomial.Polynomial) error {
	matched, err := n.addLinearRootLiteral(k, y, p)
	if err != nil || matched {
		return err
	}
	matched, err = n.addQuadraticRootLiteral(k, y, index, p)
	if err != nil || matched {
		return err
	}
	b, err := n.search.store.atoms.mkRoot(k, y, index, p)
	if err != nil {
		return err
	}
	return n.addLiteral(sat.MakeLiteral(b, true))
}

func (n *explanationAssumptions) addCellLiterals(ps *polynomial.PolynomialRefVector, y uint32) (err error) {
	s := &n.cell
	if s.active {
		return ErrSearchPending
	}
	assigned, err := n.search.assignment.Contains(y)
	if err != nil {
		return err
	}
	if err = atomAssert("nlsat_explain.cpp", 881, "m_assignment.is_assigned(y)", assigned); err != nil {
		return err
	}
	if !assigned {
		return arena.ErrConfig
	}
	s.active, s.executing = true, true
	defer func() {
		s.executing = false
		finish := n.finishCell()
		err = errors.Join(err, finish)
	}()
	am := n.search.evaluator.am
	if s.roots == nil {
		s.roots, err = am.NewVector()
		if err != nil {
			return err
		}
	}
	yValue, err := n.search.assignment.Value(y)
	if err != nil {
		return err
	}
	var lowerPolynomial, upperPolynomial polynomial.Polynomial
	var lowerIndex, upperIndex uint32
	count, err := ps.Size()
	if err != nil {
		return err
	}
	core := n.search.store.atoms.ineq.core
	for k := uint32(0); k < count; k++ {
		p, e := ps.Get(k)
		if e != nil {
			return e
		}
		max, e := core.MaxVariable(p)
		if e != nil {
			return e
		}
		if max != y {
			continue
		}
		if e = s.roots.Reset(); e != nil {
			return e
		}
		if e = am.IsolateMultivariateRoots(n.search.store.atoms.ineq.pm, p, UndefVarAssignment{Assignment: n.search.assignment, Variable: y}, s.roots); e != nil {
			return e
		}
		for i := uint32(0); i < s.roots.Len(); i++ {
			root, e := s.roots.Borrow(i)
			if e != nil {
				return e
			}
			comparison, e := am.Compare(yValue, root)
			if e != nil {
				return e
			}
			if comparison == 0 {
				if e = n.addRootLiteral(AtomRootEQ, y, i+1, p); e != nil {
					return e
				}
				return nil
			}
			if comparison < 0 {
				better := !s.upperActive
				if !better {
					better, e = am.Less(root, &s.upper)
					if e != nil {
						return e
					}
				}
				if better {
					s.upperActive = true
					if e = am.Copy(&s.upper, root); e != nil {
						return e
					}
					upperPolynomial, upperIndex = p, i+1
				}
			} else {
				better := !s.lowerActive
				if !better {
					better, e = am.Less(&s.lower, root)
					if e != nil {
						return e
					}
				}
				if better {
					s.lowerActive = true
					if e = am.Copy(&s.lower, root); e != nil {
						return e
					}
					lowerPolynomial, lowerIndex = p, i+1
				}
			}
		}
	}
	if s.lowerActive {
		kind := AtomRootGT
		if n.fullDimensional {
			kind = AtomRootGE
		}
		if err = n.addRootLiteral(kind, y, lowerIndex, lowerPolynomial); err != nil {
			return err
		}
	}
	if s.upperActive {
		kind := AtomRootLT
		if n.fullDimensional {
			kind = AtomRootLE
		}
		if err = n.addRootLiteral(kind, y, upperIndex, upperPolynomial); err != nil {
			return err
		}
	}
	return nil
}

func (n *explanationAssumptions) collectCellLiterals(ps *polynomial.PolynomialRefVector, y uint32, fullDimensional bool) (err error) {
	if ps == nil {
		return arena.ErrConfig
	}
	if err = n.start(); err != nil {
		return err
	}
	n.fullDimensional = fullDimensional
	defer func() { err = n.finish(err) }()
	return n.addCellLiterals(ps, y)
}
