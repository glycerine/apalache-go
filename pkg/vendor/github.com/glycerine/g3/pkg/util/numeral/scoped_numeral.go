package numeral

// Concrete translations of util/scoped_numeral.h and scoped_numeral_vector.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrScopedCopy = errors.New("mpnum: shallow scoped owner copy")
var ErrScopedOwner = errors.New("mpnum: invalid scoped numeral owner")

// ScopedInt owns one numeral and borrows its manager. Get returns the source's
// mutable numeral reference; it is valid only until Close and must not be copied
// as an owning Int. Clone is the explicit deep copy operation. All objects are
// owner-confined; close them before their manager. A self pointer rejects Go
// struct copies without unsafe or per-record interfaces.
type ScopedInt struct {
	manager     *IntegerManager
	num         Int
	self        *ScopedInt
	sourceCause error
	closed      bool
}
type ScopedRational struct {
	manager     *RationalManager
	num         Rational
	self        *ScopedRational
	sourceCause error
	closed      bool
}

func NewScopedInt(m *IntegerManager) (*ScopedInt, error) {
	if m == nil || m.Cells == nil {
		return nil, ErrScopedOwner
	}
	if m.closed {
		return nil, arena.ErrClosed
	}
	if e := m.BudgetField.Reserve(uint64(reflect.TypeOf(ScopedInt{}).Size())); e != nil {
		return nil, e
	}
	s := &ScopedInt{manager: m}
	s.self = s
	return s, nil
}
func NewScopedRational(m *RationalManager) (*ScopedRational, error) {
	if m == nil || m.Z == nil {
		return nil, ErrScopedOwner
	}
	if m.closed {
		return nil, arena.ErrClosed
	}
	if e := m.BudgetField.Reserve(uint64(reflect.TypeOf(ScopedRational{}).Size())); e != nil {
		return nil, e
	}
	s := &ScopedRational{manager: m, num: NewRational()}
	s.self = s
	return s, nil
}
func (s *ScopedInt) Get() (*Int, error) {
	if s == nil || s.self != s {
		return nil, ErrScopedCopy
	}
	if s.closed {
		return nil, arena.ErrClosed
	}
	if e := s.manager.check(&s.num); e != nil {
		return nil, e
	}
	return &s.num, nil
}
func (s *ScopedRational) Get() (*Rational, error) {
	if s == nil || s.self != s {
		return nil, ErrScopedCopy
	}
	if s.closed {
		return nil, arena.ErrClosed
	}
	if e := s.manager.check(&s.num); e != nil {
		return nil, e
	}
	return &s.num, nil
}
func (s *ScopedInt) Manager() (*IntegerManager, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	return s.manager, nil
}
func (s *ScopedRational) Manager() (*RationalManager, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	return s.manager, nil
}
func (s *ScopedInt) Set(v *Int) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Copy(&s.num, v)
}
func (s *ScopedRational) Set(v *Rational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Copy(&s.num, v)
}
func (s *ScopedInt) SetInt64(v int64) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.SetInt64(&s.num, v)
}
func (s *ScopedRational) SetInt64(v int64) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.SetInt64(&s.num, v)
}
func (s *ScopedInt) Assign(other *ScopedInt) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	v, e := other.Get()
	if e != nil {
		return e
	}
	if s.manager != other.manager {
		return ErrScopedOwner
	}
	return s.manager.Copy(&s.num, v)
}
func (s *ScopedRational) Assign(other *ScopedRational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	v, e := other.Get()
	if e != nil {
		return e
	}
	if s.manager != other.manager {
		return ErrScopedOwner
	}
	return s.manager.Copy(&s.num, v)
}
func (s *ScopedInt) Clone() (*ScopedInt, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	c, e := NewScopedInt(s.manager)
	if e != nil {
		return nil, e
	}
	if e = c.Set(&s.num); e != nil {
		// A failed source copy constructor has no scoped destructor. Its
		// partially acquired numeral belongs to explicit owning cleanup.
		return nil, &ScopedIntCopyCleanupError{Cause: e, owner: c}
	}
	return c, nil
}
func (s *ScopedRational) Clone() (*ScopedRational, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	c, e := NewScopedRational(s.manager)
	if e != nil {
		return nil, e
	}
	if e = c.Set(&s.num); e != nil {
		return nil, &ScopedRationalCopyCleanupError{Cause: e, owner: c}
	}
	return c, nil
}
func (s *ScopedInt) Reset() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Reset(&s.num)
}
func (s *ScopedRational) Reset() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Reset(&s.num)
}
func (s *ScopedInt) SwapValue(v *Int) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Swap(&s.num, v)
}
func (s *ScopedRational) SwapValue(v *Rational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Swap(&s.num, v)
}
func (s *ScopedInt) Swap(other *ScopedInt) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	v, e := other.Get()
	if e != nil {
		return e
	}
	if s.manager != other.manager {
		return ErrScopedOwner
	}
	return s.SwapValue(v)
}
func (s *ScopedRational) Swap(other *ScopedRational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	v, e := other.Get()
	if e != nil {
		return e
	}
	if s.manager != other.manager {
		return ErrScopedOwner
	}
	return s.SwapValue(v)
}
func (s *ScopedInt) Neg() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Neg(&s.num)
}
func (s *ScopedRational) Neg() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Neg(&s.num)
}
func (s *ScopedInt) AbsCopy() (*ScopedInt, error) {
	c, e := s.Clone()
	if e != nil {
		return nil, e
	}
	if e = c.manager.Abs(&c.num); e != nil {
		return nil, finishScopedIntExpressionFailure(c, e)
	}
	return c, nil
}
func (s *ScopedRational) AbsCopy() (*ScopedRational, error) {
	c, e := s.Clone()
	if e != nil {
		return nil, e
	}
	if e = c.manager.Abs(&c.num); e != nil {
		return nil, finishScopedRationalExpressionFailure(c, e)
	}
	return c, nil
}
func (s *ScopedInt) apply(v *Int, op uint8) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	switch op {
	case 0:
		return s.manager.Add(&s.num, v, &s.num)
	case 1:
		return s.manager.Sub(&s.num, v, &s.num)
	case 2:
		return s.manager.Mul(&s.num, v, &s.num)
	case 3:
		return s.manager.Div(&s.num, v, &s.num)
	default:
		return s.manager.Rem(&s.num, v, &s.num)
	}
}
func (s *ScopedRational) apply(v *Rational, op uint8) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	switch op {
	case 0:
		return s.manager.Add(&s.num, v, &s.num)
	case 1:
		return s.manager.Sub(&s.num, v, &s.num)
	case 2:
		return s.manager.Mul(&s.num, v, &s.num)
	case 3:
		return s.manager.Div(&s.num, v, &s.num)
	default:
		return s.manager.Rem(&s.num, v, &s.num)
	}
}
func (s *ScopedInt) AddAssign(v *Int) error           { return s.apply(v, 0) }
func (s *ScopedInt) SubAssign(v *Int) error           { return s.apply(v, 1) }
func (s *ScopedInt) MulAssign(v *Int) error           { return s.apply(v, 2) }
func (s *ScopedInt) DivAssign(v *Int) error           { return s.apply(v, 3) }
func (s *ScopedInt) RemAssign(v *Int) error           { return s.apply(v, 4) }
func (s *ScopedRational) AddAssign(v *Rational) error { return s.apply(v, 0) }
func (s *ScopedRational) SubAssign(v *Rational) error { return s.apply(v, 1) }
func (s *ScopedRational) MulAssign(v *Rational) error { return s.apply(v, 2) }
func (s *ScopedRational) DivAssign(v *Rational) error { return s.apply(v, 3) }
func (s *ScopedRational) RemAssign(v *Rational) error { return s.apply(v, 4) }
func (s *ScopedInt) result(v *Int, op uint8) (*ScopedInt, error) {
	c, e := s.Clone()
	if e != nil {
		return nil, e
	}
	if e = c.apply(v, op); e != nil {
		return nil, finishScopedIntExpressionFailure(c, e)
	}
	return c, nil
}
func (s *ScopedRational) result(v *Rational, op uint8) (*ScopedRational, error) {
	c, e := s.Clone()
	if e != nil {
		return nil, e
	}
	if e = c.apply(v, op); e != nil {
		return nil, finishScopedRationalExpressionFailure(c, e)
	}
	return c, nil
}
func (s *ScopedInt) Add(v *Int) (*ScopedInt, error)                { return s.result(v, 0) }
func (s *ScopedInt) Sub(v *Int) (*ScopedInt, error)                { return s.result(v, 1) }
func (s *ScopedInt) Mul(v *Int) (*ScopedInt, error)                { return s.result(v, 2) }
func (s *ScopedInt) Div(v *Int) (*ScopedInt, error)                { return s.result(v, 3) }
func (s *ScopedRational) Add(v *Rational) (*ScopedRational, error) { return s.result(v, 0) }
func (s *ScopedRational) Sub(v *Rational) (*ScopedRational, error) { return s.result(v, 1) }
func (s *ScopedRational) Mul(v *Rational) (*ScopedRational, error) { return s.result(v, 2) }
func (s *ScopedRational) Div(v *Rational) (*ScopedRational, error) { return s.result(v, 3) }
func (s *ScopedInt) Compare(v *Int) (int, error) {
	if _, e := s.Get(); e != nil {
		return 0, e
	}
	return s.manager.Compare(&s.num, v)
}
func (s *ScopedRational) Compare(v *Rational) (int, error) {
	if _, e := s.Get(); e != nil {
		return 0, e
	}
	return s.manager.Compare(&s.num, v)
}
func (s *ScopedInt) Equal(v *Int) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c == 0, nil
}
func (s *ScopedRational) Equal(v *Rational) (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.Equal(&s.num, v)
}
func (s *ScopedInt) NotEqual(v *Int) (bool, error) {
	v1, e := s.Equal(v)
	if e != nil {
		return false, e
	}
	return !v1, nil
}
func (s *ScopedRational) NotEqual(v *Rational) (bool, error) {
	v1, e := s.Equal(v)
	if e != nil {
		return false, e
	}
	return !v1, nil
}
func (s *ScopedInt) Less(v *Int) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c < 0, nil
}
func (s *ScopedInt) Greater(v *Int) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c > 0, nil
}
func (s *ScopedInt) LessEqual(v *Int) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c <= 0, nil
}
func (s *ScopedInt) GreaterEqual(v *Int) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c >= 0, nil
}
func (s *ScopedRational) Less(v *Rational) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c < 0, nil
}
func (s *ScopedRational) Greater(v *Rational) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c > 0, nil
}
func (s *ScopedRational) LessEqual(v *Rational) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c <= 0, nil
}
func (s *ScopedRational) GreaterEqual(v *Rational) (bool, error) {
	c, e := s.Compare(v)
	if e != nil {
		return false, e
	}
	return c >= 0, nil
}
func (s *ScopedInt) sign() (int32, error) {
	if _, e := s.Get(); e != nil {
		return 0, e
	}
	return s.num.Val, nil
}
func (s *ScopedRational) sign() (int32, error) {
	if _, e := s.Get(); e != nil {
		return 0, e
	}
	return s.num.Num.Val, nil
}
func (s *ScopedInt) IsZero() (bool, error)        { v, e := s.sign(); return v == 0, e }
func (s *ScopedInt) IsPos() (bool, error)         { v, e := s.sign(); return v > 0, e }
func (s *ScopedInt) IsNeg() (bool, error)         { v, e := s.sign(); return v < 0, e }
func (s *ScopedInt) IsNonpos() (bool, error)      { v, e := s.sign(); return v <= 0, e }
func (s *ScopedInt) IsNonneg() (bool, error)      { v, e := s.sign(); return v >= 0, e }
func (s *ScopedRational) IsZero() (bool, error)   { v, e := s.sign(); return v == 0, e }
func (s *ScopedRational) IsPos() (bool, error)    { v, e := s.sign(); return v > 0, e }
func (s *ScopedRational) IsNeg() (bool, error)    { v, e := s.sign(); return v < 0, e }
func (s *ScopedRational) IsNonpos() (bool, error) { v, e := s.sign(); return v <= 0, e }
func (s *ScopedRational) IsNonneg() (bool, error) { v, e := s.sign(); return v >= 0, e }
func (s *ScopedInt) Display() (string, error) {
	if _, e := s.Get(); e != nil {
		return "", e
	}
	return s.manager.String(&s.num)
}
func (s *ScopedRational) Display() (string, error) {
	if _, e := s.Get(); e != nil {
		return "", e
	}
	return s.manager.String(&s.num)
}
func (s *ScopedInt) Close() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if s.manager.closed {
		s.manager.BudgetField.Release(uint64(reflect.TypeOf(ScopedInt{}).Size()))
		s.num = Int{}
		s.closed = true
		return arena.ErrClosed
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := SourceIntegerDel(s.manager, &s.num); e != nil {
		s.sourceCause = e
		return e
	}
	s.manager.BudgetField.Release(uint64(reflect.TypeOf(ScopedInt{}).Size()))
	s.num = Int{}
	s.closed = true
	return nil
}
func (s *ScopedRational) Close() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if s.manager.closed {
		s.manager.BudgetField.Release(uint64(reflect.TypeOf(ScopedRational{}).Size()))
		s.num = NewRational()
		s.closed = true
		return arena.ErrClosed
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := s.manager.checkCleanup(&s.num); e != nil {
		s.sourceCause = e
		return e
	}
	if e := SourceRationalDel(s.manager, &s.num); e != nil {
		s.sourceCause = e
		return e
	}
	s.manager.BudgetField.Release(uint64(reflect.TypeOf(ScopedRational{}).Size()))
	s.num = NewRational()
	s.closed = true
	return nil
}

