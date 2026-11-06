package numeral

// Concrete MPBQ instantiation of scoped_numeral[_vector].h and MPBQ macros.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type ScopedBinary struct {
	manager     *BinaryRationalManager
	num         BinaryRational
	self        *ScopedBinary
	sourceCause error
	closed      bool
}

func NewScopedBinary(m *BinaryRationalManager) (*ScopedBinary, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if e := m.Z.BudgetField.Reserve(uint64(reflect.TypeOf(ScopedBinary{}).Size())); e != nil {
		return nil, e
	}
	s := &ScopedBinary{manager: m}
	s.self = s
	return s, nil
}
func (s *ScopedBinary) Get() (*BinaryRational, error) {
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
func (s *ScopedBinary) Manager() (*BinaryRationalManager, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	return s.manager, nil
}
func (s *ScopedBinary) Set(v *BinaryRational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Copy(&s.num, v)
}
func (s *ScopedBinary) SetInt32(n int32) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.SetInt32(&s.num, n)
}
func (s *ScopedBinary) SetInteger(n *Int) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.SetInteger(&s.num, n)
}
func (s *ScopedBinary) Assign(other *ScopedBinary) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	if _, e := other.Get(); e != nil {
		return e
	}
	if s.manager != other.manager {
		return ErrScopedOwner
	}
	if s == other {
		return nil
	}
	return s.Set(&other.num)
}
func (s *ScopedBinary) Clone() (*ScopedBinary, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	r, e := NewScopedBinary(s.manager)
	if e != nil {
		return nil, e
	}
	if e = r.Set(&s.num); e != nil {
		// A failed source copy constructor has no scoped destructor; retain
		// its partially acquired numeral for explicit owning cleanup.
		return nil, &ScopedBinaryExpressionCleanupError{Cause: e, local: r}
	}
	return r, nil
}
func (s *ScopedBinary) Reset() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Reset(&s.num)
}
func (s *ScopedBinary) Swap(other *ScopedBinary) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	if _, e := other.Get(); e != nil {
		return e
	}
	if s.manager != other.manager {
		return ErrScopedOwner
	}
	return s.manager.Swap(&s.num, &other.num)
}
func (s *ScopedBinary) SwapValue(other *BinaryRational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Swap(&s.num, other)
}
func (s *ScopedBinary) Close() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := s.manager.checkCleanup(&s.num); e != nil {
		s.sourceCause = e
		return e
	}
	if _, e := sourceNumeralDel(s.manager.Z, &s.num.Num); e != nil {
		s.sourceCause = e
		return e
	}
	s.manager.Z.BudgetField.Release(uint64(reflect.TypeOf(ScopedBinary{}).Size()))
	s.closed = true
	return nil
}

