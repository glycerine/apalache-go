package arith

// Target-owned services for arithmetic probes used in tactic conditionals.
import (
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TranslateContextProbe(value tactic.ProbeEvaluator, budget *arena.Budget, params *uparams.ParamsContext, globals *uparams.GlobalParamsService) (tactic.ProbeEvaluator, error) {
	if err := value.Check(); err != nil {
		return nil, err
	}
	switch p := value.(type) {
	case *ArithmeticFragmentProbe:
		next, err := NewArithmeticFragmentProbe(budget, params, globals, p.kind)
		if err != nil {
			return nil, err
		}
		next.trace = p.trace
		return next, nil
	case *ArithmeticStatisticProbe:
		return newArithmeticStatisticProbe(budget, p.kind)
	case *IsUnboundedProbe:
		return NewIsUnboundedProbe(budget, params, globals)
	case *IsPBProbe:
		return NewIsPBProbe(budget, params, globals)
	case *QuasiPBProbe:
		return NewQuasiPBProbe(budget, params, globals)
	default:
		return nil, fmt.Errorf("tactic: cross-context translation of this probe is not yet supported")
	}
}
