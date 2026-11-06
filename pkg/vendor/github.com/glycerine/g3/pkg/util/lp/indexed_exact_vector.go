package lp

// Source: src/util/lp/indexed_vector.h/.hpp, live mpq and numeric_pair<mpq> instances.

import "github.com/glycerine/g3/pkg/util/numeral"

// IndexedRationalVector owns initialized exact dense slots and a stable sparse
// index in a borrowed manager. Data and Index are public for retained matrix
// algorithms. Never shallow-copy an owning slot or change Data's length by hand;
// use Clone/Assign/Resize and close the vector before its manager. After a
// numerical failure Close owns all completed and partial vector-call roots,
// including integer GCD/division invocations reached by rational normalization.
type IndexedRationalVector struct {
	Data    []numeral.Rational
	Index   []uint32
	manager *numeral.RationalManager
	calls   exactEtaCalls
}

func NewIndexedRationalVector(m *numeral.RationalManager, size uint32) *IndexedRationalVector {
	v := &IndexedRationalVector{manager: m, Data: make([]numeral.Rational, size)}
	for i := range v.Data {
		v.Data[i] = numeral.NewRational()
	}
	return v
}
func (v *IndexedRationalVector) DataSize() uint32 { return uint32(len(v.Data)) }
func (v *IndexedRationalVector) Size() uint32     { return uint32(len(v.Index)) }
func (v *IndexedRationalVector) IsOK() bool       { return true }
func (v *IndexedRationalVector) resetSlot(loc *numeral.Rational) error {
	z := numeral.NewRational()
	return v.manager.Copy(loc, &z)
}
func (v *IndexedRationalVector) copySlot(dst, src *numeral.Rational) (err error) {
	before, err := v.calls.begin(v.manager)
	if err != nil {
		return err
	}
	defer func() { err = v.calls.after(v.manager, before, err) }()
	return v.manager.Copy(dst, src)
}
func (v *IndexedRationalVector) Clear() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		if err := v.resetSlot(&v.Data[i]); err != nil {
			return err
		}
	}
	v.Index = v.Index[:0]
	return nil
}
func (v *IndexedRationalVector) ClearAll() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	for i := len(v.Data); i > 0; {
		i--
		if err := v.resetSlot(&v.Data[i]); err != nil {
			return err
		}
	}
	v.Index = v.Index[:0]
	return nil
}
func (v *IndexedRationalVector) resizeData(size uint32) error {
	// The source destroys removed coefficients from the new end forward,
	// publishing the shorter extent only after all destructors finish.
	if uint64(size) < uint64(len(v.Data)) {
		for i := size; uint64(i) < uint64(len(v.Data)); i++ {
			if err := v.manager.Del(&v.Data[i]); err != nil {
				return err
			}
			v.Data[i] = numeral.NewRational()
		}
		v.Data = v.Data[:size]
	}
	for uint64(len(v.Data)) < uint64(size) {
		v.Data = append(v.Data, numeral.NewRational())
	}
	return nil
}
func (v *IndexedRationalVector) Resize(size uint32) error {
	if err := v.Clear(); err != nil {
		return err
	}
	return v.resizeData(size)
}
func (v *IndexedRationalVector) Assign(y *IndexedRationalVector) error {
	if v.manager != y.manager {
		return ErrExactVectorOwner
	}
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	// Clear old indexed data, preserving the source self-assignment behavior.
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		if err := v.resetSlot(&v.Data[i]); err != nil {
			return err
		}
	}
	v.Index = append(v.Index[:0], y.Index...)
	if err := v.resizeData(y.DataSize()); err != nil {
		return err
	}
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		if err := v.copySlot(&v.Data[i], &y.Data[i]); err != nil {
			return err
		}
	}
	return nil
}