// Cleanup owns a stopped source destructor without invoking it again.
func (s *ScopedInt) Cleanup() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if e := s.manager.CleanupRawNumeral(&s.num); e != nil {
		return e
	}
	s.manager.BudgetField.Release(uint64(reflect.TypeOf(ScopedInt{}).Size()))
	s.num = Int{}
	s.closed = true
	return nil
}

// Cleanup releases the remaining numerator and denominator in source order.
func (s *ScopedRational) Cleanup() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if e := s.manager.checkCleanup(&s.num); e != nil {
		return e
	}
	if e := s.manager.Z.CleanupRawNumeral(&s.num.Num); e != nil {
		return e
	}
	if e := s.manager.Z.CleanupRawNumeral(&s.num.Den); e != nil {
		return e
	}
	s.manager.BudgetField.Release(uint64(reflect.TypeOf(ScopedRational{}).Size()))
	s.num = NewRational()
	s.closed = true
	return nil
}

// Vectors remain source-noncopyable. Read makes a deep caller-owned numeral;
// Set performs manager copy. The explicit BorrowSlot/BorrowedValues and
// SwapRaw methods expose the source numeral_buffer boundaries to its concrete
// Euclidean caller, which must not keep a borrow across growth or swap.
type ScopedIntVector struct {
	manager     *IntegerManager
	data        []Int
	self        *ScopedIntVector
	sourceCause error
	closed      bool
}
type ScopedRationalVector struct {
	manager     *RationalManager
	data        []Rational
	self        *ScopedRationalVector
	sourceCause error
	closed      bool
}

