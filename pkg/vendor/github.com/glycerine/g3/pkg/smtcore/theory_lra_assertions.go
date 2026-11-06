package smtcore

// Source: retained SASSERT gates in src/smt/theory_lra.cpp.

import (
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func lraAssertionContext() (*diagnostic.DebugContext, bool, error) {
	if !diagnostic.DebugBuild {
		return nil, false, nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	return d, enabled, err
}

func (t *SMTLRATheory) rawLRAValue(v SMTTheoryVar) (simplex.Infinitesimal, error) {
	if err := t.numeric.ready(); err != nil {
		return simplex.Infinitesimal{}, err
	}
	d, enabled, err := lraAssertionContext()
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	if enabled {
		index, mapped := t.numeric.index(v)
		if !mapped || !t.numeric.solver.VarIsRegistered(index) {
			return simplex.Infinitesimal{}, diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 1027, "can_get_ivalue(v)")
		}
	}
	return t.numeric.value(v)
}
