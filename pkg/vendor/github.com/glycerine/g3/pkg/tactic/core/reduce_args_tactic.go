package core

// Source: src/tactic/core/reduce_args_tactic.cpp/.h.

import (
	"errors"
	"io"
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

type ReduceArgsTactic struct {
	self          *ReduceArgsTactic
	manager       *ast.ASTManager
	context       *uparams.ParamsContext
	globals       *uparams.GlobalParamsService
	services      model.ModelEvaluatorConfig
	family        ast.FamilyID
	pending       []*reduceArgsFrame
	trace         *diagnostic.TraceTags
	header        uint64
	refs          uint32
	ready, closed bool
}

type ReduceArgsConstructionFailure struct {
	owner          *ReduceArgsTactic
	cause, cleanup error
}

func (e *ReduceArgsConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *ReduceArgsConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *ReduceArgsConstructionFailure) OriginalCause() error { return e.cause }
func (e *ReduceArgsConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *ReduceArgsConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.services.Memory
}
func (e *ReduceArgsConstructionFailure) Cleanup() error { return e.owner.Close() }

func NewReduceArgsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (*ReduceArgsTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ReduceArgsTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &ReduceArgsTactic{manager: m, context: c, globals: g, services: services, header: n}
	t.self = t
	var err error
	t.family, err = m.BitVectorUtilityFamilyID()
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	t.ready = true
	return t, nil
}
func (t *ReduceArgsTactic) constructionFailure(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &ReduceArgsConstructionFailure{owner: t, cause: cause, cleanup: cleanup}
	}
	return cause
}

