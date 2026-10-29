package lp

// Source: src/util/lp/random_updater.h:40-80;
// random_updater.hpp:37-128,153-163; lar_core_solver.h:806-814.

import "github.com/glycerine/g3/pkg/util/numeral"

// RandomUpdateInterval owns exact epsilon endpoints and interrupted arithmetic.
// A returned interval must close before its borrowed core and numeral manager,
// including when FindRandomShiftInterval returns an error with a nonnil owner.
// Equal endpoints are empty for shifting, but Contains includes both endpoints.
type RandomUpdateInterval struct {
	LowSet, UpperSet bool
	Low, Upper       RationalPair
	manager          *numeral.RationalManager
	work             *IndexedPairVector
	coefficient      *IndexedRationalVector
	scalars          *IndexedRationalVector
	column           RationalLinearIterator
	calls            exactEtaCalls
	stopped          bool
}

func NewRandomUpdateInterval(m *numeral.RationalManager) *RandomUpdateInterval {
	return &RandomUpdateInterval{Low: NewRationalPair(), Upper: NewRationalPair(), manager: m}
}
func (r *RandomUpdateInterval) ready() error {
	if r.stopped || r.work != nil || r.scalars != nil || r.calls.pending() {
		return ErrExactVectorPending
	}
	if r.manager == nil {
		return ErrExactVectorOwner
	}
	return nil
}
func (r *RandomUpdateInterval) setBound(v *RationalPair, low bool) error {
	if v == nil {
		return ErrBounds
	}
	bound, set := &r.Upper, r.UpperSet
	if low {
		bound, set = &r.Low, r.LowSet
	}
	if set {
		var keep bool
		var err error
		if low {
			keep, err = v.Less(r.manager, bound)
		} else {
			keep, err = bound.Less(r.manager, v)
		}
		if err != nil {
			return err
		}
		if keep {
			return nil
		}
	}
	if err := bound.Copy(r.manager, v); err != nil {
		return err
	}
	if low {
		r.LowSet = true
	} else {
		r.UpperSet = true
	}
	return nil
}
func (r *RandomUpdateInterval) SetLowBound(v *RationalPair) (err error) {
	if err = r.ready(); err != nil {
		return err
	}
	b, err := r.calls.begin(r.manager)
	if err != nil {
		return err
	}
	defer func() { err = r.calls.after(r.manager, b, err); r.stopped = err != nil }()
	return r.setBound(v, true)
}
func (r *RandomUpdateInterval) SetUpperBound(v *RationalPair) (err error) {
	if err = r.ready(); err != nil {
		return err
	}
	b, err := r.calls.begin(r.manager)
	if err != nil {
		return err
	}
	defer func() { err = r.calls.after(r.manager, b, err); r.stopped = err != nil }()
	return r.setBound(v, false)
}
func (r *RandomUpdateInterval) IsEmpty() (empty bool, err error) {
	if err = r.ready(); err != nil {
		return false, err
	}
	if !r.LowSet || !r.UpperSet {
		return false, nil
	}
	b, err := r.calls.begin(r.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = r.calls.after(r.manager, b, err); r.stopped = err != nil }()
	empty, err = r.Low.Greater(r.manager, &r.Upper)
	if err != nil || empty {
		return empty, err
	}
	return r.Low.Equal(r.manager, &r.Upper)
}
func (r *RandomUpdateInterval) Contains(v *RationalPair) (contains bool, err error) {
	if err = r.ready(); err != nil {
		return false, err
	}
	if v == nil {
		return false, ErrBounds
	}
	b, err := r.calls.begin(r.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = r.calls.after(r.manager, b, err); r.stopped = err != nil }()
	if r.LowSet {
		ok, e := v.Greater(r.manager, &r.Low)
		if e != nil {
			return false, e
		}
		if !ok {
			ok, e = r.Low.Equal(r.manager, v)
			if e != nil || !ok {
				return false, e
			}
		}
	}
	if r.UpperSet {
		ok, e := v.Less(r.manager, &r.Upper)
		if e != nil {
			return false, e
		}
		if !ok {
			ok, e = v.Equal(r.manager, &r.Upper)
			if e != nil || !ok {
				return false, e
			}
		}
	}
	return true, nil
}