// Cleanup releases a stopped scoped source destructor without replaying it.
func (s *ScopedBinary) Cleanup() error {
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
	s.manager.Z.BudgetField.Release(uint64(reflect.TypeOf(ScopedBinary{}).Size()))
	s.closed = true
	return nil
}
func (s *ScopedBinary) Neg() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Neg(&s.num)
}
func (s *ScopedBinary) IsZero() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsZero(&s.num)
}
func (s *ScopedBinary) IsPos() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsPos(&s.num)
}
func (s *ScopedBinary) IsNeg() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsNeg(&s.num)
}
func (s *ScopedBinary) IsNonPos() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsNonPos(&s.num)
}
func (s *ScopedBinary) IsNonNeg() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsNonNeg(&s.num)
}
func (s *ScopedBinary) String() (string, error) {
	if _, e := s.Get(); e != nil {
		return "", e
	}
	return s.manager.String(&s.num)
}
func (s *ScopedBinary) compound(other *ScopedBinary, op uint8) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	if _, e := other.Get(); e != nil {
		return e
	}
	if s.manager != other.manager {
		return ErrScopedOwner
	}
	return s.compoundRaw(&other.num, op)
}
func (s *ScopedBinary) compoundRaw(other *BinaryRational, op uint8) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	switch op {
	case 0:
		return s.manager.Add(&s.num, other, &s.num)
	case 1:
		return s.manager.Sub(&s.num, other, &s.num)
	default:
		return s.manager.Mul(&s.num, other, &s.num)
	}
}
func (s *ScopedBinary) AddAssign(other *ScopedBinary) error        { return s.compound(other, 0) }
func (s *ScopedBinary) SubAssign(other *ScopedBinary) error        { return s.compound(other, 1) }
func (s *ScopedBinary) MulAssign(other *ScopedBinary) error        { return s.compound(other, 2) }
func (s *ScopedBinary) AddAssignValue(other *BinaryRational) error { return s.compoundRaw(other, 0) }
func (s *ScopedBinary) SubAssignValue(other *BinaryRational) error { return s.compoundRaw(other, 1) }
func (s *ScopedBinary) MulAssignValue(other *BinaryRational) error { return s.compoundRaw(other, 2) }
func (s *ScopedBinary) value(other *ScopedBinary, op uint8) (*ScopedBinary, error) {
	if _, e := other.Get(); e != nil {
		return nil, e
	}
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	if s.manager != other.manager {
		return nil, ErrScopedOwner
	}
	return s.valueRaw(&other.num, op)
}
func (s *ScopedBinary) valueRaw(other *BinaryRational, op uint8) (result *ScopedBinary, err error) {
	r, e := s.Clone()
	if e != nil {
		return nil, e
	}
	defer finishScopedBinaryExpressionSource(&result, &err, r)
	if e = r.compoundRaw(other, op); e != nil {
		return nil, e
	}
	return r.Clone()
}
func (s *ScopedBinary) Add(other *ScopedBinary) (*ScopedBinary, error) { return s.value(other, 0) }
func (s *ScopedBinary) Sub(other *ScopedBinary) (*ScopedBinary, error) { return s.value(other, 1) }
func (s *ScopedBinary) Mul(other *ScopedBinary) (*ScopedBinary, error) { return s.value(other, 2) }
func (s *ScopedBinary) AddValue(other *BinaryRational) (*ScopedBinary, error) {
	return s.valueRaw(other, 0)
}
func (s *ScopedBinary) SubValue(other *BinaryRational) (*ScopedBinary, error) {
	return s.valueRaw(other, 1)
}
func (s *ScopedBinary) MulValue(other *BinaryRational) (*ScopedBinary, error) {
	return s.valueRaw(other, 2)
}
func (s *ScopedBinary) macroValue(n *Int, op uint8) (result *ScopedBinary, err error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	if e := s.manager.Z.check(n); e != nil {
		return nil, e
	}
	b, e := NewScopedBinary(s.manager)
	if e != nil {
		return nil, e
	}
	defer finishScopedBinaryExpressionSource(&result, &err, b)
	if e = b.SetInteger(n); e != nil {
		return nil, e
	}
	r, e := NewScopedBinary(s.manager)
	if e != nil {
		return nil, e
	}
	switch op {
	case 0:
		e = s.manager.Add(&s.num, &b.num, &r.num)
	case 1:
		e = s.manager.Sub(&s.num, &b.num, &r.num)
	case 2:
		e = s.manager.Mul(&s.num, &b.num, &r.num)
	}
	if e != nil {
		finishScopedBinaryExpressionSource(&result, &e, r)
		return nil, e
	}
	return r, nil
}
func (s *ScopedBinary) AddInt(n int32) (*ScopedBinary, error) {
	v := NewInt(n)
	return s.macroValue(&v, 0)
}
func (s *ScopedBinary) SubInt(n int32) (*ScopedBinary, error) {
	v := NewInt(n)
	return s.macroValue(&v, 1)
}
func (s *ScopedBinary) MulInt(n int32) (*ScopedBinary, error) {
	v := NewInt(n)
	return s.macroValue(&v, 2)
}
func (s *ScopedBinary) AddInteger(n *Int) (*ScopedBinary, error) { return s.macroValue(n, 0) }
func (s *ScopedBinary) SubInteger(n *Int) (*ScopedBinary, error) { return s.macroValue(n, 1) }
func (s *ScopedBinary) MulInteger(n *Int) (*ScopedBinary, error) { return s.macroValue(n, 2) }
func (s *ScopedBinary) compareRaw(b *BinaryRational, op uint8) (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	switch op {
	case 0:
		return s.manager.Equal(&s.num, b)
	case 1:
		v, e := s.manager.Equal(&s.num, b)
		return !v, e
	case 2:
		return s.manager.Less(&s.num, b)
	case 3:
		return s.manager.LessEqual(&s.num, b)
	case 4:
		return s.manager.Greater(&s.num, b)
	default:
		return s.manager.GreaterEqual(&s.num, b)
	}
}
func (s *ScopedBinary) macroCompare(n *Int, op uint8) (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	if e := s.manager.Z.check(n); e != nil {
		return false, e
	}
	b, e := NewScopedBinary(s.manager)
	if e != nil {
		return false, e
	}
	defer b.Close()
	if e = b.SetInteger(n); e != nil {
		return false, e
	}
	if op == 1 {
		return s.manager.NotEqual(&s.num, &b.num)
	}
	return s.compareRaw(&b.num, op)
}