func NewScopedIntVector(m *IntegerManager) (*ScopedIntVector, error) {
	if m == nil || m.Cells == nil {
		return nil, ErrScopedOwner
	}
	if m.closed {
		return nil, arena.ErrClosed
	}
	v := &ScopedIntVector{manager: m}
	v.self = v
	return v, nil
}
func NewScopedRationalVector(m *RationalManager) (*ScopedRationalVector, error) {
	if m == nil || m.Z == nil {
		return nil, ErrScopedOwner
	}
	if m.closed {
		return nil, arena.ErrClosed
	}
	v := &ScopedRationalVector{manager: m}
	v.self = v
	return v, nil
}
func (v *ScopedIntVector) open() error {
	if v == nil || v.self != v {
		return ErrScopedCopy
	}
	if v.closed || v.manager.closed {
		return arena.ErrClosed
	}
	if v.sourceCause != nil {
		return v.sourceCause
	}
	return nil
}
func (v *ScopedRationalVector) open() error {
	if v == nil || v.self != v {
		return ErrScopedCopy
	}
	if v.closed || v.manager.closed {
		return arena.ErrClosed
	}
	if v.sourceCause != nil {
		return v.sourceCause
	}
	return nil
}
func (v *ScopedIntVector) Size() uint32 {
	if v == nil || v.self != v || v.closed {
		return 0
	}
	return uint32(len(v.data))
}
func (v *ScopedRationalVector) Size() uint32 {
	if v == nil || v.self != v || v.closed {
		return 0
	}
	return uint32(len(v.data))
}
func (v *ScopedIntVector) Capacity() uint32 {
	if v == nil || v.self != v || v.closed {
		return 0
	}
	return uint32(cap(v.data))
}
func (v *ScopedRationalVector) Capacity() uint32 {
	if v == nil || v.self != v || v.closed {
		return 0
	}
	return uint32(cap(v.data))
}
func (v *ScopedIntVector) Manager() (*IntegerManager, error) {
	if e := v.open(); e != nil {
		return nil, e
	}
	return v.manager, nil
}
func (v *ScopedRationalVector) Manager() (*RationalManager, error) {
	if e := v.open(); e != nil {
		return nil, e
	}
	return v.manager, nil
}
func (v *ScopedIntVector) Read(i uint32, out *Int) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.data)) {
		return arena.ErrBounds
	}
	return v.manager.Copy(out, &v.data[i])
}
func (v *ScopedRationalVector) Read(i uint32, out *Rational) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.data)) {
		return arena.ErrBounds
	}
	return v.manager.Copy(out, &v.data[i])
}
func (v *ScopedIntVector) Set(i uint32, num *Int) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.data)) {
		return arena.ErrBounds
	}
	return v.manager.Copy(&v.data[i], num)
}
func (v *ScopedRationalVector) Set(i uint32, num *Rational) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.data)) {
		return arena.ErrBounds
	}
	return v.manager.Copy(&v.data[i], num)
}

