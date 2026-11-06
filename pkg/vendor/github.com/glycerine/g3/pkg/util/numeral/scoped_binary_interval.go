package numeral

// Concrete scoped_interval.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Lexical owner; Get/Lower/Upper are borrows ending at mutation or Close.
// No implicit copy, assignment or clone is a source scoped_interval operation.
type ScopedBinaryInterval struct {
	manager *BinaryIntervalManager
	num     BinaryInterval
	self    *ScopedBinaryInterval
	closed  bool
}

func NewScopedBinaryInterval(m *BinaryIntervalManager) (*ScopedBinaryInterval, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if e := m.bounds.Z.BudgetField.Reserve(uint64(reflect.TypeOf(ScopedBinaryInterval{}).Size())); e != nil {
		return nil, e
	}
	s := &ScopedBinaryInterval{manager: m}
	s.self = s
	return s, nil
}
func (s *ScopedBinaryInterval) Get() (*BinaryInterval, error) {
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
func (s *ScopedBinaryInterval) Manager() (*BinaryIntervalManager, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	return s.manager, nil
}
func (s *ScopedBinaryInterval) Lower() (*BinaryRational, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	return &s.num.lower, nil
}
func (s *ScopedBinaryInterval) Upper() (*BinaryRational, error) {
	if _, e := s.Get(); e != nil {
		return nil, e
	}
	return &s.num.upper, nil
}
func (s *ScopedBinaryInterval) LowerCopy(dst *BinaryRational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Lower(&s.num, dst)
}
func (s *ScopedBinaryInterval) UpperCopy(dst *BinaryRational) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Upper(&s.num, dst)
}
func (s *ScopedBinaryInterval) Reset() error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Reset(&s.num)
}
func (s *ScopedBinaryInterval) Swap(o *ScopedBinaryInterval) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	if _, e := o.Get(); e != nil {
		return e
	}
	if s.manager != o.manager {
		return ErrScopedOwner
	}
	return s.manager.Swap(&s.num, &o.num)
}
func (s *ScopedBinaryInterval) SwapValue(o *BinaryInterval) error {
	if _, e := s.Get(); e != nil {
		return e
	}
	return s.manager.Swap(&s.num, o)
}
func (s *ScopedBinaryInterval) Close() error {
	if s == nil || s.self != s {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if e := s.manager.Del(&s.num); e != nil {
		return e
	}
	s.manager.bounds.Z.BudgetField.Release(uint64(reflect.TypeOf(ScopedBinaryInterval{}).Size()))
	s.closed = true
	return nil
}
func (s *ScopedBinaryInterval) String() (string, error) {
	if _, e := s.Get(); e != nil {
		return "", e
	}
	return s.manager.String(&s.num)
}
func (s *ScopedBinaryInterval) FormatInto(dst []byte) (int, error) {
	if _, e := s.Get(); e != nil {
		return 0, e
	}
	return s.manager.FormatInto(&s.num, dst)
}
