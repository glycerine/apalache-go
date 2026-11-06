package sat

// Source: src/sat/sat_types.h:153-289.

// MUS and consequence sets use the same concrete native literal storage as
// probing. Set operations preserve the source insertion/compaction order.
type satMUSLiteralSet struct{ satLiteralSetStorage }

func (s *satMUSLiteralSet) insert(l Literal) error { return s.Insert(l) }
func (s *satMUSLiteralSet) reset()                 { s.Reset() }
func (s *satMUSLiteralSet) close()                 { s.Finalize() }
func (s *satMUSLiteralSet) intersect(other *satMUSLiteralSet) {
	s.Intersect(&other.satLiteralSetStorage)
}
func (s *satMUSLiteralSet) union(other *satMUSLiteralSet) error {
	return s.Union(&other.satLiteralSetStorage)
}
func (s *satMUSLiteralSet) assign(other *satMUSLiteralSet) {
	s.Assign(&other.satLiteralSetStorage)
}