// DiminishForBasic intersects the nonbasic variable's interval with the values
// that keep one actual basic column feasible. coefficient is the solved B^-1 A
// column entry, not necessarily the corresponding raw matrix coefficient.
func (r *RandomUpdateInterval) DiminishForBasic(c *LARCoreSolver, nb *RationalPair, j uint32, a *numeral.Rational) (err error) {
	if err = r.ready(); err != nil {
		return err
	}
	if c == nil || c.manager != r.manager {
		return ErrExactVectorOwner
	}
	if c.stopped {
		return ErrExactVectorPending
	}
	if nb == nil || a == nil {
		return ErrBounds
	}
	if uint64(j) >= uint64(len(c.Bounds.Types.values)) || uint64(j) >= uint64(len(c.RX.Data)) {
		return ErrBounds
	}
	if sourceDebugEnabled {
		if uint64(j) >= uint64(len(c.RHeading)) {
			return ErrBounds
		}
		if c.RHeading[j] < 0 {
			return ErrAssertion
		}
		zero, e := r.manager.IsZero(a)
		if e != nil {
			return e
		}
		if zero {
			return ErrAssertion
		}
	}
	b, err := r.calls.begin(r.manager)
	if err != nil {
		return err
	}
	defer func() { err = r.calls.after(r.manager, b, err); r.stopped = err != nil }()
	r.work = NewIndexedPairVector(r.manager, 3)
	typ := c.Bounds.Types.values[j]
	switch typ {
	case ColumnFree:
	case ColumnFixed:
		if err = r.setBound(nb, true); err != nil {
			return err
		}
		if err = r.setBound(nb, false); err != nil {
			return err
		}
	case ColumnLowBound, ColumnUpperBound, ColumnBoxed:
		positive, e := r.manager.IsPos(a)
		if e != nil {
			return e
		}
		// Preserve source boxed ordering: positive visits lower then upper;
		// negative visits upper then lower. Stop at the reached error prefix.
		if typ == ColumnLowBound || typ == ColumnBoxed && positive {
			if err = r.diminishBound(c, nb, j, a, true, positive); err != nil {
				return err
			}
		}
		if typ == ColumnUpperBound || typ == ColumnBoxed {
			if err = r.diminishBound(c, nb, j, a, false, positive); err != nil {
				return err
			}
		}
		if typ == ColumnBoxed && !positive {
			if err = r.diminishBound(c, nb, j, a, true, positive); err != nil {
				return err
			}
		}
	default:
		if sourceDebugEnabled {
			return ErrAssertion
		}
	}
	if err = r.work.Close(); err != nil {
		return err
	}
	r.work = nil
	return nil
}
func (r *RandomUpdateInterval) diminishBound(c *LARCoreSolver, nb *RationalPair, j uint32, a *numeral.Rational, low, positive bool) error {
	delta, quotient, value := &r.work.Data[0], &r.work.Data[1], &r.work.Data[2]
	if low {
		if uint64(j) >= uint64(len(c.Bounds.Low.values)) {
			return ErrBounds
		}
		if err := delta.Sub(r.manager, &c.RX.Data[j], &c.Bounds.Low.values[j]); err != nil {
			return err
		}
	} else {
		if uint64(j) >= uint64(len(c.Bounds.Upper.values)) {
			return ErrBounds
		}
		if err := delta.Sub(r.manager, &c.Bounds.Upper.values[j], &c.RX.Data[j]); err != nil {
			return err
		}
	}
	if sourceDebugEnabled {
		z := NewRationalPair()
		ok, e := delta.Greater(r.manager, &z)
		if e != nil {
			return e
		}
		if !ok {
			ok, e = z.Equal(r.manager, delta)
			if e != nil {
				return e
			}
			if !ok {
				return ErrAssertion
			}
		}
	}
	if err := quotient.Divide(r.manager, delta, a); err != nil {
		return err
	}
	if low {
		if err := value.Add(r.manager, nb, quotient); err != nil {
			return err
		}
	} else {
		if err := value.Sub(r.manager, nb, quotient); err != nil {
			return err
		}
	}
	return r.setBound(value, low != positive)
}

func (r *RandomUpdateInterval) Close() error {
	if r.manager == nil {
		return nil
	}
	if err := r.calls.cleanupRaw(r.manager); err != nil {
		return err
	}
	if r.column != nil {
		if err := r.column.Close(); err != nil {
			return err
		}
		r.column = nil
	}
	if r.coefficient != nil {
		if err := r.coefficient.Close(); err != nil {
			return err
		}
		r.coefficient = nil
	}
	if r.scalars != nil {
		if err := r.scalars.Close(); err != nil {
			return err
		}
		r.scalars = nil
	}
	if r.work != nil {
		if err := r.work.Close(); err != nil {
			return err
		}
		r.work = nil
	}
	if err := r.Upper.Del(r.manager); err != nil {
		return err
	}
	if err := r.Low.Del(r.manager); err != nil {
		return err
	}
	return r.calls.cleanupRoots(r.manager)
}

