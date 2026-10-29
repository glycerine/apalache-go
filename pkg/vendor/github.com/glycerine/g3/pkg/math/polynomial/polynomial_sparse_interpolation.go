package polynomial

// Source: src/math/polynomial/polynomial.cpp:3134-3182;
// src/util/scoped_numeral_vector.h; src/math/polynomial/linear_eq_solver.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Concrete numeral locals own native slices and are constructed at their
// declaration sites. The scalar cursor stops before older destruction.
type polynomialSparseInterpolationScope struct {
	vectors                                    [3][]unumeral.Int // cs, new_as, as
	mons                                       []Monomial
	aux                                        unumeral.Int
	solver                                     ModularLinearSolver
	active, solverConstructed, sourceFinalized bool
	cursor                                     uint8
	numCursor                                  [3]uint32
	append                                     polynomialCheapAppendInvocationReceipt
	factory                                    polynomialFactoryInvocationReceipt
	bodyCause, cleanupCause                    error
}

func (s *PolynomialSparseInterpolator) scopedPush(v *[]unumeral.Int, a *unumeral.Int) error {
	*v = append(*v, unumeral.Int{})
	return s.copyField(*v, uint32(len(*v)-1), a)
}
func (s *PolynomialSparseInterpolator) scopedReset(v *[]unumeral.Int) error {
	var cursor uint32
	if e := s.deleteFields(*v, &cursor, false); e != nil {
		return e
	}
	*v = (*v)[:0]
	return nil
}
func (s *PolynomialSparseInterpolator) scopedResize(v *[]unumeral.Int, n uint32) error {
	old := uint32(len(*v))
	if n <= old {
		cursor := n
		if e := s.deleteFields(*v, &cursor, false); e != nil {
			return e
		}
		*v = (*v)[:n]
		return nil
	}
	if uint64(n) <= uint64(cap(*v)) {
		*v = (*v)[:n]
		clear((*v)[old:])
	} else {
		values := make([]unumeral.Int, int(n))
		copy(values, *v)
		*v = values
	}
	return nil
}
func (s *PolynomialSparseInterpolator) finishInterpolationSource() (err error) {
	f := &s.interpolation
	m := s.owner.core
	if !f.active || f.sourceFinalized {
		return nil
	}
	if f.cleanupCause != nil {
		return f.cleanupCause
	}
	defer func() {
		if err != nil {
			f.cleanupCause = err
		}
	}()
	if f.cursor == 0 {
		if f.solverConstructed {
			if e := f.solver.SourceFinalize(); e != nil {
				return e
			}
		}
		f.cursor = 1
	}
	if f.cursor == 1 {
		if _, e := unumeral.SourceNumeralDel(m.integer, &f.aux); e != nil {
			return e
		}
		f.cursor = 2
	}
	if f.cursor == 2 {
		f.mons = nil
		f.cursor = 3
	}
	for f.cursor < 6 {
		index := 5 - f.cursor
		v := &f.vectors[index]
		if e := s.deleteFields(*v, &f.numCursor[index], false); e != nil {
			return e
		}
		*v = nil
		f.cursor++
	}
	f.sourceFinalized = true
	return nil
}
func (s *PolynomialSparseInterpolator) CleanupInterpolation() (err error) {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.running {
		return ErrPolynomialPending
	}
	s.beginExecution()
	defer func() { s.endExecution(err) }()
	return s.cleanupInterpolation()
}
func (s *PolynomialSparseInterpolator) cleanupInterpolation() error {
	f := &s.interpolation
	if !f.active {
		return nil
	}
	m := s.owner.core
	// Owning continuation follows completed reverse lexical destruction, without
	// replaying a stopped source destructor or discarding published fields.
	if f.solverConstructed {
		// This exact completed local already attempted its source destructor. An
		// owning abort resumes the existing owning cursor without replaying it.
		if f.solver.sourceCause != nil {
			if e := f.solver.identity(); e != nil {
				return e
			}
			if f.solver.manager != m.modular {
				return ErrPolynomialOwner
			}
			f.solver.closing = true
		}
		if e := f.solver.Close(); e != nil {
			return e
		}
	}
	if e := s.cleanupNumeric(); e != nil {
		return e
	}
	if e := m.deleteNumeral(&f.aux); e != nil {
		return e
	}
	f.mons = nil
	for i := 2; i >= 0; i-- {
		if e := s.deleteFields(f.vectors[i], &f.numCursor[i], true); e != nil {
			return e
		}
		f.vectors[i] = nil
	}
	if e := m.cleanupFactoryInvocation(&f.factory, true); e != nil {
		return e
	}
	if e := m.som.cleanupAppendInvocation(&f.append, true); e != nil {
		return e
	}
	*f = polynomialSparseInterpolationScope{}
	return nil
}
func (s *PolynomialSparseInterpolator) Interpolate(out *PolynomialRef) (ok bool, err error) {
	if e := s.open(); e != nil {
		return false, e
	}
	s.beginExecution()
	defer func() { s.endExecution(err) }()
	m := s.owner.core
	if out == nil || out.owner != m {
		return false, ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return false, e
	}
	if e := polynomialSparseSourceAssert(3135, "m_skeleton", func() (bool, error) { return s.skeleton != nil, nil }); e != nil {
		return false, e
	}
	if s.skeleton == nil {
		return false, ErrPolynomialOwner
	}
	f := &s.interpolation
	if f.active {
		return false, ErrPolynomialPending
	}
	if e := m.pscEnter(); e != nil {
		return false, e
	}
	*f = polynomialSparseInterpolationScope{active: true}
	defer func() {
		m.pscLeave()
		f.bodyCause = err
		if e := s.finishInterpolationSource(); e != nil {
			err = errors.Join(err, e)
			return
		}
		pending := m.matchesFactoryInvocation(&f.factory) || m.som.matchesAppendInvocation(&f.append) || s.numeric.PendingObject() != (umemory.ObjectRef{})
		for _, root := range s.numeric.PendingAllocatorBlocks() {
			pending = pending || root != (umemory.Block{})
		}
		if !pending {
			if e := s.cleanupInterpolation(); e != nil {
				err = errors.Join(err, e)
			}
		}
	}()
	// cs, new_as, as, mons and aux have completed default construction above.
	// The actual solver is constructed last, only after its source field guard.
	if e := f.solver.constructDefault(m.modular); e != nil {
		return false, e
	}
	f.solverConstructed = true
	count, e := s.skeleton.NumEntries()
	if e != nil {
		return false, e
	}
	cs, newAs, as := &f.vectors[0], &f.vectors[1], &f.vectors[2]
	for k := uint32(0); k < count; k++ {
		entry, e := s.skeleton.Entry(k)
		if e != nil {
			return false, e
		}
		n := entry.NumPowers
		if e = f.solver.Resize(n); e != nil {
			return false, e
		}
		if e = s.scopedResize(newAs, n); e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			if uint64(i) >= uint64(len(s.inputs)) {
				return false, arena.ErrBounds
			}
			in := s.inputs[i]
			if e = s.scopedReset(cs); e != nil {
				return false, e
			}
			for j := uint32(0); j < n; j++ {
				power, e := s.skeleton.Power(entry, j)
				if e != nil {
					return false, e
				}
				before, x := s.numericBefore()
				if x != nil {
					return false, x
				}
				e = m.modular.Power(&in, power, &f.aux)
				e = polynomialNewtonJoin(e, s.numericAfter(before))
				if e != nil {
					return false, e
				}
				if e = s.scopedPush(cs, &f.aux); e != nil {
					return false, e
				}
			}
			outputIndex := entry.FirstPower + i
			if uint64(outputIndex) >= uint64(len(s.outputs)) {
				return false, arena.ErrBounds
			}
			b := s.outputs[outputIndex]
			if e = s.traceEquation(*cs); e != nil {
				return false, e
			}
			before, x := s.numericBefore()
			if x != nil {
				return false, x
			}
			e = f.solver.Add(i, *cs, &b)
			e = polynomialNewtonJoin(e, s.numericAfter(before))
			if e != nil {
				return false, e
			}
		}
		if e = s.traceSystem(entry, &f.solver); e != nil {
			return false, e
		}
		before, x := s.numericBefore()
		if x != nil {
			return false, x
		}
		solved, e := f.solver.Solve(*newAs)
		e = polynomialNewtonJoin(e, s.numericAfter(before))
		if e != nil {
			return false, e
		}
		if !solved {
			return false, nil
		}
		for i := uint32(0); i < n; i++ {
			a := (*newAs)[i]
			zero, e := m.modular.IsZero(&a)
			if e != nil {
				return false, e
			}
			if zero {
				continue
			}
			if e = s.scopedPush(as, &a); e != nil {
				return false, e
			}
			mono, e := s.skeleton.Original(entry, i)
			if e != nil {
				return false, e
			}
			f.mons = append(f.mons, mono)
		}
	}
	view := monomialArrayView{values: f.mons, count: uint32(len(f.mons))}
	before, x := s.numericBefore()
	if x != nil {
		return false, x
	}
	var result Polynomial
	e = m.som.reset()
	if e == nil {
		result, e = m.mkTermsSourceReceipts(uint32(len(*as)), polynomialUnivariateCoefficients{values: *as}, view, false, &f.append, &f.factory)
	}
	e = polynomialNewtonJoin(e, s.numericAfter(before))
	if e != nil {
		return false, e
	}
	// Reference assignment may invoke a caller deletion handler. Its newly
	// allocated numeric roots belong to that caller, never this lexical scope.
	e = out.root.assignWithFactoryReceipt(m, result, &f.factory)
	if e != nil {
		return false, e
	}
	return true, nil
}
