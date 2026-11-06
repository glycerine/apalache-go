package numeral

// Concrete scoped_numeral<mpff>.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type ScopedFastFloat struct {
	manager     *FastFloatManager
	num         FastFloat
	self        *ScopedFastFloat
	sourceCause error
	closed      bool
}

func NewScopedFastFloat(m *FastFloatManager) (*ScopedFastFloat, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if e := m.config.Storage.Descriptors.Budget.Reserve(uint64(reflect.TypeOf(ScopedFastFloat{}).Size())); e != nil {
		return nil, e
	}
	s := &ScopedFastFloat{manager: m}
	s.self = s
	return s, nil
}
func (s *ScopedFastFloat) Get() (*FastFloat, error) {
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
func (s *ScopedFastFloat) Manager() (*FastFloatManager, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	return s.manager, nil
}
func (s *ScopedFastFloat) Set(a *FastFloat) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Copy(&s.num, a)
}
func (s *ScopedFastFloat) SetInt32(n int32) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.SetInt32(&s.num, n)
}
func (s *ScopedFastFloat) Assign(a *ScopedFastFloat) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	if _, e := a.Get(); e != nil {
		return e
	}
	if s.manager != a.manager {
		return ErrScopedOwner
	}
	if s == a {
		return nil
	}
	return s.Set(&a.num)
}
func (s *ScopedFastFloat) Clone() (*ScopedFastFloat, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	r, e := NewScopedFastFloat(s.manager)
	if e != nil {
		return nil, e
	}
	if e = r.Set(&s.num); e != nil {
		// A failed source copy constructor has no scoped destructor. Keep
		// its acquired ID and header for explicit owning cleanup.
		return nil, &ScopedFastFloatCopyCleanupError{Cause: e, owner: r}
	}
	return r, nil
}
func (s *ScopedFastFloat) Close() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := s.cleanup(); e != nil {
		s.sourceCause = e
		return e
	}
	return nil
}

// Cleanup releases a partially constructed scoped value without invoking a
// source destructor. The physical release is the same allocation-free Del.
func (s *ScopedFastFloat) Cleanup() error {
	return s.cleanup()
}

