package lp

// Source: src/util/lp/init_lar_solver.h:308-603 and lar_solver.h witness
// updates. This is the concrete bound state of the retained LAR solver;
// matrix/basis construction, constraint scopes and optimization remain separate.

import "github.com/glycerine/g3/pkg/util/numeral"

// LARBounds owns the exact bound vectors and journals, column types, witnesses
// and changed-column set. The manager is borrowed and must outlive Close.
// Do not copy. Bounds returned by Low/Upper.At are read-only borrowed values.
type LARBounds struct {
	Types            StackedColumnVector
	Low, Upper       *StackedPairVector
	Witnesses        StackedLARWitnessVector
	ChangedColumns   IntSet
	Status           StackedStatus
	InfeasibleColumn StackedInt32
	manager          *numeral.RationalManager
	calls            exactEtaCalls
	work             *IndexedPairVector
}

func NewLARBounds(m *numeral.RationalManager) *LARBounds {
	return &LARBounds{Low: NewStackedPairVector(m), Upper: NewStackedPairVector(m), Status: NewStackedStatus(StatusOptimal), InfeasibleColumn: NewStackedInt32(-1), manager: m}
}

func (b *LARBounds) ready() error {
	if b.work != nil || b.calls.pending() || b.Low.unfinished() || b.Upper.unfinished() {
		return ErrExactVectorPending
	}
	return nil
}

// AppendFreeColumn initializes only the bound fields of a new source column.
// Full external-variable/matrix/basis registration is not performed here.
func (b *LARBounds) AppendFreeColumn(row uint32) error {
	if err := b.ready(); err != nil {
		return err
	}
	b.Witnesses.Append(NewLARBoundWitnesses(row))
	b.Types.Append(ColumnFree)
	if err := b.ChangedColumns.IncreaseSizeByOne(); err != nil {
		return err
	}
	if err := b.Low.IncreaseSizeByOne(); err != nil {
		return err
	}
	return b.Upper.IncreaseSizeByOne()
}

func (b *LARBounds) setWitness(j, ci uint32, low bool) error {
	w, err := b.Witnesses.At(j)
	if err != nil {
		return err
	}
	if low {
		w.LowBoundWitness = ci
	} else {
		w.UpperBoundWitness = ci
	}
	return b.Witnesses.Set(j, w)
}

// makeBound preserves numeric_pair's by-value constructor parameters and
// owning result. Its parameter copy dies before the source bound assignment.
func (b *LARBounds) makeBound(rhs *numeral.Rational, y int32) (*RationalPair, error) {
	b.work = NewIndexedPairVector(b.manager, 2)
	parameter, result := &b.work.Data[0], &b.work.Data[1]
	if err := b.manager.Copy(&parameter.X, rhs); err != nil {
		return nil, err
	}
	if err := b.manager.SetInt32(&parameter.Y, y); err != nil {
		return nil, err
	}
	if err := result.Copy(b.manager, parameter); err != nil {
		return nil, err
	}
	if err := parameter.Del(b.manager); err != nil {
		return nil, err
	}
	return result, nil
}

// Update implements update_column_type_and_bound and its five source branches.
// Numerical failures leave complete and partial state owned for explicit Close.
func (b *LARBounds) Update(j uint32, kind LConstraintKind, rhs *numeral.Rational, ci uint32) (err error) {
	if err = b.ready(); err != nil {
		return err
	}
	before, err := b.calls.begin(b.manager)
	if err != nil {
		return err
	}
	defer func() {
		err = b.calls.after(b.manager, before, err)
		if err == nil && b.work != nil {
			err = b.work.Close()
			if err == nil {
				b.work = nil
			}
		}
	}()
	t, err := b.Types.At(j)
	if err != nil {
		return err
	}
	switch t {
	case ColumnFree:
		return b.updateFree(j, kind, rhs, ci)
	case ColumnUpperBound:
		return b.updateUpper(j, kind, rhs, ci)
	case ColumnLowBound:
		return b.updateLow(j, kind, rhs, ci)
	case ColumnBoxed:
		return b.updateBoxed(j, kind, rhs, ci)
	case ColumnFixed:
		return b.updateFixed(j, kind, rhs, ci)
	default:
		return sourceAssertionFailure()
	}
}

