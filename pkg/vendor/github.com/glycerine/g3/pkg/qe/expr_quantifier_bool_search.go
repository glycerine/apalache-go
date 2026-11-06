package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::check/final_check scalar subset.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Fresh arithmetic, bit-vector, datatype, and finite-domain variables use the
// same refreshed solver context and branch machinery as source add_var.
func exprQENewVariablesSupported(variables *exprQEBranchVariables, context *SolverContext, start int) (bool, error) {
	if variables == nil || context == nil || start < 0 || start > len(variables.entries) || variables.manager != context.manager {
		return false, ast.ErrASTHandle
	}
	for _, entry := range variables.entries[start:] {
		value, err := variables.manager.RefGet(entry.variable)
		if err != nil {
			return false, err
		}
		found, err := context.HasPlugin(value)
		if err != nil || !found {
			return false, err
		}
		plugin, err := context.PluginFor(value)
		if err != nil {
			return false, err
		}
		switch plugin.(type) {
		case *ArithmeticPlugin, *NonlinearArithmeticPlugin, *BitVectorPlugin, *BoolPlugin, *DatatypeSolverPlugin, *DatalogPlugin:
		default:
			return false, nil
		}
	}
	return true, nil
}

// searchExprQEScalarTree explores model-selected scalar assignments and
// Boolean partitions. Unsupported plugin branches report incomplete.
func searchExprQEScalarTree(root *exprQETree, variables *exprQEBranchVariables, context *SolverContext, plugin *BoolPlugin, partitioner *ConjunctionPartitioner, solver *smtcore.SMTSolver, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (result ast.ASTRef, complete bool, err error) {
	if root == nil || root.manager == nil || root.formula == (ast.ASTRef{}) || variables == nil || context == nil || plugin == nil || partitioner == nil || solver == nil || rw == nil || nnf == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	m := root.manager
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	var free []ast.ASTRef
	defer func() {
		for i := range free {
			err = errors.Join(err, m.DelRef(&free[i]))
		}
	}()
	for {
		canceled, e := m.Canceled()
		if e != nil {
			return result, false, e
		}
		if canceled {
			return result, false, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
		}
		if e = config.Cooperation.Cooperate("qe"); e != nil {
			return result, false, e
		}
		status, e := solver.CheckSat(nil)
		if e != nil {
			return result, false, e
		}
		if status == lbool.Undef {
			return result, false, nil
		}
		if status == lbool.False {
			break
		}
		md, e := solver.AcquireModel()
		if e != nil || md == nil {
			return result, false, errors.Join(e, ast.ErrASTDomain)
		}
		eval, e := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace})
		if e != nil {
			if eval != nil {
				e = errors.Join(e, eval.Close())
			}
			return result, false, errors.Join(e, md.DecRef())
		}
		variableCount := len(variables.entries)
		current, step, e := advanceExprQEScalarTree(root, variables, context, eval, config, rw, nnf, solver, false)
		if e == nil && len(variables.entries) != variableCount {
			var supported bool
			supported, e = exprQENewVariablesSupported(variables, context, variableCount)
			if e != nil || !supported {
				e = errors.Join(e, eval.Close(), md.DecRef())
				return result, false, e
			}
		}
		if e == nil && step == exprQEChooseVariable {
			var checked lbool.LBool
			checked, e = solver.CheckSat(nil)
			if e == nil && checked != lbool.True {
				e = errors.Join(eval.Close(), md.DecRef())
				if e != nil || checked == lbool.Undef {
					return result, false, e
				}
				continue
			}
			if e == nil {
				var refreshed *model.ModelCore
				refreshed, e = solver.AcquireModel()
				if e == nil && refreshed == nil {
					e = ast.ErrASTDomain
				}
				if e == nil {
					var nextEval *model.ModelEvaluator
					nextEval, e = model.NewModelEvaluator(refreshed, model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace})
					if e == nil {
						e = errors.Join(eval.Close(), md.DecRef())
						eval, md = nextEval, refreshed
					} else {
						if nextEval != nil {
							e = errors.Join(e, nextEval.Close())
						}
						e = errors.Join(e, refreshed.DecRef())
					}
				}
			}
			if e == nil {
				var selected *exprQETree
				var nextStep exprQETreeStatus
				selected, nextStep, _, e = seekExprQETree(root, variables, eval)
				if e == nil && (selected != current || nextStep != exprQEChooseVariable) {
					e = errors.Join(eval.Close(), md.DecRef())
					if e != nil {
						return result, false, e
					}
					continue
				}
			}
		}
		if e == nil && step == exprQEChooseVariable {
			var outcome exprQEPopStatus
			current, outcome, e = popExprQEBoolTree(current, variables, context, plugin, partitioner, solver, eval, md, config, rw, nnf, &free, false)
			if e == nil && outcome == exprQEPopNeedsPartition {
				part, cause := partitionExprQETree(current, variables, partitioner)
				if cause == nil {
					var done bool
					done, cause = processExprQEScalarPartition(current, part, solver, config)
					cause = errors.Join(cause, part.Close())
					if cause == nil && !done {
						outcome = exprQEPopNoBranch
					}
				}
				e = cause
			}
			if e == nil && outcome == exprQEPopNoBranch {
				e = errors.Join(eval.Close(), md.DecRef())
				return result, false, e
			}
		}
		e = errors.Join(e, eval.Close(), md.DecRef())
		if e != nil {
			return result, false, e
		}
	}
	if len(free) != 0 {
		return result, false, nil
	}
	leaves, err := root.leaves()
	if err != nil {
		return result, false, err
	}
	defer func() {
		for i := range leaves {
			err = errors.Join(err, m.DelRef(&leaves[i]))
		}
	}()
	forms := make([]ast.ASTHandle, len(leaves))
	for i, ref := range leaves {
		forms[i], err = m.RefGet(ref)
		if err != nil {
			return result, false, err
		}
	}
	formula, err := m.MkOrArgs(forms)
	if err != nil {
		return result, false, err
	}
	result, err = m.NewRef(formula)
	if err != nil {
		return result, false, err
	}
	if err = rw.Rewrite(formula, result); err != nil {
		return result, false, err
	}
	return result, true, nil
}
