package sls

// Source: src/tactic/sls/sls_tactic.cpp:93-97 (mk_sls_tactic).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func retireUnownedTactic(value tactic.Tactic) error {
	if value == nil {
		return nil
	}
	if err := value.IncRef(); err != nil {
		return err
	}
	return value.DecRef()
}

// NewSLSTactic is the source mk_sls_tactic layer: reject goals outside QF_BV,
// then run a clean SLS core. The larger mk_qfbv_sls_tactic preamble remains a
// distinct composition so this constructor is also useful to retained callers
// that already normalized their goal.
func NewSLSTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (tactic.Tactic, error) {
	core, err := NewCoreTactic(manager, context, globals, params)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(core)
	if err != nil {
		return nil, errors.Join(err, core.Close())
	}
	probe, err := tactic.NewProbe(manager.Budget(), tactic.ProbeIsQFBitVector, 0)
	if err != nil {
		return nil, errors.Join(err, retireUnownedTactic(clean))
	}
	guard, err := tactic.NewFailIfNotTactic(probe)
	if err != nil {
		return nil, errors.Join(err, probe.Close(), retireUnownedTactic(clean))
	}
	result, err := tactic.NewAndThenTactic(guard, clean)
	if err != nil {
		return nil, errors.Join(err, retireUnownedTactic(guard), retireUnownedTactic(clean))
	}
	return result, nil
}
