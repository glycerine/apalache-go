package sat

// Source: src/sat/sat_solver.cpp:2091–2219.

func (s *satSolverConflictResolution) getMaxLevel(consequent Literal, j satJustification) (uint32, error) {
	p := s.analysis.propagation
	v := p.variables
	if !p.extensionPresent {
		return v.scopeLevel, nil
	}
	if v.scopeLevel == 0 {
		return 0, nil
	}
	r := uint32(0)
	if consequent != DefaultLiteral() {
		x, e := v.levelVar(consequent.variable())
		if e != nil {
			return r, e
		}
		r = x
	}
	switch j.Kind() {
	case 0:
		return r, nil
	case 1:
		lit, e := j.Literal()
		if e != nil {
			return r, e
		}
		x, e := v.levelVar(lit.variable())
		if x > r {
			r = x
		}
		return r, e
	case 2:
		a, e := j.Literal1()
		if e != nil {
			return r, e
		}
		x, e := v.levelVar(a.variable())
		if e != nil {
			return r, e
		}
		if x > r {
			r = x
		}
		b, e := j.Literal2()
		if e != nil {
			return r, e
		}
		x, e = v.levelVar(b.variable())
		if x > r {
			r = x
		}
		return r, e
	case 3:
		c, e := p.clauses.GetClause(j.ClauseOffset())
		if e != nil {
			return r, e
		}
		i := uint32(0)
		if consequent != DefaultLiteral() {
			first, e := p.clauses.Literal(c, 0)
			if e != nil {
				return r, e
			}
			if satStorageSourceDebug && first != consequent {
				second, e := p.clauses.Literal(c, 1)
				if e != nil {
					return r, e
				}
				if second != consequent {
					return r, ErrSATAssertion
				}
			}
			if first == consequent {
				i = 1
			} else {
				x, e := v.levelVar(first.variable())
				if e != nil {
					return r, e
				}
				if x > r {
					r = x
				}
				i = 2
			}
		}
		n, e := p.clauses.Size(c)
		if e != nil {
			return r, e
		}
		for ; i < n; i++ {
			lit, e := p.clauses.Literal(c, i)
			if e != nil {
				return r, e
			}
			x, e := v.levelVar(lit.variable())
			if e != nil {
				return r, e
			}
			if x > r {
				r = x
			}
		}
		return r, nil
	case 4:
		return r, s.analysis.fillExtAntecedents(consequent, j)
	default:
		return r, ErrSATAssertion
	}
}
func (s *satSolverConflictResolution) skipLiteralsAboveConflictLevel() (uint32, error) {
	v := s.analysis.propagation.variables
	idx, e := v.trail.Size(v.trailRef)
	if e != nil {
		return idx, e
	}
	if idx == 0 {
		return 0, nil
	}
	idx--
	for {
		lit, e := v.trailRead(idx)
		if e != nil {
			return idx, e
		}
		level, e := v.levelVar(lit.variable())
		if e != nil {
			return idx, e
		}
		if level <= s.conflictLevel {
			return idx, nil
		}
		if satStorageSourceDebug && idx == 0 {
			return idx, ErrSATAssertion
		}
		idx--
	}
}
func (s *satSolverConflictResolution) processAntecedent(l Literal, marks *uint32) error {
	a := s.analysis
	v := a.propagation.variables
	id := uint32(l.variable())
	level, e := v.levelVar(l.variable())
	if e != nil {
		return e
	}
	if satStorageSourceDebug {
		n, e := v.numVarsChecked()
		if e != nil {
			return e
		}
		if id >= n {
			return ErrSATAssertion
		}
	}
	marked, e := a.marked(id)
	if e != nil {
		return e
	}
	if !marked && level > 0 {
		if e = a.mark(id); e != nil {
			return e
		}
		if e = v.incActivity(l.variable()); e != nil {
			return e
		}
		if level == s.conflictLevel {
			*marks++
		} else {
			return a.lemma.Push(uint32(l.Negated()))
		}
	}
	return nil
}
func (s *satSolverConflictResolution) forgetPhaseOfVars(from uint32) error {
	l := s.dynamic.probing.lifecycle
	v := s.analysis.propagation.variables
	head := uint32(0)
	if from != 0 {
		i := from - 1
		if uint64(i) >= uint64(len(l.scopes)) {
			return ErrSATAssertion
		}
		head = l.scopes[i].trailLimit
	}
	n, e := v.trail.Size(v.trailRef)
	if e != nil {
		return e
	}
	for i := head; i < n; i++ {
		lit, e := v.trailRead(i)
		if e != nil {
			return e
		}
		s.traceForgetPhase(lit)
		if e = v.setPhase(lit.variable(), 2); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSolverConflictResolution) updatePhaseCounters() error {
	s.phaseCounter++
	cfg, e := s.analysis.propagation.config.State()
	if e != nil {
		return e
	}
	if s.phaseCacheOn {
		if s.phaseCounter >= cfg.PhaseCachingOn {
			s.phaseCounter = 0
			s.phaseCacheOn = false
		}
	} else if s.phaseCounter >= cfg.PhaseCachingOff {
		s.phaseCounter = 0
		s.phaseCacheOn = true
	}
	return nil
}
func (s *satSolverConflictResolution) checkMarks() error {
	v := s.analysis.propagation.variables
	n, e := v.numVarsChecked()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		marked, e := s.analysis.marked(i)
		if e != nil {
			return e
		}
		if marked {
			return ErrSATAssertion
		}
	}
	return nil
}

