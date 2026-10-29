package lp

// Source: src/util/lp/sparse_vector.h, retained mpq coefficient storage.

import "github.com/glycerine/g3/pkg/util/numeral"

type RationalSparseEntry struct {
	Index uint32
	Value numeral.Rational
}

// SparseRationalVector owns its coefficient values in a borrowed manager.
// Entries retain insertion order, duplicates and explicit zeros. Do not copy
// the owner or its owning values; close it before closing the manager.
type SparseRationalVector struct {
	Data         []RationalSparseEntry
	manager      *numeral.RationalManager
	calls        exactEtaCalls
	appendValues *IndexedRationalVector
}

func NewSparseRationalVector(m *numeral.RationalManager) *SparseRationalVector {
	return &SparseRationalVector{manager: m}
}
func (v *SparseRationalVector) Size() uint32 { return uint32(len(v.Data)) }
func (v *SparseRationalVector) ready() error {
	if v.appendValues != nil || v.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}

func (v *SparseRationalVector) Append(index uint32, value *numeral.Rational) error {
	if err := v.ready(); err != nil {
		return err
	}
	// Source value parameter, pair temporary, then stored coefficient. These
	// are concrete numeral lifetimes, independent of native slice capacity.
	v.appendValues = NewIndexedRationalVector(v.manager, 3)
	w := v.appendValues
	if err := w.copySlot(&w.Data[0], value); err != nil {
		return err
	}
	if err := w.copySlot(&w.Data[1], &w.Data[0]); err != nil {
		return err
	}
	if err := w.copySlot(&w.Data[2], &w.Data[1]); err != nil {
		return err
	}
	v.Data = append(v.Data, RationalSparseEntry{Index: index, Value: w.Data[2]})
	w.Data[2] = numeral.NewRational() // Transfer the stored value's ownership.
	if err := w.Close(); err != nil {
		return err
	}
	v.appendValues = nil
	return nil
}

// At copies the first matching coefficient into caller-owned initialized out,
// or zero if absent. It implements the source debug lookup's value result.
func (v *SparseRationalVector) At(index uint32, out *numeral.Rational) (err error) {
	if err = v.ready(); err != nil {
		return err
	}
	b, err := v.calls.begin(v.manager)
	if err != nil {
		return err
	}
	defer func() { err = v.calls.after(v.manager, b, err) }()
	for i := range v.Data {
		if v.Data[i].Index == index {
			return v.manager.Copy(out, &v.Data[i].Value)
		}
	}
	z := numeral.NewRational()
	return v.manager.Copy(out, &z)
}

func (v *SparseRationalVector) Divide(value *numeral.Rational) (err error) {
	if err = v.ready(); err != nil {
		return err
	}
	// Precise numeric traits ignore the floating tolerance and test only zero.
	zero, e := v.manager.IsZero(value)
	if e != nil {
		return e
	}
	if zero {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := range v.Data {
		b, e := v.calls.begin(v.manager)
		if e != nil {
			return e
		}
		loc := &v.Data[i].Value
		e = v.manager.Div(loc, value, loc)
		if err = v.calls.after(v.manager, b, e); err != nil {
			return err
		}
	}
	return nil
}

// closeCall unwinds only this vector's interrupted call temporaries. A
// containing eta calls it before destroying its own lexical values and members;
// the vector's stored coefficients remain owned until Close.
func (v *SparseRationalVector) closeCall() error {
	if err := v.calls.cleanupRaw(v.manager); err != nil {
		return err
	}
	if v.appendValues != nil {
		if err := v.appendValues.Close(); err != nil {
			return err
		}
		v.appendValues = nil
	}
	return nil
}

func (v *SparseRationalVector) Close() error {
	if err := v.closeCall(); err != nil {
		return err
	}
	for len(v.Data) > 0 {
		i := len(v.Data) - 1
		if err := v.manager.Del(&v.Data[i].Value); err != nil {
			return err
		}
		v.Data = v.Data[:i]
	}
	return v.calls.cleanupRoots(v.manager)
}