// Clone is the source copy constructor, which copies ALL dense slots, including
// unindexed values. A non-nil result owns its partial construction on error.
func (v *IndexedRationalVector) Clone() (*IndexedRationalVector, error) {
	out := NewIndexedRationalVector(v.manager, v.DataSize())
	for i := range v.Data {
		if err := out.copySlot(&out.Data[i], &v.Data[i]); err != nil {
			return out, err
		}
	}
	out.Index = append(out.Index, v.Index...)
	return out, nil
}
func (v *IndexedRationalVector) SetValue(value *numeral.Rational, index uint32) error {
	if uint64(index) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	if err := v.copySlot(&v.Data[index], value); err != nil {
		return err
	}
	for _, i := range v.Index {
		if i == index {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
			break
		}
	}
	v.Index = append(v.Index, index)
	return nil
}
func (v *IndexedRationalVector) EraseFromIndex(j uint32) {
	for k, i := range v.Index {
		if i == j {
			copy(v.Index[k:], v.Index[k+1:])
			v.Index = v.Index[:len(v.Index)-1]
			return
		}
	}
}

func (v *IndexedRationalVector) Add(j uint32, value *numeral.Rational) (err error) {
	if uint64(j) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	before, err := v.calls.begin(v.manager)
	if err != nil {
		return err
	}
	defer func() { err = v.calls.after(v.manager, before, err) }()
	loc := &v.Data[j]
	wasZero, err := v.manager.IsZero(loc)
	if err != nil {
		return err
	}
	if err := v.manager.Add(loc, value, loc); err != nil {
		return err
	}
	isZero, err := v.manager.IsZero(loc)
	if err != nil {
		return err
	}
	// Exact traits ignore epsilon. Drop's explicit zero assignment remains.
	if isZero {

		if !wasZero {
			v.EraseFromIndex(j)
		}
	} else if wasZero {
		v.Index = append(v.Index, j)
	}
	return nil
}

func (v *IndexedRationalVector) AddWithDropTolerance(j uint32, value *numeral.Rational) (err error) {
	if uint64(j) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	before, err := v.calls.begin(v.manager)
	if err != nil {
		return err
	}
	defer func() { err = v.calls.after(v.manager, before, err) }()
	loc := &v.Data[j]
	wasZero, err := v.manager.IsZero(loc)
	if err != nil {
		return err
	}
	if err := v.manager.Add(loc, value, loc); err != nil {
		return err
	}
	isZero, err := v.manager.IsZero(loc)
	if err != nil {
		return err
	}
	// Exact traits ignore epsilon. Drop's explicit zero assignment remains.
	if isZero {
		if err := v.resetSlot(loc); err != nil {
			return err
		}
		if !wasZero {
			v.EraseFromIndex(j)
		}
	} else if wasZero {
		v.Index = append(v.Index, j)
	}
	return nil
}

func (v *IndexedRationalVector) CleanUp() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	var next []uint32
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		loc := &v.Data[i]
		zero, err := v.manager.IsZero(loc)
		if err != nil {
			return err
		}
		if !zero {
			next = append(next, i)
		}
	}
	v.Index = next
	return nil
}
func (v *IndexedRationalVector) RestoreIndex() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	v.Index = v.Index[:0]
	for i := range v.Data {
		loc := &v.Data[i]
		zero, err := v.manager.IsZero(loc)
		if err != nil {
			return err
		}
		if zero {
			if err := v.resetSlot(loc); err != nil {
				return err
			}
		} else {
			v.Index = append(v.Index, uint32(i))
		}
	}
	return nil
}
func (v *IndexedRationalVector) Close() error {
	if err := v.calls.cleanupRaw(v.manager); err != nil {
		return err
	}
	for len(v.Data) > 0 {
		last := len(v.Data) - 1
		loc := &v.Data[last]
		if err := v.manager.Del(loc); err != nil {
			return err
		}
		v.Data = v.Data[:last]
	}
	if err := v.calls.cleanupRoots(v.manager); err != nil {
		return err
	}
	v.Data = nil
	v.Index = nil
	return nil
}

// CloseForward destroys a retained source vector's dense elements in the
// forward order used by util/vector.h. Close remains the reverse lexical
// cleanup used when this holder stores C++ expression temporaries.
func (v *IndexedRationalVector) CloseForward() error {
	if err := v.calls.cleanupRaw(v.manager); err != nil {
		return err
	}
	for i := range v.Data {
		if err := v.manager.Del(&v.Data[i]); err != nil {
			return err
		}
		v.Data[i] = numeral.NewRational()
	}
	if err := v.calls.cleanupRoots(v.manager); err != nil {
		return err
	}
	v.Data = nil
	v.Index = nil
	return nil
}

