package core

// Source: src/tactic/core/ctx_simplify_tactic.cpp/.h.

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

// ContextSimplifier is the source's typed virtual simplifier contract. It owns
// its assertion state and borrows expressions passed to its methods. Translate
// creates fresh state; the returned simplifier is owned by the tactic.
type ContextSimplifier interface {
	AssertExpression(ast.ASTHandle, bool) (bool, error)
	Simplify(ast.ASTHandle, ast.ASTRef) (bool, error)
	MaySimplify(ast.ASTHandle) (bool, error)
	Pop(uint32) error
	ScopeLevel() uint32
	Translate(*ast.ASTManager) (ContextSimplifier, error)
	UpdateParams(*uparams.ParamsRef) error
	SetOccurrences(*tactic.GoalNumOccurs)
	Close() error
}

// ContextSimplifierShared preserves both source sharing gates. The occurrence
// counter alone is insufficient after goal rewrites change reference counts.
func ContextSimplifierShared(m *ast.ASTManager, occs *tactic.GoalNumOccurs, h ast.ASTHandle) (bool, error) {
	if occs == nil {
		return false, ast.ErrASTDomain
	}
	n, err := m.GetRefCount(h)
	return n > 1 && occs.GetNumOccurrences(h) > 1, err
}

type contextAssertion struct {
	key ast.ASTHandle
	ref ast.ASTRef
}
type contextAssertions struct {
	m          *ast.ASTManager
	occs       *tactic.GoalNumOccurs
	assertions map[ast.ASTHandle]ast.ASTHandle
	trail      []contextAssertion
	scopes     []int
}

