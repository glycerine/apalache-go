package smtlogics

// Source: src/tactic/smtlogics/qfidl_tactic.cpp/.h.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/aig"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func NewQFIDLTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFIDLTacticWithOutput(m, p, config, os.Stdout)
}

// NewQFIDLTacticWithOutput borrows the command session's stdout transport for
// the native SAT stage's optional DIMACS output.
func NewQFIDLTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (result tactic.Tactic, err error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	var owners []tactic.Tactic
	var probes []*tactic.Probe
	var parameters []*uparams.ParamsRef
	defer func() {
		for i := len(owners) - 1; i >= 0; i-- {
			err = errors.Join(err, owners[i].DecRef())
		}
		for i := len(probes) - 1; i >= 0; i-- {
			err = errors.Join(err, probes[i].DecRef())
		}
		for i := len(parameters) - 1; i >= 0; i-- {
			err = errors.Join(err, c.Del(parameters[i]))
		}
		if err != nil && result != nil {
			if e := result.IncRef(); e != nil {
				err = errors.Join(err, e)
			} else {
				err = errors.Join(err, result.DecRef())
			}
			result = nil
		}
	}()
	keep := func(value tactic.Tactic, cause error) (tactic.Tactic, error) {
		if cause != nil {
			return nil, cause
		}
		if cause = value.IncRef(); cause != nil {
			return nil, cause
		}
		owners = append(owners, value)
		return value, nil
	}
	keepProbe := func(value *tactic.Probe, cause error) (*tactic.Probe, error) {
		if cause != nil {
			return nil, cause
		}
		if cause = value.IncRef(); cause != nil {
			return nil, cause
		}
		probes = append(probes, value)
		return value, nil
	}
	var main, lhs, lia, pb, bits, difference uparams.ParamsRef
	for _, value := range []*uparams.ParamsRef{&main, &lhs, &lia, &pb, &bits, &difference} {
		*value, err = c.NewValue()
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, value)
	}
	for _, key := range []string{"elim_and", "blast_distinct", "som"} {
		if err = c.SetBoolName(&main, &key, true); err != nil {
			return nil, err
		}
	}
	key := "arith_lhs"
	if err = c.SetBoolName(&lhs, &key, true); err != nil {
		return nil, err
	}
	key = "lia2pb_max_bits"
	if err = c.SetUIntName(&lia, &key, 4); err != nil {
		return nil, err
	}
	key = "pb2bv_all_clauses_limit"
	if err = c.SetUIntName(&pb, &key, 8); err != nil {
		return nil, err
	}
	// Source pull_ite_p is never consumed by this strategy; no unused policy
	// or C++ parameter-container implementation is reproduced.
	for _, key := range []string{"flat", "som"} {
		if err = c.SetBoolName(&bits, &key, false); err != nil {
			return nil, err
		}
	}
	policy, err := m.Symbols.Intern("dyn_psm")
	if err != nil {
		return nil, err
	}
	key = "gc"
	if err = c.SetSymbolName(&bits, &key, policy); err != nil {
		return nil, err
	}
	key = "diff_neq_max_k"
	if err = c.SetUIntName(&difference, &key, 25); err != nil {
		return nil, err
	}

	first, err := keep(core.MakeSimplifyTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	fix, err := keep(arith.MakeFixDLVarTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	propagate, err := keep(core.NewPropagateValuesTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	eliminate, err := keep(core.MakeEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	initial, err := keep(tactic.NewAndThenTactic(first, fix, propagate, eliminate))
	if err != nil {
		return nil, err
	}
	solve, err := keep(core.MakeSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	left, err := keep(core.MakeSimplifyTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	left, err = keep(tactic.NewUsingParamsTactic(c, left, &lhs))
	if err != nil {
		return nil, err
	}
	propagate, err = keep(core.NewPropagateValuesTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	normalize, err := keep(arith.MakeNormalizeBoundsTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	lastSolve, err := keep(core.MakeSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	last, err := keep(tactic.NewAndThenTactic(solve, left, propagate, normalize, lastSolve))
	if err != nil {
		return nil, err
	}
	preamble, err := keep(tactic.NewAndThenTactic(initial, last))
	if err != nil {
		return nil, err
	}

	simplifyBV, err := keep(core.MakeSimplifyTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	propagateBV, err := keep(core.NewPropagateValuesTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	solveBV, err := keep(core.MakeSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	sharing, err := keep(bv.MakeMaxBVSharingTactic(m, c, nil, config.ModelEvaluator.Cooperation))
	if err != nil {
		return nil, err
	}
	blaster, err := keep(bv.MakeBitBlasterTactic(m, c, g, nil, config.ModelEvaluator.Cooperation))
	if err != nil {
		return nil, err
	}
	aigSolver, err := keep(aig.MakeTactic(c, nil, config.ModelEvaluator.Cooperation))
	if err != nil {
		return nil, err
	}
	satSolver, err := keep(sattactic.MakeSATTactic(c, nil, config.ModelEvaluator, output))
	if err != nil {
		return nil, err
	}
	bvSolver, err := keep(tactic.NewAndThenTactic(simplifyBV, propagateBV, solveBV, sharing, blaster, aigSolver, satSolver))
	if err != nil {
		return nil, err
	}
	bvSolver, err = keep(tactic.NewUsingParamsTactic(c, bvSolver, &bits))
	if err != nil {
		return nil, err
	}
	liaSolver, err := keep(arith.MakeLIA2PBTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	liaSolver, err = keep(tactic.NewUsingParamsTactic(c, liaSolver, &lia))
	if err != nil {
		return nil, err
	}
	ineq, err := keep(arith.MakePropagateIneqsTactic(m, c, nil))
	if err != nil {
		return nil, err
	}
	pbSolver, err := keep(arith.MakePB2BVTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	pbSolver, err = keep(tactic.NewUsingParamsTactic(c, pbSolver, &pb))
	if err != nil {
		return nil, err
	}
	qfbv, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeIsQFBitVector, 0))
	if err != nil {
		return nil, err
	}
	notQFBV, err := keepProbe(tactic.NewNotProbe(m.Budget(), qfbv))
	if err != nil {
		return nil, err
	}
	guard, err := keep(tactic.NewFailIfTactic(notQFBV))
	if err != nil {
		return nil, err
	}
	tryBV, err := keep(tactic.NewAndThenTactic(liaSolver, ineq, pbSolver, guard, bvSolver))
	if err != nil {
		return nil, err
	}
	diff, err := keep(arith.MakeDiffNeqTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	diff, err = keep(tactic.NewUsingParamsTactic(c, diff, &difference))
	if err != nil {
		return nil, err
	}
	smtFallback, err := keep(smtcore.NewSMTTactic(m, nil, config))
	if err != nil {
		return nil, err
	}
	solvers, err := keep(tactic.NewOrElseTactic(diff, tryBV, smtFallback))
	if err != nil {
		return nil, err
	}
	small, err := keep(tactic.NewAndThenTactic(preamble, solvers))
	if err != nil {
		return nil, err
	}
	small, err = keep(tactic.NewUsingParamsTactic(c, small, &main))
	if err != nil {
		return nil, err
	}
	directSMT, err := keep(smtcore.NewSMTTactic(m, nil, config))
	if err != nil {
		return nil, err
	}
	count, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeNumConsts, 0))
	if err != nil {
		return nil, err
	}
	limit, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeConstant, 5000))
	if err != nil {
		return nil, err
	}
	belowLimit, err := keepProbe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeLess, count, limit))
	if err != nil {
		return nil, err
	}
	proofs, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeProduceProofs, 0))
	if err != nil {
		return nil, err
	}
	noProofs, err := keepProbe(tactic.NewNotProbe(m.Budget(), proofs))
	if err != nil {
		return nil, err
	}
	cores, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeProduceUnsatCores, 0))
	if err != nil {
		return nil, err
	}
	noCores, err := keepProbe(tactic.NewNotProbe(m.Budget(), cores))
	if err != nil {
		return nil, err
	}
	noMetadata, err := keepProbe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeAnd, noProofs, noCores))
	if err != nil {
		return nil, err
	}
	chooseSmall, err := keepProbe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeAnd, belowLimit, noMetadata))
	if err != nil {
		return nil, err
	}
	result, err = tactic.NewCondTactic(chooseSmall, small, directSMT)
	if err != nil {
		return nil, err
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	err = result.UpdateParams(p)
	return result, err
}