func larBoundEpsilon(kind LConstraintKind) int32 {
	if kind == LConstraintLT {
		return -1
	}
	if kind == LConstraintGT {
		return 1
	}
	return 0
}

func (b *LARBounds) updateFree(j uint32, kind LConstraintKind, rhs *numeral.Rational, ci uint32) error {
	switch kind {
	case LConstraintLT, LConstraintLE:
		if err := b.Types.Set(j, ColumnUpperBound); err != nil {
			return err
		}
		if sourceDebugEnabled && b.Upper.Size() <= j {
			return ErrAssertion
		}
		v, err := b.makeBound(rhs, larBoundEpsilon(kind))
		if err != nil {
			return err
		}
		if err = b.Upper.Set(j, v); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, false); err != nil {
			return err
		}
	case LConstraintGT, LConstraintGE:
		if err := b.Types.Set(j, ColumnLowBound); err != nil {
			return err
		}
		if sourceDebugEnabled && b.Upper.Size() <= j {
			return ErrAssertion
		}
		v, err := b.makeBound(rhs, larBoundEpsilon(kind))
		if err != nil {
			return err
		}
		if err = b.Low.Set(j, v); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, true); err != nil {
			return err
		}
	case LConstraintEQ:
		if err := b.Types.Set(j, ColumnFixed); err != nil {
			return err
		}
		v, err := b.makeBound(rhs, 0)
		if err != nil {
			return err
		}
		if err = b.Upper.Set(j, v); err != nil {
			return err
		}
		if err = b.Low.Set(j, v); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, false); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, true); err != nil {
			return err
		}
	default:
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return b.ChangedColumns.Insert(j)
}

func (b *LARBounds) updateUpper(j uint32, kind LConstraintKind, rhs *numeral.Rational, ci uint32) error {
	if kind != LConstraintLT && kind != LConstraintLE && kind != LConstraintGT && kind != LConstraintGE && kind != LConstraintEQ {
		return sourceAssertionFailure()
	}
	if kind == LConstraintGT || kind == LConstraintGE {
		if err := b.Types.Set(j, ColumnBoxed); err != nil {
			return err
		}
	}
	v, err := b.makeBound(rhs, larBoundEpsilon(kind))
	if err != nil {
		return err
	}
	upper, err := b.Upper.At(j)
	if err != nil {
		return err
	}
	switch kind {
	case LConstraintLT, LConstraintLE:
		less, e := v.Less(b.manager, upper)
		if e != nil {
			return e
		}
		if less {
			if err = b.Upper.Set(j, v); err != nil {
				return err
			}
			if err = b.setWitness(j, ci, false); err != nil {
				return err
			}
			return b.ChangedColumns.Insert(j)
		}
	case LConstraintGT, LConstraintGE:
		if err = b.Low.Set(j, v); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, true); err != nil {
			return err
		}
		if err = b.ChangedColumns.Insert(j); err != nil {
			return err
		}
		greater, e := v.Greater(b.manager, upper)
		if e != nil {
			return e
		}
		if greater {
			b.Status.Value = StatusInfeasible
			b.InfeasibleColumn.Value = int32(j)
		} else {
			low, e := b.Low.At(j)
			if e != nil {
				return e
			}
			less, e := low.Less(b.manager, upper)
			if e != nil {
				return e
			}
			t := ColumnFixed
			if less {
				t = ColumnBoxed
			}
			return b.Types.Set(j, t)
		}
	case LConstraintEQ:
		greater, e := v.Greater(b.manager, upper)
		if e != nil {
			return e
		}
		if greater {
			b.Status.Value = StatusInfeasible
			if err = b.setWitness(j, ci, true); err != nil {
				return err
			}
			b.InfeasibleColumn.Value = int32(j)
		} else {
			if err = b.Upper.Set(j, v); err != nil {
				return err
			}
			if err = b.Low.Set(j, v); err != nil {
				return err
			}
			if err = b.ChangedColumns.Insert(j); err != nil {
				return err
			}
			if err = b.setWitness(j, ci, true); err != nil {
				return err
			}
			if err = b.setWitness(j, ci, false); err != nil {
				return err
			}
			return b.Types.Set(j, ColumnFixed)
		}
	}
	return nil
}

