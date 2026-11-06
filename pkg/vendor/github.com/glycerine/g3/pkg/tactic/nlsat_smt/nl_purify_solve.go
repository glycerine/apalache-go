package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, solve.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

type nlCooperation struct {
	prepared   *nlPurification
	smt        *smtcore.SMTSolver
	nonlinear  *nlsattactic.NLSATTactic
	services   model.ModelEvaluatorConfig
	equalities *nlEqualityState
	cores      *nlCoreTracker
}

func newNLCooperation(prepared *nlPurification, smt *smtcore.SMTSolver, nonlinear *nlsattactic.NLSATTactic, services model.ModelEvaluatorConfig) (*nlCooperation, error) {
	if prepared == nil || prepared.goal == nil || prepared.filter == nil || smt == nil || nonlinear == nil {
		return nil, ast.ErrASTHandle
	}
	equalities, err := newNLEqualityState(prepared.goal.Manager(), prepared.filter)
	if err != nil {
		return nil, err
	}
	cores, err := newNLCoreTracker(prepared.goal.Manager(), prepared.extraction)
	if err != nil {
		return nil, errors.Join(err, equalities.Close())
	}
	return &nlCooperation{prepared: prepared, smt: smt, nonlinear: nonlinear, services: services, equalities: equalities, cores: cores}, nil
}

func (c *nlCooperation) Close() error {
	if c == nil || c.equalities == nil {
		return nil
	}
	err := c.equalities.Close()
	c.equalities = nil
	return err
}

// solve runs the source cooperation loop after assertSMT has initialized the
// SMT side. Both input solvers and prepared goal are borrowed.
func (c *nlCooperation) solve(out *tactic.TacticResult) (resultErr error) {
	if c == nil || c.prepared == nil || c.equalities == nil || out == nil {
		return ast.ErrASTHandle
	}
	m := c.prepared.goal.Manager()
	if err := out.Prepare(m); err != nil {
		return err
	}
	for {
		canceled, err := m.Canceled()
		if err != nil {
			return err
		}
		if canceled {
			return tactic.NewTacticException(messages.Canceled)
		}
		again, err := c.iterate(out)
		if err != nil {
			return err
		}
		if !again {
			return traceNLResults(m, c.services, out)
		}
	}
}