// IndexedPairVector owns initialized exact dense slots and a stable sparse
// index in a borrowed manager. Data and Index are public for retained matrix
// algorithms. Never shallow-copy an owning slot or change Data's length by hand;
// use Clone/Assign/Resize and close the vector before its manager. After a
// numerical failure Close owns all completed and partial vector-call roots,
// including integer GCD/division invocations reached by rational normalization.
type IndexedPairVector struct {
	Data    []RationalPair
	Index   []uint32
	manager *numeral.RationalManager
	calls   exactEtaCalls
}

func NewIndexedPairVector(m *numeral.RationalManager, size uint32) *IndexedPairVector {
	v := &IndexedPairVector{manager: m, Data: make([]RationalPair, size)}
	for i := range v.Data {
		v.Data[i] = NewRationalPair()
	}
	return v
}
func (v *IndexedPairVector) DataSize() uint32 { return uint32(len(v.Data)) }
func (v *IndexedPairVector) Size() uint32     { return uint32(len(v.Index)) }
func (v *IndexedPairVector) IsOK() bool       { return true }
func (v *IndexedPairVector) resetSlot(loc *RationalPair) error {
	z := NewRationalPair()
	return loc.Copy(v.manager, &z)
}
func (v *IndexedPairVector) copySlot(dst, src *RationalPair) (err error) {
	before, err := v.calls.begin(v.manager)
	if err != nil {
		return err
	}
	defer func() { err = v.calls.after(v.manager, before, err) }()
	return dst.Copy(v.manager, src)
}
func (v *IndexedPairVector) Clear() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		if err := v.resetSlot(&v.Data[i]); err != nil {
			return err
		}
	}
	v.Index = v.Index[:0]
	return nil
}
func (v *IndexedPairVector) ClearAll() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	for i := len(v.Data); i > 0; {
		i--
		if err := v.resetSlot(&v.Data[i]); err != nil {
			return err
		}
	}
	v.Index = v.Index[:0]
	return nil
}
func (v *IndexedPairVector) resizeData(size uint32) error {
	if uint64(size) < uint64(len(v.Data)) {
		for i := size; uint64(i) < uint64(len(v.Data)); i++ {
			if err := v.Data[i].Del(v.manager); err != nil {
				return err
			}
			v.Data[i] = NewRationalPair()
		}
		v.Data = v.Data[:size]
	}
	for uint64(len(v.Data)) < uint64(size) {
		v.Data = append(v.Data, NewRationalPair())
	}
	return nil
}
func (v *IndexedPairVector) Resize(size uint32) error {
	if err := v.Clear(); err != nil {
		return err
	}
	return v.resizeData(size)
}
func (v *IndexedPairVector) Assign(y *IndexedPairVector) error {
	if v.manager != y.manager {
		return ErrExactVectorOwner
	}
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	// Clear old indexed data, preserving the source self-assignment behavior.
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		if err := v.resetSlot(&v.Data[i]); err != nil {
			return err
		}
	}
	v.Index = append(v.Index[:0], y.Index...)
	if err := v.resizeData(y.DataSize()); err != nil {
		return err
	}
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		if err := v.copySlot(&v.Data[i], &y.Data[i]); err != nil {
			return err
		}
	}
	return nil
}

// Clone is the source copy constructor, which copies ALL dense slots, including
// unindexed values. A non-nil result owns its partial construction on error.
func (v *IndexedPairVector) Clone() (*IndexedPairVector, error) {
	out := NewIndexedPairVector(v.manager, v.DataSize())
	for i := range v.Data {
		if err := out.copySlot(&out.Data[i], &v.Data[i]); err != nil {
			return out, err
		}
	}
	out.Index = append(out.Index, v.Index...)
	return out, nil
}
func (v *IndexedPairVector) SetValue(value *RationalPair, index uint32) error {
	if uint64(index) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	if err := v.copySlot(&v.Data[index], value); err != nil {
		return err
	}
	for _, i := range v.Index {
		if i == index {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
			break
		}
	}
	v.Index = append(v.Index, index)
	return nil
}
func (v *IndexedPairVector) EraseFromIndex(j uint32) {
	for k, i := range v.Index {
		if i == j {
			copy(v.Index[k:], v.Index[k+1:])
			v.Index = v.Index[:len(v.Index)-1]
			return
		}
	}
}

