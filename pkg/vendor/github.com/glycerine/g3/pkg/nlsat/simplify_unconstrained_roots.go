package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:248-250.

func (s *Solver) originalClausesHaveRootAtom() (bool, error) {
	for _, clause := range s.clauses.originals {
		for _, literal := range clause.Literals() {
			atom, err := s.clauses.literalAtom(literal)
			if err != nil {
				return false, err
			}
			if _, root := atom.(*RootAtom); root {
				return true, nil
			}
		}
	}
	return false, nil
}
