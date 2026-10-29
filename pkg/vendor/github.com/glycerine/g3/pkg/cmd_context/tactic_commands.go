package cmd_context

// Source: src/cmd_context/tactic_cmds.cpp check_sat_using_tactict_cmd;
// src/cmd_context/cmd_context_to_goal.cpp assert_exprs_from.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	muzfp "github.com/glycerine/g3/pkg/muz/fp"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/qe"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/aig"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	fpatactic "github.com/glycerine/g3/pkg/tactic/fpa"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	ufbvtactic "github.com/glycerine/g3/pkg/tactic/ufbv"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// TacticCommandFailure is the caught source z3_exception branch of the
// check-sat-using command. The parser prints its diagnostic and continues.
// Source z3_error values and Go ownership failures continue to propagate.
type TacticCommandFailure struct {
	Cause   error
	Message string
}

func (e TacticCommandFailure) Error() string { return e.Message }
func (e TacticCommandFailure) Unwrap() error { return e.Cause }

// NewBuiltinTactic returns the uncounted tactic produced by a retained native
// factory. The parser or composition wrapper must acquire its own reference.
func (c *Context) NewBuiltinTactic(name string) (tactic.Tactic, error) {
	return c.NewBuiltinTacticWithOutput(name, os.Stdout)
}

