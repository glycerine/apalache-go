package sat

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *satSolverPropagationState) setConflict(c satJustification, l Literal) {
	if s.inconsistent {
		return
	}
	s.inconsistent = true
	s.conflict = c
	s.notLiteral = l
}
func (s *satSolverPropagationState) assign(l Literal, j satJustification) error {
	if e := s.open(); e != nil {
		return e
	}
	v, e := s.variables.valueLiteral(l)
	if e != nil {
		return e
	}
	switch v {
	case lbool.False:
		s.setConflict(j, l.Negated())
	case lbool.Undef:
		return s.assignCore(l, j)
	}
	return nil
}
func (s *satSolverPropagationState) assignCore(l Literal, j satJustification) error {
	if e := s.open(); e != nil {
		return e
	}
	v := s.variables
	if satStorageSourceDebug {
		value, e := v.valueLiteral(l)
		if e != nil {
			return e
		}
		if value != lbool.Undef {
			return ErrSATAssertion
		}
	}
	s.traceAssignCore(l, j)
	if v.scopeLevel == 0 {
		j = satJustification{}
	}
	if e := v.setAssignment(l, lbool.True); e != nil {
		return e
	}
	if e := v.setAssignment(l.Negated(), lbool.False); e != nil {
		return e
	}
	id := uint32(l.variable())
	if e := v.setLevel(BoolVar(id), v.scopeLevel); e != nil {
		return e
	}
	if e := v.justification.write(id, j); e != nil {
		return e
	}
	phase := int8(0)
	if l.sign() {
		phase = 1
	}
	if e := v.setPhase(BoolVar(id), phase); e != nil {
		return e
	}
	if e := v.setGCAssigned(BoolVar(id), true); e != nil {
		return e
	}
	if e := v.trailPush(l); e != nil {
		return e
	}
	if s.extensionPresent {
		ext, e := v.IsExternal(BoolVar(id))
		if e != nil {
			return e
		}
		if ext {
			return ErrSATMissingSourceExtBinding
		}
	}
	if satStorageSourceDebug {
		p, e := v.phaseAt(BoolVar(id))
		if e != nil {
			return e
		}
		if (l.sign() && p != 1) || (!l.sign() && p != 0) {
			return ErrSATAssertion
		}
		a, e := v.valueVar(l.variable())
		if e != nil {
			return e
		}
		if (l.sign() && a != lbool.False) || (!l.sign() && a != lbool.True) {
			return ErrSATAssertion
		}
		a, e = v.valueLiteral(l)
		if e != nil {
			return e
		}
		if a != lbool.True {
			return ErrSATAssertion
		}
		a, e = v.valueLiteral(l.Negated())
		if e != nil {
			return e
		}
		if a != lbool.False {
			return ErrSATAssertion
		}
	}
	return nil
}
func (s *satSolverPropagationState) status(c StoredClauseRef) (lbool.LBool, error) {
	if e := s.open(); e != nil {
		return lbool.Undef, e
	}
	n, e := s.clauses.Size(c)
	if e != nil {
		return lbool.Undef, e
	}
	found := false
	for i := uint32(0); i < n; i++ {
		l, e := s.clauses.Literal(c, i)
		if e != nil {
			return lbool.Undef, e
		}
		v, e := s.variables.valueLiteral(l)
		if e != nil {
			return lbool.Undef, e
		}
		switch v {
		case lbool.True:
			return lbool.True, nil
		case lbool.Undef:
			found = true
		}
	}
	if found {
		return lbool.Undef, nil
	}
	return lbool.False, nil
}
