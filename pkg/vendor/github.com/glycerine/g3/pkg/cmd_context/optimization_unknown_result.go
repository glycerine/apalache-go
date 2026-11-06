package cmd_context

// Source: src/api/api_opt.cpp:139-148, Z3_optimize_check exception result.

import (
	"errors"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) publishOptimizationUnknown(reason string) (status lbool.LBool, resultErr error) {
	result, err := solver.NewSimpleCheckSatResult(c.manager)
	if err != nil {
		return lbool.Undef, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.SetStatus(lbool.Undef); err != nil {
		return lbool.Undef, err
	}
	if err = result.SetReasonUnknown(reason); err != nil {
		return lbool.Undef, err
	}
	c.tacticResult = result
	c.status, c.checked = lbool.Undef, true
	published = true
	return lbool.Undef, nil
}