func (s *ScopedFastFloat) cleanup() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if e := s.manager.Del(&s.num); e != nil {
		return e
	}
	s.num = FastFloat{}
	s.manager.config.Storage.Descriptors.Budget.Release(uint64(reflect.TypeOf(ScopedFastFloat{}).Size()))
	s.closed = true
	return nil
}
func (s *ScopedFastFloat) Reset() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Reset(&s.num)
}
func (s *ScopedFastFloat) Swap(a *ScopedFastFloat) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	if _, e := a.Get(); e != nil {
		return e
	}
	if s.manager != a.manager {
		return ErrScopedOwner
	}
	return s.manager.Swap(&s.num, &a.num)
}
func (s *ScopedFastFloat) SwapValue(a *FastFloat) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Swap(&s.num, a)
}
func (s *ScopedFastFloat) Neg() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Neg(&s.num)
}
func (s *ScopedFastFloat) Abs() (*ScopedFastFloat, error) {
	r, e := s.Clone()
	if e != nil {
		return nil, e
	}
	if e = r.manager.Abs(&r.num); e != nil {
		var result *ScopedFastFloat
		finishScopedFastFloatExpression(r, &result, &e)
		return nil, e
	}
	return r, nil
}
func (s *ScopedFastFloat) compound(a *ScopedFastFloat, op uint32) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	if _, e := a.Get(); e != nil {
		return e
	}
	if s.manager != a.manager {
		return ErrScopedOwner
	}
	switch op {
	case 0:
		return s.manager.Add(&s.num, &a.num, &s.num)
	case 1:
		return s.manager.Sub(&s.num, &a.num, &s.num)
	case 2:
		return s.manager.Mul(&s.num, &a.num, &s.num)
	default:
		return s.manager.Div(&s.num, &a.num, &s.num)
	}
}
func (s *ScopedFastFloat) value(a *ScopedFastFloat, op uint32) (result *ScopedFastFloat, cause error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	if _, e := a.Get(); e != nil {
		return nil, e
	}
	if s.manager != a.manager {
		return nil, ErrScopedOwner
	}
	tmp, cause := s.Clone()
	if cause != nil {
		return nil, cause
	}
	defer finishScopedFastFloatExpression(tmp, &result, &cause)
	if cause = tmp.compound(a, op); cause != nil {
		return nil, cause
	}
	return tmp.Clone()
}
func (s *ScopedFastFloat) Add(a *ScopedFastFloat) (*ScopedFastFloat, error) { return s.value(a, 0) }
func (s *ScopedFastFloat) Sub(a *ScopedFastFloat) (*ScopedFastFloat, error) { return s.value(a, 1) }
func (s *ScopedFastFloat) Mul(a *ScopedFastFloat) (*ScopedFastFloat, error) { return s.value(a, 2) }
func (s *ScopedFastFloat) Div(a *ScopedFastFloat) (*ScopedFastFloat, error) { return s.value(a, 3) }
func (s *ScopedFastFloat) AddAssign(a *ScopedFastFloat) error               { return s.compound(a, 0) }
func (s *ScopedFastFloat) SubAssign(a *ScopedFastFloat) error               { return s.compound(a, 1) }
func (s *ScopedFastFloat) MulAssign(a *ScopedFastFloat) error               { return s.compound(a, 2) }
func (s *ScopedFastFloat) DivAssign(a *ScopedFastFloat) error               { return s.compound(a, 3) }
func (s *ScopedFastFloat) IsZero() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsZero(&s.num)
}
func (s *ScopedFastFloat) IsPos() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsPos(&s.num)
}
func (s *ScopedFastFloat) IsNeg() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsNeg(&s.num)
}
func (s *ScopedFastFloat) IsNonPos() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsNonPos(&s.num)
}
func (s *ScopedFastFloat) IsNonNeg() (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	return s.manager.IsNonNeg(&s.num)
}
func (s *ScopedFastFloat) compare(a *ScopedFastFloat, op uint32) (bool, error) {
	if _, e := s.Get(); e != nil {
		return false, e
	}
	if _, e := a.Get(); e != nil {
		return false, e
	}
	if s.manager != a.manager {
		return false, ErrScopedOwner
	}
	switch op {
	case 0:
		return s.manager.Equal(&s.num, &a.num)
	case 1:
		return s.manager.NotEqual(&s.num, &a.num)
	case 2:
		return s.manager.Less(&s.num, &a.num)
	case 3:
		return s.manager.Greater(&s.num, &a.num)
	case 4:
		return s.manager.LessEqual(&s.num, &a.num)
	default:
		return s.manager.GreaterEqual(&s.num, &a.num)
	}
}
func (s *ScopedFastFloat) Equal(a *ScopedFastFloat) (bool, error)        { return s.compare(a, 0) }
func (s *ScopedFastFloat) NotEqual(a *ScopedFastFloat) (bool, error)     { return s.compare(a, 1) }
func (s *ScopedFastFloat) Less(a *ScopedFastFloat) (bool, error)         { return s.compare(a, 2) }
func (s *ScopedFastFloat) Greater(a *ScopedFastFloat) (bool, error)      { return s.compare(a, 3) }
func (s *ScopedFastFloat) LessEqual(a *ScopedFastFloat) (bool, error)    { return s.compare(a, 4) }
func (s *ScopedFastFloat) GreaterEqual(a *ScopedFastFloat) (bool, error) { return s.compare(a, 5) }
func (s *ScopedFastFloat) String() (string, error) {
	if _, e := s.Get(); e != nil {
		return "", e
	}
	return s.manager.String(&s.num)
}
func (s *ScopedFastFloat) FormatInto(dst []byte) (int, error) {
	if _, e := s.Get(); e != nil {
		return 0, e
	}
	return s.manager.FormatInto(&s.num, dst)
}
