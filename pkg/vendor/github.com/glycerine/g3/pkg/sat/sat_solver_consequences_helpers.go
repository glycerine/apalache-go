package sat

// Source: src/sat/sat_solver.cpp:3285–3299,3473–3636.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *Solver) addConsequenceAssumption(l Literal) error {
	if e := s.conflict.assumptionSet.Insert(l); e != nil {
		return e
	}
	return s.conflict.assumptions.Push(l.index())
}
func (s *Solver) consequenceDomain(l, antecedent Literal) (bool, error) {
	yes, e := s.antecedents.contains(uint32(antecedent.variable()))
	if e != nil {
		return false, e
	}
	if yes {
		return true, nil
	}
	if satStorageSourceDebug {
		value, e := s.Variables.valueLiteral(antecedent)
		if e != nil {
			return false, e
		}
		if value != lbool.True {
			return false, ErrSATAssertion
		}
		n, e := s.todoAntecedents.Size()
		if e != nil {
			return false, e
		}
		if n != 0 {
			back, e := s.todoAntecedents.Read(n - 1)
			if e != nil {
				return false, e
			}
			if back == antecedent.index() {
				return false, ErrSATAssertion
			}
		}
	}
	return false, s.todoAntecedents.Push(antecedent.index())
}
func (s *Solver) consequenceUnion(target *satSolverIndexSet, l Literal) error {
	other, e := s.antecedents.find(uint32(l.variable()))
	if e != nil {
		return e
	}
	return target.union(other)
}
func (s *Solver) extractConsequenceAssumptions(lit Literal, target *satSolverIndexSet) (bool, error) {
	j, e := s.Variables.justification.read(uint32(lit.variable()))
	if e != nil {
		return false, e
	}
	s.traceConsequenceReason(lit, j)
	all := true
	switch j.Kind() {
	case 0:
	case 1:
		l, e := j.Literal()
		if e != nil {
			return false, e
		}
		yes, e := s.consequenceDomain(lit, l.Negated())
		if e != nil || !yes {
			return false, e
		}
		if e = s.consequenceUnion(target, l); e != nil {
			return false, e
		}
	case 2:
		l1, e := j.Literal1()
		if e != nil {
			return false, e
		}
		yes, e := s.consequenceDomain(lit, l1.Negated())
		if e != nil || !yes {
			return false, e
		}
		l2, e := j.Literal2()
		if e != nil {
			return false, e
		}
		yes, e = s.consequenceDomain(lit, l2.Negated())
		if e != nil || !yes {
			return false, e
		}
		if e = s.consequenceUnion(target, l1); e != nil {
			return false, e
		}
		if e = s.consequenceUnion(target, l2); e != nil {
			return false, e
		}
	case 3:
		c, e := s.Clauses.GetClause(j.ClauseOffset())
		if e != nil {
			return false, e
		}
		for i := uint32(0); ; i++ {
			n, e := s.Clauses.Size(c)
			if e != nil {
				return false, e
			}
			if i >= n {
				break
			}
			l, e := s.Clauses.Literal(c, i)
			if e != nil {
				return false, e
			}
			if l != lit {
				// check_domain is LEFT of &&all_found, including after an earlier miss.
				yes, e := s.consequenceDomain(lit, l.Negated())
				if e != nil {
					return false, e
				}
				if yes && all {
					l, e = s.Clauses.Literal(c, i)
					if e != nil {
						return false, e
					}
					if e = s.consequenceUnion(target, l); e != nil {
						return false, e
					}
				} else {
					all = false
				}
			}
		}
	case 4:
		if e = s.analysis.fillExtAntecedents(lit, j); e != nil {
			return false, e
		}
		span := satConflictColumnSpan(&s.analysis.extAntecedents)
		n, e := s.analysis.extAntecedents.Size()
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			l, e := span.literal(i)
			if e != nil {
				return false, e
			}
			yes, e := s.consequenceDomain(lit, l)
			if e != nil {
				return false, e
			}
			if yes && all {
				l, e = span.literal(i)
				if e != nil {
					return false, e
				}
				if e = s.consequenceUnion(target, l); e != nil {
					return false, e
				}
			} else {
				all = false
			}
		}
	default:
		return false, ErrSATAssertion
	}
	s.traceConsequenceReasonResult(lit, target)
	return all, nil
}
func (s *Solver) extractFixedConsequence1(lit Literal, assumptions, unfixed *satMUSLiteralSet, out *satSolverLiteralRows) (got bool, err error) {
	f, e := s.consequenceFrame()
	if e != nil {
		return false, e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	f.indexSet()
	yes, e := s.antecedents.contains(uint32(lit.variable()))
	if e != nil {
		return false, e
	}
	if yes {
		return true, nil
	}
	yes = assumptions.contains(lit)
	if yes {
		if err = f.index.insert(lit.index()); err != nil {
			return false, err
		}
	} else {
		yes, err = s.extractConsequenceAssumptions(lit, &f.index)
		if err != nil {
			return false, err
		}
		if !yes {
			if satStorageSourceDebug {
				n, e := s.todoAntecedents.Size()
				if e != nil {
					return false, e
				}
				if n == 0 {
					return false, ErrSATAssertion
				}
			}
			return false, nil
		}
		if err = s.addConsequenceAssumption(lit); err != nil {
			return false, err
		}
	}
	if err = s.antecedents.insert(uint32(lit.variable()), &f.index); err != nil {
		return false, err
	}
	yes = unfixed.contains(Literal(uint32(lit.variable())))
	if yes {
		if err = f.vector(s, 0, nil); err != nil {
			return false, err
		}
		cons := &f.vectors[0]
		if err = cons.Push(lit.index()); err != nil {
			return false, err
		}
		it, e := f.index.capture()
		if e != nil {
			return false, e
		}
		for {
			v, e := it.next()
			if e != nil {
				return false, e
			}
			if !v.present {
				break
			}
			if err = cons.Push(v.value); err != nil {
				return false, err
			}
		}
		if err = satConsequenceSetRemove(unfixed, uint32(lit.variable())); err != nil {
			return false, err
		}
		if err = out.appendColumn(cons); err != nil {
			return false, err
		}
	}
	return true, nil
}
func (s *Solver) extractFixedConsequence(lit Literal, assumptions, unfixed *satMUSLiteralSet, out *satSolverLiteralRows) error {
	if satStorageSourceDebug {
		n, e := s.todoAntecedents.Size()
		if e != nil {
			return e
		}
		if n != 0 {
			return ErrSATAssertion
		}
	}
	if e := s.todoAntecedents.Push(lit.index()); e != nil {
		return e
	}
	for {
		n, e := s.todoAntecedents.Size()
		if e != nil {
			return e
		}
		if n == 0 {
			return nil
		}
		top, e := s.todoAntecedents.Read(n - 1)
		if e != nil {
			return e
		}
		yes, e := s.extractFixedConsequence1(Literal(top), assumptions, unfixed, out)
		if e != nil {
			return e
		}
		if yes {
			n, e = s.todoAntecedents.Size()
			if e != nil {
				return e
			}
			if n == 0 {
				return ErrSATAssertion
			}
			if e = s.todoAntecedents.Shrink(n - 1); e != nil {
				return e
			}
		}
	}
}
func (s *Solver) extractFixedSet(lits, assumptions, unfixed *satMUSLiteralSet, out *satSolverLiteralRows) error {
	span := satSolverLiteralSpan{literals: lits.values}
	n := lits.size()
	for i := uint32(0); i < n; i++ {
		l, e := span.literal(i)
		if e != nil {
			return e
		}
		s.traceExtractFixedLiteral(l)
		level, e := s.Variables.levelVar(l.variable())
		if e != nil {
			return e
		}
		if level <= 1 {
			value, e := s.Variables.valueLiteral(l)
			if e != nil {
				return e
			}
			if value == lbool.True {
				if e = s.extractFixedConsequence(l, assumptions, unfixed, out); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func (s *Solver) extractFixedTrail(start *uint32, assumptions, unfixed *satMUSLiteralSet, out *satSolverLiteralRows) error {
	if satStorageSourceDebug && s.propagation.inconsistent {
		return ErrSATAssertion
	}
	n, e := s.Variables.trail.Size(s.Variables.trailRef)
	if e != nil {
		return e
	}
	for i := *start; i < n; i++ {
		l, e := s.Variables.trail.Read(s.Variables.trailRef, i)
		if e != nil {
			return e
		}
		level, e := s.Variables.levelVar(l.variable())
		if e != nil {
			return e
		}
		if level > 1 {
			break
		}
		// Source reads m_trail[i] again in the actual call expression.
		l, e = s.Variables.trail.Read(s.Variables.trailRef, i)
		if e != nil {
			return e
		}
		if e = s.extractFixedConsequence(l, assumptions, unfixed, out); e != nil {
			return e
		}
	}
	*start = n
	return nil
}
func (s *Solver) deleteUnfixed(lits, vars *satMUSLiteralSet) (err error) {
	f, e := s.consequenceFrame()
	if e != nil {
		return e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	if err = f.set(s, 0, nil); err != nil {
		return err
	}
	span := satSolverLiteralSpan{literals: lits.values}
	n := lits.size()
	for i := uint32(0); i < n; i++ {
		l, e := span.literal(i)
		if e != nil {
			return e
		}
		value, e := s.Variables.valueLiteral(l)
		if e != nil {
			return e
		}
		if value == lbool.True {
			if err = f.sets[0].insert(l); err != nil {
				return err
			}
		} else {
			if err = satConsequenceSetRemove(vars, uint32(l.variable())); err != nil {
				return err
			}
		}
	}
	lits.assign(&f.sets[0])
	return nil
}
func (s *Solver) updateUnfixed(lits, vars *satMUSLiteralSet) (err error) {
	f, e := s.consequenceFrame()
	if e != nil {
		return e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	if err = f.vector(s, 0, nil); err != nil {
		return err
	}
	span := satSolverLiteralSpan{literals: lits.values}
	n := lits.size()
	for i := uint32(0); i < n; i++ {
		l, e := span.literal(i)
		if e != nil {
			return e
		}
		yes := vars.contains(Literal(uint32(l.variable())))
		if !yes {
			if err = f.vectors[0].Push(l.index()); err != nil {
				return err
			}
		}
	}
	for i := uint32(0); ; i++ {
		n, e := f.vectors[0].Size()
		if e != nil {
			return e
		}
		if i >= n {
			return nil
		}
		v, e := f.vectors[0].Read(i)
		if e != nil {
			return e
		}
		if err = satConsequenceSetRemove(lits, v); err != nil {
			return err
		}
	}
}
func (s *Solver) fixupConsequenceCore() (err error) {
	f, e := s.consequenceFrame()
	if e != nil {
		return e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	f.indexSet()
	s.traceConsequenceCore(3287)
	for i := uint32(0); ; i++ {
		n, e := s.conflict.core.Size()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		l, e := s.conflict.core.Read(i)
		if e != nil {
			return e
		}
		other, e := s.antecedents.find(uint32(Literal(l).variable()))
		if e != nil {
			return e
		}
		s.traceConsequenceAntecedents(Literal(l), other)
		if err = f.index.union(other); err != nil {
			return err
		}
	}
	if err = s.conflict.core.Shrink(0); err != nil {
		return err
	}
	it, e := f.index.capture()
	if e != nil {
		return e
	}
	for {
		v, e := it.next()
		if e != nil {
			return e
		}
		if !v.present {
			s.traceConsequenceCore(3297)
			return nil
		}
		if err = s.conflict.core.Push(v.value); err != nil {
			return err
		}
	}
}
