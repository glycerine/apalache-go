package cmd_context

// Source: src/cmd_context/tactic_cmds.cpp:249-347, apply_tactic_cmd.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ApplyTactic applies selected to a copy of the current assertion goal and
// writes the requested source command views without changing solver state.
func (c *Context) ApplyTactic(out io.Writer, selected tactic.Tactic, options *uparams.ParamsRef) (result error) {
	if out == nil || selected == nil {
		return fmt.Errorf("apply needs a tactic argument")
	}
	m, err := c.Manager()
	if err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if options == nil {
		empty := c.config.Params.EmptyValue()
		options = &empty
	}
	params, err := c.parameters.MergeDefaultParams(options)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, c.config.Params.Del(&params)) }()
	settings, err := c.applyTacticSettings(&params, state.Timeout, state.RLimit)
	if err != nil {
		return err
	}
	wrapped, err := tactic.NewUsingParamsTactic(c.config.Params, selected, &params)
	if err != nil {
		return err
	}
	if err = wrapped.IncRef(); err != nil {
		return err
	}
	defer func(owned tactic.Tactic) { result = errors.Join(result, owned.DecRef()) }(wrapped)
	var logic usymbol.Symbol
	if c.logic != "" {
		if logic, err = m.Symbols.Intern(c.logic); err != nil {
			return err
		}
	}
	if err = wrapped.SetLogic(logic); err != nil {
		return err
	}
	if settings.timeout != 0 && settings.timeout != ^uint32(0) {
		timed, e := tactic.NewTryForTactic(wrapped, settings.timeout)
		if e != nil {
			return e
		}
		if e = timed.IncRef(); e != nil {
			return e
		}
		defer func(owned tactic.Tactic) { result = errors.Join(result, owned.DecRef()) }(timed)
		wrapped = timed
	}
	goal, err := tactic.NewGoal(m, state.Proof, state.Model, state.UnsatCore)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, goal.Close()) }()
	if err = c.AssertGoal(goal); err != nil {
		return err
	}
	var applied tactic.TacticResult
	if err = applied.Prepare(m); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, applied.Close()) }()
	if err = m.ResourceLimit().Push(settings.rlimit); err != nil {
		return err
	}
	defer m.ResourceLimit().Pop()
	watch, err := stopwatch.NewScopedWatch(&c.watch, true)
	if err != nil {
		return err
	}
	applyErr := tactic.Exec(wrapped, goal, &applied)
	if closeErr := watch.Close(); closeErr != nil {
		applyErr = errors.Join(applyErr, closeErr)
	}
	var tacticFailure tactic.TacticException
	if applyErr != nil && !errors.As(applyErr, &tacticFailure) {
		return applyErr
	}
	if applyErr == nil {
		if err = c.writeAppliedTacticResult(out, &applied, settings); err != nil {
			return err
		}
	}
	if settings.printStatistics {
		if err = c.writeAppliedTacticStatistics(out, wrapped); err != nil {
			return err
		}
	}
	if applyErr != nil {
		return TacticCommandFailure{Cause: applyErr, Message: tacticFailure.Message()}
	}
	return nil
}
