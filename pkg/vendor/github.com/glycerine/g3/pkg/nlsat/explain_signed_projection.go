package nlsat

// Source: src/nlsat/nlsat_explain.cpp:1561-1741.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (n *explanationAssumptions) signedProjectSingle(x uint32, p polynomial.Polynomial) (err error) {
	ps, err := polynomial.NewPolynomialRefVector(n.search.store.atoms.ineq.core)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ps.Close()) }()
	if err = ps.Push(p); err != nil {
		return err
	}
	return n.project(ps, x)
}

func (n *explanationAssumptions) signedProjectPair(x uint32, p, q polynomial.Polynomial) (err error) {
	ps, err := polynomial.NewPolynomialRefVector(n.search.store.atoms.ineq.core)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ps.Close()) }()
	if err = ps.Push(p); err != nil {
		return err
	}
	if err = ps.Push(q); err != nil {
		return err
	}
	return n.project(ps, x)
}

func (n *explanationAssumptions) signedProjectPairs(x, selected uint32, ps *polynomial.PolynomialRefVector) error {
	count, err := ps.Size()
	if err != nil {
		return err
	}
	p, err := ps.Get(selected)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		if i == selected {
			continue
		}
		q, err := ps.Get(i)
		if err != nil {
			return err
		}
		if err = n.signedProjectPair(x, q, p); err != nil {
			return err
		}
	}
	return nil
}

func (n *explanationAssumptions) signedProjectInfinity(x uint32, ps *polynomial.PolynomialRefVector, positive bool) (err error) {
	core := n.search.store.atoms.ineq.core
	coefficient, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, coefficient.Close()) }()
	count, err := ps.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		p, err := ps.Get(i)
		if err != nil {
			return err
		}
		degree, err := core.Degree(p, x)
		if err != nil {
			return err
		}
		lc, err := core.Coeff(p, x, degree)
		if err != nil {
			return err
		}
		if err = coefficient.Assign(lc); err != nil {
			return err
		}
		lc, err = coefficient.Get()
		if err != nil {
			return err
		}
		constant, err := core.IsConstant(lc)
		if err != nil || constant {
			if err != nil {
				return err
			}
			continue
		}
		sign, err := n.sign(p)
		if err != nil {
			return err
		}
		if err = atomAssert("nlsat_explain.cpp", 1669, "s != 0", sign != 0); err != nil {
			return err
		}
		kind := AtomLT
		positiveAtInfinity := sign > 0
		if !positive && degree&1 != 0 {
			positiveAtInfinity = !positiveAtInfinity
		}
		if positiveAtInfinity {
			kind = AtomGT
		}
		if err = n.addSimpleAssumption(kind, lc, false); err != nil {
			return err
		}
	}
	return nil
}

func (n *explanationAssumptions) signedProject(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	core := n.search.store.atoms.ineq.core
	count, err := ps.Size()
	if err != nil {
		return err
	}
	equality, equalityDegree := uint32(0), uint32(0)
	equalityValid := false
	for i := uint32(0); i < count; {
		p, err := ps.Get(i)
		if err != nil {
			return err
		}
		sign, err := n.sign(p)
		if err != nil {
			return err
		}
		max, err := core.MaxVariable(p)
		if err != nil {
			return err
		}
		if max != x {
			kind := AtomGT
			if sign == 0 {
				kind = AtomEQ
			} else if sign < 0 {
				kind = AtomLT
			}
			if err = n.addSimpleAssumption(kind, p, false); err != nil {
				return err
			}
			last, err := ps.Get(count - 1)
			if err != nil {
				return err
			}
			if err = ps.Set(i, last); err != nil {
				return err
			}
			count--
			if err = ps.Shrink(count); err != nil {
				return err
			}
			continue
		}
		if sign == 0 {
			degree, err := core.Degree(p, x)
			if err != nil {
				return err
			}
			if !equalityValid || degree < equalityDegree {
				equality, equalityDegree, equalityValid = i, degree, true
			}
		}
		i++
	}
	if count == 0 {
		return nil
	}
	if count == 1 {
		p, err := ps.Get(0)
		if err != nil {
			return err
		}
		return n.signedProjectSingle(x, p)
	}
	if equalityValid {
		p, err := ps.Get(equality)
		if err != nil {
			return err
		}
		degree, err := core.Degree(p, x)
		if err != nil {
			return err
		}
		if degree == 1 {
			return n.signedSolveLinearEquality(x, equality, ps)
		}
		return n.signedProjectPairs(x, equality, ps)
	}

	am := n.search.evaluator.am
	roots, err := am.NewVector()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, roots.Close()) }()
	xValue, err := n.search.assignment.Value(x)
	if err != nil {
		return err
	}
	var lower, upper polynomial.Algebraic
	lowerValid, upperValid := false, false
	defer func() {
		if upperValid {
			err = errors.Join(err, am.Del(&upper))
		}
		if lowerValid {
			err = errors.Join(err, am.Del(&lower))
		}
	}()
	var lowerIndex, upperIndex, lowerCount, upperCount uint32
	for i := uint32(0); i < count; i++ {
		if err = roots.Reset(); err != nil {
			return err
		}
		p, err := ps.Get(i)
		if err != nil {
			return err
		}
		if err = n.search.evaluator.isolateRoots(p, x, roots); err != nil {
			return err
		}
		hasLower, hasUpper := false, false
		for j := uint32(0); j < roots.Len(); j++ {
			root, err := roots.Borrow(j)
			if err != nil {
				return err
			}
			order, err := am.Compare(xValue, root)
			if err != nil {
				return err
			}
			if err = atomAssert("nlsat_explain.cpp", 1635, "s != 0", order != 0); err != nil {
				return err
			}
			if order < 0 {
				hasUpper = true
				better := !upperValid
				if upperValid {
					order, err = am.Compare(root, &upper)
					if err != nil {
						return err
					}
					better = order < 0
				}
				if better {
					if err = am.Copy(&upper, root); err != nil {
						return err
					}
					upperValid, upperIndex = true, i
				}
			} else if order > 0 {
				hasLower = true
				better := !lowerValid
				if lowerValid {
					order, err = am.Compare(&lower, root)
					if err != nil {
						return err
					}
					better = order < 0
				}
				if better {
					if err = am.Copy(&lower, root); err != nil {
						return err
					}
					lowerValid, lowerIndex = true, i
				}
			}
		}
		if hasLower {
			lowerCount++
		}
		if hasUpper {
			upperCount++
		}
	}
	if upperCount == 0 {
		return n.signedProjectInfinity(x, ps, true)
	}
	if lowerCount == 0 {
		return n.signedProjectInfinity(x, ps, false)
	}
	selected := lowerIndex
	if upperCount <= lowerCount {
		selected = upperIndex
	}
	return n.signedProjectPairs(x, selected, ps)
}

func (n *explanationAssumptions) collectSignedProjection(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	if err = n.projectionReady(); err != nil {
		return err
	}
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.signedProject(ps, x)
}