// BorrowSlot exposes the mutable source numeral_buffer::operator[] slot.
// The borrow ends at the next operation that may grow, reset or swap storage.
func (v *ScopedIntVector) BorrowSlot(i uint32) (*Int, error) {
	if e := v.open(); e != nil {
		return nil, e
	}
	if uint64(i) >= uint64(len(v.data)) {
		return nil, arena.ErrBounds
	}
	return &v.data[i], nil
}

func (v *ScopedRationalVector) BorrowSlot(i uint32) (*Rational, error) {
	if e := v.open(); e != nil {
		return nil, e
	}
	if uint64(i) >= uint64(len(v.data)) {
		return nil, arena.ErrBounds
	}
	return &v.data[i], nil
}

// BorrowedValues exposes source numeral_buffer::c_ptr as a Go slice. The
// caller may read the current slots only until growth, reset or swap.
func (v *ScopedIntVector) BorrowedValues() ([]Int, error) {
	if e := v.open(); e != nil {
		return nil, e
	}
	return v.data, nil
}

func (v *ScopedRationalVector) BorrowedValues() ([]Rational, error) {
	if e := v.open(); e != nil {
		return nil, e
	}
	return v.data, nil
}

// SwapRaw is source numeral_buffer::swap(svector&): it transfers the whole
// raw numeral slices without copying. The caller owns the old buffer values
// moved into *raw and must later delete them with this same manager.
func (v *ScopedIntVector) SwapRaw(raw *[]Int) error {
	if e := v.open(); e != nil {
		return e
	}
	if raw == nil {
		return arena.ErrInvalidSlot
	}
	v.data, *raw = *raw, v.data
	return nil
}

