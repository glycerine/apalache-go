package sat

// Source: src/sat/sat_types.h:153-289.

// MUS and consequence sets use the same concrete native literal storage as
// probing. Set operations preserve the source insertion/compaction order.
type satMUSLiteralSet struct{ satLiteralSetStorage }

func (s *satMUSLiteralSet) insert(l Literal) error { return s.Insert(l) }
func (s *satMUSLiteralSet) reset()                 { s.Reset() }
func (s *satMUSLiteralSet) close()                 { s.Finalize() }
func (s *satMUSLiteralSet) intersect(other *satMUSLiteralSet) {
	j := 0
	for _, v := range s.values {
		if other.contains(v) {
			s.values[j] = v
			j++
		} else {
			s.membership[v.index()] = false
		}
	}
	s.values = s.values[:j]
}
func (s *satMUSLiteralSet) union(other *satMUSLiteralSet) error {
	for _, v := range other.values {
		if e := s.insert(v); e != nil {
			return e
		}
	}
	return nil
}
func (s *satMUSLiteralSet) assign(other *satMUSLiteralSet) {
	if s == other {
		return
	}
	s.membership = append([]bool(nil), other.membership...)
	s.values = append([]Literal(nil), other.values...)
}