func newContextAssertions(m *ast.ASTManager) *contextAssertions {
	return &contextAssertions{m: m, assertions: make(map[ast.ASTHandle]ast.ASTHandle)}
}
func (s *contextAssertions) SetOccurrences(n *tactic.GoalNumOccurs)  { s.occs = n }
func (s *contextAssertions) ScopeLevel() uint32                      { return uint32(len(s.scopes)) }
func (s *contextAssertions) MaySimplify(ast.ASTHandle) (bool, error) { return true, nil }
func (s *contextAssertions) UpdateParams(*uparams.ParamsRef) error   { return nil }
func (s *contextAssertions) Translate(m *ast.ASTManager) (ContextSimplifier, error) {
	return newContextAssertions(m), nil
}
func (s *contextAssertions) push() { s.scopes = append(s.scopes, len(s.trail)) }
func (s *contextAssertions) insert(key, value ast.ASTHandle) error {
	if _, ok := s.assertions[key]; ok {
		return nil
	} // max_depth may leave a repeated equation unsimplified.
	r, err := s.m.NewRef(key)
	if err != nil {
		return err
	}
	s.assertions[key] = value
	s.trail = append(s.trail, contextAssertion{key, r})
	return nil
}
func (s *contextAssertions) AssertExpression(h ast.ASTHandle, sign bool) (bool, error) {
	original := h
	for {
		neg, arg, err := s.m.IsNot(h)
		if err != nil {
			return false, err
		}
		if !neg {
			break
		}
		h = arg
		sign = !sign
	}
	mkScope := true
	a, err := ContextSimplifierShared(s.m, s.occs, h)
	if err != nil {
		return false, err
	}
	b := false
	if !a {
		b, err = ContextSimplifierShared(s.m, s.occs, original)
		if err != nil {
			return false, err
		}
	}
	if a || b {
		s.push()
		mkScope = false
		var value ast.ASTHandle
		if sign {
			value, err = s.m.MkFalse()
		} else {
			value, err = s.m.MkTrue()
		}
		if err != nil {
			return false, err
		}
		if err = s.insert(h, value); err != nil {
			return false, err
		}
	}
	if !sign {
		kind, err := s.m.GetKind(h)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return true, nil
		}
		family, err := s.m.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		op, err := s.m.GetDeclKind(h)
		if err != nil {
			return false, err
		}
		if family == s.m.BasicFamilyID() && op == ast.ASTOpEq {
			lhs, err := s.m.GetArg(h, 0)
			if err != nil {
				return false, err
			}
			rhs, err := s.m.GetArg(h, 1)
			if err != nil {
				return false, err
			}
			yes, err := s.m.IsValue(rhs)
			if err != nil {
				return false, err
			}
			if !yes {
				yes, err = s.m.IsValue(lhs)
				if err != nil {
					return false, err
				}
				lhs, rhs = rhs, lhs
			}
			if yes {
				shared, err := ContextSimplifierShared(s.m, s.occs, lhs)
				if err != nil {
					return false, err
				}
				if shared {
					if mkScope {
						s.push()
					}
					if err = s.insert(lhs, rhs); err != nil {
						return false, err
					}
				}
			}
		}
	}
	return true, nil
}
func (s *contextAssertions) Simplify(h ast.ASTHandle, out ast.ASTRef) (bool, error) {
	x, ok := s.assertions[h]
	if !ok {
		return false, nil
	}
	return true, s.m.SetRef(out, x)
}
func (s *contextAssertions) Pop(n uint32) error {
	if n == 0 {
		return nil
	}
	if n > uint32(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(s.scopes) - int(n)
	size := s.scopes[level]
	for len(s.trail) > size {
		i := len(s.trail) - 1
		if err := s.m.DelRef(&s.trail[i].ref); err != nil {
			return err
		}
		delete(s.assertions, s.trail[i].key)
		s.trail = s.trail[:i]
	}
	s.scopes = s.scopes[:level]
	return nil
}
func (s *contextAssertions) Close() error { return s.Pop(s.ScopeLevel()) }

type contextCacheResult struct {
	value    ast.ASTHandle
	ref      ast.ASTRef
	traceRef ast.ASTRef
	level    uint32
	bytes    uint64
}
type contextCacheCell struct {
	key     ast.ASTRef
	results []contextCacheResult
}
type contextSimplifyImplementation struct {
	m                                *ast.ASTManager
	simp                             ContextSimplifier
	occs                             *tactic.GoalNumOccurs
	mkApp                            *rewriter.SimplifiedApp
	cache                            map[ast.ASTHandle]*contextCacheCell
	undo                             [][]ast.ASTHandle
	depth, steps, maxDepth, maxSteps uint32
	maxMemory                        uint64
	bailOnBlowup                     bool
	header                           uint64
}
type contextSimplifyRefs struct {
	m    *ast.ASTManager
	refs []ast.ASTRef
}

func (f *contextSimplifyRefs) newRef(h ast.ASTHandle) (ast.ASTRef, error) {
	r, e := f.m.NewRef(h)
	if e == nil {
		f.refs = append(f.refs, r)
	}
	return r, e
}
func (f *contextSimplifyRefs) Close() error {
	for len(f.refs) > 0 {
		i := len(f.refs) - 1
		if err := f.m.DelRef(&f.refs[i]); err != nil {
			return err
		}
		f.refs = f.refs[:i]
	}
	return nil
}
func (f *contextSimplifyRefs) dropLast() error {
	i := len(f.refs) - 1
	if i < 0 {
		return ast.ErrASTDomain
	}
	if err := f.m.DelRef(&f.refs[i]); err != nil {
		return err
	}
	f.refs = f.refs[:i]
	return nil
}

type ContextSimplifyTactic struct {
	self        *ContextSimplifyTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	services    model.ModelEvaluatorConfig
	trace       *diagnostic.TraceTags
	params      uparams.ParamsRef
	paramsReady bool
	current     *contextSimplifyImplementation
	retired     []*contextSimplifyImplementation
	retained    []*contextSimplifyRefs
	unbuilt     []ContextSimplifier
	refs        uint32
	header      uint64
	closed      bool
}

// ContextSimplifyConstructionFailure retains the already charged tactic when
// a constructor's real unwind refuses. Cleanup retries that same owner.
type ContextSimplifyConstructionFailure struct {
	owner          *ContextSimplifyTactic
	cause, cleanup error
}

func (e *ContextSimplifyConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *ContextSimplifyConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *ContextSimplifyConstructionFailure) OriginalCause() error { return e.cause }
func (e *ContextSimplifyConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *ContextSimplifyConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.services.Memory
}
func (e *ContextSimplifyConstructionFailure) Cleanup() error { return e.owner.Close() }
func (t *ContextSimplifyTactic) constructionFailure(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &ContextSimplifyConstructionFailure{owner: t, cause: cause, cleanup: cleanup}
	}
	return cause
}

