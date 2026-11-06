package smtcore

// Source: src/smt/tactic/ctx_solver_simplify_tactic.cpp/.h.

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ContextSolverSimplifyTactic performs the source solver-guided, first-occurrence
// contextual traversal. It owns a private SMT solver and simplified-app helper.
type ContextSolverSimplifyTactic struct {
	self        *ContextSolverSimplifyTactic
	manager     *ast.ASTManager
	config      SMTSolverConfig
	params      uparams.ParamsRef
	paramsReady bool
	boolSort    ast.ASTHandle
	solver      *SMTSolver
	mkApp       *rewriter.SimplifiedApp
	steps       uint32
	refs        uint32
	closed      bool
}

var _ tactic.Tactic = (*ContextSolverSimplifyTactic)(nil)

func NewContextSolverSimplifyTactic(m *ast.ASTManager, p *uparams.ParamsRef, config SMTSolverConfig) (*ContextSolverSimplifyTactic, error) {
	if m == nil || config.Params == nil || config.Globals == nil || config.Globals.Context != config.Params {
		return nil, uparams.ErrParamsOwner
	}
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ContextSolverSimplifyTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &ContextSolverSimplifyTactic{manager: m, config: config}
	t.self = t
	params, err := config.Params.NewValue()
	t.paramsReady = err == nil
	if err == nil {
		t.params = params
	}
	if err == nil && p != nil {
		err = config.Params.Assign(&t.params, p)
	}
	if err == nil {
		t.boolSort, err = m.MkBoolSort()
	}
	if err == nil {
		t.solver, err = NewSMTSolver(m, &t.params, "", config)
	}
	if err == nil {
		t.mkApp, err = rewriter.NewSimplifiedApp(m, config.Params, config.Globals, &t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *ContextSolverSimplifyTactic) open() error {
	if t == nil || t.self != t || t.manager == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *ContextSolverSimplifyTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *ContextSolverSimplifyTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
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
func (t *ContextSolverSimplifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.mkApp != nil {
		if err := t.mkApp.Close(); err != nil {
			return err
		}
		t.mkApp = nil
	}
	if t.solver != nil {
		if err := t.solver.Close(); err != nil {
			return err
		}
		t.solver = nil
	}
	if t.manager != nil && t.paramsReady {
		if err := t.config.Params.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	if t.manager != nil {
		t.manager.Budget().Release(uint64(reflect.TypeOf(ContextSolverSimplifyTactic{}).Size()))
	}
	t.manager, t.config = nil, SMTSolverConfig{}
	t.closed = true
	return nil
}

func (t *ContextSolverSimplifyTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("ctx-solver-simplify", g)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { resultErr = errors.Join(resultErr, report.Close()) }()
	}
	inconsistent, err := g.Inconsistent()
	if err != nil {
		return err
	}
	if !inconsistent {
		n, err := g.Size()
		if err != nil {
			return err
		}
		forms := make([]ast.ASTHandle, n)
		for i := uint32(0); i < n; i++ {
			forms[i], err = g.Form(i)
			if err != nil {
				return err
			}
		}
		formula, err := t.manager.MkAndArray(forms)
		if err != nil {
			return fmt.Errorf("ctx-solver-simplify: combine goal: %w", err)
		}
		var refs ctxSolverRefs
		refs.manager = t.manager
		defer func() { resultErr = errors.Join(resultErr, refs.Close()) }()
		if err = refs.Pin(formula); err != nil {
			return err
		}
		base, err := t.solver.ScopeLevel()
		if err != nil {
			return err
		}
		if err = t.solver.Push(); err != nil {
			return err
		}
		defer func() {
			level, e := t.solver.ScopeLevel()
			if e != nil {
				resultErr = errors.Join(resultErr, e)
				return
			}
			if level > base {
				resultErr = errors.Join(resultErr, t.solver.Pop(level-base))
			}
		}()
		root, err := t.manager.MkFreshConst("#ctx-solver-simplify", t.boolSort)
		if err != nil {
			return fmt.Errorf("ctx-solver-simplify: create root proxy: %w", err)
		}
		if err = refs.Pin(root); err != nil {
			return err
		}
		equiv, err := t.manager.MkIff(formula, root)
		if err != nil {
			return err
		}
		if err = refs.Pin(equiv); err != nil {
			return err
		}
		neg, err := t.manager.MkNot(equiv)
		if err != nil {
			return err
		}
		if err = refs.Pin(neg); err != nil {
			return err
		}
		if err = t.solver.Assert(neg); err != nil {
			return fmt.Errorf("ctx-solver-simplify: assert root inequivalence: %w", err)
		}
		if err = t.solver.Push(); err != nil {
			return err
		}
		result, err := t.reduce(formula, root, &refs)
		popErr := t.solver.Pop(1)
		if err != nil || popErr != nil {
			return fmt.Errorf("ctx-solver-simplify: reduce goal: %w", errors.Join(err, popErr))
		}
		if err = g.Reset(); err != nil {
			return err
		}
		if err = g.Assert(result); err != nil {
			return err
		}
		t.steps++
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}

type ctxSolverCache struct {
	result ast.ASTHandle
	parent uint64
	index  uint32
}
type ctxSolverReducer struct {
	t      *ContextSolverSimplifyTactic
	cache  map[ast.ASTHandle]ctxSolverCache
	nextID uint64
	refs   *ctxSolverRefs
}

type ctxSolverRefs struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (r *ctxSolverRefs) Pin(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err == nil {
		r.refs = append(r.refs, ref)
	}
	return err
}
func (r *ctxSolverRefs) Close() (err error) {
	for i := len(r.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, r.manager.DelRef(&r.refs[i]))
	}
	r.refs = nil
	return err
}

func (t *ContextSolverSimplifyTactic) reduce(root, proxy ast.ASTHandle, refs *ctxSolverRefs) (ast.ASTHandle, error) {
	r := &ctxSolverReducer{t: t, cache: make(map[ast.ASTHandle]ctxSolverCache), refs: refs}
	result, err := r.visit(root, proxy, 0, 0)
	return result, err
}

func (r *ctxSolverReducer) forced(proxy ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := r.t.solver.Push(); err != nil {
		return ast.ASTHandle{}, false, fmt.Errorf("forced push: %w", err)
	}
	if err := r.t.solver.Assert(proxy); err != nil {
		return ast.ASTHandle{}, false, fmt.Errorf("forced assert: %w", errors.Join(err, r.t.solver.Pop(1)))
	}
	status, err := r.t.solver.CheckSat(nil)
	popErr := r.t.solver.Pop(1)
	if err != nil || popErr != nil {
		return ast.ASTHandle{}, false, fmt.Errorf("forced check true: %w", errors.Join(err, popErr))
	}
	if status == lbool.False {
		truth, e := r.t.manager.MkTrue()
		return truth, true, e
	}
	neg, err := r.t.manager.MkNot(proxy)
	if err != nil {
		return ast.ASTHandle{}, false, fmt.Errorf("forced negation: %w", err)
	}
	if err = r.refs.Pin(neg); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = r.t.solver.Push(); err != nil {
		return ast.ASTHandle{}, false, fmt.Errorf("forced second push: %w", err)
	}
	if err = r.t.solver.Assert(neg); err != nil {
		return ast.ASTHandle{}, false, fmt.Errorf("forced assert negation: %w", errors.Join(err, r.t.solver.Pop(1)))
	}
	status, err = r.t.solver.CheckSat(nil)
	popErr = r.t.solver.Pop(1)
	if err != nil || popErr != nil {
		return ast.ASTHandle{}, false, fmt.Errorf("forced check false: %w", errors.Join(err, popErr))
	}
	if status == lbool.False {
		truth, e := r.t.manager.MkFalse()
		return truth, true, e
	}
	return ast.ASTHandle{}, false, nil
}

func (r *ctxSolverReducer) makeApp(node ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	decl, err := r.t.manager.GetDecl(node)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("simplified application declaration: %w", err)
	}
	out, err := r.t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { _ = r.t.manager.DelRef(&out) }()
	if err = r.t.mkApp.Make(decl, ast.SliceArguments(args), out); err != nil {
		return ast.ASTHandle{}, fmt.Errorf("simplified application make: %w", err)
	}
	result, err := r.t.manager.RefGet(out)
	if err == nil {
		err = r.refs.Pin(result)
	}
	return result, err
}

func (r *ctxSolverReducer) visit(node, proxy ast.ASTHandle, parent uint64, index uint32) (ast.ASTHandle, error) {
	if cached, ok := r.cache[node]; ok {
		return cached.result, nil
	}
	canceled, err := r.t.manager.Canceled()
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("visit cancellation query: %w", err)
	}
	if canceled {
		return ast.ASTHandle{}, fmt.Errorf("ctx-solver-simplify: canceled")
	}
	kind, err := r.t.manager.GetKind(node)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("visit kind: %w", err)
	}
	sort, err := r.t.manager.GetSort(node)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("visit sort: %w", err)
	}
	if sort == r.t.boolSort {
		forced, yes, err := r.forced(proxy)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if yes {
			r.cache[node] = ctxSolverCache{forced, parent, index}
			return forced, nil
		}
	}
	if kind != ast.ASTApp {
		r.cache[node] = ctxSolverCache{node, parent, index}
		return node, nil
	}
	id := r.nextID + 1
	r.nextID = id
	return r.visitUncached(node, proxy, parent, index, id)
}