func (b *LARBounds) updateLow(j uint32, kind LConstraintKind, rhs *numeral.Rational, ci uint32) error {
	if kind != LConstraintLT && kind != LConstraintLE && kind != LConstraintGT && kind != LConstraintGE && kind != LConstraintEQ {
		return sourceAssertionFailure()
	}
	v, err := b.makeBound(rhs, larBoundEpsilon(kind))
	if err != nil {
		return err
	}
	low, err := b.Low.At(j)
	if err != nil {
		return err
	}
	switch kind {
	case LConstraintLT, LConstraintLE:
		if err = b.Upper.Set(j, v); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, false); err != nil {
			return err
		}
		if err = b.ChangedColumns.Insert(j); err != nil {
			return err
		}
		less, e := v.Less(b.manager, low)
		if e != nil {
			return e
		}
		if less {
			b.Status.Value = StatusInfeasible
			b.InfeasibleColumn.Value = int32(j)
		} else {
			upper, e := b.Upper.At(j)
			if e != nil {
				return e
			}
			less, e := low.Less(b.manager, upper)
			if e != nil {
				return e
			}
			t := ColumnFixed
			if less {
				t = ColumnBoxed
			}
			return b.Types.Set(j, t)
		}
	case LConstraintGT, LConstraintGE:
		greater, e := v.Greater(b.manager, low)
		if e != nil {
			return e
		}
		if greater {
			if err = b.Low.Set(j, v); err != nil {
				return err
			}
			if err = b.ChangedColumns.Insert(j); err != nil {
				return err
			}
			return b.setWitness(j, ci, true)
		}
	case LConstraintEQ:
		less, e := v.Less(b.manager, low)
		if e != nil {
			return e
		}
		if less {
			b.Status.Value = StatusInfeasible
			b.InfeasibleColumn.Value = int32(j)
			if err = b.setWitness(j, ci, false); err != nil {
				return err
			}
		} else {
			if err = b.Upper.Set(j, v); err != nil {
				return err
			}
			if err = b.Low.Set(j, v); err != nil {
				return err
			}
			if err = b.setWitness(j, ci, true); err != nil {
				return err
			}
			if err = b.setWitness(j, ci, false); err != nil {
				return err
			}
			if err = b.Types.Set(j, ColumnFixed); err != nil {
				return err
			}
		}
		return b.ChangedColumns.Insert(j)
	}
	return nil
}

