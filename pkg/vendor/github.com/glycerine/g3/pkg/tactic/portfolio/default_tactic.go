package portfolio

// Source: src/tactic/portfolio/default_tactic.cpp mk_default_tactic.
import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewDefaultTactic constructs the source's simplify-first classifier and applies
// the supplied parameters to the complete strategy.
func NewDefaultTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (result tactic.Tactic, resultErr error) {
	if m == nil || config.Params == nil || config.Globals == nil || config.Globals.Context != config.Params || output == nil {
		return nil, uparams.ErrParamsOwner
	}
	var tactics []tactic.Tactic
	var probes []tactic.ProbeEvaluator
	defer func() {
		for i := len(tactics) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, tactics[i].DecRef())
		}
		for i := len(probes) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, probes[i].DecRef())
		}
		if resultErr != nil && result != nil {
			if e := result.IncRef(); e == nil {
				resultErr = errors.Join(resultErr, result.DecRef())
			} else {
				resultErr = errors.Join(resultErr, e)
			}
			result = nil
		}
	}()
	keepTactic := func(v tactic.Tactic, err error) (tactic.Tactic, error) {
		if err != nil {
			return nil, err
		}
		if err = v.IncRef(); err != nil {
			return nil, err
		}
		tactics = append(tactics, v)
		return v, nil
	}
	keepProbe := func(v tactic.ProbeEvaluator, err error) (tactic.ProbeEvaluator, error) {
		if err != nil {
			return nil, err
		}
		if err = v.IncRef(); err != nil {
			return nil, err
		}
		probes = append(probes, v)
		return v, nil
	}
	ordinary, err := smtcore.NewSMTTactic(m, nil, config)
	route, err := keepTactic(ordinary, err)
	if err != nil {
		return nil, err
	}
	branches := []struct {
		makeTactic func() (tactic.Tactic, error)
		makeProbe  func() (tactic.ProbeEvaluator, error)
	}{
		{func() (tactic.Tactic, error) { return smtlogics.NewQFFPLRATacticWithOutput(m, p, config, output) }, func() (tactic.ProbeEvaluator, error) { return smtlogics.NewIsQFFPLRAProbe(m.Budget()) }},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFFPTacticWithOutput(m, p, config, output) }, func() (tactic.ProbeEvaluator, error) { return smtlogics.NewIsQFFPProbe(m.Budget()) }},
		{func() (tactic.Tactic, error) { return smtlogics.NewNRATactic(m, nil, config) }, func() (tactic.ProbeEvaluator, error) {
			return arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentNRA)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewLIRATactic(m, p, config) }, func() (tactic.ProbeEvaluator, error) {
			return arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentLIRA)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFNIATacticWithOutput(m, nil, config, output) }, func() (tactic.ProbeEvaluator, error) {
			return arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFNIA)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFNRATactic(m, nil, config) }, func() (tactic.ProbeEvaluator, error) {
			return arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFNRA)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFLRATactic(m, nil, config) }, func() (tactic.ProbeEvaluator, error) {
			return arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFLRA)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFAUFLIATactic(m, nil, config) }, func() (tactic.ProbeEvaluator, error) {
			return arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFAUFLIA)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFLIATacticWithOutput(m, nil, config, output) }, func() (tactic.ProbeEvaluator, error) {
			return arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFLIA)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFAUFBVTactic(m, nil, config) }, func() (tactic.ProbeEvaluator, error) {
			return tactic.NewProbe(m.Budget(), tactic.ProbeIsQFArrayUFBitVector, 0)
		}},
		{func() (tactic.Tactic, error) { return smtlogics.NewQFBVTacticWithOutput(m, nil, config, output) }, func() (tactic.ProbeEvaluator, error) {
			return tactic.NewProbe(m.Budget(), tactic.ProbeIsQFBitVector, 0)
		}},
	}
	for _, b := range branches {
		branch, e := b.makeTactic()
		branch, e = keepTactic(branch, e)
		if e != nil {
			return nil, e
		}
		probe, e := b.makeProbe()
		probe, e = keepProbe(probe, e)
		if e != nil {
			return nil, e
		}
		next, e := tactic.NewCondTactic(probe, branch, route)
		route, e = keepTactic(next, e)
		if e != nil {
			return nil, e
		}
	}
	simplify, err := core.NewSimplifyTactic(m, config.Params, config.Globals, nil)
	first, err := keepTactic(simplify, err)
	if err != nil {
		return nil, err
	}
	sequence, err := tactic.NewAndThenTactic(first, route)
	sequence, err = keepTactic(sequence, err)
	if err != nil {
		return nil, err
	}
	if p == nil {
		empty := config.Params.EmptyValue()
		return tactic.NewUsingParamsTactic(config.Params, sequence, &empty)
	}
	return tactic.NewUsingParamsTactic(config.Params, sequence, p)
}