// NewBuiltinTacticWithOutput supplies the command session's stdout transport
// to tactics that emit source diagnostic output, including DIMACS instances.
func (c *Context) NewBuiltinTacticWithOutput(name string, output io.Writer) (tactic.Tactic, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	switch name {
	case "qe-light", "qsat", "qe2", "qe_rec", "smt", "nlsat", "horn", "sat", "sat-preprocess", "simplify", "ctx-simplify", "add-bounds", "elim-and", "elim-term-ite", "tseitin-cnf", "tseitin-cnf-core", "factor", "propagate-values", "propagate-ineqs", "max-bv-sharing", "bit-blast", "bv1-blast", "fpa2bv", "dt2bv", "bv_bound_chk", "reduce-bv-size", "reduce-args", "ackermannize_bv", "aig", "solve-eqs", "elim-uncnstr", "symmetry-reduce", "der", "distribute-forall", "nnf", "snf", "macro-finder", "quasi-macros", "ufbv", "bv", "fix-dl-var", "normalize-bounds", "nla2bv", "diff-neq", "lia2pb", "card2bv", "pb2bv", "qfbv", "qfaufbv", "qfufbv", "qfufbv_ackr", "qfidl", "qflia", "qfnia", "qfnra", "qffp", "qffpbv", "qffplra", "qfauflia", "qflra", "qfuf", "ufnia", "uflra", "auflia", "auflira", "aufnira", "skip", "fail", "fail-if-undecided":
	default:
		return nil, fmt.Errorf("invalid tactic, unknown tactic %s", name)
	}
	m, err := c.Manager()
	if err != nil {
		return nil, err
	}
	switch name {
	case "qsat", "qe2", "qe_rec":
		config := qe.Config{Context: c.config.Params, Globals: c.config.Globals, Cooperation: c.config.ModelEvaluator.Cooperation, Trace: c.config.ModelEvaluator.Trace}
		var created *qe.QSATTactic
		switch name {
		case "qsat":
			created, err = qe.NewQSATTactic(m, nil, config)
		case "qe2":
			created, err = qe.NewQE2Tactic(m, nil, config)
		case "qe_rec":
			created, err = qe.NewQERecursiveTactic(m, nil, config)
		}
		if created == nil {
			return nil, err
		}
		return created, err
	case "qe-light":
		created, err := qe.NewLiteTactic(m, nil, qe.Config{Context: c.config.Params, Globals: c.config.Globals, Cooperation: c.config.ModelEvaluator.Cooperation})
		if created == nil {
			return nil, err
		}
		return created, err
	case "smt":
		return smtcore.NewSMTTactic(m, nil, c.config)
	case "nlsat":
		return nlsattactic.MakeNLSATTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "horn":
		return muzfp.NewHornTactic(m, c.config.Params, c.config.Globals)
	case "sat":
		return sattactic.MakeSATTactic(c.config.Params, nil, c.config.ModelEvaluator, output)
	case "sat-preprocess":
		return sattactic.MakeSATPreprocessorTactic(c.config.Params, nil, c.config.ModelEvaluator, output)
	case "simplify":
		return core.MakeSimplifyTactic(m, c.config.Params, c.config.Globals, nil)
	case "ctx-simplify":
		return core.MakeContextSimplifyTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "add-bounds":
		return arith.MakeAddBoundsTactic(m, c.config.Params, c.config.Globals, nil)
	case "elim-and":
		return core.MakeEliminateAndTactic(m, c.config.Params, c.config.Globals, nil)
	case "elim-term-ite":
		return core.MakeEliminateTermITETactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "tseitin-cnf":
		return core.MakeTseitinCNFTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "tseitin-cnf-core":
		return core.MakeTseitinCNFCoreTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "factor":
		return arith.MakeFactorTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "propagate-values":
		return core.NewPropagateValuesTactic(m, c.config.Params, c.config.Globals, nil)
	case "propagate-ineqs":
		return arith.MakePropagateIneqsTactic(m, c.config.Params, nil)
	case "max-bv-sharing":
		return bv.MakeMaxBVSharingTactic(m, c.config.Params, nil, c.config.ModelEvaluator.Cooperation)
	case "bit-blast":
		return bv.MakeBitBlasterTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator.Cooperation)
	case "bv1-blast":
		return bv.MakeBV1BlasterTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator.Cooperation)
	case "fpa2bv":
		return fpatactic.NewFPA2BVTactic(m, c.config.Params, c.config.Globals, nil)
	case "dt2bv":
		return bv.NewDT2BVTactic(m, c.config.Params, nil, c.config.ModelEvaluator)
	case "reduce-bv-size":
		return bv.MakeBVSizeReductionTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "bv_bound_chk":
		return bv.NewBVBoundCheckTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "ackermannize_bv":
		return ackermannization.NewTactic(m, nil, c.config.ModelEvaluator)
	case "aig":
		return aig.MakeTactic(c.config.Params, nil, c.config.ModelEvaluator.Cooperation)
	case "solve-eqs":
		return core.MakeSolveEquationsTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "elim-uncnstr":
		return core.MakeEliminateUnconstrainedTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "qfauflia":
		return smtlogics.NewQFAUFLIATactic(m, nil, c.config)
	case "qfaufbv":
		return smtlogics.NewQFAUFBVTactic(m, nil, c.config)
	case "qfufbv":
		return smtlogics.NewQFUFBVTacticWithOutput(m, nil, c.config, output)
	case "qfufbv_ackr":
		return smtlogics.NewQFUFBVAckrTacticWithOutput(m, nil, c.config, output)
	case "qfidl":
		return smtlogics.NewQFIDLTacticWithOutput(m, nil, c.config, output)
	case "qflia":
		return smtlogics.NewQFLIATacticWithOutput(m, nil, c.config, output)
	case "qfnia":
		return smtlogics.NewQFNIATacticWithOutput(m, nil, c.config, output)
	case "qfnra":
		return smtlogics.NewQFNRATactic(m, nil, c.config)
	case "qffp":
		return smtlogics.NewQFFPTacticWithOutput(m, nil, c.config, output)
	case "qffpbv":
		return smtlogics.NewQFFPBVTacticWithOutput(m, nil, c.config, output)
	case "qffplra":
		return smtlogics.NewQFFPLRATacticWithOutput(m, nil, c.config, output)
	case "qfbv":
		return smtlogics.NewQFBVTacticWithOutput(m, nil, c.config, output)
	case "qflra":
		return smtlogics.NewQFLRATactic(m, nil, c.config)
	case "uflra":
		return smtlogics.NewUFLRATactic(m, nil, c.config)
	case "ufnia":
		return smtlogics.NewUFNIATactic(m, nil, c.config)
	case "auflia":
		return smtlogics.NewAUFLIATactic(m, nil, c.config)
	case "auflira":
		return smtlogics.NewAUFLIRATactic(m, nil, c.config)
	case "aufnira":
		return smtlogics.NewAUFNIRATactic(m, nil, c.config)
	case "qfuf":
		return smtlogics.NewQFUFTactic(m, nil, c.config)
	case "symmetry-reduce":
		return core.NewSymmetryReduceTactic(m, c.config.Params, c.config.Globals)
	case "der":
		return core.NewDERTactic(m)
	case "distribute-forall":
		return core.NewDistributeForallTactic(m, nil)
	case "nnf":
		return core.NewNNFTactic(m, c.config.Params, c.config.Globals, nil)
	case "snf":
		return core.NewSNFTactic(m, c.config.Params, c.config.Globals, nil)
	case "macro-finder":
		return ufbvtactic.NewMacroFinderTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "quasi-macros":
		return ufbvtactic.NewQuasiMacrosTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "ufbv", "bv":
		return ufbvtactic.NewUFBVTactic(m, nil, c.config)
	case "reduce-args":
		return core.MakeReduceArgsTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "fix-dl-var":
		return arith.MakeFixDLVarTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "nla2bv":
		return arith.NewNLA2BVTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "normalize-bounds":
		return arith.MakeNormalizeBoundsTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "diff-neq":
		return arith.MakeDiffNeqTactic(m, c.config.Params, c.config.Globals, nil)
	case "lia2pb":
		return arith.MakeLIA2PBTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "card2bv":
		return arith.MakeCard2BVTactic(m, c.config.Params, c.config.Globals, nil)
	case "pb2bv":
		return arith.MakePB2BVTactic(m, c.config.Params, c.config.Globals, nil, c.config.ModelEvaluator)
	case "skip":
		return tactic.NewSkipTactic(), nil
	case "fail":
		return tactic.NewFailTactic(), nil
	case "fail-if-undecided":
		return tactic.NewFailIfUndecidedTactic(), nil
	default:
		return nil, fmt.Errorf("invalid tactic, unknown tactic %s", name)
	}
}

