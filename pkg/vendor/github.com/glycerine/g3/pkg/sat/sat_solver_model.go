package sat

// Source: src/sat/sat_solver.cpp:1257–1346; src/sat/sat_clause.cpp.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (s *Solver) mkModel() error {
	if e := s.open(); e != nil {
		return e
	}
	if e := s.model.Shrink(0); e != nil {
		return e
	}
	s.Variables.modelIsCurrent = true
	s.Variables.modelKnown = true
	n, e := s.Variables.numVarsChecked()
	if e != nil {
		return e
	}
	if e = satSolverResizeModel(&s.model, n); e != nil {
		return e
	}
	model := satSolverModelSpan{column: &s.model}
	for v := uint32(0); v < n; v++ {
		eliminated, e := s.Variables.wasEliminated(BoolVar(v))
		if e != nil {
			return e
		}
		if !eliminated {
			x, e := s.Variables.valueVar(BoolVar(v))
			if e != nil {
				return e
			}
			if e = model.write(BoolVar(v), x); e != nil {
				return e
			}
		}
	}
	s.traceModelConverterBeforeApply()
	if e = s.converter.applyModelSpan(model); e != nil {
		return e
	}
	s.traceCompletedModel(n, model)
	// Selected ordinary source profile does not define _EXTERNAL_RELEASE.
	if e := s.emitModelCheckNotice(); e != nil {
		return e
	}
	good, e := s.checkModel()
	if e != nil {
		return e
	}
	if !good {
		return &SATSolverException{DefaultException: z3exception.NewDefaultException("check model failed")}
	}
	return nil
}
func (s *Solver) checkModel() (bool, error) {
	return s.checkModelSpan(satSolverModelSpan{column: &s.model})
}

// Source check_model accepts a borrowed model, independently of m_model.
func (s *Solver) checkModelSpan(model satSolverModelSpan) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	ok := true
	for _, cs := range [2][]StoredClauseRef{s.Lifecycle.Original, s.Lifecycle.learned} {
		for _, c := range cs {
			yes, e := s.Clauses.satisfiedByModelSpan(c, model)
			if e != nil {
				return false, e
			}
			if !yes {
				s.traceModelFailedClause(c, model)
				ok = false
			}
		}
	}
	for idx := uint32(0); idx < uint32(len(s.Variables.watches.lists)); idx++ {
		value, e := model.value(Literal(idx).Negated())
		if e != nil {
			return false, e
		}
		if value != lbool.True {
			list, e := s.Variables.watches.Get(idx)
			if e != nil {
				return false, e
			}
			n, e := list.Size()
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				w, e := list.Read(i)
				if e != nil {
					return false, e
				}
				if !w.Binary() {
					continue
				}
				l, e := w.Literal()
				if e != nil {
					return false, e
				}
				value, e = model.value(l)
				if e != nil {
					return false, e
				}
				if value != lbool.True {
					s.traceModelFailedBinary(Literal(idx).Negated(), l, w.LearnedUnchecked())
					ok = false
				}
			}
		}
	}
	n, e := s.conflict.assumptions.Size()
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		l, e := s.conflict.assumptions.Read(i)
		if e != nil {
			return false, e
		}
		v, e := model.value(Literal(l))
		if e != nil {
			return false, e
		}
		if v != lbool.True {
			s.traceModelFailedAssumption(Literal(l), model)
			ok = false
		}
	}
	if ok {
		good, e := s.converter.checkModelSpan(model)
		if e != nil {
			return false, e
		}
		if !good {
			s.traceModelFailedConverter(model)
		}
		return good, nil
	}
	return false, nil
}

// The source model getter is borrowed. This explicit append adapter returns
// caller-owned output while all working model storage stays in the Runtime.
func (s *Solver) AppendModel(dst Model) (Model, error) {
	if e := s.open(); e != nil {
		return dst, e
	}
	n, e := s.model.Size()
	if e != nil {
		return dst, e
	}
	model := satSolverModelSpan{column: &s.model}
	for i := uint32(0); i < n; i++ {
		v, e := model.read(BoolVar(i))
		if e != nil {
			return dst, e
		}
		dst = append(dst, v)
	}
	return dst, nil
}
