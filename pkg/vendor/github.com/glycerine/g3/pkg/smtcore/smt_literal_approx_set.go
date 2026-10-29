package smtcore

// Source: src/util/approx_set.h:43-154; src/smt/smt_literal.h:114-116.

// SMTLiteralApproxSet is the concrete default unsigned-long-long source
// specialization, an inline eight-byte value with no owning payload. Collision
// predicates are deliberately approximate and do not imply exact membership.
type SMTLiteralApproxSet uint64

func SMTLiteralToUnsigned(l SMTLiteral) uint32 { return uint32(l.Index()) }
func SMTLiteralApproxSingleton(l SMTLiteral) SMTLiteralApproxSet {
	return SMTLiteralApproxSet(1) << (SMTLiteralToUnsigned(l) & 63)
}
func SMTLiteralApproxFrom(literals []SMTLiteral) SMTLiteralApproxSet {
	var s SMTLiteralApproxSet
	for _, l := range literals {
		s.Insert(l)
	}
	return s
}
func (s *SMTLiteralApproxSet) Insert(l SMTLiteral) { *s |= SMTLiteralApproxSingleton(l) }
func (s SMTLiteralApproxSet) MayContain(l SMTLiteral) bool {
	return s&SMTLiteralApproxSingleton(l) != 0
}
func (s SMTLiteralApproxSet) MustNotContain(l SMTLiteral) bool                    { return !s.MayContain(l) }
func (s SMTLiteralApproxSet) Union(other SMTLiteralApproxSet) SMTLiteralApproxSet { return s | other }
func (s SMTLiteralApproxSet) Intersection(other SMTLiteralApproxSet) SMTLiteralApproxSet {
	return s & other
}
func (s *SMTLiteralApproxSet) UnionAssign(other SMTLiteralApproxSet)       { *s |= other }
func (s *SMTLiteralApproxSet) IntersectAssign(other SMTLiteralApproxSet)   { *s &= other }
func (s *SMTLiteralApproxSet) SubtractAssign(other SMTLiteralApproxSet)    { *s &^= other }
func (s SMTLiteralApproxSet) Empty() bool                                  { return s == 0 }
func (s SMTLiteralApproxSet) MustNotSubset(other SMTLiteralApproxSet) bool { return s&^other != 0 }
func (s SMTLiteralApproxSet) MustNotSubsume(other SMTLiteralApproxSet) bool {
	return s.MustNotSubset(other)
}
func (s SMTLiteralApproxSet) MustNotEqual(other SMTLiteralApproxSet) bool { return s != other }
func (s SMTLiteralApproxSet) MayEqual(other SMTLiteralApproxSet) bool     { return s == other }
func (s SMTLiteralApproxSet) Equivalent(other SMTLiteralApproxSet) bool   { return s == other }
func (s SMTLiteralApproxSet) ApproxSubset(other SMTLiteralApproxSet) bool {
	return other.Equivalent(s.Union(other))
}
func (s *SMTLiteralApproxSet) Reset() { *s = 0 }
func (s SMTLiteralApproxSet) EmptyIntersection(other SMTLiteralApproxSet) bool {
	return s.Intersection(other).Empty()
}
