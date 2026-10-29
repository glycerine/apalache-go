package simplex

import "math/big"

// RecordBoundConflictDependencies retains a failed endpoint update's logical
// premises. The owner decides whether this tableau represents logical replay
// or a temporary candidate; recording never changes the SMT context.
func (s *Solver) RecordBoundConflictDependencies(dependencies []BoundDependency, known bool) {
	s.boundConflict, s.boundConflictKnown = nil, known
	if known {
		s.boundConflict = cloneBoundDependencies(dependencies)
	}
}

func (s *Solver) BoundConflictDependencies() ([]BoundDependency, bool) {
	return cloneBoundDependencies(s.boundConflict), s.boundConflictKnown
}

// BoundDependency is a weighted literal index or a nonzero equality-premise
// identifier supplied by the arithmetic owner. The tableau retains identifiers
// without borrowing context nodes. Equality zero selects the literal variant.
type BoundDependency struct {
	Literal     int32
	Equality    uint32
	Coefficient big.Rat
}

// ArithmeticBoundUse identifies an endpoint used by an arithmetic deduction.
// Its coefficient is relative to the normalized tableau row.
type ArithmeticBoundUse struct {
	Column      uint32
	Lower       bool
	Coefficient big.Rat
}

func cloneBoundDependencies(dependencies []BoundDependency) []BoundDependency {
	result := make([]BoundDependency, len(dependencies))
	for i := range dependencies {
		result[i].Literal = dependencies[i].Literal
		result[i].Equality = dependencies[i].Equality
		result[i].Coefficient.Set(&dependencies[i].Coefficient)
	}
	return result
}

// SetLowerWithDependencies installs a logical bound. An empty dependency
// list denotes an unconditional bound; ordinary SetLower leaves it unexplained.
func (s *Solver) SetLowerWithDependencies(column uint32, bound Infinitesimal, dependencies []BoundDependency) error {
	if err := s.SetLower(column, bound); err != nil {
		return err
	}
	x := &s.vars[column]
	x.lowerDependencies = cloneBoundDependencies(dependencies)
	x.lowerExplained = true
	return nil
}

func (s *Solver) SetUpperWithDependencies(column uint32, bound Infinitesimal, dependencies []BoundDependency) error {
	if err := s.SetUpper(column, bound); err != nil {
		return err
	}
	x := &s.vars[column]
	x.upperDependencies = cloneBoundDependencies(dependencies)
	x.upperExplained = true
	return nil
}

// BoundDependencies returns an owning copy, and distinguishes an unconditional
// endpoint from a bound without a logical explanation (such as a model guess).
func (s *Solver) BoundDependencies(column uint32, lower bool) ([]BoundDependency, bool) {
	if int(column) >= len(s.vars) {
		return nil, false
	}
	x := &s.vars[column]
	if lower {
		return cloneBoundDependencies(x.lowerDependencies), x.lowerValid && x.lowerExplained
	}
	return cloneBoundDependencies(x.upperDependencies), x.upperValid && x.upperExplained
}
