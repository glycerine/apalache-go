package lp

// Source: src/util/lp/stacked_unordered_set.h; src/test/lp.cpp:2029-2050.

type stackedInt32SetDelta struct {
	inserted, erased, debugCopy map[int32]struct{}
}

// StackedInt32Set is the retained int scoped-set algorithm using native Go
// maps and slices. The zero value is ready. Do not copy after use. Values
// returns a borrowed read-only view, matching source operator().
type StackedInt32Set struct {
	values map[int32]struct{}
	stack  []stackedInt32SetDelta
}

func (s *StackedInt32Set) Insert(key int32) {
	if s.values == nil {
		s.values = make(map[int32]struct{})
	}
	if len(s.stack) == 0 {
		s.values[key] = struct{}{}
		return
	}
	if _, exists := s.values[key]; exists {
		return
	}
	s.values[key] = struct{}{}
	d := &s.stack[len(s.stack)-1]
	if _, erased := d.erased[key]; erased {
		delete(d.erased, key)
	} else {
		d.inserted[key] = struct{}{}
	}
}
func (s *StackedInt32Set) Values() map[int32]struct{} { return s.values }
func (s *StackedInt32Set) Push() {
	d := stackedInt32SetDelta{inserted: make(map[int32]struct{}), erased: make(map[int32]struct{}), debugCopy: make(map[int32]struct{}, len(s.values))}
	for key := range s.values {
		d.debugCopy[key] = struct{}{}
	}
	s.stack = append(s.stack, d)
}
func (s *StackedInt32Set) Pop(k uint32) error {
	for k > 0 && len(s.stack) > 0 {
		last := len(s.stack) - 1
		d := &s.stack[last]
		for key := range d.inserted {
			delete(s.values, key)
		}
		for key := range d.erased {
			s.values[key] = struct{}{}
		}
		if sourceDebugEnabled {
			if len(d.debugCopy) != len(s.values) {
				return ErrAssertion
			}
			for key := range d.debugCopy {
				if _, exists := s.values[key]; !exists {
					return ErrAssertion
				}
			}
		}
		s.stack[last] = stackedInt32SetDelta{}
		s.stack = s.stack[:last]
		k--
	}
	return nil
}
