package smtlogics

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
)

// TranslateContextProbe rebuilds the floating-point logic probes in the target budget.
func TranslateContextProbe(value tactic.ProbeEvaluator, budget *arena.Budget) (tactic.ProbeEvaluator, bool, error) {
	switch value.(type) {
	case *isQFFPProbe, *isQFFPLRAProbe, *isFPQFNRAProbe:
		if err := value.Check(); err != nil {
			return nil, true, err
		}
	default:
		return nil, false, nil
	}
	switch value.(type) {
	case *isQFFPProbe:
		p, err := NewIsQFFPProbe(budget)
		return p, true, err
	case *isQFFPLRAProbe:
		p, err := NewIsQFFPLRAProbe(budget)
		return p, true, err
	default:
		p, err := NewIsFPQFNRAProbe(budget)
		return p, true, err
	}
}
