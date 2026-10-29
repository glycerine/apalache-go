package bv

// Source: src/tactic/bv/bv1_blaster_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type BV1BlasterTactic struct {
	self        *BV1BlasterTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	params      uparams.ParamsRef
	paramsReady bool
	cooperation *cooperate.Cooperation
	trace       *diagnostic.TraceTags
	current     *bv1BlasterImplementation
	retired     []*bv1BlasterImplementation
	refs        uint32
	header      uint64
	closed      bool
}

var _ tactic.Tactic = (*BV1BlasterTactic)(nil)

type BV1BlasterConstructionFailure struct {
	owner          *BV1BlasterTactic
	cause, cleanup error
}

func (e *BV1BlasterConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *BV1BlasterConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *BV1BlasterConstructionFailure) OriginalCause() error { return e.cause }
func (e *BV1BlasterConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *BV1BlasterConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *BV1BlasterConstructionFailure) Cleanup() error { return e.owner.Close() }
func (t *BV1BlasterTactic) constructionFailure(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &BV1BlasterConstructionFailure{owner: t, cause: cause, cleanup: cleanup}
	}
	return cause
}
func NewBV1BlasterTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (*BV1BlasterTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(BV1BlasterTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &BV1BlasterTactic{manager: m, context: c, globals: g, cooperation: cooperation, header: n}
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
func MakeBV1BlasterTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (tactic.Tactic, error) {
	t, err := NewBV1BlasterTactic(m, c, g, p, cooperation)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return w, nil
}
func (t *BV1BlasterTactic) policy(r *bv1BlasterImplementation) error {
	key := "max_memory"
	n, err := t.context.GetUIntName(&t.params, &key, ^uint32(0))
	if err != nil {
		return err
	}
	r.maxMemory = memory.MegabytesToBytes(n)
	key = "max_steps"
	r.maxSteps, err = t.context.GetUIntName(&t.params, &key, ^uint32(0))
	if err != nil {
		return err
	}
	key = "produce_models"
	r.produceModels, err = t.context.GetBoolName(&t.params, &key, false)
	return err
}
func (t *BV1BlasterTactic) build() (r *bv1BlasterImplementation, err error) {
	r = &bv1BlasterImplementation{owner: t, constants: make(map[ast.ASTHandle]ast.ASTHandle)}
	r.family, err = t.manager.BitVectorUtilityFamilyID()
	if err != nil {
		return r, err
	}
	r.one, err = t.manager.BitVectorNumeralUnsigned(1, 1)
	if err != nil {
		return r, err
	}
	if err = r.hold(r.one, false); err != nil {
		return r, err
	}
	r.zero, err = t.manager.BitVectorNumeralUnsigned(0, 1)
	if err != nil {
		return r, err
	}
	if err = r.hold(r.zero, false); err != nil {
		return r, err
	}
	if err = t.policy(r); err != nil {
		return r, err
	}
	proofs, err := t.manager.ProofsEnabled()
	if err != nil {
		return r, err
	}
	if proofs {
		r.traversal, err = rewriter.NewProofTraversal(t.manager, r)
	} else {
		r.traversal, err = rewriter.NewTraversal(t.manager, r)
	}
	if err != nil {
		return r, err
	}
	r.out, err = t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return r, err
	}
	r.proof, err = t.manager.NewRef(ast.ASTHandle{})
	return r, err
}
func (t *BV1BlasterTactic) closeImplementation(r *bv1BlasterImplementation) error {
	if r == nil {
		return nil
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	for _, ref := range []*ast.ASTRef{&r.proof, &r.out} {
		if *ref != (ast.ASTRef{}) {
			if err := t.manager.DelRef(ref); err != nil {
				return err
			}
			*ref = ast.ASTRef{}
		}
	}
	if err := r.releaseScratch(0); err != nil {
		return err
	}
	for len(r.held) > 0 {
		i := len(r.held) - 1
		if err := t.manager.DecRef(r.held[i]); err != nil {
			return err
		}
		r.held = r.held[:i]
	}
	r.bindings = nil
	r.constants = nil
	return nil
}
func (t *BV1BlasterTactic) drainRetired() error {
	for len(t.retired) > 0 {
		if err := t.closeImplementation(t.retired[0]); err != nil {
			return err
		}
		t.retired = t.retired[1:]
	}
	return nil
}
func (t *BV1BlasterTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || !t.paramsReady || t.current == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *BV1BlasterTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *BV1BlasterTactic) DecRef() error {
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
func (t *BV1BlasterTactic) Close() error {
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
	if err := t.drainRetired(); err != nil {
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
func (t *BV1BlasterTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.drainRetired(); err != nil {
		return err
	}
	fresh, err := t.build()
	if err != nil {
		t.retired = append(t.retired, fresh)
		return errors.Join(err, t.drainRetired())
	}
	t.retired = append(t.retired, t.current)
	t.current = fresh
	return t.drainRetired()
}
func (t *BV1BlasterTactic) UpdateParams(p *uparams.ParamsRef) error {
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
	return t.policy(t.current)
}
func (t *BV1BlasterTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	r, err := NewBV1BlasterTactic(m, t.context, t.globals, &t.params, t.cooperation)
	if err == nil {
		r.trace = t.trace
	}
	return r, err
}
func (t *BV1BlasterTactic) IsTarget(g *tactic.Goal) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if g == nil || g.Manager() != t.manager {
		return false, ast.ErrASTHandle
	}
	return bv1Target(g, t.current.family)
}

func bv1Target(g *tactic.Goal, family ast.FamilyID) (bool, error) {
	m := g.Manager()
	size, err := g.Size()
	if err != nil {
		return false, err
	}
	seen := make(map[ast.ASTHandle]bool)
	for i := uint32(0); i < size; i++ {
		root, err := g.Form(i)
		if err != nil {
			return false, err
		}
		stack := []ast.ASTHandle{root}
		for len(stack) > 0 {
			h := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[h] {
				continue
			}
			seen[h] = true
			kind, err := m.GetKind(h)
			if err != nil {
				return false, err
			}
			if kind == ast.ASTVar || kind == ast.ASTQuantifier {
				return false, nil
			}
			if kind != ast.ASTApp {
				return false, ast.ErrASTDomain
			}
			f, err := m.GetFamilyID(h)
			if err != nil {
				return false, err
			}
			if f == family {
				op, err := m.GetDeclKind(h)
				if err != nil {
					return false, err
				}
				if op != 0 && op != 37 && op != 40 {
					return false, nil
				}
			}
			n, err := m.GetNumArgs(h)
			if err != nil {
				return false, err
			}
			for j := uint32(0); j < n; j++ {
				arg, err := m.GetArg(h, j)
				if err != nil {
					return false, err
				}
				stack = append(stack, arg)
			}
		}
	}
	return true, nil
}
func (t *BV1BlasterTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	yes, err := t.IsTarget(g)
	if err != nil {
		return err
	}
	if !yes {
		return tactic.NewTacticException("bv1 blaster cannot be applied to goal")
	}
	report, err := tactic.NewTacticReport("bv1-blaster", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	r := t.current
	r.steps = 0
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	generate, err := t.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	size, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		bad, e := g.Inconsistent()
		if e != nil {
			return e
		}
		if bad {
			break
		}
		curr, e := g.Form(i)
		if e != nil {
			return e
		}
		if generate {
			e = r.traversal.RewriteWithProof(curr, r.out, r.proof)
		} else {
			e = t.manager.ResetRef(r.proof)
			if e == nil {
				e = r.traversal.Rewrite(curr, r.out)
			}
		}
		if e != nil {
			return e
		}
		n, e := r.traversal.NumSteps()
		if e != nil {
			return e
		}
		r.steps += n
		if proofs {
			pr, e := g.Proof(i)
			if e != nil {
				return e
			}
			rewrite, e := t.manager.RefGet(r.proof)
			if e != nil {
				return e
			}
			joined, e := t.manager.MkProofModusPonens(pr, rewrite)
			if e != nil {
				return e
			}
			if e = t.manager.SetRef(r.proof, joined); e != nil {
				return e
			}
		}
		value, e := t.manager.RefGet(r.out)
		if e != nil {
			return e
		}
		pr, e := t.manager.RefGet(r.proof)
		if e != nil {
			return e
		}
		dep, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, value, pr, dep); e != nil {
			return e
		}
	}
	models, err := g.ModelsEnabled()
	if err != nil {
		return err
	}
	if models {
		mc, e := NewBV1BlasterModelConverter(t.manager, t.context, t.globals, r.bindings)
		if e != nil {
			return e
		}
		mc.trace = t.trace
		if e = result.SetModelConverter(mc); e != nil {
			return errors.Join(e, mc.Close())
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(g); err != nil {
		return err
	}
	// Source calls cfg.cleanup(), whose default implementation is a no-op.
	// The raw tactic retains its mapping/cache; the clean factory rebuilds it.
	r.DefaultConfiguration.Cleanup()
	return nil
}
func (t *BV1BlasterTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	return uparams.InsertMaxSteps(out)
}
func (t *BV1BlasterTactic) NumSteps() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.current.steps, nil
}
func (t *BV1BlasterTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *BV1BlasterTactic) CollectStatistics() map[string]float64  { return nil }
func (t *BV1BlasterTactic) ResetStatistics() error                 { return t.open() }
func (t *BV1BlasterTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *BV1BlasterTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *BV1BlasterTactic) Reset() error                           { return t.Cleanup() }