func (r *ctxSolverReducer) visitUncached(node, proxy ast.ASTHandle, parent uint64, index uint32, id uint64) (ast.ASTHandle, error) {
	sort, err := r.t.manager.GetSort(node)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort == r.t.boolSort {
		forced, yes, e := r.forced(proxy)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if yes {
			r.cache[node] = ctxSolverCache{forced, parent, index}
			return forced, nil
		}
	}
	count, err := r.t.manager.GetNumArgs(node)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("traversal arity: %w", err)
	}
	args := make([]ast.ASTHandle, count)
	var child ast.ASTHandle
	var childProxy ast.ASTHandle
	var childIndex uint32
	for i := uint32(0); i < count; i++ {
		arg, e := r.t.manager.GetArg(node, i)
		if e != nil {
			return ast.ASTHandle{}, fmt.Errorf("traversal argument %d: %w", i, e)
		}
		if cached, ok := r.cache[arg]; ok {
			if cached.parent == id && cached.index == i {
				args[i] = cached.result
			} else {
				args[i] = arg
			}
			continue
		}
		if child.IsNull() {
			child, childIndex = arg, i
			sort, e := r.t.manager.GetSort(arg)
			if e != nil {
				return ast.ASTHandle{}, fmt.Errorf("child sort: %w", e)
			}
			childProxy, e = r.t.manager.MkFreshConst("#ctx-solver-simplify", sort)
			if e != nil {
				return ast.ASTHandle{}, fmt.Errorf("child proxy: %w", e)
			}
			args[i] = childProxy
		} else {
			args[i] = arg
		}
	}
	if !child.IsNull() {
		proxyApp, e := r.makeApp(node, args)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.refs.Pin(childProxy); e != nil {
			return ast.ASTHandle{}, e
		}
		eq, e := r.t.manager.MkEq(proxyApp, proxy)
		if e != nil {
			return ast.ASTHandle{}, fmt.Errorf("application proxy equality: %w", e)
		}
		if e = r.t.solver.Push(); e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.t.solver.Assert(eq); e != nil {
			return ast.ASTHandle{}, fmt.Errorf("assert application proxy: %w", errors.Join(e, r.t.solver.Pop(1)))
		}
		_, e = r.visit(child, childProxy, id, childIndex)
		popErr := r.t.solver.Pop(1)
		if e != nil || popErr != nil {
			return ast.ASTHandle{}, errors.Join(e, popErr)
		}
		return r.visitUncached(node, proxy, parent, index, id)
	}
	result, err := r.makeApp(node, args)
	if err == nil {
		r.cache[node] = ctxSolverCache{result, parent, index}
	}
	return result, err
}

