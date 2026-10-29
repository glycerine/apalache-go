package qe

// Source: src/qe/qe_lite.cpp qe_lite_tactic and mk_qe_lite_tactic.

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

type LiteTactic struct {
	self           *LiteTactic
	manager        *ast.ASTManager
	config         Config
	params         uparams.ParamsRef
	paramsReady    bool
	implementation *Lite
	pending        []*liteTacticCall
	refs           uint32
	bytes          uint64
	closed         bool
}

var _ tactic.Tactic = (*LiteTactic)(nil)

type liteTacticCall struct {
	owner    *LiteTactic
	refs     []ast.ASTRef
	report   *tactic.TacticReport
	failures []memory.ConstructionFailure
}

func (c *liteTacticCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *liteTacticCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.refs) != 0 {
		i := len(c.refs) - 1
		if c.refs[i] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.refs[i]); err != nil {
				c.retainFailure(err)
				return err
			}
		}
		c.refs = c.refs[:i]
	}
	if c.report != nil {
		if err := c.report.Close(); err != nil {
			c.retainFailure(err)
			return err
		}
		c.report = nil
	}
	return nil
}
func (c *liteTacticCall) retainFailure(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}

type LiteTacticFailure struct {
	owner          *LiteTactic
	cause, cleanup error
	constructor    bool
}

var _ memory.ConstructionFailure = (*LiteTacticFailure)(nil)

