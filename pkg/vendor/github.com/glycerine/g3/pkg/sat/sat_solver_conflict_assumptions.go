package sat

// Source: src/sat/sat_solver.cpp:898–926,1141–1167,1822–1883,1999–2087,2545–2549.

func (s *satSolverConflictResolution) trackingAssumptions() (bool, error) {
	n, e := s.assumptions.Size()
	if e != nil {
		return false, e
	}
	if n != 0 {
		return true, nil
	}
	n, e = s.userScopeLiterals.Size()
	return n != 0, e
}
func (s *satSolverConflictResolution) isAssumption(l Literal) (bool, error) {
	yes, e := s.trackingAssumptions()
	if e != nil || !yes {
		return false, e
	}
	return s.assumptionSet.contains(l), nil
}
func (s *satSolverConflictResolution) exchangeParallel() error {
	// Source constructor m_par=nullptr, independently of configured num_threads.
	// No actual par instance is supplied yet; nonnull dispatch cannot fake exchange.
	if s.parallelPresent && s.analysis.propagation.variables.scopeLevel == 0 {
		return ErrSATMissingParallelBinding
	}
	return nil
}
func (s *satSolverConflictResolution) popReinit(n uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := s.dynamic.probing.lifecycle.pop(n); e != nil {
		return e
	}
	if e := s.exchangeParallel(); e != nil {
		return e
	}
	return s.reinitAssumptions()
}
func (s *satSolverConflictResolution) reinitAssumptions() error {
	if e := s.open(); e != nil {
		return e
	}
	yes, e := s.trackingAssumptions()
	if e != nil {
		return e
	}
	p := s.analysis.propagation
	if !yes || p.variables.scopeLevel != 0 || p.inconsistent {
		return nil
	}
	s.traceReinitAssumptions()
	if e = s.dynamic.probing.lifecycle.push(); e != nil {
		return e
	}
	for i := uint32(0); !p.inconsistent; i++ {
		n, e := s.userScopeLiterals.Size()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		l, e := s.userScopeLiterals.Read(i)
		if e != nil {
			return e
		}
		if e = p.assign(Literal(l).Negated(), satJustification{}); e != nil {
			return e
		}
	}
	for i := uint32(0); !p.inconsistent; i++ {
		n, e := s.assumptions.Size()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		l, e := s.assumptions.Read(i)
		if e != nil {
			return e
		}
		if e = p.assign(Literal(l), satJustification{}); e != nil {
			return e
		}
	}
	s.traceReinitAntecedents()
	return nil
}
func (s *satSolverConflictResolution) processAntecedentForUnsatCore(l Literal) error {
	a := s.analysis
	v := a.propagation.variables
	id := uint32(l.variable())
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
	s.traceUnsatCoreAntecedent(l, marked)
	if marked {
		return nil
	}
	if e = a.mark(id); e != nil {
		return e
	}
	if e = a.unmark.Push(id); e != nil {
		return e
	}
	yes, e := s.isAssumption(l)
	if e != nil {
		return e
	}
	if yes {
		return s.core.Push(uint32(l))
	}
	return nil
}
func (s *satSolverConflictResolution) resolveConflictForUnsatCore() error {
	a := s.analysis
	p := a.propagation
	v := p.variables
	s.traceUnsatCoreStart()
	if e := s.core.Shrink(0); e != nil {
		return e
	}
	if s.conflictLevel == 0 {
		return nil
	}
	if satStorageSourceDebug {
		n, e := a.unmark.Size()
		if e != nil {
			return e
		}
		if n != 0 {
			return ErrSATAssertion
		}
		n, e = v.trail.Size(v.trailRef)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			lit, e := v.trailRead(i)
			if e != nil {
				return e
			}
			marked, e := a.marked(uint32(lit.variable()))
			if e != nil {
				return e
			}
			if marked {
				return ErrSATAssertion
			}
		}
	}
	old, e := a.unmark.Size()
	if e != nil {
		return e
	}
	u, e := s.skipLiteralsAboveConflictLevel()
	if e != nil {
		return e
	}
	idx := int32(u)
	if p.notLiteral != DefaultLiteral() {
		j, e := v.justification.read(uint32(p.notLiteral.variable()))
		if e != nil {
			return e
		}
		s.traceUnsatCoreNotLiteral(p.notLiteral, j)
		if e = s.processAntecedentForUnsatCore(p.notLiteral); e != nil {
			return e
		}
		yes, e := s.isAssumption(p.notLiteral.Negated())
		if e != nil {
			return e
		}
		if yes {
			if e = s.core.Push(uint32(p.notLiteral.Negated())); e != nil {
				return e
			}
		} else if e = s.processConsequent(p.notLiteral, j, 2, nil); e != nil {
			return e
		}
	}
	consequent, j := p.notLiteral, p.conflict
	for {
		if e = s.processConsequent(consequent, j, 2, nil); e != nil {
			return e
		}
		for idx >= 0 {
			lit, e := v.trailRead(uint32(idx))
			if e != nil {
				return e
			}
			marked, e := a.marked(uint32(lit.variable()))
			if e != nil {
				return e
			}
			if marked {
				break
			}
			idx--
		}
		if idx < 0 {
			break
		}
		consequent, e = v.trailRead(uint32(idx))
		if e != nil {
			return e
		}
		level, e := v.levelVar(consequent.variable())
		if e != nil {
			return e
		}
		if level < s.conflictLevel {
			s.traceUnsatCoreLowerLevel(consequent, level)
			break
		}
		if satStorageSourceDebug && level != s.conflictLevel {
			return ErrSATAssertion
		}
		j, e = v.justification.read(uint32(consequent.variable()))
		if e != nil {
			return e
		}
		idx--
	}
	if e = a.resetUnmark(old); e != nil {
		return e
	}
	cfg, e := p.config.State()
	if e != nil {
		return e
	}
	if cfg.CoreMinimize {
		if s.minCoreValid {
			mn, e := s.minCore.Size()
			if e != nil {
				return e
			}
			cn, e := s.core.Size()
			if e != nil {
				return e
			}
			if mn < cn {
				if e = s.emitCoreUpdateNotice(mn, cn); e != nil {
					return e
				}
				if e = s.core.Shrink(0); e != nil {
					return e
				}
				span := satConflictColumnSpan(&s.minCore)
				for i := uint32(0); i < mn; i++ {
					l, e := span.literal(i)
					if e != nil {
						return e
					}
					if e = s.core.Push(uint32(l)); e != nil {
						return e
					}
				}
			}
		}
		if s.solver == nil {
			return ErrSATMissingMUSBinding
		}
		if _, e := s.solver.mus.run(); e != nil {
			return e
		}
		if e = s.solver.setModel(&s.solver.mus.model); e != nil {
			return e
		}
		return s.emitCoreReport()
	}
	return nil
}
func (s *satSolverConflictResolution) processAntecedentForInit(l Literal) error {
	a := s.analysis
	v := a.propagation.variables
	id := uint32(l.variable())
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
	if marked {
		return nil
	}
	level, e := v.levelVar(l.variable())
	if e != nil {
		return e
	}
	if level == 0 {
		return nil
	}
	if e = a.mark(id); e != nil {
		return e
	}
	return a.lemma.Push(uint32(l.Negated()))
}
func (s *satSolverConflictResolution) resolveConflictForInit() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if s.conflictLevel == 0 {
		return false, nil
	}
	a := s.analysis
	p := a.propagation
	v := p.variables
	if e := a.lemma.Shrink(0); e != nil {
		return false, e
	}
	if e := a.lemma.Push(uint32(DefaultLiteral())); e != nil {
		return false, e
	}
	if p.notLiteral != DefaultLiteral() {
		s.traceInitNotLiteral(p.notLiteral)
		if e := s.processAntecedentForInit(p.notLiteral); e != nil {
			return false, e
		}
	}
	consequent, j := p.notLiteral, p.conflict
	idx, e := v.trail.Size(v.trailRef)
	if e != nil {
		return false, e
	}
	if satStorageSourceDebug && idx == 0 {
		return false, ErrSATAssertion
	}
	for !j.None() {
		if e = s.processConsequent(consequent, j, 1, nil); e != nil {
			return false, e
		}
		for {
			idx--
			l, e := v.trailRead(idx)
			if e != nil {
				return false, e
			}
			marked, e := a.marked(uint32(l.variable()))
			if e != nil {
				return false, e
			}
			if marked {
				break
			}
			if satStorageSourceDebug && idx == 0 {
				return false, ErrSATAssertion
			}
		}
		consequent, e = v.trailRead(idx)
		if e != nil {
			return false, e
		}
		id := uint32(consequent.variable())
		level, e := v.levelVar(consequent.variable())
		if e != nil {
			return false, e
		}
		if level == 0 {
			return false, nil
		}
		if satStorageSourceDebug && s.conflictLevel != 1 {
			return false, ErrSATAssertion
		}
		j, e = v.justification.read(id)
		if e != nil {
			return false, e
		}
		if e = a.resetMark(id); e != nil {
			return false, e
		}
	}
	newLevel := uint32(0)
	if e = a.lemma.Write(0, uint32(consequent.Negated())); e != nil {
		return false, e
	}
	for i := uint32(1); ; i++ {
		n, e := a.lemma.Size()
		if e != nil {
			return false, e
		}
		if i >= n {
			break
		}
		lit, e := a.lemma.Read(i)
		if e != nil {
			return false, e
		}
		id := uint32(Literal(lit).variable())
		marked, e := a.marked(id)
		if e != nil {
			return false, e
		}
		if marked {
			if e = a.resetMark(id); e != nil {
				return false, e
			}
			level, e := v.levelVar(BoolVar(id))
			if e != nil {
				return false, e
			}
			if level > newLevel {
				newLevel = level
			}
		} else {
			last, e := a.lemma.Read(n - 1)
			if e != nil {
				return false, e
			}
			if e = a.lemma.Write(i, last); e != nil {
				return false, e
			}
			if e = a.lemma.Shrink(n - 1); e != nil {
				return false, e
			}
			i--
		}
	}
	s.traceInitLemmaState()
	if newLevel == 0 {
		if e = s.popReinit(v.scopeLevel); e != nil {
			return false, e
		}
	} else {
		if e = s.dynamic.probing.lifecycle.unassignVars(idx); e != nil {
			return false, e
		}
	}
	n, e := a.lemma.Size()
	if e != nil {
		return false, e
	}
	_, e = s.dynamic.probing.lifecycle.mkClauseCoreSpan(n, satConflictColumnSpan(&a.lemma), true)
	if e != nil {
		return false, e
	}
	s.traceInitTrail()
	p.inconsistent = false
	return true, nil
}

// Explicit checked owner-domain accessor useful for source consumer bindings.
func (s *satSolverConflictResolution) coreLiteral(i uint32) (Literal, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	v, e := s.core.Read(i)
	return Literal(v), e
}