func (t *ContextSolverSimplifyTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.config.Params.Assign(&t.params, p); err != nil {
		return err
	}
	return t.solver.UpdateParams(p)
}
func (t *ContextSolverSimplifyTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.solver.CollectParamDescrs(d)
}
func (t *ContextSolverSimplifyTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.closed {
		return nil
	}
	return map[string]float64{"solver-simplify-steps": float64(t.steps)}
}
func (t *ContextSolverSimplifyTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.steps = 0
	return nil
}
func (t *ContextSolverSimplifyTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	t.steps = 0
	old := t.solver
	next, err := NewSMTSolver(t.manager, &t.params, "", t.config)
	if err != nil {
		return err
	}
	if err = old.Close(); err != nil {
		return errors.Join(err, next.Close())
	}
	t.solver = next
	return nil
}
func (t *ContextSolverSimplifyTactic) Reset() error                             { return t.Cleanup() }
func (t *ContextSolverSimplifyTactic) SetLogic(_ usymbol.Symbol) error          { return t.open() }
func (t *ContextSolverSimplifyTactic) SetProgressCallback(_ func(uint32)) error { return t.open() }
func (t *ContextSolverSimplifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	config := t.config
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		if config.TargetConfig == nil {
			return nil, err
		}
		config, err = config.TargetConfig(m)
		if err != nil {
			return nil, err
		}
	}
	params, err := t.config.Params.TranslateTo(&t.params, config.Params)
	if err != nil {
		return nil, err
	}
	defer config.Params.Del(&params)
	return NewContextSolverSimplifyTactic(m, &params, config)
}