// GetColumnIterator borrows the actual tableau column or the primal solver's
// solved LU column. Close the iterator before modifying or closing that core.
func (c *LARCoreSolver) GetColumnIterator(j uint32) (it RationalLinearIterator, err error) {
	if c.stopped {
		return nil, ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if c.settings.UseTableau() {
		column, e := NewRationalColumnIterator(c.RA, j)
		if e != nil {
			return nil, e
		}
		return column, nil
	}
	if err = c.RPrimal.SolveBd(j); err != nil {
		return nil, err
	}
	return NewRationalIndexedIterator(c.RPrimal.Ed), nil
}

func (c *LARCoreSolver) FindRandomShiftInterval(j uint32) (r *RandomUpdateInterval, err error) {
	if c.stopped {
		return nil, ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	r = NewRandomUpdateInterval(c.manager)
	r.column, err = c.GetColumnIterator(j)
	if err != nil {
		return r, err
	}
	if err = r.findForColumn(c, j); err != nil {
		return r, err
	}
	if err = r.column.Close(); err != nil {
		return r, err
	}
	r.column = nil
	return r, nil
}

// findForColumn retains the same live iterator for the subsequent model shift.
func (r *RandomUpdateInterval) findForColumn(c *LARCoreSolver, j uint32) (err error) {
	if uint64(j) >= uint64(len(c.Bounds.Types.values)) || uint64(j) >= uint64(len(c.RX.Data)) {
		return ErrBounds
	}
	switch c.Bounds.Types.values[j] {
	case ColumnFree:
	case ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnFixed:
		typ := c.Bounds.Types.values[j]
		if typ != ColumnUpperBound {
			if uint64(j) >= uint64(len(c.Bounds.Low.values)) {
				return ErrBounds
			}
			if err = r.SetLowBound(&c.Bounds.Low.values[j]); err != nil {
				return err
			}
		}
		if typ != ColumnLowBound {
			if uint64(j) >= uint64(len(c.Bounds.Upper.values)) {
				return ErrBounds
			}
			if err = r.SetUpperBound(&c.Bounds.Upper.values[j]); err != nil {
				return err
			}
		}
	default:
		if sourceDebugEnabled {
			return ErrAssertion
		}
	}
	r.coefficient = NewIndexedRationalVector(c.manager, 1)
	if err = r.column.Reset(); err != nil {
		return err
	}
	for {
		i, ok, e := r.column.Next(&r.coefficient.Data[0])
		if e != nil {
			return e
		}
		if !ok {
			break
		}
		if uint64(i) >= uint64(len(c.RBasis)) {
			return ErrBounds
		}
		if err = r.DiminishForBasic(c, &c.RX.Data[j], c.RBasis[i], &r.coefficient.Data[0]); err != nil {
			return err
		}
		empty, e := r.IsEmpty()
		if e != nil {
			return e
		}
		if empty {
			break
		}
	}
	if err = r.coefficient.Close(); err != nil {
		return err
	}
	r.coefficient = nil
	return nil
}

// Draw preserves get_random_from_interval's one RNG read and 100000-point
// lattice, including unbounded endpoints and epsilon coordinates. The output
// must remain alive until interval cleanup after an interrupted assignment.
func (r *RandomUpdateInterval) Draw(s *Settings, out *RationalPair) (err error) {
	if err = r.ready(); err != nil {
		return err
	}
	if s == nil || out == nil {
		return ErrBounds
	}
	rand := s.RandomNext() % 100000
	r.work = NewIndexedPairVector(r.manager, 4)
	r.scalars = NewIndexedRationalVector(r.manager, 2)
	w := r.work.Data
	b, err := r.calls.begin(r.manager)
	if err != nil {
		return err
	}
	defer func() { err = r.calls.after(r.manager, b, err); r.stopped = err != nil }()
	if err = r.manager.SetInt64(&r.scalars.Data[0], int64(rand)); err != nil {
		return err
	}
	if err = r.manager.SetInt32(&r.scalars.Data[1], 100000); err != nil {
		return err
	}
	switch {
	case !r.LowSet && !r.UpperSet:
		if err = r.manager.Copy(&w[3].X, &r.scalars.Data[0]); err != nil {
			return err
		}
	case r.LowSet && !r.UpperSet:
		if err = r.manager.Copy(&w[0].X, &r.scalars.Data[0]); err != nil {
			return err
		}
		if err = w[3].Add(r.manager, &r.Low, &w[0]); err != nil {
			return err
		}
	case !r.LowSet && r.UpperSet:
		if err = r.manager.Copy(&w[0].X, &r.scalars.Data[0]); err != nil {
			return err
		}
		if err = w[3].Sub(r.manager, &r.Upper, &w[0]); err != nil {
			return err
		}
	default:
		if err = w[0].Sub(r.manager, &r.Upper, &r.Low); err != nil {
			return err
		}
		if err = r.manager.Mul(&r.scalars.Data[0], &w[0].X, &w[1].X); err != nil {
			return err
		}
		if err = r.manager.Mul(&r.scalars.Data[0], &w[0].Y, &w[1].Y); err != nil {
			return err
		}
		if err = w[2].Divide(r.manager, &w[1], &r.scalars.Data[1]); err != nil {
			return err
		}
		if err = w[3].Add(r.manager, &r.Low, &w[2]); err != nil {
			return err
		}
	}
	if err = out.Copy(r.manager, &w[3]); err != nil {
		return err
	}
	if err = r.work.Close(); err != nil {
		return err
	}
	r.work = nil
	if err = r.scalars.Close(); err != nil {
		return err
	}
	r.scalars = nil
	return nil
}