func (e *LiteTacticFailure) Error() string        { return e.Unwrap().Error() }
func (e *LiteTacticFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *LiteTacticFailure) OriginalCause() error { return e.cause }
func (e *LiteTacticFailure) CleanupCause() error  { return e.cleanup }
func (e *LiteTacticFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *LiteTacticFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func NewLiteTactic(m *ast.ASTManager, p *uparams.ParamsRef, config Config) (result *LiteTactic, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(LiteTactic{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &LiteTactic{manager: m, config: config, bytes: bytes}
	t.self = t
	defer func() {
		if err != nil {
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				t.pending = append(t.pending, &liteTacticCall{owner: t, failures: []memory.ConstructionFailure{failure}})
			}
			if cleanup := t.Close(); cleanup != nil {
				result = t
				err = &LiteTacticFailure{owner: t, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	t.params, err = config.Context.NewValue()
	if err != nil {
		return nil, err
	}
	t.paramsReady = true
	if p != nil {
		if err = config.Context.Assign(&t.params, p); err != nil {
			return nil, err
		}
	}
	t.implementation, err = NewLite(m, &t.params, true, config)
	if err != nil {
		return nil, err
	}
	return t, nil
}
func (t *LiteTactic) ownerOpen() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || !t.paramsReady {
		return arena.ErrClosed
	}
	return t.config.Context.Check(&t.params)
}
func (t *LiteTactic) open() error {
	if err := t.ownerOpen(); err != nil {
		return err
	}
	if t.implementation == nil {
		return arena.ErrClosed
	}
	return nil
}
func (t *LiteTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *LiteTactic) DecRef() error {
	if err := t.ownerOpen(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		if err := t.Close(); err != nil {
			if len(t.pending) == 0 {
				c := &liteTacticCall{owner: t}
				c.retainFailure(err)
				t.pending = append(t.pending, c)
			}
			return &LiteTacticFailure{owner: t, cause: err, constructor: true}
		}
	}
	return nil
}
func (t *LiteTactic) drain() error {
	for len(t.pending) != 0 {
		i := len(t.pending) - 1
		if err := t.pending[i].close(); err != nil {
			return err
		}
		t.pending = t.pending[:i]
	}
	return nil
}
func (t *LiteTactic) finish(c *liteTacticCall, err *error) {
	var failure memory.ConstructionFailure
	if errors.As(*err, &failure) {
		c.failures = append(c.failures, failure)
	}
	if cleanup := t.drain(); cleanup != nil {
		*err = &LiteTacticFailure{owner: t, cause: *err, cleanup: cleanup}
		return
	}
	if !errors.As(*err, &failure) && rewriter.IsRewriterException(*err) {
		*err = tactic.NewTacticException((*err).Error())
	}
}
func (t *LiteTactic) checkpoint() error {
	canceled, err := t.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
	}
	return t.config.Cooperation.Cooperate("qe-lite")
}
func (t *LiteTactic) emit(line int, body func(io.Writer) error) error {
	tags := t.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe", "operator()", "src/qe/qe_lite.cpp", line, func() bool { return true }, func(out io.Writer) error { return body(traceBodyWriter{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *LiteTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.drain(); err != nil {
		return err
	}
	c := &liteTacticCall{owner: t}
	t.pending = append(t.pending, c)
	defer t.finish(c, &err)
	if good, err := g.IsWellSorted(); err != nil {
		return err
	} else if !good {
		return ast.ErrASTDomain
	}
	if err = result.BindManager(t.manager); err != nil {
		return err
	}
	if err = result.SetModelConverter(nil); err != nil {
		return err
	}
	if err = result.SetProofConverter(nil); err != nil {
		return err
	}
	if err = result.SetCore(ast.ASTDependency{}); err != nil {
		return err
	}
	c.report, err = tactic.NewTacticReport("qe-lite", g)
	if err != nil {
		return err
	}
	proof, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	formula, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	produceProofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	size, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		if err = t.checkpoint(); err != nil {
			return err
		}
		inconsistent, err := g.Inconsistent()
		if err != nil {
			return err
		}
		if inconsistent {
			break
		}
		original, err := g.Form(i)
		if err != nil {
			return err
		}
		quantified, err := t.manager.HasQuantifiers(original)
		if err != nil {
			return err
		}
		if !quantified {
			continue
		}
		if err = t.manager.SetRef(formula, original); err != nil {
			return err
		}
		if err = t.implementation.Rewrite(formula, proof); err != nil {
			return err
		}
		if produceProofs {
			pr, err := t.manager.RefGet(proof)
			if err != nil {
				return err
			}
			// Source get_fact requires a nonnull proof, but elim_star's
			// no-proof specialization leaves new_pr empty. Keep the native
			// AST guard rather than dereferencing the source null pointer.
			fact, err := t.manager.GetProofFact(pr)
			if err != nil {
				return err
			}
			left, err := t.manager.GetArg(fact, 0)
			if err != nil {
				return err
			}
			right, err := t.manager.GetArg(fact, 1)
			if err != nil {
				return err
			}
			old, err := g.Proof(i)
			if err != nil {
				return err
			}
			if left != right {
				pr, err = t.manager.MkProofModusPonens(old, pr)
				if err != nil {
					return err
				}
			} else {
				pr = old
			}
			if err = t.manager.SetRef(proof, pr); err != nil {
				return err
			}
		}
		updated, err := t.manager.RefGet(formula)
		if err != nil {
			return err
		}
		if updated != original {
			if err = t.emit(2589, func(out io.Writer) error {
				if err := writeExpression(t.manager, t.config, out, original); err != nil {
					return err
				}
				_, _ = io.WriteString(out, "\n")
				if err := writeExpression(t.manager, t.config, out, updated); err != nil {
					return err
				}
				_, _ = io.WriteString(out, "\n")
				return nil
			}); err != nil {
				return err
			}
			pr, err := t.manager.RefGet(proof)
			if err != nil {
				return err
			}
			dep, err := g.Dependency(i)
			if err != nil {
				return err
			}
			if err = g.Update(i, updated, pr, dep); err != nil {
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
	if err = t.emit(2595, func(out io.Writer) error { return g.Display(out) }); err != nil {
		return err
	}
	if good, err := g.IsWellSorted(); err != nil {
		return err
	} else if !good {
		return ast.ErrASTDomain
	}
	return nil
}

func (t *LiteTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.ownerOpen(); err != nil {
		return err
	}
	if p == nil {
		empty := t.config.Context.EmptyValue()
		p = &empty
	}
	return t.config.Context.Assign(&t.params, p)
}
func (t *LiteTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (*LiteTactic) CollectStatistics() map[string]float64           { return nil }
func (t *LiteTactic) ResetStatistics() error                        { return t.open() }
func (t *LiteTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *LiteTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *LiteTactic) Reset() error                                  { return t.Cleanup() }
func (t *LiteTactic) Cleanup() error {
	if err := t.ownerOpen(); err != nil {
		return err
	}
	if err := t.drain(); err != nil {
		return err
	}
	if t.implementation != nil {
		if err := t.implementation.Close(); err != nil {
			return err
		}
		t.implementation = nil
	}
	var err error
	t.implementation, err = NewLite(t.manager, &t.params, true, t.config)
	return err
}
func (t *LiteTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	copy, err := NewLiteTactic(m, &t.params, t.config)
	if copy == nil {
		return nil, err
	}
	return copy, err
}
func (t *LiteTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.drain(); err != nil {
		return err
	}
	if t.implementation != nil {
		if err := t.implementation.Close(); err != nil {
			return err
		}
		t.implementation = nil
	}
	if t.paramsReady {
		if err := t.config.Context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
