package lp

// Source: src/util/lp/random_updater.hpp:28-35,130-151,166-225;
// random_updater.h; lar_solver.h:1363-1381.

import (
	"sort"
)

type randomUpdateValue struct {
	value RationalPair
	count uint32
}

// RandomUpdater borrows a feasible core and owns value keys and shift locals.
// Close before that core, including a nonnil partial constructor result. Do
// not copy. Value occurrence counts deliberately include duplicate requests,
// while the variable set visits each nonbasic column once, in ascending order.
type RandomUpdater struct {
	core        *LARCoreSolver
	vars        map[uint32]struct{}
	values      map[[2]uint32][]*randomUpdateValue
	owned       []*randomUpdateValue
	distinct    int
	column      RationalLinearIterator
	interval    *RandomUpdateInterval
	work        *IndexedPairVector
	coefficient *IndexedRationalVector
	calls       exactEtaCalls
	stopped     bool
}

func NewRandomUpdater(c *LARCoreSolver, columns []uint32) (u *RandomUpdater, err error) {
	if c == nil {
		return nil, ErrBounds
	}
	if c.stopped {
		return nil, ErrExactVectorPending
	}
	u = &RandomUpdater{core: c, vars: make(map[uint32]struct{}), values: make(map[[2]uint32][]*randomUpdateValue)}
	defer func() { u.stopped = err != nil }()
	for _, j := range columns {
		if err = u.addColumnToSets(j); err != nil {
			return u, err
		}
	}
	return u, nil
}
func (u *RandomUpdater) ready() error {
	if u.stopped || u.calls.pending() || u.column != nil || u.interval != nil || u.work != nil || u.coefficient != nil || u.core.stopped {
		return ErrExactVectorPending
	}
	return nil
}
func (u *RandomUpdater) valueKey(v *RationalPair) ([2]uint32, error) {
	x, err := u.core.manager.Hash(&v.X)
	if err != nil {
		return [2]uint32{}, err
	}
	y, err := u.core.manager.Hash(&v.Y)
	return [2]uint32{x, y}, err
}
func (u *RandomUpdater) addValue(v *RationalPair) error {
	key, err := u.valueKey(v)
	if err != nil {
		return err
	}
	for _, entry := range u.values[key] {
		eq, e := entry.value.Equal(u.core.manager, v)
		if e != nil {
			return e
		}
		if eq {
			entry.count++
			return nil
		}
	}
	entry := &randomUpdateValue{value: NewRationalPair()}
	u.owned = append(u.owned, entry) // Own the key before its possibly partial copy.
	if err = u.copyPair(&entry.value, v); err != nil {
		return err
	}
	entry.count = 1
	u.values[key] = append(u.values[key], entry)
	u.distinct++
	return nil
}
func (u *RandomUpdater) removeValue(v *RationalPair) error {
	key, err := u.valueKey(v)
	if err != nil {
		return err
	}
	bucket := u.values[key]
	for i, entry := range bucket {
		eq, e := entry.value.Equal(u.core.manager, v)
		if e != nil {
			return e
		}
		if !eq {
			continue
		}
		entry.count--
		if entry.count == 0 {
			if err = entry.value.Del(u.core.manager); err != nil {
				return err
			}
			bucket = append(bucket[:i], bucket[i+1:]...)
			if len(bucket) == 0 {
				delete(u.values, key)
			} else {
				u.values[key] = bucket
			}
			u.distinct--
		}
		return nil
	}
	if sourceDebugEnabled {
		return ErrAssertion
	}
	return ErrBounds
}
func (u *RandomUpdater) copyPair(out, source *RationalPair) (err error) {
	b, err := u.calls.begin(u.core.manager)
	if err != nil {
		return err
	}
	err = out.Copy(u.core.manager, source)
	return u.calls.after(u.core.manager, b, err)
}
func (u *RandomUpdater) addColumnToSets(j uint32) error {
	c := u.core
	if uint64(j) >= uint64(len(c.RHeading)) {
		return ErrBounds
	}
	if c.RHeading[j] < 0 {
		if uint64(j) >= uint64(len(c.RX.Data)) {
			return ErrBounds
		}
		u.vars[j] = struct{}{}
		return u.addValue(&c.RX.Data[j])
	}
	row := uint32(c.RHeading[j])
	if uint64(row) >= uint64(len(c.RA.Rows)) {
		return ErrBounds
	}
	for _, cell := range c.RA.Rows[row] {
		cj := cell.Column
		if uint64(cj) >= uint64(len(c.RHeading)) {
			return ErrBounds
		}
		if c.RHeading[cj] < 0 {
			if uint64(cj) >= uint64(len(c.RX.Data)) {
				return ErrBounds
			}
			u.vars[cj] = struct{}{}
			if err := u.addValue(&c.RX.Data[cj]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *RandomUpdater) Update() (err error) {
	if err = u.ready(); err != nil {
		return err
	}
	defer func() { u.stopped = err != nil }()
	columns := make([]uint32, 0, len(u.vars))
	for j := range u.vars {
		columns = append(columns, j)
	}
	sort.Slice(columns, func(i, j int) bool { return columns[i] < columns[j] })
	for _, j := range columns {
		if len(u.vars) <= u.distinct {
			break
		}
		if err = u.randomShiftVar(j); err != nil {
			return err
		}
	}
	return nil
}
func (u *RandomUpdater) randomShiftVar(j uint32) error {
	var err error
	u.column, err = u.core.GetColumnIterator(j)
	if err != nil {
		return err
	}
	if u.column.Size() >= 50 {
		if err = u.column.Close(); err != nil {
			return err
		}
		u.column = nil
		return nil
	}
	u.interval = NewRandomUpdateInterval(u.core.manager)
	u.interval.column = u.column
	u.column = nil
	if err = u.interval.findForColumn(u.core, j); err != nil {
		return err
	}
	empty, err := u.interval.IsEmpty()
	if err != nil {
		return err
	}
	if !empty {
		if err = u.shiftVar(j); err != nil {
			return err
		}
	}
	if err = u.interval.Close(); err != nil {
		return err
	}
	u.interval = nil
	return nil
}
func (u *RandomUpdater) validateShiftColumn(j uint32) error {
	c := u.core
	if uint64(j) >= uint64(len(c.RX.Data)) {
		return ErrBounds
	}
	inside, err := u.interval.Contains(&c.RX.Data[j])
	if err != nil {
		return err
	}
	if !inside {
		return ErrAssertion
	}
	feasible, err := c.RPrimal.ColumnIsFeasible(j)
	if err != nil {
		return err
	}
	if !feasible {
		return ErrAssertion
	}
	return nil
}
func (u *RandomUpdater) shiftVar(j uint32) (err error) {
	c, m := u.core, u.core.manager
	if uint64(j) >= uint64(len(c.RX.Data)) {
		return ErrBounds
	}
	if sourceDebugEnabled {
		if err = u.validateShiftColumn(j); err != nil {
			return err
		}
	}
	u.work = NewIndexedPairVector(m, 5)
	old, result, newValue, delta, product := &u.work.Data[0], &u.work.Data[1], &u.work.Data[2], &u.work.Data[3], &u.work.Data[4]
	if err = u.copyPair(old, &c.RX.Data[j]); err != nil {
		return err
	}
	if err = u.removeValue(old); err != nil {
		return err
	}
	if err = u.interval.Draw(c.settings, result); err != nil {
		return err
	}
	if err = u.copyPair(&c.RX.Data[j], result); err != nil {
		return err
	}
	if err = u.copyPair(newValue, &c.RX.Data[j]); err != nil {
		return err
	}
	if err = u.addValue(newValue); err != nil {
		return err
	}
	if sourceDebugEnabled {
		if err = u.validateShiftColumn(j); err != nil {
			return err
		}
	}
	b, err := u.calls.begin(m)
	if err != nil {
		return err
	}
	err = delta.Sub(m, &c.RX.Data[j], old)
	if err = u.calls.after(m, b, err); err != nil {
		return err
	}
	if err = u.interval.column.Reset(); err != nil {
		return err
	}
	u.coefficient = NewIndexedRationalVector(m, 1)
	for {
		row, ok, e := u.interval.column.Next(&u.coefficient.Data[0])
		if e != nil {
			return e
		}
		if !ok {
			break
		}
		if uint64(row) >= uint64(len(c.RBasis)) {
			return ErrBounds
		}
		bj := c.RBasis[row]
		if uint64(bj) >= uint64(len(c.RX.Data)) {
			return ErrBounds
		}
		a := &u.coefficient.Data[0]
		b, e := u.calls.begin(m)
		if e != nil {
			return e
		}
		e = m.Mul(a, &delta.X, &product.X)
		if e == nil {
			e = m.Mul(a, &delta.Y, &product.Y)
		}
		if e == nil {
			e = c.RX.Data[bj].Sub(m, &c.RX.Data[bj], product)
		}
		if err = u.calls.after(m, b, e); err != nil {
			return err
		}
		if sourceDebugEnabled {
			feasible, e := c.RPrimal.ColumnIsFeasible(bj)
			if e != nil {
				return e
			}
			if !feasible {
				return ErrAssertion
			}
		}
	}
	if sourceDebugEnabled {
		off, e := c.RPrimal.AMultXIsOff()
		if e != nil {
			return e
		}
		if off {
			return ErrAssertion
		}
	}
	if err = u.coefficient.Close(); err != nil {
		return err
	}
	u.coefficient = nil
	if err = u.work.Close(); err != nil {
		return err
	}
	u.work = nil
	return nil
}
func (u *RandomUpdater) Close() error {
	if u == nil {
		return nil
	}
	m := u.core.manager
	if err := u.calls.cleanupRaw(m); err != nil {
		return err
	}
	// Draw and iterator copies may still borrow this updater's output locals.
	if u.interval != nil {
		if err := u.interval.Close(); err != nil {
			return err
		}
		u.interval = nil
	}
	if u.column != nil {
		if err := u.column.Close(); err != nil {
			return err
		}
		u.column = nil
	}
	if u.coefficient != nil {
		if err := u.coefficient.Close(); err != nil {
			return err
		}
		u.coefficient = nil
	}
	if u.work != nil {
		if err := u.work.Close(); err != nil {
			return err
		}
		u.work = nil
	}
	for _, entry := range u.owned {
		if err := entry.value.Del(m); err != nil {
			return err
		}
	}
	u.owned = nil
	clear(u.values)
	clear(u.vars)
	u.distinct = 0
	return u.calls.cleanupRoots(m)
}

// FillVarSetForRandomUpdate appends the source internal columns without
// deduplication. Term coefficient order follows the retained term dictionary.
func (s *LARSolver) FillVarSetForRandomUpdate(vars []uint32, out *[]uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	for _, j := range vars {
		if j < LARTermStartIndex {
			*out = append(*out, j)
			continue
		}
		k := j - LARTermStartIndex
		if uint64(k) >= uint64(len(s.Terms)) || s.Terms[k] == nil {
			return ErrBounds
		}
		for column := range s.Terms[k].Coefficients().All() {
			*out = append(*out, column)
		}
	}
	return nil
}
func (s *LARSolver) RandomUpdate(vars []uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	var columns []uint32
	if err = s.FillVarSetForRandomUpdate(vars, &columns); err != nil {
		return err
	}
	s.randomUpdater, err = NewRandomUpdater(s.Core, columns)
	if err != nil {
		return err
	}
	if err = s.randomUpdater.Update(); err != nil {
		return err
	}
	if err = s.randomUpdater.Close(); err != nil {
		return err
	}
	s.randomUpdater = nil
	return nil
}