// mode0=FIRST-UIP,1=init,2=unsat-core. Only the source-identical reason
// traversal is shared; each concrete antecedent method retains its own order.
func (s *satSolverConflictResolution) processConsequent(consequent Literal, j satJustification, mode uint8, marks *uint32) error {
	p := s.analysis.propagation
	if mode == 2 {
		s.traceUnsatCoreConsequent(consequent, j)
	}
	switch j.Kind() {
	case 0:
		return nil
	case 1:
		if mode == 2 && satStorageSourceDebug && consequent == DefaultLiteral() {
			return ErrSATAssertion
		}
		lit, e := j.Literal()
		if e != nil {
			return e
		}
		return s.antecedentMode(lit.Negated(), mode, marks)
	case 2:
		if mode == 2 && satStorageSourceDebug && consequent == DefaultLiteral() {
			return ErrSATAssertion
		}
		a, e := j.Literal1()
		if e != nil {
			return e
		}
		if e = s.antecedentMode(a.Negated(), mode, marks); e != nil {
			return e
		}
		b, e := j.Literal2()
		if e != nil {
			return e
		}
		return s.antecedentMode(b.Negated(), mode, marks)
	case 3:
		c, e := p.clauses.GetClause(j.ClauseOffset())
		if e != nil {
			return e
		}
		i := uint32(0)
		if consequent != DefaultLiteral() {
			first, e := p.clauses.Literal(c, 0)
			if e != nil {
				return e
			}
			if satStorageSourceDebug && first != consequent {
				second, e := p.clauses.Literal(c, 1)
				if e != nil {
					return e
				}
				if second != consequent {
					return ErrSATAssertion
				}
			}
			if first == consequent {
				i = 1
			} else {
				if e = s.antecedentMode(first.Negated(), mode, marks); e != nil {
					return e
				}
				i = 2
			}
		}
		n, e := p.clauses.Size(c)
		if e != nil {
			return e
		}
		for ; i < n; i++ {
			lit, e := p.clauses.Literal(c, i)
			if e != nil {
				return e
			}
			if e = s.antecedentMode(lit.Negated(), mode, marks); e != nil {
				return e
			}
		}
		return nil
	case 4:
		return s.analysis.fillExtAntecedents(consequent, j)
	default:
		return ErrSATAssertion
	}
}
func (s *satSolverConflictResolution) antecedentMode(l Literal, mode uint8, marks *uint32) error {
	switch mode {
	case 0:
		return s.processAntecedent(l, marks)
	case 1:
		return s.processAntecedentForInit(l)
	default:
		return s.processAntecedentForUnsatCore(l)
	}
}
