package core

// Source: src/tactic/core/simplify_tactic.cpp/.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SimplifyTactic struct {
	self        *SimplifyTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	params      uparams.ParamsRef
	rewriter    *rewriter.TheoryRewriter
	trace       *diagnostic.TraceTags
	steps, refs uint32
	ownerBytes  uint64
	paramsReady bool
	closed      bool
}

var _ tactic.Tactic = (*SimplifyTactic)(nil)

// SimplifyConstructionFailure retains the actual partially constructed owner
// when a rewriter destructor's debug/resource guard refuses cleanup.
type SimplifyConstructionFailure struct {
	owner          *SimplifyTactic
	cause, cleanup error
}

func (e *SimplifyConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *SimplifyConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *SimplifyConstructionFailure) OriginalCause() error { return e.cause }
func (e *SimplifyConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *SimplifyConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *SimplifyConstructionFailure) Cleanup() error { return e.owner.Close() }
func (t *SimplifyTactic) constructionFailure(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &SimplifyConstructionFailure{owner: t, cause: cause, cleanup: cleanup}
	}
	return cause
}

func NewSimplifyTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (*SimplifyTactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(SimplifyTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &SimplifyTactic{manager: m, context: context, globals: globals, ownerBytes: amount}
	t.self = t
	var err error
	if t.params, err = context.NewValue(); err != nil {
		m.Budget().Release(amount)
		return nil, err
	}
	t.paramsReady = true
	if params != nil {
		if err = context.Assign(&t.params, params); err != nil {
			return nil, t.constructionFailure(err)
		}
	}
	t.rewriter, err = rewriter.NewTheoryRewriter(m, context, globals, &t.params)
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return t, nil
}

func (t *SimplifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || !t.paramsReady || t.rewriter == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *SimplifyTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *SimplifyTactic) DecRef() error {
	// Reference release must also destroy a non-executable owner left by a
	// failed cleanup/reconstruction. The source dec_ref has no live-imp check.
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}

func (t *SimplifyTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	defer func() {
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = goal.CheckOwner(); err != nil {
		return err
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil {
		return checkErr
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	if err = t.applyCore(goal); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}

func (t *SimplifyTactic) applyCore(goal *tactic.Goal) (err error) {
	report, err := tactic.NewTacticReport("simplifier", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.traceGoal("before_simplifier", 48, goal, false); err != nil {
		return err
	}
	t.steps = 0
	inconsistent, err := goal.Inconsistent()
	if err != nil {
		return err
	}
	if inconsistent {
		return nil
	}
	return t.rewriteGoal(goal)
}

func (t *SimplifyTactic) rewriteGoal(goal *tactic.Goal) (err error) {
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	proof, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&proof)) }()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		inconsistent, err := goal.Inconsistent()
		if err != nil {
			return err
		}
		if inconsistent {
			break
		}
		formula, err := goal.Form(i)
		if err != nil {
			return err
		}
		if proofs {
			err = t.rewriter.RewriteWithProof(formula, out, proof)
		} else {
			err = t.rewriter.Rewrite(formula, out)
		}
		if err != nil {
			return err
		}
		steps, err := t.rewriter.NumSteps()
		if err != nil {
			return err
		}
		t.steps += steps
		value, err := t.manager.RefGet(out)
		if err != nil {
			return err
		}
		var newProof ast.ASTHandle
		if proofs {
			oldProof, err := goal.Proof(i)
			if err != nil {
				return err
			}
			rewriteProof, err := t.manager.RefGet(proof)
			if err != nil {
				return err
			}
			newProof, err = t.manager.MkProofModusPonens(oldProof, rewriteProof)
			if err != nil {
				return err
			}
			if err = t.manager.SetRef(proof, newProof); err != nil {
				return err
			}
		}
		dependency, err := goal.Dependency(i)
		if err != nil {
			return err
		}
		if err = goal.Update(i, value, newProof, dependency); err != nil {
			return err
		}
	}
	if err = t.traceGoal("after_simplifier_bug", 67, goal, false); err != nil {
		return err
	}
	if err = goal.ElimRedundancies(); err != nil {
		return err
	}
	if err = t.traceGoal("after_simplifier", 69, goal, false); err != nil {
		return err
	}
	if err = t.traceGoal("after_simplifier_detail", 70, goal, true); err != nil {
		return err
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil {
		return checkErr
	} else if !sorted {
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SimplifyTactic) NumSteps() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.steps, nil
}

func SimplifyParameterDescriptions(out *uparams.ParamDescrs) error {
	return rewriter.TheoryRewriterParameterDescriptions(out)
}

func (t *SimplifyTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, params); err != nil {
		return err
	}
	return t.rewriter.UpdateParams(params)
}

func (t *SimplifyTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return SimplifyParameterDescriptions(out)
}

func (*SimplifyTactic) CollectStatistics() map[string]float64    { return nil }
func (t *SimplifyTactic) ResetStatistics() error                 { return t.open() }
func (t *SimplifyTactic) Reset() error                           { return t.Cleanup() }
func (t *SimplifyTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *SimplifyTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *SimplifyTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.rewriter.Close(); err != nil {
		return err
	}
	t.rewriter = nil
	var err error
	t.rewriter, err = rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, &t.params)
	t.steps = 0
	return err
}

func (t *SimplifyTactic) Translate(manager *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	translated, err := NewSimplifyTactic(manager, t.context, t.globals, &t.params)
	if err != nil {
		return nil, err
	}
	translated.trace = t.trace
	return translated, nil
}

func (t *SimplifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			return err
		}
		t.rewriter = nil
	}
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *SimplifyTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}

func (t *SimplifyTactic) traceGoal(tag string, line int, goal *tactic.Goal, dependencies bool) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, "operator()", "src/tactic/core/simplify_tactic.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			if dependencies {
				return goal.DisplayWithDependencies(out)
			}
			return goal.Display(out)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// MakeSimplifyTactic is mk_simplify_tactic's clean wrapper.
func MakeSimplifyTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (tactic.Tactic, error) {
	child, err := NewSimplifyTactic(manager, context, globals, params)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

func MakeEliminateAndTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (result tactic.Tactic, err error) {
	if context == nil {
		return nil, uparams.ErrParamsOwner
	}
	settings, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, context.Del(&settings)) }()
	if params != nil {
		if err = context.Assign(&settings, params); err != nil {
			return nil, err
		}
	}
	name := "elim_and"
	if err = context.SetBoolName(&settings, &name, true); err != nil {
		return nil, err
	}
	child, err := MakeSimplifyTactic(manager, context, globals, &settings)
	if err != nil {
		return nil, err
	}
	if err = child.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, child.DecRef()) }()
	return tactic.NewUsingParamsTactic(context, child, &settings)
}