func (b *LARBounds) updateBoxed(j uint32, kind LConstraintKind, rhs *numeral.Rational, ci uint32) error {
	low, err := b.Low.At(j)
	if err != nil {
		return err
	}
	upper, err := b.Upper.At(j)
	if err != nil {
		return err
	}
	if sourceDebugEnabled && b.Status.Value != StatusInfeasible {
		less, e := low.Less(b.manager, upper)
		if e != nil {
			return e
		}
		if !less {
			return ErrAssertion
		}
	}
	if kind != LConstraintLT && kind != LConstraintLE && kind != LConstraintGT && kind != LConstraintGE && kind != LConstraintEQ {
		return sourceAssertionFailure()
	}
	v, err := b.makeBound(rhs, larBoundEpsilon(kind))
	if err != nil {
		return err
	}
	switch kind {
	case LConstraintLT, LConstraintLE:
		less, e := v.Less(b.manager, upper)
		if e != nil {
			return e
		}
		if less {
			if err = b.Upper.Set(j, v); err != nil {
				return err
			}
			if err = b.setWitness(j, ci, false); err != nil {
				return err
			}
			if err = b.ChangedColumns.Insert(j); err != nil {
				return err
			}
		}
		less, e = v.Less(b.manager, low)
		if e != nil {
			return e
		}
		if less {
			b.Status.Value = StatusInfeasible
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
			b.InfeasibleColumn.Value = int32(j)
		} else {
			equal, e := low.Equal(b.manager, upper)
			if e != nil {
				return e
			}
			if equal {
				return b.Types.Set(j, ColumnFixed)
			}
		}
	case LConstraintGT, LConstraintGE:
		greater, e := v.Greater(b.manager, low)
		if e != nil {
			return e
		}
		if greater {
			if err = b.Low.Set(j, v); err != nil {
				return err
			}
			if err = b.ChangedColumns.Insert(j); err != nil {
				return err
			}
			if err = b.setWitness(j, ci, true); err != nil {
				return err
			}
		}
		greater, e = v.Greater(b.manager, upper)
		if e != nil {
			return e
		}
		if greater {
			b.Status.Value = StatusInfeasible
			b.InfeasibleColumn.Value = int32(j)
		} else {
			equal, e := v.Equal(b.manager, upper)
			if e != nil {
				return e
			}
			if equal {
				return b.Types.Set(j, ColumnFixed)
			}
		}
	case LConstraintEQ:
		less, e := v.Less(b.manager, low)
		if e != nil {
			return e
		}
		if less {
			b.Status.Value = StatusInfeasible
			b.InfeasibleColumn.Value = int32(j)
			return b.setWitness(j, ci, false)
		}
		greater, e := v.Greater(b.manager, upper)
		if e != nil {
			return e
		}
		if greater {
			b.Status.Value = StatusInfeasible
			b.InfeasibleColumn.Value = int32(j)
			return b.setWitness(j, ci, true)
		}
		if err = b.Upper.Set(j, v); err != nil {
			return err
		}
		if err = b.Low.Set(j, v); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, true); err != nil {
			return err
		}
		if err = b.setWitness(j, ci, false); err != nil {
			return err
		}
		if err = b.Types.Set(j, ColumnFixed); err != nil {
			return err
		}
		return b.ChangedColumns.Insert(j)
	}
	return nil
}

func (b *LARBounds) updateFixed(j uint32, kind LConstraintKind, rhs *numeral.Rational, ci uint32) error {
	low, err := b.Low.At(j)
	if err != nil {
		return err
	}
	upper, err := b.Upper.At(j)
	if err != nil {
		return err
	}
	if sourceDebugEnabled && b.Status.Value != StatusInfeasible {
		equal, e := low.Equal(b.manager, upper)
		if e != nil {
			return e
		}
		if !equal {
			return ErrAssertion
		}
		lz, e := b.manager.IsZero(&low.Y)
		if e != nil {
			return e
		}
		if !lz {
			return ErrAssertion
		}
		uz, e := b.manager.IsZero(&upper.Y)
		if e != nil {
			return e
		}
		if !uz {
			return ErrAssertion
		}
	}
	v, err := b.makeBound(rhs, 0)
	if err != nil {
		return err
	}
	var conflict, lowerWitness bool
	switch kind {
	case LConstraintLT, LConstraintLE:
		conflict, err = v.Less(b.manager, low)
		if err == nil && !conflict && kind == LConstraintLT {
			conflict, err = v.Equal(b.manager, low)
		}
	case LConstraintGT, LConstraintGE:
		lowerWitness = true
		conflict, err = v.Greater(b.manager, upper)
		if err == nil && !conflict && kind == LConstraintGT {
			conflict, err = upper.Equal(b.manager, v)
		}
	case LConstraintEQ:
		conflict, err = v.Less(b.manager, low)
		if err == nil && !conflict {
			conflict, err = v.Greater(b.manager, upper)
			lowerWitness = true
		}
	default:
		return sourceAssertionFailure()
	}
	if err != nil {
		return err
	}
	if conflict {
		b.Status.Value = StatusInfeasible
		b.InfeasibleColumn.Value = int32(j)
		return b.setWitness(j, ci, lowerWitness)
	}
	return nil
}

func (b *LARBounds) Close() error {
	if err := b.calls.cleanupRaw(b.manager); err != nil {
		return err
	}
	if b.work != nil {
		if err := b.work.Close(); err != nil {
			return err
		}
		b.work = nil
	}
	// Core fields declare low before upper; destroy them in reverse order.
	if err := b.Upper.Close(); err != nil {
		return err
	}
	if err := b.Low.Close(); err != nil {
		return err
	}
	return b.calls.cleanupRoots(b.manager)
}