// The source factory ignores parameters and wraps the raw tactic in clean().
func MakeReduceArgsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, _ *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	t, err := NewReduceArgsTactic(m, c, g, services)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return w, nil
}
func (t *ReduceArgsTactic) shell() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *ReduceArgsTactic) open() error {
	if err := t.shell(); err != nil {
		return err
	}
	if !t.ready {
		return ast.ErrASTPending
	}
	return nil
}
func (t *ReduceArgsTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *ReduceArgsTactic) DecRef() error {
	if err := t.shell(); err != nil {
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
func (t *ReduceArgsTactic) drain() error {
	for len(t.pending) > 0 {
		i := len(t.pending) - 1
		if err := t.pending[i].Close(); err != nil {
			return err
		}
		t.pending = t.pending[:i]
	}
	return nil
}
func (t *ReduceArgsTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.drain(); err != nil {
		return err
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.ready = false
	t.closed = true
	return nil
}
func (t *ReduceArgsTactic) Cleanup() error {
	if err := t.shell(); err != nil {
		return err
	}
	t.ready = false
	if err := t.drain(); err != nil {
		return err
	}
	var err error
	t.family, err = t.manager.BitVectorUtilityFamilyID()
	t.ready = err == nil
	return err
}
func (t *ReduceArgsTactic) Reset() error                                  { return t.Cleanup() }
func (t *ReduceArgsTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *ReduceArgsTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *ReduceArgsTactic) CollectStatistics() map[string]float64         { return nil }
func (t *ReduceArgsTactic) ResetStatistics() error                        { return t.open() }
func (t *ReduceArgsTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *ReduceArgsTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *ReduceArgsTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewReduceArgsTactic(m, t.context, t.globals, t.services)
}
func (t *ReduceArgsTactic) checkpoint() error {
	canceled, err := t.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
	}
	return t.services.Cooperation.Cooperate("reduce-args")
}
func (t *ReduceArgsTactic) assertSorted(g *tactic.Goal) error {
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
func (t *ReduceArgsTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.assertSorted(g); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("reduce-args", g); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("reduce-args", g); err != nil {
		return err
	}
	if err = t.drain(); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	if err = t.runReduceArgs(g, result); err != nil {
		return err
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(g); err != nil {
		return err
	}
	return t.assertSorted(g)
}

func (t *ReduceArgsTactic) runReduceArgs(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	inconsistent, err := g.Inconsistent()
	if err != nil {
		return err
	}
	if !inconsistent {
		if err = t.emitGoal(g, nil, 449); err != nil {
			return err
		}
		report, e := tactic.NewTacticReport("reduce-args", g)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, report.Close()) }()
		f := &reduceArgsFrame{owner: t, candidates: make(map[ast.ASTHandle]*reduceArgsCandidate), groups: make(map[ast.ASTHandle]map[string]*reduceArgsEntry)}
		t.pending = append(t.pending, f)
		defer func() {
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				f.failures = append(f.failures, failure)
			}
			err = errors.Join(err, t.drain())
		}()
		if err = f.analyze(g); err != nil {
			return err
		}
		if len(f.candidates) > 0 {
			f.rewriter, err = rewriter.NewTraversal(t.manager, &reduceArgsConfiguration{frame: f})
			if err != nil {
				return err
			}
			f.out, err = t.manager.NewRef(ast.ASTHandle{})
			if err != nil {
				return err
			}
			sz, e := g.Size()
			if e != nil {
				return e
			}
			for i := uint32(0); i < sz; i++ {
				bad, e := g.Inconsistent()
				if e != nil {
					return e
				}
				if bad {
					break
				}
				form, e := g.Form(i)
				if e != nil {
					return e
				}
				if err = f.rewriter.Rewrite(form, f.out); err != nil {
					return err
				}
				h, e := t.manager.RefGet(f.out)
				if e != nil {
					return e
				}
				if err = g.Update(i, h, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
					return err
				}
			}
			if err = tactic.ReportTacticProgress(":reduced-funcs", uint32(len(f.candidates))); err != nil {
				return err
			}
			models, e := g.ModelsEnabled()
			if e != nil {
				return e
			}
			if models {
				if err = f.makeModelConverter(result); err != nil {
					return err
				}
			}
			if err = t.emitGoal(g, result.ModelConverter, 477); err != nil {
				return err
			}
		}
	}
	return nil
}

type reduceArgsCandidate struct {
	mask  []bool
	bases []ast.ASTHandle
}
type reduceArgsEntry struct{ application, declaration ast.ASTHandle }
type reduceArgsFrame struct {
	owner         *ReduceArgsTactic
	candidates    map[ast.ASTHandle]*reduceArgsCandidate
	groups        map[ast.ASTHandle]map[string]*reduceArgsEntry
	rewriter      *rewriter.Traversal
	out           ast.ASTRef
	held, scratch []ast.ASTHandle
	extension     *tactic.ExtensionModelConverter
	filter        *tactic.FilterModelConverter
	pending       []io.Closer
	failures      []memory.ConstructionFailure
}

func (f *reduceArgsFrame) hold(h ast.ASTHandle, scratch bool) error {
	if err := f.owner.manager.IncRef(h); err != nil {
		return err
	}
	if scratch {
		f.scratch = append(f.scratch, h)
	} else {
		f.held = append(f.held, h)
	}
	return nil
}
func (f *reduceArgsFrame) release(handles *[]ast.ASTHandle, limit int) error {
	for len(*handles) > limit {
		i := len(*handles) - 1
		consumed, err := f.owner.manager.DecRefConsumed((*handles)[i])
		if consumed {
			*handles = (*handles)[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (f *reduceArgsFrame) Close() error {
	for len(f.failures) > 0 {
		i := len(f.failures) - 1
		if err := f.failures[i].Cleanup(); err != nil {
			return err
		}
		f.failures = f.failures[:i]
	}
	for len(f.pending) > 0 {
		i := len(f.pending) - 1
		if err := f.pending[i].Close(); err != nil {
			return err
		}
		f.pending = f.pending[:i]
	}
	if f.rewriter != nil {
		if err := f.rewriter.Close(); err != nil {
			return err
		}
		f.rewriter = nil
	}
	if f.out != (ast.ASTRef{}) {
		if err := f.owner.manager.DelRef(&f.out); err != nil {
			return err
		}
		f.out = ast.ASTRef{}
	}
	if f.filter != nil {
		o := f.filter
		consumed, err := o.DecRefConsumed()
		if consumed {
			f.filter = nil
			if err != nil {
				f.pending = append(f.pending, o)
			}
		}
		if err != nil {
			return err
		}
	}
	if f.extension != nil {
		o := f.extension
		consumed, err := o.DecRefConsumed()
		if consumed {
			f.extension = nil
			if err != nil {
				f.pending = append(f.pending, o)
			}
		}
		if err != nil {
			return err
		}
	}
	if err := f.release(&f.scratch, 0); err != nil {
		return err
	}
	return f.release(&f.held, 0)
}
