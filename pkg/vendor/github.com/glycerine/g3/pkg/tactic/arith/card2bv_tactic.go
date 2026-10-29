package arith

// Source: src/tactic/arith/card2bv_tactic.cpp/.h, retained tactic wrapper.

import (
	"errors"
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

type Card2BVTactic struct {
	self        *Card2BVTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	params      uparams.ParamsRef
	paramsReady bool
	pending     []*card2BVApplication
	trace       *diagnostic.TraceTags
	refs        uint32
	header      uint64
	closed      bool
}
type card2BVApplication struct {
	owner          *Card2BVTactic
	theory         *rewriter.TheoryRewriter
	pseudo         *rewriter.PB2BVRewriter
	first, second  ast.ASTRef
	proof1, proof2 ast.ASTRef
	clauses        []ast.ASTHandle
	filter         *tactic.FilterModelConverter
	filterOwned    bool
	filterCleanup  bool
	failures       []memory.ConstructionFailure
}
type Card2BVConstructionFailure struct {
	owner          *Card2BVTactic
	cause, cleanup error
}

func (e *Card2BVConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *Card2BVConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *Card2BVConstructionFailure) OriginalCause() error { return e.cause }
func (e *Card2BVConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *Card2BVConstructionFailure) Cleanup() error       { return e.owner.Close() }
func (e *Card2BVConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}

var _ tactic.Tactic = (*Card2BVTactic)(nil)

func NewCard2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*Card2BVTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(Card2BVTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &Card2BVTactic{manager: m, context: c, globals: g, header: n}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err != nil {
		if cleanup := t.Close(); cleanup != nil {
			return nil, &Card2BVConstructionFailure{owner: t, cause: err, cleanup: cleanup}
		}
		return nil, err
	}
	return t, nil
}
func MakeCard2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (tactic.Tactic, error) {
	child, err := NewCard2BVTactic(m, c, g, p)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *Card2BVTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if !t.paramsReady {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *Card2BVTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *Card2BVTactic) DecRef() error {
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
func (t *Card2BVTactic) drain() error {
	for len(t.pending) > 0 {
		i := len(t.pending) - 1
		if err := t.pending[i].close(); err != nil {
			return err
		}
		t.pending = t.pending[:i]
	}
	return nil
}
func (t *Card2BVTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.drain(); err != nil {
		return err
	}
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
func (t *Card2BVTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		empty := t.context.EmptyValue()
		p = &empty
	}
	return t.context.Assign(&t.params, p)
}
func (t *Card2BVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewCard2BVTactic(m, t.context, t.globals, &t.params)
}
func (t *Card2BVTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.drain() // Source no-op after lexical application state unwinds.
}
func (t *Card2BVTactic) Reset() error                                  { return t.Cleanup() }
func (t *Card2BVTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *Card2BVTactic) CollectStatistics() map[string]float64         { return map[string]float64{} }
func (t *Card2BVTactic) ResetStatistics() error                        { return t.open() }
func (t *Card2BVTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *Card2BVTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *Card2BVTactic) assertSorted(g *tactic.Goal) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	wellSorted, err := g.IsWellSorted()
	if err == nil && !wellSorted {
		return ast.ErrASTDomain
	}
	return err
}
func (t *Card2BVTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.drain(); err != nil {
		return err
	}
	if err = t.traceGoal("card2bv-before", 59, g); err != nil {
		return err
	}
	if err = t.assertSorted(g); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("card2bv", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	w := &card2BVApplication{owner: t}
	t.pending = append(t.pending, w)
	defer func() {
		var failure memory.ConstructionFailure
		if errors.As(err, &failure) {
			w.failures = append(w.failures, failure)
		}
		err = errors.Join(err, t.drain())
	}()
	w.theory, err = rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, &t.params)
	if err == nil {
		w.pseudo, err = rewriter.NewPB2BVRewriter(t.manager, t.context, &t.params)
	}
	if err != nil {
		return err
	}
	if err = w.pseudo.UpdateParams(&t.params); err != nil {
		return err
	}
	bad, err := g.Inconsistent()
	if err != nil {
		return err
	}
	if bad {
		return result.AddGoal(g) // Source leaves depth unchanged on this path.
	}
	for _, ref := range []*ast.ASTRef{&w.first, &w.second, &w.proof1, &w.proof2} {
		*ref, err = t.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
	}
	if err = w.rewriteGoal(g); err != nil {
		return err
	}
	if w.filter != nil {
		if err = result.SetModelConverter(w.filter); err != nil {
			return err
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(g); err != nil {
		return err
	}
	if err = t.traceGoal("card2bv", 100, g); err != nil {
		return err
	}
	return t.assertSorted(g)
}
func (w *card2BVApplication) rewriteGoal(g *tactic.Goal) error {
	t := w.owner
	m := t.manager
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return err
	}
	for i := uint32(0); ; i++ {
		bad, err := g.Inconsistent()
		if err != nil || bad {
			if err != nil {
				return err
			}
			break
		}
		n, err := g.Size()
		if err != nil {
			return err
		}
		if i >= n {
			break
		}
		f, err := g.Form(i)
		if err != nil {
			return err
		}
		if proofs {
			err = w.theory.RewriteWithProof(f, w.first, w.proof1)
		} else {
			err = w.theory.Rewrite(f, w.first)
		}
		if err != nil {
			return err
		}
		f, err = m.RefGet(w.first)
		if err != nil {
			return err
		}
		if err = t.traceRewrite(f); err != nil {
			return err
		}
		// The source PB traversal is non-proof-generating and leaves the
		// initially empty new_pr2 slot untouched, even in proof-enabled goals.
		if err = w.pseudo.Rewrite(f, w.second, w.proof2); err != nil {
			return err
		}
		if proofs {
			original, err := g.Proof(i)
			if err != nil {
				return err
			}
			for _, rewrite := range []ast.ASTRef{w.proof1, w.proof2} {
				pr, err := m.RefGet(rewrite)
				if err != nil {
					return err
				}
				original, err = m.MkProofModusPonens(original, pr)
				if err != nil {
					return err
				}
				if err = m.SetRef(w.proof1, original); err != nil {
					return err
				}
			}
		}
		f, err = m.RefGet(w.second)
		if err != nil {
			return err
		}
		pr, err := m.RefGet(w.proof1)
		if err != nil {
			return err
		}
		dep, err := g.Dependency(i)
		if err != nil {
			return err
		}
		if err = g.Update(i, f, pr, dep); err != nil {
			return err
		}
	}
	w.clauses, err = w.pseudo.FlushSideConstraints()
	if err != nil {
		return err
	}
	for _, clause := range w.clauses {
		bad, err := g.Inconsistent()
		if err != nil {
			return err
		}
		if bad {
			break
		}
		if err = g.Assert(clause); err != nil {
			return err
		}
	}
	fresh, err := w.pseudo.FreshConstants()
	if err != nil || len(fresh) == 0 {
		return err
	}
	w.filter, err = tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = w.filter.IncRef(); err != nil {
		return err
	}
	w.filterOwned = true
	for _, d := range fresh {
		if err = w.filter.Insert(d); err != nil {
			return err
		}
	}
	return nil
}
func (w *card2BVApplication) close() error {
	for len(w.failures) > 0 {
		i := len(w.failures) - 1
		if err := w.failures[i].Cleanup(); err != nil {
			return err
		}
		w.failures = w.failures[:i]
	}
	if w.filter != nil {
		if w.filterCleanup || !w.filterOwned {
			if err := w.filter.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := w.filter.DecRefConsumed()
			if consumed {
				w.filterOwned = false
				w.filterCleanup = err != nil
			}
			if err != nil {
				return err
			}
		}
		w.filter = nil
	}
	m := w.owner.manager
	for len(w.clauses) > 0 {
		i := len(w.clauses) - 1
		consumed, err := m.DecRefConsumed(w.clauses[i])
		if consumed {
			w.clauses = w.clauses[:i]
		}
		if err != nil {
			return err
		}
	}
	for _, ref := range []*ast.ASTRef{&w.proof2, &w.proof1, &w.second, &w.first} {
		if *ref != (ast.ASTRef{}) {
			if err := m.DelRef(ref); err != nil {
				return err
			}
			*ref = ast.ASTRef{}
		}
	}
	if w.pseudo != nil {
		if err := w.pseudo.Close(); err != nil {
			return err
		}
		w.pseudo = nil
	}
	if w.theory != nil {
		if err := w.theory.Close(); err != nil {
			return err
		}
		w.theory = nil
	}
	return nil
}