var _ tactic.Tactic = (*ContextSimplifyTactic)(nil)

func NewContextSimplifyTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*ContextSimplifyTactic, error) {
	return NewContextSimplifyTacticWithSimplifier(m, newContextAssertions(m), c, g, p, services)
}
func NewContextSimplifyTacticWithSimplifier(m *ast.ASTManager, simp ContextSimplifier, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*ContextSimplifyTactic, error) {
	if m == nil || simp == nil || c == nil || g == nil || g.Context != c || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ContextSimplifyTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &ContextSimplifyTactic{manager: m, context: c, globals: g, services: services, trace: services.Trace, header: n}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.current, err = t.build(simp)
	} else {
		cleanup := simp.Close()
		if cleanup != nil {
			t.unbuilt = append(t.unbuilt, simp)
		}
		err = errors.Join(err, cleanup)
	}
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return t, nil
}
func MakeContextSimplifyTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	t, err := NewContextSimplifyTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return w, nil
}
func (t *ContextSimplifyTactic) build(simp ContextSimplifier) (*contextSimplifyImplementation, error) {
	n := uint64(reflect.TypeOf(contextSimplifyImplementation{}).Size())
	if err := t.manager.Budget().Reserve(n); err != nil {
		cleanup := simp.Close()
		if cleanup != nil {
			t.unbuilt = append(t.unbuilt, simp)
		}
		return nil, errors.Join(err, cleanup)
	}
	r := &contextSimplifyImplementation{m: t.manager, simp: simp, cache: make(map[ast.ASTHandle]*contextCacheCell), header: n}
	var err error
	if r.occs, err = tactic.NewGoalNumOccurs(t.manager, true, true); err != nil {
		return r, err
	}
	if r.mkApp, err = rewriter.NewSimplifiedApp(t.manager, t.context, t.globals, &t.params); err != nil {
		return r, err
	}
	if err = t.readPolicy(r); err != nil {
		return r, err
	}
	simp.SetOccurrences(r.occs)
	return r, nil
}
func (t *ContextSimplifyTactic) readPolicy(r *contextSimplifyImplementation) (err error) {
	for _, field := range []struct {
		name     string
		target   *uint32
		fallback uint32
	}{{"max_depth", &r.maxDepth, 1024}, {"max_steps", &r.maxSteps, ^uint32(0)}} {
		if *field.target, err = t.context.GetUIntName(&t.params, &field.name, field.fallback); err != nil {
			return err
		}
	}
	key := "max_memory"
	mb, err := t.context.GetUIntName(&t.params, &key, ^uint32(0))
	if err != nil {
		return err
	}
	r.maxMemory = memory.MegabytesToBytes(mb)
	key = "bail_on_blowup"
	if r.bailOnBlowup, err = t.context.GetBoolName(&t.params, &key, false); err != nil {
		return err
	}
	return r.simp.UpdateParams(&t.params)
}
func (t *ContextSimplifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.current == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *ContextSimplifyTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *ContextSimplifyTactic) DecRef() error {
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
func (t *ContextSimplifyTactic) release(f *contextSimplifyRefs, result *error) {
	if e := f.Close(); e != nil {
		t.retained = append(t.retained, f)
		*result = errors.Join(*result, e)
	}
}
func (t *ContextSimplifyTactic) checkpoint() error {
	if err := t.services.Cooperation.Cooperate("ctx_simplify_tactic"); err != nil {
		return err
	}
	n, err := t.services.Memory.AllocationSize()
	if err != nil {
		return err
	}
	if n > t.current.maxMemory {
		return tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	canceled, err := t.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
	}
	return nil
}
func (t *ContextSimplifyTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.readPolicy(t.current)
}
func (t *ContextSimplifyTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return ContextSimplifyParameterDescriptions(d)
}

func ContextSimplifyParameterDescriptions(d *uparams.ParamDescrs) error {
	if err := uparams.InsertMaxMemory(d); err != nil {
		return err
	}
	if err := uparams.InsertMaxSteps(d); err != nil {
		return err
	}
	text := "(default: 1024) maximum term depth."
	if err := d.InsertName("max_depth", uparams.ParamUInt, &text, nil, nil); err != nil {
		return err
	}
	text = "(default: false) enable equality propagation from bounds."
	return d.InsertName("propagate_eq", uparams.ParamBool, &text, nil, nil)
}
func (*ContextSimplifyTactic) CollectStatistics() map[string]float64    { return nil }
func (t *ContextSimplifyTactic) ResetStatistics() error                 { return t.open() }
func (t *ContextSimplifyTactic) Reset() error                           { return t.Cleanup() }
func (t *ContextSimplifyTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *ContextSimplifyTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *ContextSimplifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	s, err := t.current.simp.Translate(m)
	if err != nil {
		return nil, err
	}
	copy, err := NewContextSimplifyTacticWithSimplifier(m, s, t.context, t.globals, &t.params, t.services)
	if copy != nil {
		copy.trace = t.trace
	}
	return copy, err
}
func (t *ContextSimplifyTactic) closeImplementation(r *contextSimplifyImplementation) error {
	if r == nil {
		return nil
	}
	if err := t.pop(r, r.simp.ScopeLevel()); err != nil {
		return err
	}
	// A trace operand refusal can interrupt restoration after the simplifier
	// itself has already popped. Keep and finish every remaining undo level.
	for level := len(r.undo) - 1; level >= 0; level-- {
		if err := t.restoreCache(r, uint32(level)); err != nil {
			return err
		}
	}
	if err := t.assertContext(r.simp.ScopeLevel() == 0); err != nil {
		return err
	}
	if err := t.checkDestroyedContextCache(r); err != nil {
		return err
	}
	if err := r.simp.Close(); err != nil {
		return err
	}
	if r.mkApp != nil {
		if err := r.mkApp.Close(); err != nil {
			return err
		}
		r.mkApp = nil
	}
	t.manager.Budget().Release(r.header)
	r.header = 0
	return nil
}
func (t *ContextSimplifyTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	s, err := t.current.simp.Translate(t.manager)
	if err != nil {
		return err
	}
	r, err := t.build(s)
	if err != nil {
		if r != nil {
			t.retired = append(t.retired, r)
		}
		return err
	}
	old := t.current
	t.current = r
	t.retired = append(t.retired, old)
	return t.closeRetired()
}
func (t *ContextSimplifyTactic) closeRetired() error {
	for len(t.retired) > 0 {
		i := len(t.retired) - 1
		if err := t.closeImplementation(t.retired[i]); err != nil {
			return err
		}
		t.retired = t.retired[:i]
	}
	return nil
}
func (t *ContextSimplifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	for len(t.unbuilt) > 0 {
		i := len(t.unbuilt) - 1
		if err := t.unbuilt[i].Close(); err != nil {
			return err
		}
		t.unbuilt = t.unbuilt[:i]
	}
	for len(t.retained) > 0 {
		i := len(t.retained) - 1
		if err := t.retained[i].Close(); err != nil {
			return err
		}
		t.retained = t.retained[:i]
	}
	if err := t.closeRetired(); err != nil {
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

func (t *ContextSimplifyTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		if ok, e := g.IsWellSorted(); e != nil {
			return e
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	r := t.current
	r.occs.Reset()
	if err = r.occs.Count(g); err != nil {
		return err
	}
	r.steps = 0
	report, err := tactic.NewTacticReport("ctx-simplify", g)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		err = t.processProofGoal(g)
	} else {
		err = t.processGoal(g)
	}
	if err != nil {
		return err
	}
	if enabled, e := diagnostic.ProcessVerboseEnabled(10); e != nil {
		return e
	} else if enabled {
		stream, e := diagnostic.BorrowProcessVerboseStream()
		if e != nil {
			return e
		}
		if e = stream.AppendCString("(ctx-simplify :num-steps "); e != nil {
			return e
		}
		if e = stream.AppendUint32(r.steps); e != nil {
			return e
		}
		if e = stream.AppendCString(")\n"); e != nil {
			return e
		}
		if e = stream.Flush(); e != nil {
			return e
		}
	}
	if ast.ASTSourceDebug {
		if ok, e := g.IsWellSorted(); e != nil {
			return e
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}
