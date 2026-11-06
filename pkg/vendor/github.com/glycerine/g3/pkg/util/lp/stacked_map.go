package lp

// Source: src/util/lp/stacked_map.h; src/test/lp.cpp:1933-1970.

type stackedInt32FloatDelta struct {
	newKeys  map[int32]struct{}
	original map[int32]float64
}

// StackedInt32FloatMap is the retained int/double scoped-map algorithm.
// Native Go maps/slices own storage; the zero value is ready. Do not copy
// after use. Only operations reached by the retained source test are exposed.
type StackedInt32FloatMap struct {
	values map[int32]float64
	stack  []stackedInt32FloatDelta
}

func (s *StackedInt32FloatMap) Set(key int32, value float64) {
	if s.values == nil {
		s.values = make(map[int32]float64)
	}
	if len(s.stack) == 0 {
		s.values[key] = value
		return
	}
	d := &s.stack[len(s.stack)-1]
	old, exists := s.values[key]
	if !exists {
		d.newKeys[key] = struct{}{}
		s.values[key] = value
	} else if old != value {
		if _, newKey := d.newKeys[key]; !newKey {
			original, changed := d.original[key]
			if !changed {
				d.original[key] = old
			} else if original == value {
				delete(d.original, key)
			}
		}
		s.values[key] = value
	}
}
func (s *StackedInt32FloatMap) Push() {
	s.stack = append(s.stack, stackedInt32FloatDelta{newKeys: make(map[int32]struct{}), original: make(map[int32]float64)})
}
func (s *StackedInt32FloatMap) Pop(k uint32) {
	for k > 0 && len(s.stack) > 0 {
		last := len(s.stack) - 1
		d := &s.stack[last]
		for key := range d.newKeys {
			delete(s.values, key)
		}
		for key, value := range d.original {
			s.values[key] = value
		}
		s.stack[last] = stackedInt32FloatDelta{}
		s.stack = s.stack[:last]
		k--
	}
}
func (s *StackedInt32FloatMap) Erase(key int32) error {
	if len(s.stack) == 0 {
		delete(s.values, key)
		return nil
	}
	d := &s.stack[len(s.stack)-1]
	value, exists := s.values[key]
	_, newKey := d.newKeys[key]
	if !exists {
		if sourceDebugEnabled && newKey {
			return ErrAssertion
		}
		return nil
	}
	if !newKey {
		if _, changed := d.original[key]; !changed {
			d.original[key] = value
		}
	} else {
		if _, changed := d.original[key]; sourceDebugEnabled && changed {
			return ErrAssertion
		}
		delete(d.newKeys, key)
	}
	delete(s.values, key)
	return nil
}