func (v *IndexedPairVector) Add(j uint32, value *RationalPair) (err error) {
	if uint64(j) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	before, err := v.calls.begin(v.manager)
	if err != nil {
		return err
	}
	defer func() { err = v.calls.after(v.manager, before, err) }()
	loc := &v.Data[j]
	wasZero, err := loc.IsZero(v.manager)
	if err != nil {
		return err
	}
	if err := loc.Add(v.manager, loc, value); err != nil {
		return err
	}
	isZero, err := loc.IsZero(v.manager)
	if err != nil {
		return err
	}
	// Exact traits ignore epsilon. Drop's explicit zero assignment remains.
	if isZero {

		if !wasZero {
			v.EraseFromIndex(j)
		}
	} else if wasZero {
		v.Index = append(v.Index, j)
	}
	return nil
}

func (v *IndexedPairVector) AddWithDropTolerance(j uint32, value *RationalPair) (err error) {
	if uint64(j) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	before, err := v.calls.begin(v.manager)
	if err != nil {
		return err
	}
	defer func() { err = v.calls.after(v.manager, before, err) }()
	loc := &v.Data[j]
	wasZero, err := loc.IsZero(v.manager)
	if err != nil {
		return err
	}
	if err := loc.Add(v.manager, loc, value); err != nil {
		return err
	}
	isZero, err := loc.IsZero(v.manager)
	if err != nil {
		return err
	}
	// Exact traits ignore epsilon. Drop's explicit zero assignment remains.
	if isZero {
		if err := v.resetSlot(loc); err != nil {
			return err
		}
		if !wasZero {
			v.EraseFromIndex(j)
		}
	} else if wasZero {
		v.Index = append(v.Index, j)
	}
	return nil
}

func (v *IndexedPairVector) CleanUp() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	var next []uint32
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		loc := &v.Data[i]
		zero, err := loc.IsZero(v.manager)
		if err != nil {
			return err
		}
		if !zero {
			next = append(next, i)
		}
	}
	v.Index = next
	return nil
}
func (v *IndexedPairVector) RestoreIndex() error {
	if v.calls.pending() {
		return ErrExactVectorPending
	}
	v.Index = v.Index[:0]
	for i := range v.Data {
		loc := &v.Data[i]
		zero, err := loc.IsZero(v.manager)
		if err != nil {
			return err
		}
		if zero {
			if err := v.resetSlot(loc); err != nil {
				return err
			}
		} else {
			v.Index = append(v.Index, uint32(i))
		}
	}
	return nil
}
func (v *IndexedPairVector) Close() error {
	if err := v.calls.cleanupRaw(v.manager); err != nil {
		return err
	}
	for len(v.Data) > 0 {
		last := len(v.Data) - 1
		loc := &v.Data[last]
		if err := loc.Del(v.manager); err != nil {
			return err
		}
		v.Data = v.Data[:last]
	}
	if err := v.calls.cleanupRoots(v.manager); err != nil {
		return err
	}
	v.Data = nil
	v.Index = nil
	return nil
}

// CloseForward destroys a retained source vector's pair elements in source
// forward order. Pair.Del still destroys each element's Y before X.
func (v *IndexedPairVector) CloseForward() error {
	if err := v.calls.cleanupRaw(v.manager); err != nil {
		return err
	}
	for i := range v.Data {
		if err := v.Data[i].Del(v.manager); err != nil {
			return err
		}
		v.Data[i] = NewRationalPair()
	}
	if err := v.calls.cleanupRoots(v.manager); err != nil {
		return err
	}
	v.Data = nil
	v.Index = nil
	return nil
}
