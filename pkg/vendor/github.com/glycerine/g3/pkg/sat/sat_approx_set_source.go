package sat

// Source: src/util/approx_set.h:43-158; src/sat/sat_types.h:115-119;
// src/sat/sat_solver.h:419; src/smt/smt_conflict_resolution.h:34.

// All production 32-bit-word aliases use this concrete inline word. Unsigned
// variables/levels use identity conversion; SAT literals use their full index.
// Range constructors borrow input and retain no storage or reference counts.
func newSATApproxSet(element uint32) satApproxSet {
	return satApproxSet(1) << (element & 31)
}
func satApproxSetFromUnsigned(elements []uint32) satApproxSet {
	var s satApproxSet
	for _, element := range elements {
		s.insert(element)
	}
	return s
}
func satApproxSetFromVariables(elements []BoolVar) satApproxSet {
	var s satApproxSet
	for _, element := range elements {
		s.insert(uint32(element))
	}
	return s
}
func satApproxSetFromLiterals(elements []Literal) satApproxSet {
	var s satApproxSet
	for _, element := range elements {
		s.insertLiteral(element)
	}
	return s
}
func (s *satApproxSet) insertLiteral(element Literal) { s.insert(element.index()) }
func (s satApproxSet) mayContainLiteral(element Literal) bool {
	return s.mayContain(element.index())
}
func (s satApproxSet) unionValue(other satApproxSet) satApproxSet        { return s | other }
func (s satApproxSet) intersectionValue(other satApproxSet) satApproxSet { return s & other }

// This source alias has the same direction as must_not_subset.
func (s satApproxSet) mustNotSubsume(other satApproxSet) bool { return s.mustNotSubset(other) }
func (s satApproxSet) mayEqual(other satApproxSet) bool       { return s == other }
func (s satApproxSet) mustNotEqual(other satApproxSet) bool   { return s != other }
func (s satApproxSet) equivalent(other satApproxSet) bool     { return s == other }
