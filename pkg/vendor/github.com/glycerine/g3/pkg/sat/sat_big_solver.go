package sat

// Source: src/sat/sat_big.cpp:38-70.

func (b *satBIG) initSolver(s *Solver, learned bool) error {
	if err := s.open(); err != nil {
		return err
	}
	return b.initLifecycle(s.Lifecycle, learned)
}

func (b *satBIG) initLifecycle(l *satSolverClauseLifecycle, learned bool) error {
	if err := l.open(); err != nil {
		return err
	}
	n, err := l.propagation.variables.numVarsChecked()
	if err != nil {
		return err
	}
	if err = b.initAddingEdges(n, learned); err != nil {
		return err
	}
	for idx := uint32(0); idx < b.numLiterals(); idx++ {
		eliminated, e := l.propagation.variables.wasEliminated(Literal(idx).variable())
		if e != nil {
			return e
		}
		if eliminated {
			continue
		}
		list, e := l.propagation.variables.watches.Get(idx)
		if e != nil {
			return e
		}
		ref, e := list.ref()
		if e != nil {
			return e
		}
		it, e := list.owner.store.Iter(ref)
		if e != nil {
			return e
		}
		for {
			w, yes, e := it.Next()
			if e != nil {
				return e
			}
			if !yes {
				break
			}
			if !w.Binary() {
				continue
			}
			isLearned, e := w.Learned()
			if e != nil {
				return e
			}
			if !learned && isLearned {
				continue
			}
			v, e := w.Literal()
			if e != nil {
				return e
			}
			if e = b.addSolverEdge(Literal(idx), v); e != nil {
				return e
			}
		}
	}
	return b.doneAddingEdges()
}
