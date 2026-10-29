package lp

// Source: src/util/lp/lar_term.h, retained exact LAR terms.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/insdict"
)

var ErrLARTermPivotErased = errors.New("lp: substitution erased its live pivot coefficient")

// LARTerm owns coefficients and a constant in a borrowed numeral manager.
// Dict supplies stable insertion order instead of C++ hash bucket order.
// Values returned by Coefficients are borrowed; removal invalidates that borrow.
// Do not copy the owner or owning numerals. Close owns stopped lexical calls.
type LARTerm struct {
	Value        numeral.Rational
	coefficients *insdict.Dict[uint32, *numeral.Rational]
	manager      *numeral.RationalManager
	calls        exactEtaCalls
	work         *IndexedRationalVector
}

func NewLARTerm(m *numeral.RationalManager) *LARTerm {
	return &LARTerm{Value: numeral.NewRational(), coefficients: insdict.NewDict[uint32, *numeral.Rational](), manager: m}
}

// A nonnil result owns partial construction on error, as well as on success.
func NewLARTermFromCoefficients(m *numeral.RationalManager, coefficients []RationalSparseEntry, value *numeral.Rational) (*LARTerm, error) {
	t := NewLARTerm(m)
	if err := t.SetValue(value); err != nil {
		return t, err
	}
	for i := range coefficients {
		if err := t.AddCoefficient(coefficients[i].Index, &coefficients[i].Value); err != nil {
			return t, err
		}
	}
	return t, nil
}
func (t *LARTerm) Size() uint32                                           { return uint32(t.coefficients.Len()) }
func (t *LARTerm) Contains(index uint32) bool                             { _, ok := t.coefficients.Get2(index); return ok }
func (t *LARTerm) Coefficients() *insdict.Dict[uint32, *numeral.Rational] { return t.coefficients }
func (t *LARTerm) ready() error {
	if t.work != nil || t.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (t *LARTerm) beginWork(size uint32) (exactEtaBefore, error) {
	if err := t.ready(); err != nil {
		return exactEtaBefore{}, err
	}
	b, err := t.calls.begin(t.manager)
	if err != nil {
		return b, err
	}
	t.work = NewIndexedRationalVector(t.manager, size)
	return b, nil
}
func (t *LARTerm) finishWork(b exactEtaBefore, err *error) {
	*err = t.calls.after(t.manager, b, *err)
	if *err == nil {
		*err = t.work.Close()
		if *err == nil {
			t.work = nil
		}
	}
}
func (t *LARTerm) SetValue(value *numeral.Rational) (err error) {
	b, err := t.beginWork(0)
	if err != nil {
		return err
	}
	defer t.finishWork(b, &err)
	return t.manager.Copy(&t.Value, value)
}
func (t *LARTerm) addCoefficient(index uint32, value *numeral.Rational, slot int) error {
	if old, ok := t.coefficients.Get2(index); ok {
		if err := t.manager.Add(old, value, old); err != nil {
			return err
		}
		zero, err := t.manager.IsZero(old)
		if err != nil {
			return err
		}
		if zero {
			if err = t.manager.Del(old); err != nil {
				return err
			}
			t.coefficients.Del(index)
		}
		return nil
	}
	stored := &t.work.Data[slot]
	if err := t.manager.Copy(stored, value); err != nil {
		return err
	}
	owned := new(numeral.Rational)
	*owned = *stored
	*stored = numeral.NewRational()
	t.coefficients.Put(index, owned) // A new zero is stored, as in source emplace.
	return nil
}
func (t *LARTerm) AddCoefficient(index uint32, value *numeral.Rational) (err error) {
	b, err := t.beginWork(1)
	if err != nil {
		return err
	}
	defer t.finishWork(b, &err)
	return t.addCoefficient(index, value, 0)
}
func (t *LARTerm) Substitute(index uint32, v *IndexedRationalVector) (err error) {
	if t.manager != v.manager {
		return ErrExactVectorOwner
	}
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	b, err := t.beginWork(4)
	if err != nil {
		return err
	}
	defer t.finishWork(b, &err)
	pivot, ok := t.coefficients.Get2(index)
	if !ok {
		return nil
	}
	negative, tmp, product := &t.work.Data[0], &t.work.Data[1], &t.work.Data[2]
	for _, column := range v.Index {
		current, present := t.coefficients.Get2(index)
		if !present || current != pivot {
			return ErrLARTermPivotErased
		}
		if uint64(column) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		// Unary minus copies the LIVE pivot each time, before the binary product.
		if err = t.manager.Copy(negative, pivot); err != nil {
			return err
		}
		if err = t.manager.Neg(negative); err != nil {
			return err
		}
		if err = etaRationalProduct(t.manager, tmp, product, negative, &v.Data[column]); err != nil {
			return err
		}
		if err = t.addCoefficient(column, product, 3); err != nil {
			return err
		}
		if err = t.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
		if err = t.manager.Del(negative); err != nil {
			return err
		}
		*negative = numeral.NewRational()
	}
	current, present := t.coefficients.Get2(index)
	if !present || current != pivot {
		return ErrLARTermPivotErased
	}
	if err = t.manager.Del(pivot); err != nil {
		return err
	}
	t.coefficients.Del(index)
	return nil
}
func (t *LARTerm) CoefficientsAsVector() (*SparseRationalVector, error) {
	if err := t.ready(); err != nil {
		return nil, err
	}
	out := NewSparseRationalVector(t.manager)
	for index, value := range t.coefficients.All() {
		if err := out.appendTermCoefficient(index, value); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Source make_pair copies the coefficient, then vector publication copies the
// pair's value. Unlike sparse_vector::append, there is no outer value parameter.
func (v *SparseRationalVector) appendTermCoefficient(index uint32, value *numeral.Rational) error {
	if err := v.ready(); err != nil {
		return err
	}
	v.appendValues = NewIndexedRationalVector(v.manager, 2)
	w := v.appendValues
	if err := w.copySlot(&w.Data[0], value); err != nil {
		return err
	}
	if err := w.copySlot(&w.Data[1], &w.Data[0]); err != nil {
		return err
	}
	v.Data = append(v.Data, RationalSparseEntry{index, w.Data[1]})
	w.Data[1] = numeral.NewRational()
	if err := w.Close(); err != nil {
		return err
	}
	v.appendValues = nil
	return nil
}
func (t *LARTerm) Close() error {
	if err := t.calls.cleanupRaw(t.manager); err != nil {
		return err
	}
	if t.work != nil {
		if err := t.work.Close(); err != nil {
			return err
		}
		t.work = nil
	}
	if err := t.manager.Del(&t.Value); err != nil {
		return err
	}
	t.Value = numeral.NewRational()
	for index, value := range t.coefficients.SlowWriteAll() {
		if err := t.manager.Del(value); err != nil {
			return err
		}
		t.coefficients.Del(index)
	}
	return t.calls.cleanupRoots(t.manager)
}
