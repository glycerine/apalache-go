package sat

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source sat_solver.cpp:2238-2253. In particular mark precedes both native work-list
// appends. Checked errors preserve completed mark/work-list publication.

func (s *satSolverConflictAnalysis) processAntecedentForMinimization(l Literal) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	v := uint32(l.variable())
	level, e := s.propagation.variables.levelVar(BoolVar(v))
	if e != nil {
		return false, e
	}
	marked, e := s.marked(v)
	if e != nil {
		return false, e
	}
	if !marked && level > 0 {
		if !s.levelSet.mayContain(level) {
			return false, nil
		}
		if e = s.mark(v); e != nil {
			return false, e
		}
		if e = s.unmark.Push(v); e != nil {
			return false, e
		}
		if e = s.minStack.Push(v); e != nil {
			return false, e
		}
	}
	return true, nil
}

// Source sat_solver.cpp:2262-2334, preserving the explicit stack and the
// short-circuiting antecedent tests. A false result resets only its new suffix;
// allocation/checked-domain errors are not false and perform no mark repair.
func (s *satSolverConflictAnalysis) impliedByMarked(l Literal) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if e := s.minStack.Shrink(0); e != nil {
		return false, e
	}
	if e := s.minStack.Push(uint32(l.variable())); e != nil {
		return false, e
	}
	old, e := s.unmark.Size()
	if e != nil {
		return false, e
	}
	for {
		n, e := s.minStack.Size()
		if e != nil {
			return false, e
		}
		if n == 0 {
			break
		}
		v, e := s.minStack.Read(n - 1)
		if e != nil {
			return false, e
		}
		if e = s.minStack.Shrink(n - 1); e != nil {
			return false, e
		}
		j, e := s.propagation.variables.justification.read(v)
		if e != nil {
			return false, e
		}
		switch j.Kind() {
		case 0:
			level, e := s.propagation.variables.levelVar(BoolVar(v))
			if e != nil {
				return false, e
			}
			if level > 0 {
				if e = s.resetUnmark(old); e != nil {
					return false, e
				}
				return false, nil
			}
		case 1:
			a, e := j.Literal()
			if e != nil {
				return false, e
			}
			ok, e := s.processAntecedentForMinimization(a.Negated())
			if e != nil {
				return false, e
			}
			if !ok {
				if e = s.resetUnmark(old); e != nil {
					return false, e
				}
				return false, nil
			}
		case 2:
			a, e := j.Literal1()
			if e != nil {
				return false, e
			}
			ok, e := s.processAntecedentForMinimization(a.Negated())
			if e != nil {
				return false, e
			}
			if !ok {
				if e = s.resetUnmark(old); e != nil {
					return false, e
				}
				return false, nil
			}
			a, e = j.Literal2()
			if e != nil {
				return false, e
			}
			ok, e = s.processAntecedentForMinimization(a.Negated())
			if e != nil {
				return false, e
			}
			if !ok {
				if e = s.resetUnmark(old); e != nil {
					return false, e
				}
				return false, nil
			}
		case 3:
			a := s.propagation.clauses
			c, e := a.GetClause(j.ClauseOffset())
			if e != nil {
				return false, e
			}
			first, e := a.Literal(c, 0)
			if e != nil {
				return false, e
			}
			i := uint32(0)
			if uint32(first.variable()) == v {
				i = 1
			} else {
				if satStorageSourceDebug {
					second, e := a.Literal(c, 1)
					if e != nil {
						return false, e
					}
					if uint32(second.variable()) != v {
						return false, ErrSATAssertion
					}
				}
				ok, e := s.processAntecedentForMinimization(first.Negated())
				if e != nil {
					return false, e
				}
				if !ok {
					if e = s.resetUnmark(old); e != nil {
						return false, e
					}
					return false, nil
				}
				i = 2
			}
			n, e := a.Size(c)
			if e != nil {
				return false, e
			}
			for ; i < n; i++ {
				l, e := a.Literal(c, i)
				if e != nil {
					return false, e
				}
				ok, e := s.processAntecedentForMinimization(l.Negated())
				if e != nil {
					return false, e
				}
				if !ok {
					if e = s.resetUnmark(old); e != nil {
						return false, e
					}
					return false, nil
				}
			}
		case 4:
			value, e := s.propagation.variables.valueVar(BoolVar(v))
			if e != nil {
				return false, e
			}
			consequent := MakeLiteral(BoolVar(v), value == lbool.False)
			if e = s.fillExtAntecedents(consequent, j); e != nil {
				return false, e
			}
			// The actual extension dispatch above remains unimplemented, so this
			// captured source loop is unreachable until a real binding exists.
			n, e := s.extAntecedents.Size()
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				l, e := s.extAntecedents.Read(i)
				if e != nil {
					return false, e
				}
				ok, e := s.processAntecedentForMinimization(Literal(l))
				if e != nil {
					return false, e
				}
				if !ok {
					if e = s.resetUnmark(old); e != nil {
						return false, e
					}
					return false, nil
				}
			}
		default:
			return false, ErrSATAssertion
		}
	}
	return true, nil
}
func (s *satSolverConflictAnalysis) resetUnmark(old uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	n, e := s.unmark.Size()
	if e != nil {
		return e
	}
	for i := old; i < n; i++ {
		v, e := s.unmark.Read(i)
		if e != nil {
			return e
		}
		if e = s.resetMark(v); e != nil {
			return e
		}
	}
	return s.unmark.Shrink(old)
}
func (s *satSolverConflictAnalysis) updateLemmaLevelSet() error {
	if e := s.open(); e != nil {
		return e
	}
	s.levelSet.reset()
	n, e := s.lemma.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		l, e := s.lemma.Read(i)
		if e != nil {
			return e
		}
		level, e := s.propagation.variables.levelVar(Literal(l).variable())
		if e != nil {
			return e
		}
		s.levelSet.insert(level)
	}
	return nil
}
func (s *satSolverConflictAnalysis) minimizeLemma() error {
	if e := s.open(); e != nil {
		return e
	}
	if satStorageSourceDebug {
		n, e := s.unmark.Size()
		if e != nil {
			return e
		}
		if n != 0 {
			return ErrSATAssertion
		}
	}
	if e := s.updateLemmaLevelSet(); e != nil {
		return e
	}
	n, e := s.lemma.Size()
	if e != nil {
		return e
	}
	j := uint32(1)
	for i := uint32(1); i < n; i++ {
		l, e := s.lemma.Read(i)
		if e != nil {
			return e
		}
		ok, e := s.impliedByMarked(Literal(l))
		if e != nil {
			return e
		}
		if ok {
			s.traceDroppedLemmaLiteral(Literal(l))
			if e = s.unmark.Push(uint32(Literal(l).variable())); e != nil {
				return e
			}
		} else {
			if j != i {
				l, e = s.lemma.Read(i)
				if e != nil {
					return e
				}
				if e = s.lemma.Write(j, l); e != nil {
					return e
				}
			}
			j++
		}
	}
	if e = s.resetUnmark(0); e != nil {
		return e
	}
	if e = s.lemma.Shrink(j); e != nil {
		return e
	}
	s.minimizedLits += n - j
	return nil
}

// The defined source iterator route requires a nonempty lemma. An empty
// DEBUG dereference/empty release iterator continuation is undefined in C++;
// checked reads bound that domain rather than inventing an empty source lemma.
func (s *satSolverConflictAnalysis) resetLemmaVarMarks() error {
	if e := s.open(); e != nil {
		return e
	}
	if satStorageSourceDebug {
		l, e := s.lemma.Read(0)
		if e != nil {
			return e
		}
		marked, e := s.marked(uint32(Literal(l).variable()))
		if e != nil {
			return e
		}
		if marked {
			return ErrSATAssertion
		}
	}
	n, e := s.lemma.Size()
	if e != nil {
		return e
	}
	for i := uint32(1); i < n; i++ {
		l, e := s.lemma.Read(i)
		if e != nil {
			return e
		}
		if e = s.resetMark(uint32(Literal(l).variable())); e != nil {
			return e
		}
	}
	return nil
}
