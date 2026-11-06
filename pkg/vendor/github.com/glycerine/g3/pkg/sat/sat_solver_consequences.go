package sat

// Source: src/sat/sat_solver.cpp:3252–3471; src/sat/sat_solver.h:477–503.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *Solver) consequenceEntry(out *satSolverLiteralRows) error {
	if e := s.open(); e != nil {
		return e
	}
	return out.checkRuntime(s.runtime.memory)
}
func (s *Solver) getConsequences(asms []Literal, vars []BoolVar, out *satSolverLiteralRows) (lbool.LBool, error) {
	return s.getConsequencesInputs(satConsequenceLiterals(asms), satConsequenceInput{variables: vars, count: uint32(len(vars))}, out)
}
func (s *Solver) getConsequencesInputs(asms, vars satConsequenceInput, out *satSolverLiteralRows) (result lbool.LBool, err error) {
	if err = s.consequenceEntry(out); err != nil {
		return lbool.Undef, err
	}
	s.consequenceRunning = true
	defer func() { s.consequenceRunning = false }()
	f, e := s.consequenceFrame()
	if e != nil {
		return lbool.Undef, e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	if err = f.vector(s, 0, nil); err != nil {
		return lbool.Undef, err
	}
	result = lbool.True
	cfg, e := s.config.State()
	if e != nil {
		return lbool.Undef, e
	}
	if cfg.RestartMax != ^uint32(0) && !s.Variables.modelIsCurrent {
		return s.getBoundedConsequencesBody(asms, vars, out)
	}
	if !s.Variables.modelIsCurrent {
		n, e := asms.size()
		if e != nil {
			return lbool.Undef, e
		}
		result, err = s.checkSpan(n, asms.cPtr())
		if err != nil {
			return lbool.Undef, err
		}
	}
	if result != lbool.True {
		return result, nil
	}
	if err = f.vector(s, 1, &s.model); err != nil {
		return lbool.Undef, err
	}
	for i := uint32(0); ; i++ {
		n, e := vars.size()
		if e != nil {
			return lbool.Undef, e
		}
		if i >= n {
			break
		}
		v, e := vars.read(i)
		if e != nil {
			return lbool.Undef, e
		}
		// Source get_model()[v] reads the live member, not saved mdl.
		value, e := s.model.Read(v)
		if e != nil {
			return lbool.Undef, e
		}
		if lbool.LBool(int32(value)) == lbool.True {
			err = f.vectors[0].Push(MakeLiteral(BoolVar(v), false).index())
		} else if lbool.LBool(int32(value)) == lbool.False {
			err = f.vectors[0].Push(MakeLiteral(BoolVar(v), true).index())
		}
		if err != nil {
			return lbool.Undef, err
		}
	}
	// The literal source false&&asms.empty() core_chunking dispatch remains
	// unreachable; that separate static algorithm family is explicitly pending.
	result, err = s.getLiteralConsequencesBody(asms, satConsequenceColumn(&f.vectors[0]), out)
	if err != nil {
		return result, err
	}
	if err = s.setModel(&f.vectors[1]); err != nil {
		return result, err
	}
	return result, nil
}
func (s *Solver) getLiteralConsequences(asms, lits satConsequenceInput, out *satSolverLiteralRows) (result lbool.LBool, err error) {
	if err = s.consequenceEntry(out); err != nil {
		return lbool.Undef, err
	}
	s.consequenceRunning = true
	defer func() { s.consequenceRunning = false }()
	return s.getLiteralConsequencesBody(asms, lits, out)
}
func (s *Solver) consequenceInitialize(asms satConsequenceInput) (result lbool.LBool, err error) {
	if err = s.popToBase(); err != nil {
		return lbool.Undef, err
	}
	if s.propagation.inconsistent {
		return lbool.False, nil
	}
	if err = s.gc.initSearch(); err != nil {
		return lbool.Undef, err
	}
	s.traceSearchInitialized()
	if _, err = s.Lifecycle.propagate(false); err != nil {
		return lbool.Undef, err
	}
	if s.propagation.inconsistent {
		return lbool.False, nil
	}
	n, e := asms.size()
	if e != nil {
		return lbool.Undef, e
	}
	if n == 0 {
		v, e := s.MkVar(true, false)
		if e != nil {
			return lbool.Undef, e
		}
		lit := [1]Literal{MakeLiteral(v, false)}
		err = s.initAssumptionsSpan(1, satSolverLiteralSpan{literals: lit[:]})
	} else {
		err = s.initAssumptionsSpan(n, asms.cPtr())
	}
	if err != nil {
		return lbool.Undef, err
	}
	if _, err = s.Lifecycle.propagate(false); err != nil {
		return lbool.Undef, err
	}
	bad, e := s.checkInconsistent()
	if e != nil {
		return lbool.Undef, e
	}
	if bad {
		return lbool.False, nil
	}
	return lbool.True, nil
}
func (s *Solver) getLiteralConsequencesBody(asms, lits satConsequenceInput, out *satSolverLiteralRows) (result lbool.LBool, err error) {
	s.traceConsequenceAssumptions("get_consequences", 3366, asms)
	// Source reset precedes actual unfixed_lits, assumptions, bool_var_set ctors.
	if err = s.antecedents.reset(); err != nil {
		return lbool.Undef, err
	}
	f, e := s.consequenceFrame()
	if e != nil {
		return lbool.Undef, e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	if err = f.set(s, 0, &lits); err != nil {
		return lbool.Undef, err
	}
	if err = f.set(s, 1, &asms); err != nil {
		return lbool.Undef, err
	}
	if err = f.set(s, 2, nil); err != nil {
		return lbool.Undef, err
	}
	unfixed, assumptions, vars := &f.sets[0], &f.sets[1], &f.sets[2]
	for i := uint32(0); ; i++ {
		n, e := lits.size()
		if e != nil {
			return lbool.Undef, e
		}
		if i >= n {
			break
		}
		v, e := lits.read(i)
		if e != nil {
			return lbool.Undef, e
		}
		if err = vars.insert(Literal(uint32(Literal(v).variable()))); err != nil {
			return lbool.Undef, err
		}
	}
	result, err = s.consequenceInitialize(asms)
	if err != nil || result != lbool.True {
		return result, err
	}
	iterations := uint32(0)
	if err = s.extractFixedSet(unfixed, assumptions, vars, out); err != nil {
		return lbool.Undef, err
	}
	if err = s.updateUnfixed(unfixed, vars); err != nil {
		return lbool.Undef, err
	}
	for {
		n := unfixed.size()
		if n == 0 {
			return lbool.True, nil
		}
		if s.Variables.scopeLevel > 1 {
			if err = s.Lifecycle.pop(s.Variables.scopeLevel - 1); err != nil {
				return lbool.Undef, err
			}
		}
		if _, err = s.Lifecycle.propagate(false); err != nil {
			return lbool.Undef, err
		}
		iterations++
		if err = s.propagation.checkpoint(); err != nil {
			return lbool.Undef, err
		}
		span := satSolverLiteralSpan{literals: unfixed.values}
		n = unfixed.size()
		resolves := uint32(0)
		fixed := uint32(0)
		assigned := uint32(0)
		isSat := lbool.True
		for i := uint32(0); i < n; i++ {
			lit, e := span.literal(i)
			if e != nil {
				return lbool.Undef, e
			}
			value, e := s.Variables.valueLiteral(lit)
			if e != nil {
				return lbool.Undef, e
			}
			if value != lbool.Undef {
				fixed++
				level, e := s.Variables.levelVar(lit.variable())
				if e != nil {
					return lbool.Undef, e
				}
				if level <= 1 {
					value, e = s.Variables.valueLiteral(lit)
					if e != nil {
						return lbool.Undef, e
					}
					if value == lbool.True {
						if err = s.extractFixedConsequence(lit, assumptions, vars, out); err != nil {
							return lbool.Undef, err
						}
					}
				}
				continue
			}
			if err = s.Lifecycle.push(); err != nil {
				return lbool.Undef, err
			}
			assigned++
			if err = s.propagation.assign(lit.Negated(), satJustification{}); err != nil {
				return lbool.Undef, err
			}
			if _, err = s.Lifecycle.propagate(false); err != nil {
				return lbool.Undef, err
			}
			for s.propagation.inconsistent {
				yes, e := s.conflict.resolveConflict()
				if e != nil {
					return lbool.Undef, e
				}
				if !yes {
					s.traceConsequenceInconsistent()
					s.propagation.inconsistent = false
					isSat = lbool.Undef
					break
				}
				if _, err = s.Lifecycle.propagate(false); err != nil {
					return lbool.Undef, err
				}
				resolves++
			}
			// Literal source false&&scope_lvl()==1 branch remains unreachable.
		}
		_ = assigned // Source verbose-only counters; no fake report output.
		if err = s.extractFixedSet(unfixed, assumptions, vars, out); err != nil {
			return lbool.Undef, err
		}
		if isSat == lbool.True {
			if s.Variables.scopeLevel == 1 && resolves > 0 {
				if err = s.emitConsequenceBackjump(); err != nil {
					return lbool.Undef, err
				}
				isSat = lbool.Undef
			} else {
				isSat, err = s.boundedSearch()
				if err != nil {
					return lbool.Undef, err
				}
				if isSat == lbool.Undef {
					if err = s.gc.restart(); err != nil {
						return lbool.Undef, err
					}
				}
				if err = s.extractFixedSet(unfixed, assumptions, vars, out); err != nil {
					return lbool.Undef, err
				}
			}
		}
		if isSat == lbool.False {
			s.traceConsequenceUnsat()
			s.propagation.inconsistent = false
		}
		if isSat == lbool.True {
			if err = s.deleteUnfixed(unfixed, vars); err != nil {
				return lbool.Undef, err
			}
		}
		if err = s.updateUnfixed(unfixed, vars); err != nil {
			return lbool.Undef, err
		}
		if err = s.emitConsequenceProgress(iterations, unfixed, out, isSat, fixed, lits); err != nil {
			return lbool.Undef, err
		}
		n = unfixed.size()
		if n != 0 {
			cfg, e := s.config.State()
			if e != nil {
				return lbool.Undef, e
			}
			if cfg.RestartMax <= iterations {
				return lbool.Undef, nil
			}
		}
	}
}
func (s *Solver) getBoundedConsequences(asms, vars satConsequenceInput, out *satSolverLiteralRows) (result lbool.LBool, err error) {
	if err = s.consequenceEntry(out); err != nil {
		return lbool.Undef, err
	}
	s.consequenceRunning = true
	defer func() { s.consequenceRunning = false }()
	return s.getBoundedConsequencesBody(asms, vars, out)
}
func (s *Solver) boundedExtract(start *uint32, asms satConsequenceInput, vars *satMUSLiteralSet, out *satSolverLiteralRows) (err error) {
	// The literal_set(asms) converting temporary is a real full-expression owner
	// for EACH source call, destroyed before the next simplify/search action.
	f, e := s.consequenceFrame()
	if e != nil {
		return e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	if err = f.set(s, 0, &asms); err != nil {
		return err
	}
	return s.extractFixedTrail(start, &f.sets[0], vars, out)
}
func (s *Solver) getBoundedConsequencesBody(asms, vars satConsequenceInput, out *satSolverLiteralRows) (result lbool.LBool, err error) {
	f, e := s.consequenceFrame()
	if e != nil {
		return lbool.Undef, e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	if err = f.set(s, 0, nil); err != nil {
		return lbool.Undef, err
	}
	unfixed := &f.sets[0]
	units, iterations := uint32(0), uint32(0)
	for i := uint32(0); ; i++ {
		n, e := vars.size()
		if e != nil {
			return lbool.Undef, e
		}
		if i >= n {
			break
		}
		v, e := vars.read(i)
		if e != nil {
			return lbool.Undef, e
		}
		if err = unfixed.insert(Literal(v)); err != nil {
			return lbool.Undef, err
		}
	}
	s.traceConsequenceAssumptions("get_bounded_consequences", 3307, asms)
	if err = s.antecedents.reset(); err != nil {
		return lbool.Undef, err
	}
	result, err = s.consequenceInitialize(asms)
	if err != nil || result != lbool.True {
		return result, err
	}
	if err = s.boundedExtract(&units, asms, unfixed, out); err != nil {
		return lbool.Undef, err
	}
	if err = s.simplifyProblem(); err != nil {
		return lbool.Undef, err
	}
	bad, e := s.checkInconsistent()
	if e != nil {
		return lbool.Undef, e
	}
	if bad {
		err = s.fixupConsequenceCore()
		return lbool.False, err
	}
	for {
		iterations++
		if satStorageSourceDebug && s.propagation.inconsistent {
			return lbool.Undef, ErrSATAssertion
		}
		result, err = s.boundedSearch()
		if err != nil {
			return lbool.Undef, err
		}
		if result != lbool.Undef {
			err = s.fixupConsequenceCore()
			return result, err
		}
		if err = s.boundedExtract(&units, asms, unfixed, out); err != nil {
			return lbool.Undef, err
		}
		cfg, e := s.config.State()
		if e != nil {
			return lbool.Undef, e
		}
		if s.conflict.conflicts > cfg.MaxConflicts {
			if err = s.emitSearchAbort("max-conflicts = ", true); err != nil {
				return lbool.Undef, err
			}
			return lbool.Undef, nil
		}
		if err = s.gc.restart(); err != nil {
			return lbool.Undef, err
		}
		if err = s.simplifyProblem(); err != nil {
			return lbool.Undef, err
		}
		bad, e = s.checkInconsistent()
		if e != nil {
			return lbool.Undef, e
		}
		if bad {
			err = s.fixupConsequenceCore()
			return lbool.False, err
		}
		if err = s.gc.gc(); err != nil {
			return lbool.Undef, err
		}
		cfg, e = s.config.State()
		if e != nil {
			return lbool.Undef, e
		}
		if cfg.RestartMax <= iterations {
			if err = s.emitSearchAbort("max-restarts", false); err != nil {
				return lbool.Undef, err
			}
			return lbool.Undef, nil
		}
	}
}
