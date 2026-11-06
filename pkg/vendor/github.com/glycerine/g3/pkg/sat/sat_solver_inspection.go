package sat

// Source: src/sat/sat_solver.cpp:2818-2838,2852-2857,2901-2924; src/sat/sat_solver.h:286,543,566.

func (s *Solver) numClauses() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	n, e := s.Variables.trail.Size(s.Variables.trailRef)
	if e != nil {
		return 0, e
	}
	for idx := 0; idx < len(s.Variables.watches.lists); idx++ {
		l := Literal(uint32(idx)).Negated()
		list, e := s.Variables.watches.Get(uint32(idx))
		if e != nil {
			return 0, e
		}
		r, e := list.ref()
		if e != nil {
			return 0, e
		}
		it, e := s.Variables.watches.store.Iter(r)
		if e != nil {
			return 0, e
		}
		for {
			w, yes, e := it.Next()
			if e != nil {
				return 0, e
			}
			if !yes {
				break
			}
			if w.Binary() {
				l2, e := w.Literal()
				if e != nil {
					return 0, e
				}
				if l.index() < l2.index() {
					n++
				}
			}
		}
	}
	n += uint32(len(s.Lifecycle.Original))
	n += uint32(len(s.Lifecycle.learned))
	return n, nil
}
func (s *Solver) checkMarks() error {
	if e := s.open(); e != nil {
		return e
	}
	for v := uint32(0); ; v++ {
		n, e := s.Variables.numVarsChecked()
		if e != nil {
			return e
		}
		if v >= n {
			break
		}
		if satStorageSourceDebug {
			marked, e := s.conflict.analysis.marked(v)
			if e != nil {
				return e
			}
			if marked {
				return ErrSATAssertion
			}
		}
	}
	return nil
}

func (s *Solver) collectBinClauses(out []BinaryLiteralPair, learned bool) ([]BinaryLiteralPair, error) {
	if e := s.open(); e != nil {
		return out, e
	}
	sz := len(s.Variables.watches.lists)
	for idx := 0; idx < sz; idx++ {
		l := Literal(uint32(idx)).Negated()
		list, e := s.Variables.watches.Get(uint32(idx))
		if e != nil {
			return out, e
		}
		r, e := list.ref()
		if e != nil {
			return out, e
		}
		it, e := s.Variables.watches.store.Iter(r)
		if e != nil {
			return out, e
		}
		for {
			w, yes, e := it.Next()
			if e != nil {
				return out, e
			}
			if !yes {
				break
			}
			if !w.Binary() {
				continue
			}
			if !learned {
				yes, e := w.Learned()
				if e != nil {
					return out, e
				}
				if yes {
					continue
				}
			}
			l2, e := w.Literal()
			if e != nil {
				return out, e
			}
			if l.index() > l2.index() {
				continue
			}
			s.traceCollectedBinClause(l, l2)
			out = append(out, newSATSolverBinPair(l, l2))
		}
	}
	return out, nil
}