func (c *Context) clearCheckSatResult() error {
	if c.tacticResult != nil {
		if err := c.tacticResult.Close(); err != nil {
			return err
		}
		c.tacticResult = nil
	}
	if c.retainedSolverResult != nil {
		if err := c.retainedSolverResult.Close(); err != nil {
			return err
		}
		c.retainedSolverResult = nil
	}
	return nil
}

func (c *Context) checkSatResult() solver.CheckSatResult {
	if c.tacticResult != nil {
		return c.tacticResult
	}
	if c.retainedSolverResult != nil {
		return c.retainedSolverResult
	}
	return c.solver
}

// AssertGoal copies the current assertions, asserted proofs and original
// assertion-name dependencies into a goal, in command-session order.
func (c *Context) AssertGoal(goal *tactic.Goal) error {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	if goal == nil || goal.Manager() != m {
		return ast.ErrASTHandle
	}
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if state.Proof && state.UnsatCore {
		return fmt.Errorf("Frontend does not support simultaneous generation of proofs and unsat cores")
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	for _, row := range c.assertions {
		var proof ast.ASTHandle
		if proofs {
			proof, err = m.MkProofAsserted(row.expression)
			if err != nil {
				return err
			}
		}
		var dep ast.ASTDependency
		if state.UnsatCore && !row.tag.IsNull() {
			dep, err = m.DependencyLeaf(row.tag)
			if err != nil {
				return err
			}
		}
		if err = goal.AssertWithProof(row.expression, proof, dep); err != nil {
			return err
		}
	}
	return nil
}

// CheckSatUsing keeps the ordinary incremental solver and its scopes intact.
// Results from the selected tactic replace the command session's last result.
func (c *Context) CheckSatUsing(selected tactic.Tactic, options *uparams.ParamsRef) (status lbool.LBool, err error) {
	m, err := c.Manager()
	if err != nil {
		return lbool.Undef, err
	}
	if selected == nil {
		return lbool.Undef, fmt.Errorf("check-sat-using needs a tactic argument")
	}
	watch, err := stopwatch.NewScopedWatch(&c.watch, true)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, watch.Close()) }()
	state, err := c.parameters.State()
	if err != nil {
		return lbool.Undef, err
	}
	if options == nil {
		empty := c.config.Params.EmptyValue()
		options = &empty
	}
	params, err := c.parameters.MergeDefaultParams(options)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, c.config.Params.Del(&params)) }()
	wrapped, err := tactic.NewUsingParamsTactic(c.config.Params, selected, &params)
	if err != nil {
		return lbool.Undef, err
	}
	if err = wrapped.IncRef(); err != nil {
		return lbool.Undef, err
	}
	defer func(owned tactic.Tactic) { err = errors.Join(err, owned.DecRef()) }(wrapped)
	var logic usymbol.Symbol
	if c.logic != "" {
		logic, err = m.Symbols.Intern(c.logic)
		if err != nil {
			return lbool.Undef, err
		}
	}
	if err = wrapped.SetLogic(logic); err != nil {
		return lbool.Undef, err
	}
	goal, err := tactic.NewGoal(m, state.Proof, state.Model, state.UnsatCore)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, goal.Close()) }()
	if err = c.AssertGoal(goal); err != nil {
		return lbool.Undef, err
	}
	c.checked = false
	if err = c.clearCheckSatResult(); err != nil {
		return lbool.Undef, err
	}
	c.tacticResult, err = solver.NewSimpleCheckSatResult(m)
	if err != nil {
		return lbool.Undef, err
	}
	var md tactic.ModelRef
	defer func() { err = errors.Join(err, md.Close()) }()
	labels := tactic.ModelLabels{Budget: m.Budget()}
	defer labels.Close()
	proofRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&proofRef)) }()
	var dep tactic.DependencyRef
	defer func() { err = errors.Join(err, dep.Close()) }()
	limitName := "rlimit"
	limit, err := c.config.Params.GetUIntName(&params, &limitName, state.RLimit)
	if err != nil {
		return lbool.Undef, err
	}
	if err = m.ResourceLimit().Push(limit); err != nil {
		return lbool.Undef, err
	}
	defer m.ResourceLimit().Pop()
	timeoutName := "timeout"
	timeout, err := c.config.Params.GetUIntName(&params, &timeoutName, state.Timeout)
	if err != nil {
		return lbool.Undef, err
	}
	if timeout != ^uint32(0) && timeout != 0 {
		timed, e := tactic.NewTryForTactic(wrapped, timeout)
		if e != nil {
			return lbool.Undef, e
		}
		if e = timed.IncRef(); e != nil {
			return lbool.Undef, e
		}
		defer func() { err = errors.Join(err, timed.DecRef()) }()
		wrapped = timed
	}
	status, reason, checkErr := tactic.CheckSat(wrapped, goal, &md, &labels, proofRef, &dep)
	if checkErr != nil {
		var fatal z3exception.Z3Error
		var source z3exception.Z3Exception
		if !errors.As(checkErr, &fatal) && errors.As(checkErr, &source) {
			if e := c.tacticResult.SetStatus(lbool.Undef); e != nil {
				return lbool.Undef, errors.Join(checkErr, e)
			}
			if e := c.tacticResult.SetReasonUnknown(source.Message()); e != nil {
				return lbool.Undef, errors.Join(checkErr, e)
			}
			c.status, c.checked = lbool.Undef, true
			stats, e := c.tacticResult.Statistics()
			if e != nil {
				return lbool.Undef, errors.Join(checkErr, e)
			}
			for key, value := range wrapped.CollectStatistics() {
				stats.UpdateDouble(statistics.Key(key), value)
			}
			return lbool.Undef, TacticCommandFailure{Cause: checkErr, Message: source.Message()}
		}
		return lbool.Undef, checkErr
	}
	r := c.tacticResult
	if err = r.SetStatus(status); err != nil {
		return lbool.Undef, err
	}
	if status == lbool.Undef {
		if reason == "" {
			reason = "unknown"
		}
		if err = r.SetReasonUnknown(reason); err != nil {
			return lbool.Undef, err
		}
	}
	model, err := md.Get()
	if err != nil {
		return lbool.Undef, err
	}
	if err = r.SetModel(model); err != nil {
		return lbool.Undef, err
	}
	proof, err := m.RefGet(proofRef)
	if err != nil {
		return lbool.Undef, err
	}
	if err = r.SetProof(proof); err != nil {
		return lbool.Undef, err
	}
	leaves, err := m.DependencyLinearize(dep.Get(), nil)
	if err != nil {
		return lbool.Undef, err
	}
	if err = r.SetUnsatCore(leaves); err != nil {
		return lbool.Undef, err
	}
	if err = r.SetLabels(labels.Values); err != nil {
		return lbool.Undef, err
	}
	stats, err := r.Statistics()
	if err != nil {
		return lbool.Undef, err
	}
	for key, value := range wrapped.CollectStatistics() {
		stats.UpdateDouble(statistics.Key(key), value)
	}
	c.status, c.checked = status, true
	return status, nil
}
