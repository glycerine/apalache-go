package nlsat

// Source: src/nlsat/nlsat_solver.cpp:223-225.

func (q *searchResolution) reset() error {
	if q == nil || q.search == nil {
		return ErrSearchOwner
	}
	if err := q.resetLemmaAssumptions(); err != nil {
		return err
	}
	if err := q.lemma.reset(); err != nil {
		return err
	}
	if err := q.lazy.reset(); err != nil {
		return err
	}
	for i := range q.marks {
		q.marks[i] = false
	}
	q.numMarks = 0
	return nil
}
