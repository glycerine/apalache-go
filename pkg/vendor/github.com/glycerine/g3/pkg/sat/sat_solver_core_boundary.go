package sat

// Source: src/sat/sat_solver.h get_core; retained DIMACS frontend consumer.
func (s *Solver) AppendCore(dst []Literal) ([]Literal, error) {
	view, err := s.BorrowCore()
	if err != nil {
		return dst, err
	}
	n, err := view.Size()
	if err != nil {
		return dst, err
	}
	for i := uint32(0); i < n; i++ {
		lit, err := view.Read(i)
		if err != nil {
			return dst, err
		}
		dst = append(dst, lit)
	}
	return dst, nil
}