func (v *ScopedRationalVector) SwapRaw(raw *[]Rational) error {
	if e := v.open(); e != nil {
		return e
	}
	if raw == nil {
		return arena.ErrInvalidSlot
	}
	v.data, *raw = *raw, v.data
	return nil
}

// Publish a default element before the deep copy. A failed copy retains that
// element, matching the observable source prefix. Invalid operands are rejected
// before either action, unlike raw source undefined references.
func (v *ScopedIntVector) Push(num *Int) error {
	if e := v.open(); e != nil {
		return e
	}
	if e := v.manager.check(num); e != nil {
		return e
	}
	if uint64(len(v.data)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	input := *num
	v.data = append(v.data, Int{})
	return v.manager.Copy(&v.data[len(v.data)-1], &input)
}
func (v *ScopedRationalVector) Push(num *Rational) error {
	if e := v.open(); e != nil {
		return e
	}
	if e := v.manager.check(num); e != nil {
		return e
	}
	if uint64(len(v.data)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	input := *num
	v.data = append(v.data, NewRational())
	return v.manager.Copy(&v.data[len(v.data)-1], &input)
}
func (v *ScopedIntVector) Shrink(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(n) > uint64(len(v.data)) {
		return arena.ErrBounds
	}
	for i := int(n); i < len(v.data); i++ {
		if e := SourceIntegerDel(v.manager, &v.data[i]); e != nil {
			v.sourceCause = e
			return e
		}
	}
	v.data = v.data[:n]
	return nil
}
func (v *ScopedRationalVector) Shrink(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(n) > uint64(len(v.data)) {
		return arena.ErrBounds
	}
	for i := int(n); i < len(v.data); i++ {
		if e := v.manager.checkCleanup(&v.data[i]); e != nil {
			v.sourceCause = e
			return e
		}
		if e := SourceRationalDel(v.manager, &v.data[i]); e != nil {
			v.sourceCause = e
			return e
		}
	}
	v.data = v.data[:n]
	return nil
}
func (v *ScopedIntVector) Pop() error {
	if e := v.open(); e != nil {
		return e
	}
	if len(v.data) == 0 {
		return arena.ErrBounds
	}
	return v.Shrink(uint32(len(v.data) - 1))
}
func (v *ScopedRationalVector) Pop() error {
	if e := v.open(); e != nil {
		return e
	}
	if len(v.data) == 0 {
		return arena.ErrBounds
	}
	return v.Shrink(uint32(len(v.data) - 1))
}
func (v *ScopedIntVector) Resize(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(n) <= uint64(len(v.data)) {
		return v.Shrink(n)
	}
	if uint64(n) > uint64(int(^uint(0)>>1)) {
		return arena.ErrBounds
	}
	oldLen := len(v.data)
	newLen := int(n)
	if newLen > cap(v.data) {
		next := make([]Int, newLen)
		copy(next, v.data)
		v.data = next
	} else {
		v.data = v.data[:newLen]
	}
	for i := oldLen; i < newLen; i++ {
		v.data[i] = Int{}
	}
	return nil
}

// Reserve follows src/util/vector.h:509-512: unlike Go slice capacity growth,
// source reserve extends the logical size with default-initialized numerals.
// linear_equation_manager indexes the new slots immediately afterward.
func (v *ScopedIntVector) Reserve(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if n <= uint32(len(v.data)) {
		return nil
	}
	return v.Resize(n)
}

func (v *ScopedRationalVector) Resize(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(n) <= uint64(len(v.data)) {
		return v.Shrink(n)
	}
	if uint64(n) > uint64(int(^uint(0)>>1)) {
		return arena.ErrBounds
	}
	oldLen := len(v.data)
	newLen := int(n)
	if newLen > cap(v.data) {
		next := make([]Rational, newLen)
		copy(next, v.data)
		v.data = next
	} else {
		v.data = v.data[:newLen]
	}
	for i := oldLen; i < newLen; i++ {
		v.data[i] = NewRational()
	}
	return nil
}
func (v *ScopedIntVector) Reset() error      { return v.Shrink(0) }
func (v *ScopedRationalVector) Reset() error { return v.Shrink(0) }
func (v *ScopedIntVector) Close() error {
	if v == nil || v.self != v {
		return ErrScopedCopy
	}
	if v.closed {
		return nil
	}
	if v.sourceCause != nil {
		return v.sourceCause
	}
	var e error
	if v.manager.closed {
		e = arena.ErrClosed
	} else {
		e = v.Reset()
	}
	if e != nil && !v.manager.closed {
		return e
	}
	v.data = nil
	v.closed = true
	return e
}
func (v *ScopedRationalVector) Close() error {
	if v == nil || v.self != v {
		return ErrScopedCopy
	}
	if v.closed {
		return nil
	}
	if v.sourceCause != nil {
		return v.sourceCause
	}
	var e error
	if v.manager.closed {
		e = arena.ErrClosed
	} else {
		e = v.Reset()
	}
	if e != nil && !v.manager.closed {
		return e
	}
	v.data = nil
	v.closed = true
	return e
}

// Cleanup drains a stopped integer vector without replaying its source loop.
func (v *ScopedIntVector) Cleanup() error {
	if v == nil || v.self != v {
		return ErrScopedCopy
	}
	if v.closed {
		return nil
	}
	for i := range v.data {
		if e := v.manager.CleanupRawNumeral(&v.data[i]); e != nil {
			return e
		}
	}
	v.data = nil
	v.closed = true
	return nil
}

// Cleanup owns the remaining numerator/denominator suffix of each element.
func (v *ScopedRationalVector) Cleanup() error {
	if v == nil || v.self != v {
		return ErrScopedCopy
	}
	if v.closed {
		return nil
	}
	for i := range v.data {
		if e := v.manager.checkCleanup(&v.data[i]); e != nil {
			return e
		}
		if e := v.manager.Z.CleanupRawNumeral(&v.data[i].Num); e != nil {
			return e
		}
		if e := v.manager.Z.CleanupRawNumeral(&v.data[i].Den); e != nil {
			return e
		}
	}
	v.data = nil
	v.closed = true
	return nil
}
