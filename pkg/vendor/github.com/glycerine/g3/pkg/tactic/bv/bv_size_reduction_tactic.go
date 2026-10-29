package bv

// Source: src/tactic/bv/bv_size_reduction_tactic.cpp/.h.

import (
	"errors"
	"io"
	"math/big"
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

type BVSizeReductionTactic struct {
	self     *BVSizeReductionTactic
	manager  *ast.ASTManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	services model.ModelEvaluatorConfig
	current  *bvSizeReductionImplementation
	trace    *diagnostic.TraceTags
	header   uint64
	refs     uint32
	closed   bool
}

var _ tactic.Tactic = (*BVSizeReductionTactic)(nil)

type BVSizeReductionConstructionFailure struct {
	owner          *BVSizeReductionTactic
	cause, cleanup error
}

func (e *BVSizeReductionConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *BVSizeReductionConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *BVSizeReductionConstructionFailure) OriginalCause() error { return e.cause }
func (e *BVSizeReductionConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *BVSizeReductionConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.services.Memory
}
func (e *BVSizeReductionConstructionFailure) Cleanup() error { return e.owner.Close() }
func (t *BVSizeReductionTactic) constructionFailure(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &BVSizeReductionConstructionFailure{owner: t, cause: cause, cleanup: cleanup}
	}
	return cause
}

func NewBVSizeReductionTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (*BVSizeReductionTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(BVSizeReductionTactic{}).Size() + reflect.TypeOf(bvSizeReductionImplementation{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &BVSizeReductionTactic{manager: m, context: c, globals: g, services: services, header: n}
	t.self = t
	var err error
	t.current, err = t.build()
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return t, nil
}

// The source factory ignores p; this tactic has no parameter policy.
func MakeBVSizeReductionTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, _ *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	t, err := NewBVSizeReductionTactic(m, c, g, services)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return w, nil
}
func (t *BVSizeReductionTactic) build() (r *bvSizeReductionImplementation, err error) {
	r = &bvSizeReductionImplementation{lower: make(map[ast.ASTHandle]*big.Int), upper: make(map[ast.ASTHandle]*big.Int), keys: make(map[ast.ASTHandle]bool)}
	r.family, err = t.manager.BitVectorUtilityFamilyID()
	if err != nil {
		return r, err
	}
	r.replacer, err = rewriter.NewDefaultExprReplacer(t.manager)
	if err != nil {
		return r, err
	}
	r.out, err = t.manager.NewRef(ast.ASTHandle{})
	r.ready = err == nil
	return r, err
}
func (t *BVSizeReductionTactic) shell() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *BVSizeReductionTactic) open() error {
	if err := t.shell(); err != nil {
		return err
	}
	if t.current == nil || !t.current.ready {
		return ast.ErrASTPending
	}
	return nil
}
func (t *BVSizeReductionTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *BVSizeReductionTactic) DecRef() error {
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
func (t *BVSizeReductionTactic) closeImplementation(r *bvSizeReductionImplementation) error {
	if r == nil {
		return nil
	}
	r.ready = false
	if r.replacer != nil {
		if err := r.replacer.Close(); err != nil {
			return err
		}
		r.replacer = nil
	}
	r.active = nil
	if err := t.releaseSubstitutions(r); err != nil {
		return err
	}
	if err := t.releaseConverters(r, true, true); err != nil {
		return err
	}
	for len(r.pending) > 0 {
		i := len(r.pending) - 1
		if err := r.pending[i].Close(); err != nil {
			return err
		}
		r.pending = r.pending[:i]
	}
	if r.out != (ast.ASTRef{}) {
		if err := t.manager.DelRef(&r.out); err != nil {
			return err
		}
		r.out = ast.ASTRef{}
	}
	if err := t.releaseScratch(r, 0); err != nil {
		return err
	}
	for len(r.held) > 0 {
		i := len(r.held) - 1
		if err := t.manager.DecRef(r.held[i]); err != nil {
			return err
		}
		r.held = r.held[:i]
	}
	r.lower = nil
	r.upper = nil
	r.keys = nil
	return nil
}
func (t *BVSizeReductionTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.closeImplementation(t.current); err != nil {
		return err
	}
	t.current = nil
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}

func (t *BVSizeReductionTactic) Cleanup() error {
	if err := t.shell(); err != nil {
		return err
	}
	// This source cleanup destroys the old imp before placement construction.
	// A refused rebuild therefore leaves a partial owner, not the old state.
	if err := t.closeImplementation(t.current); err != nil {
		return err
	}
	t.current = nil
	var err error
	t.current, err = t.build()
	return err
}
func (t *BVSizeReductionTactic) checkSorted(g *tactic.Goal) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil {
		return err
	}
	if enabled {
		good, err := g.IsWellSorted()
		if err != nil {
			return err
		}
		if !good {
			return ast.ErrASTDomain
		}
	}
	return nil
}
func (t *BVSizeReductionTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.checkSorted(g); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("bv-size-reduction", g); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("bv-size-reduction", g); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	bad, err := g.Inconsistent()
	if err != nil {
		return err
	}
	if !bad {
		if err = t.emit("before_bv_size_reduction", 185, func(w io.Writer) error { return g.Display(w) }); err != nil {
			return err
		}
		if err = t.releaseConverters(t.current, true, false); err != nil {
			return err
		}
		count, completed, e := t.reduce(g, result)
		if e != nil {
			return e
		}
		if completed {
			if err = tactic.ReportTacticProgress(":bv-reduced", count); err != nil {
				return err
			}
			if err = t.emit("after_bv_size_reduction", 372, func(w io.Writer) error {
				if err := g.Display(w); err != nil {
					return err
				}
				if t.current.extension != nil {
					return t.current.extension.Display(w)
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(g); err != nil {
		return err
	}
	return t.checkSorted(g)
}
func (t *BVSizeReductionTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	other, err := NewBVSizeReductionTactic(m, t.context, t.globals, t.services)
	if err == nil {
		other.trace = t.trace
	}
	return other, err
}
func (t *BVSizeReductionTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *BVSizeReductionTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *BVSizeReductionTactic) CollectStatistics() map[string]float64         { return nil }
func (t *BVSizeReductionTactic) ResetStatistics() error                        { return t.open() }
func (t *BVSizeReductionTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *BVSizeReductionTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *BVSizeReductionTactic) Reset() error                                  { return t.Cleanup() }
