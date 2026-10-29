package lp

// Source: src/util/lp/lu.h:54-120; src/util/lp/lu.hpp:110-125,926-934.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalDiagonalMatrix owns the retained mpq diagonal tail coefficient.
// The manager and destination vectors are borrowed. Close a failed tail before
// closing its caller-owned partial destination. Do not shallow-copy owners.
type RationalDiagonalMatrix struct {
	index          uint32
	value, inverse numeral.Rational
	manager        *numeral.RationalManager
	calls          exactEtaCalls
	work           *IndexedRationalVector
}

// A partial constructor result must be closed, including on copy refusal.
func NewRationalDiagonalMatrix(manager *numeral.RationalManager, index uint32, value *numeral.Rational) (m *RationalDiagonalMatrix, err error) {
	m = &RationalDiagonalMatrix{index: index, manager: manager, value: numeral.NewRational(), inverse: numeral.NewRational()}
	b, err := m.beginWork(2)
	if err != nil {
		return m, err
	}
	defer m.finishWork(b, &err)
	// Constructor value parameter precedes the stored member copy.
	if err = manager.Copy(&m.work.Data[0], value); err != nil {
		return m, err
	}
	if err = manager.Copy(&m.value, &m.work.Data[0]); err != nil {
		return m, err
	}
	if sourceDebugEnabled {
		one := numeral.NewRational()
		one.Num = numeral.NewInt(1)
		err = etaRationalQuotient(manager, &m.work.Data[1], &m.inverse, &one, &m.value)
	}
	return m, err
}
func (m *RationalDiagonalMatrix) IsDense() bool { return false }
func (m *RationalDiagonalMatrix) Index() uint32 { return m.index }
func (m *RationalDiagonalMatrix) ready() error {
	if m.work != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (m *RationalDiagonalMatrix) beginWork(size uint32) (exactEtaBefore, error) {
	if err := m.ready(); err != nil {
		return exactEtaBefore{}, err
	}
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return b, err
	}
	m.work = NewIndexedRationalVector(m.manager, size)
	return b, nil
}
func (m *RationalDiagonalMatrix) finishWork(b exactEtaBefore, err *error) {
	*err = m.calls.after(m.manager, b, *err)
	if *err == nil {
		*err = m.work.Close()
		if *err == nil {
			m.work = nil
		}
	}
}
func (m *RationalDiagonalMatrix) ApplyLeft(w []numeral.Rational) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.index) >= uint64(len(w)) {
		return ErrBounds
	}
	return m.manager.Div(&w[m.index], &m.value, &w[m.index])
}
func (m *RationalDiagonalMatrix) ApplyRight(w []numeral.Rational) error {
	return m.ApplyLeft(w)
}
func (m *RationalDiagonalMatrix) indexedInput(w *IndexedRationalVector) error {
	if err := m.ready(); err != nil {
		return err
	}
	if m.manager != w.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (m *RationalDiagonalMatrix) ApplyRightIndexed(w *IndexedRationalVector) (err error) {
	if err = m.indexedInput(w); err != nil {
		return err
	}
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.index) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	zero, err := m.manager.IsZero(&w.Data[m.index])
	if err != nil || zero {
		return err
	}
	if err = m.manager.Div(&w.Data[m.index], &m.value, &w.Data[m.index]); err != nil {
		return err
	}
	zero, err = m.manager.IsZero(&w.Data[m.index])
	if err != nil {
		return err
	}
	if zero {
		w.EraseFromIndex(m.index)
		z := numeral.NewRational()
		return m.manager.Copy(&w.Data[m.index], &z)
	}
	return nil
}
func (m *RationalDiagonalMatrix) ApplyLeftToT(w *IndexedRationalVector, _ *Settings) (err error) {
	if err = m.indexedInput(w); err != nil {
		return err
	}
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.index) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	zero, err := m.manager.IsZero(&w.Data[m.index])
	if err != nil || zero {
		return err
	}
	return m.manager.Div(&w.Data[m.index], &m.value, &w.Data[m.index])
}
func (m *RationalDiagonalMatrix) Conjugate(p *Permutation) error {
	if err := m.ready(); err != nil {
		return err
	}
	i, err := p.Reverse(m.index)
	if err != nil {
		return err
	}
	m.index = i
	return nil
}
func (m *RationalDiagonalMatrix) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.work != nil {
		if err := m.work.Close(); err != nil {
			return err
		}
		m.work = nil
	}
	if err := m.manager.Del(&m.inverse); err != nil {
		return err
	}
	m.inverse = numeral.NewRational()
	if err := m.manager.Del(&m.value); err != nil {
		return err
	}
	m.value = numeral.NewRational()
	return m.calls.cleanupRoots(m.manager)
}

// PairDiagonalMatrix shares the mpq coefficient and scalar applications.
type PairDiagonalMatrix struct{ *RationalDiagonalMatrix }

func NewPairDiagonalMatrix(manager *numeral.RationalManager, index uint32, value *numeral.Rational) (*PairDiagonalMatrix, error) {
	m, err := NewRationalDiagonalMatrix(manager, index, value)
	return &PairDiagonalMatrix{m}, err
}
func (m *PairDiagonalMatrix) ApplyLeft(w []RationalPair) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if uint64(m.index) >= uint64(len(w)) {
		return ErrBounds
	}
	return w[m.index].Divide(m.manager, &w[m.index], &m.value)
}

var _ RationalTailMatrix = (*RationalDiagonalMatrix)(nil)
var _ PairTailMatrix = (*PairDiagonalMatrix)(nil)
