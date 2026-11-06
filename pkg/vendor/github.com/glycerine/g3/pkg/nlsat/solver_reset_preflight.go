package nlsat

// Source: src/nlsat/nlsat_solver.cpp:222-230,504-508.

// resetAtomReferences verifies the source precondition for del_unref_atoms:
// after deleting all clauses every arithmetic atom must have reference count
// zero. Explicit host references therefore block reset without dangling them.
func (s *Solver) resetAtomReferences() error {
	clauseRefs := make(map[ArithmeticAtom]uint32)
	for _, clauses := range [][]*Clause{s.clauses.originals, s.clauses.learned} {
		for _, clause := range clauses {
			for _, literal := range clause.Literals() {
				atom := s.atoms.bools.atoms[literal.Variable()]
				if atom != nil {
					clauseRefs[atom]++
				}
			}
		}
	}
	for _, atom := range s.atoms.bools.atoms {
		if atom == nil {
			continue
		}
		if arithmeticAtomHeader(atom).refs != clauseRefs[atom] {
			return ErrAtomCacheLive
		}
	}
	return nil
}
