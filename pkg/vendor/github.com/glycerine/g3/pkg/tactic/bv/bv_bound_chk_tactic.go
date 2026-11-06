package bv

// Source: src/tactic/bv/bv_bound_chk_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type BVBoundCheckStats struct{ Unsats, Singletons, Reduces uint32 }
type BVBoundCheckTactic struct {
	self        *BVBoundCheckTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	services    model.ModelEvaluatorConfig
	params      uparams.ParamsRef
	paramsReady bool
	current     *bvBoundCheckImplementation
	retired     []*bvBoundCheckImplementation
	stats       BVBoundCheckStats
	trace       *diagnostic.TraceTags
	header      uint64
	refs        uint32
	closed      bool
}
type bvBoundCheckImplementation struct {
	configuration bvBoundCheckConfiguration
	rewriter      *rewriter.Traversal
	out           ast.ASTRef
	header        uint64
}
type BVBoundCheckConstructionFailure struct {
	owner          *BVBoundCheckTactic
	cause, cleanup error
}

func (e *BVBoundCheckConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *BVBoundCheckConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *BVBoundCheckConstructionFailure) OriginalCause() error { return e.cause }
func (e *BVBoundCheckConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *BVBoundCheckConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.services.Memory
}
func (e *BVBoundCheckConstructionFailure) Cleanup() error { return e.owner.Close() }
func (t *BVBoundCheckTactic) constructionFailure(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &BVBoundCheckConstructionFailure{owner: t, cause: cause, cleanup: cleanup}
	}
	return cause
}

// The source factory returns the raw tactic; cleanup rebuilds before swapping.
func NewBVBoundCheckTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*BVBoundCheckTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(BVBoundCheckTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &BVBoundCheckTactic{manager: m, context: c, globals: g, services: services, header: n}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.current, err = t.build()
	}
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return t, nil
}
func (t *BVBoundCheckTactic) build() (*bvBoundCheckImplementation, error) {
	n := uint64(reflect.TypeOf(bvBoundCheckImplementation{}).Size())
	if err := t.manager.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &bvBoundCheckImplementation{header: n}
	r.configuration.owner = t
	var err error
	r.rewriter, err = rewriter.NewTraversal(t.manager, &r.configuration)
	if err == nil {
		r.configuration.boolean, err = rewriter.NewBoolRewriter(t.manager, t.context, t.globals, nil)
	}
	if err == nil {
		err = r.configuration.update(&t.params)
	}
	if err == nil {
		r.out, err = t.manager.NewRef(ast.ASTHandle{})
	}
	return r, err
}
func (t *BVBoundCheckTactic) closeImplementation(r *bvBoundCheckImplementation) error {
	if r == nil {
		return nil
	}
	if r.rewriter != nil {
		if err := r.rewriter.Close(); err != nil {
			return err
		}
		r.rewriter = nil
	}
	if err := r.configuration.close(); err != nil {
		return err
	}
	if r.out != (ast.ASTRef{}) {
		if err := t.manager.DelRef(&r.out); err != nil {
			return err
		}
		r.out = ast.ASTRef{}
	}
	t.manager.Budget().Release(r.header)
	r.header = 0
	return nil
}
func (t *BVBoundCheckTactic) drain() error {
	for len(t.retired) > 0 {
		i := len(t.retired) - 1
		if err := t.closeImplementation(t.retired[i]); err != nil {
			return err
		}
		t.retired = t.retired[:i]
	}
	return nil
}
func (t *BVBoundCheckTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if !t.paramsReady || t.current == nil {
		return ast.ErrASTPending
	}
	return nil
}
func (t *BVBoundCheckTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *BVBoundCheckTactic) DecRef() error {
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
func (t *BVBoundCheckTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.drain(); err != nil {
		return err
	}
	if err := t.closeImplementation(t.current); err != nil {
		return err
	}
	t.current = nil
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}
func (t *BVBoundCheckTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		empty := t.context.EmptyValue()
		p = &empty
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.current.configuration.update(p)
}
func (t *BVBoundCheckTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.drain(); err != nil {
		return err
	}
	fresh, err := t.build()
	if err != nil {
		if fresh != nil {
			t.retired = append(t.retired, fresh)
		}
		return err
	}
	old := t.current
	t.current = fresh
	t.retired = append(t.retired, old)
	return t.drain()
}
func (t *BVBoundCheckTactic) Reset() error { return t.Cleanup() }
func (t *BVBoundCheckTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewBVBoundCheckTactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *BVBoundCheckTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *BVBoundCheckTactic) CollectStatistics() map[string]float64 {
	return map[string]float64{"unsat bv bounds": float64(t.stats.Unsats), "bv singletons": float64(t.stats.Singletons), "bv reduces": float64(t.stats.Reduces)}
}
func (t *BVBoundCheckTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats = BVBoundCheckStats{}
	return nil
}
func (t *BVBoundCheckTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *BVBoundCheckTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *BVBoundCheckTactic) assertSorted(g *tactic.Goal) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	good, err := g.IsWellSorted()
	if err != nil {
		return err
	}
	if !good {
		return ast.ErrASTDomain
	}
	return nil
}
func (t *BVBoundCheckTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.assertSorted(g); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("bv-bound-chk", g); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("bv-bound-chk", g); err != nil {
		return err
	}
	if err = t.emitGoal(g, true); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	if err = t.rewriteGoal(g); err != nil {
		return err
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(g); err != nil {
		return err
	}
	if err = t.emitGoal(g, false); err != nil {
		return err
	}
	return t.assertSorted(g)
}
func (t *BVBoundCheckTactic) rewriteGoal(g *tactic.Goal) (err error) {
	if err = t.assertSorted(g); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("bv-bound-chk", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	n, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		bad, err := g.Inconsistent()
		if err != nil {
			return err
		}
		if bad {
			break
		}
		h, err := g.Form(i)
		if err != nil {
			return err
		}
		if err = t.current.rewriter.Rewrite(h, t.current.out); err != nil {
			return err
		}
		h, err = t.manager.RefGet(t.current.out)
		if err != nil {
			return err
		}
		if err = g.Update(i, h, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	// Source calls only default cfg.cleanup(), not rewriter cleanup/reset.
	t.current.configuration.DefaultConfiguration.Cleanup()
	return nil
}
