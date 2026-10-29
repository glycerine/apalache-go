package sat

// uint32 SAT specialization of src/util/approx_set.h; unsigned low 5 bits.

type satApproxSet uint32

func (s *satApproxSet) insert(u uint32)                      { *s |= 1 << (u & 31) }
func (s satApproxSet) mayContain(u uint32) bool              { return s&(1<<(u&31)) != 0 }
func (s satApproxSet) mustNotContain(u uint32) bool          { return !s.mayContain(u) }
func (s *satApproxSet) union(other satApproxSet)             { *s |= other }
func (s *satApproxSet) intersect(other satApproxSet)         { *s &= other }
func (s *satApproxSet) subtract(other satApproxSet)          { *s &^= other }
func (s satApproxSet) empty() bool                           { return s == 0 }
func (s satApproxSet) mustNotSubset(other satApproxSet) bool { return s&^other != 0 }

// The source must_not_subsume alias uses this same direction.
func (s satApproxSet) approxSubset(other satApproxSet) bool      { return s|other == other }
func (s *satApproxSet) reset()                                   { *s = 0 }
func (s satApproxSet) emptyIntersection(other satApproxSet) bool { return s&other == 0 }
