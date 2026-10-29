package nlsat

// Source: src/nlsat/nlsat_solver.cpp:1190-1204.

import "github.com/glycerine/g3/pkg/arena"

// selectWitness chooses from the accumulated domain's complement. Integer
// checking is a later search_check operation: a nonempty integer domain can
// still select a fractional or irrational witness, exactly as in the source.
func (r *searchState) selectWitness() (err error) {
	if err = r.open(); err != nil {
		return err
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	if uint64(r.xk) >= uint64(len(r.store.vars.infeasible)) {
		return arena.ErrBounds
	}
	r.witnessActive = true
	s := r.store.vars.infeasible[r.xk]
	full, err := r.ism.IsFull(s)
	if err != nil {
		return err
	}
	if err = atomAssert("nlsat_solver.cpp", 1192, "!m_ism.is_full(m_infeasible[m_xk])", !full); err != nil {
		return err
	}
	if err = r.ism.PeekInComplement(s, r.store.vars.isInt[r.xk], &r.witness, r.config.Randomize); err != nil {
		return err
	}
	rational, err := r.evaluator.am.IsRational(&r.witness)
	if err != nil {
		return err
	}
	if !rational {
		r.irrationalAssignments++
	}
	// SetCore swaps: witness now owns the previous (possibly hidden) slot.
	// The source scoped_anum destructor must delete that old value, not the
	// newly assigned one. The statistic precedes the potentially failing SET.
	if err = r.assignment.SetCore(r.xk, &r.witness); err != nil {
		return err
	}
	if err = r.evaluator.am.Del(&r.witness); err != nil {
		return err
	}
	r.witnessActive = false
	return nil
}
