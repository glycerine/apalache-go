package nlsat

// Source: src/nlsat/nlsat_evaluator.cpp:452-662.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (e *Evaluator) addPolynomial(p polynomial.Polynomial, x uint32) error {
	max, err := e.core.MaxVariable(p)
	if err != nil {
		return err
	}
	if err = atomAssert("nlsat_evaluator.cpp", 454, "m_pm.max_var(p) <= x", max <= x); err != nil {
		return err
	}
	if max < x {
		sign, err := e.evalSign(p)
		if err != nil {
			return err
		}
		return e.table.addConst(sign)
	}
	if err = e.addRoots.Reset(); err != nil {
		return err
	}
	e.addSigns = e.addSigns[:0]
	if err = e.isolateRootsWithSigns(p, x); err != nil {
		return err
	}
	return e.table.add(e.addRoots, e.addSigns)
}
func (e *Evaluator) signAt(a *InequalityAtom, c uint32) (int, error) {
	sign := 1
	for i := uint32(0); i < a.Size(); i++ {
		curr, err := e.table.signAt(i, c)
		if err != nil {
			return 0, err
		}
		even, err := a.IsEven(i)
		if err != nil {
			return 0, err
		}
		if even && curr < 0 {
			curr = 1
		}
		sign *= curr
		if sign == 0 {
			break
		}
	}
	return sign, nil
}
func (e *Evaluator) addInfeasible(lowerOpen, lowerInf bool, lowerID uint32, upperOpen, upperInf bool, upperID uint32, jst sat.Literal) error {
	lo, err := e.table.root(lowerID)
	if err != nil {
		return err
	}
	hi, err := e.table.root(upperID)
	if err != nil {
		return err
	}
	s, err := e.ism.Mk(lowerOpen, lowerInf, lo, upperOpen, upperInf, hi, jst)
	if err != nil {
		return err
	}
	if err = e.locals[1].assign(s); err != nil {
		return err
	}
	result, err := e.ism.MkUnion(e.locals[0].value, e.locals[1].value)
	if err != nil {
		return err
	}
	return e.locals[0].assign(result)
}
func (e *Evaluator) infeasibleInequality(a *InequalityAtom, neg bool) error {
	if err := e.table.reset(); err != nil {
		return err
	}
	for i := uint32(0); i < a.Size(); i++ {
		p, err := a.Polynomial(i)
		if err != nil {
			return err
		}
		if err = e.addPolynomial(p, a.maxVar); err != nil {
			return err
		}
	}
	jst := sat.MakeLiteral(a.boolVar, neg)
	prevSat, prevInf, prevOpen := true, true, true
	prevID := uint32(NullVariable)
	numCells := e.table.numCells()
	for c := uint32(0); c < numCells; c++ {
		sign, err := e.signAt(a, c)
		if err != nil {
			return err
		}
		if satisfied(sign, a.kind, neg) {
			if !prevSat {
				if err = atomAssert("nlsat_evaluator.cpp", 531, "c > 0", c > 0); err != nil {
					return err
				}
				open := c%2 == 1
				cell := c
				if !open {
					cell = c - 1
				}
				id, err := e.table.rootID(cell)
				if err != nil {
					return err
				}
				if err = e.addInfeasible(prevOpen, prevInf, prevID, open, false, id, jst); err != nil {
					return err
				}
				prevSat = true
			}
		} else {
			if prevSat {
				if c == 0 {
					if numCells == 1 {
						s, err := e.ism.Mk(true, true, nil, true, true, nil, jst)
						if err != nil {
							return err
						}
						if err = e.locals[0].assign(s); err != nil {
							return err
						}
					} else {
						prevOpen = true
						prevInf = true
						prevID = NullVariable
					}
				} else {
					if err = atomAssert("nlsat_evaluator.cpp", 566, "c > 0", c > 0); err != nil {
						return err
					}
					prevInf = false
					prevOpen = c%2 == 0
					cell := c
					if prevOpen {
						cell = c - 1
					}
					prevID, err = e.table.rootID(cell)
					if err != nil {
						return err
					}
				}
				prevSat = false
			}
			if c == numCells-1 {
				if err = e.addInfeasible(prevOpen, prevInf, prevID, true, true, NullVariable, jst); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (e *Evaluator) infeasibleRoot(a *RootAtom, neg bool) error {
	if err := atomAssert("nlsat_evaluator.cpp", 597, "i > 0", a.index > 0); err != nil {
		return err
	}
	jst := sat.MakeLiteral(a.boolVar, neg)
	if err := e.roots.Reset(); err != nil {
		return err
	}
	if err := e.isolateRoots(a.polynomial, a.maxVar, e.roots); err != nil {
		return err
	}
	if a.index > e.roots.Len() {
		if neg {
			return nil
		}
		s, err := e.ism.Mk(true, true, nil, true, true, nil, jst)
		if err != nil {
			return err
		}
		return e.locals[0].assign(s)
	}
	if a.index == 0 {
		return arena.ErrBounds
	}
	r, err := e.roots.Borrow(a.index - 1)
	if err != nil {
		return err
	}
	if a.kind == AtomRootEQ && !neg {
		left, err := e.ism.Mk(true, true, nil, true, false, r, jst)
		if err != nil {
			return err
		}
		if err = e.locals[2].assign(left); err != nil {
			return err
		}
		right, err := e.ism.Mk(true, false, r, true, true, nil, jst)
		if err != nil {
			return err
		}
		if err = e.locals[3].assign(right); err != nil {
			return err
		}
		union, err := e.ism.MkUnion(e.locals[2].value, e.locals[3].value)
		if err != nil {
			return err
		}
		return e.locals[0].assign(union)
	}
	loOpen, loInf, hiOpen, hiInf := false, false, false, false
	switch a.kind {
	case AtomRootEQ: // negated equality: [r,r]
	case AtomRootLT:
		if neg {
			loOpen = true
			loInf = true
			hiOpen = true
		} else {
			hiOpen = true
			hiInf = true
		}
	case AtomRootGT:
		if neg {
			loOpen = true
			hiOpen = true
			hiInf = true
		} else {
			loOpen = true
			loInf = true
		}
	case AtomRootLE:
		if neg {
			loOpen = true
			loInf = true
		} else {
			loOpen = true
			hiOpen = true
			hiInf = true
		}
	case AtomRootGE:
		if neg {
			hiOpen = true
			hiInf = true
		} else {
			loOpen = true
			loInf = true
			hiOpen = true
		}
	default:
		return arena.ErrConfig
	}
	s, err := e.ism.Mk(loOpen, loInf, r, hiOpen, hiInf, r, jst)
	if err != nil {
		return err
	}
	return e.locals[0].assign(s)
}

// InfeasibleIntervals returns one owned reference to the values falsifying the
// literal. The maximal variable is masked even if currently assigned, as needed
// by the retained conflict-core minimization consumer.
func (e *Evaluator) InfeasibleIntervals(a ArithmeticAtom, neg bool) (result *IntervalSet, err error) {
	if err = e.open(); err != nil {
		return nil, err
	}
	e.running = true
	defer func() {
		e.running = false
		if err != nil {
			e.failed = true
		}
	}()
	switch a := a.(type) {
	case *InequalityAtom:
		if a == nil {
			return nil, arena.ErrConfig
		}
		err = e.infeasibleInequality(a, neg)
	case *RootAtom:
		if a == nil {
			return nil, arena.ErrConfig
		}
		err = e.infeasibleRoot(a, neg)
	default:
		err = arena.ErrConfig
	}
	if err != nil {
		return nil, err
	}
	for i := len(e.locals) - 1; i > 0; i-- {
		if err = e.locals[i].reset(); err != nil {
			return nil, err
		}
	}
	result = e.locals[0].value
	e.locals[0].value = nil
	return result, nil
}