func (c *nlCooperation) iterate(out *tactic.TacticResult) (_ bool, resultErr error) {
	p := c.prepared
	m := p.goal.Manager()
	temporary, err := tactic.NewGoalCopy(p.nonlinear)
	if err != nil {
		return false, err
	}
	defer func() {
		if temporary != nil {
			resultErr = errors.Join(resultErr, temporary.Close())
		}
	}()
	var nlResult tactic.TacticResult
	defer func() { resultErr = errors.Join(resultErr, nlResult.Close()) }()
	if err = c.nonlinear.Apply(temporary, &nlResult); err != nil {
		return false, fmt.Errorf("mixed nonlinear check: %w", err)
	}
	if nlResult.DecidedUnsat() {
		if err = traceNLMessage(m, c.services, 337, "unsat\n"); err != nil {
			return false, err
		}
		var core tactic.DependencyRef
		defer func() { resultErr = errors.Join(resultErr, core.Close()) }()
		if err = c.cores.finish(p.goal, &core); err != nil {
			return false, err
		}
		if err = out.SetCore(core.Get()); err != nil {
			return false, err
		}
		return false, out.AddGoal(p.goal)
	}
	if !nlResult.DecidedSat() {
		if err = traceNLMessage(m, c.services, 341, "not a unit\n"); err != nil {
			return false, err
		}
		// NLSAT returns its unresolved goal as the sole borrowed subgoal.
		if len(nlResult.Goals) != 1 || nlResult.Goals[0] != temporary {
			return false, ast.ErrASTDomain
		}
		if nlResult.ModelConverter != nil {
			if err = out.SetModelConverter(nlResult.ModelConverter); err != nil {
				return false, err
			}
		}
		if err = out.TakeGoal(temporary); err != nil {
			return false, err
		}
		temporary = nil
		return false, nil
	}
	var nlModel tactic.ModelRef
	defer func() { resultErr = errors.Join(resultErr, nlModel.Close()) }()
	var assumptions *nlAssumptions
	if nlResult.ModelConverter != nil {
		if err = tactic.ModelConverterToModel(m, nlResult.ModelConverter, &nlModel); err != nil {
			return false, err
		}
		core, e := nlModel.Get()
		if e != nil {
			return false, e
		}
		evaluator, e := model.NewModelEvaluator(core, c.services)
		if e != nil {
			if evaluator != nil {
				e = errors.Join(e, evaluator.Close())
			}
			return false, e
		}
		defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
		c.equalities.values, e = updateNLEqualityValues(m, evaluator, c.equalities.predicates)
		if e != nil {
			return false, e
		}
		if _, e = c.equalities.enforce(evaluator, p.interfaces.interfaceValues, p.nonlinear, c.smt); e != nil {
			return false, e
		}
		assumptions, e = makeNLAssumptions(m, evaluator, p.extraction.Assumptions, p.predicates.newPredicates, p.predicates.polarities, c.equalities.predicates, c.equalities.values)
		if e != nil {
			return false, e
		}
		defer func() { resultErr = errors.Join(resultErr, assumptions.Close()) }()
	}
	var literals []ast.ASTHandle
	if assumptions != nil {
		literals = assumptions.literals
	}
	status, err := c.smt.CheckSat(literals)
	if err != nil {
		return false, fmt.Errorf("mixed SMT check: %w", err)
	}
	switch status {
	case lbool.False:
		core, e := c.smt.UnsatCore()
		if e != nil {
			return false, e
		}
		learned, e := c.cores.learn(core, p.nonlinear)
		if e != nil || learned {
			return learned, e
		}
		var dependencies tactic.DependencyRef
		defer func() { resultErr = errors.Join(resultErr, dependencies.Close()) }()
		if e = c.cores.finish(p.goal, &dependencies); e != nil {
			return false, e
		}
		if e = out.SetCore(dependencies.Get()); e != nil {
			return false, e
		}
		return false, out.AddGoal(p.goal)
	case lbool.True:
		return c.finishSat(out, &nlModel)
	default:
		return false, traceNLMessage(m, c.services, 407, "unknown\n")
	}
}

func (c *nlCooperation) finishSat(out *tactic.TacticResult, nlModel *tactic.ModelRef) (_ bool, resultErr error) {
	p := c.prepared
	m := p.goal.Manager()
	acquired, err := c.smt.AcquireModel()
	if err != nil {
		return false, err
	}
	if acquired == nil {
		return false, ast.ErrASTHandle
	}
	var smtModel tactic.ModelRef
	if err = smtModel.Set(acquired); err != nil {
		return false, errors.Join(err, acquired.DecRef())
	}
	if err = acquired.DecRef(); err != nil {
		return false, errors.Join(err, smtModel.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, smtModel.Close()) }()
	core, err := smtModel.Get()
	if err != nil {
		return false, err
	}
	evaluator, err := model.NewModelEvaluator(core, c.services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return false, err
	}
	changed, err := c.equalities.enforce(evaluator, p.interfaces.interfaceValues, p.nonlinear, c.smt)
	err = errors.Join(err, evaluator.Close())
	if err != nil || changed {
		return changed, err
	}
	nlCore, err := nlModel.Get()
	if err != nil {
		return false, err
	}
	if nlCore != nil {
		if err = mergeNLModels(m, nlCore, core); err != nil {
			return false, err
		}
	}
	if err = p.filter.ApplyUnindexed(&smtModel); err != nil {
		return false, err
	}
	core, err = smtModel.Get()
	if err != nil {
		return false, err
	}
	converter, err := tactic.NewStoredModel(core)
	if err != nil {
		return false, err
	}
	if err = out.SetModelConverter(converter); err != nil {
		return false, errors.Join(err, converter.Close())
	}
	goal, err := tactic.NewGoal(m, false, false, false)
	if err != nil {
		return false, err
	}
	if err = out.TakeGoal(goal); err != nil {
		return false, errors.Join(err, goal.Close())
	}
	return false, nil
}
