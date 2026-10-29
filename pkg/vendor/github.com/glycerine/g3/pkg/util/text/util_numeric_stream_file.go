package text

// Source: src/util/memory_manager.cpp:55-72; src/util/small_object_allocator.cpp:172-242.

import (
	"errors"
)

// The selected file provider uses direct Go writes. It borrows the file and
// represents default nonthrowing ostream badbit; native buffering and locale
// profiles remain outside this adapter.
var ErrNumericStreamFile = errors.New("g3: file stream has no string buffer")

func (s *NumericTextStream) Bad() (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.FileBad, nil
}

// Fail reads failbit or badbit, as basic_ios::fail does; it does not clear state.
func (s *NumericTextStream) Fail() (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.fileFail || s.FileBad, nil
}
