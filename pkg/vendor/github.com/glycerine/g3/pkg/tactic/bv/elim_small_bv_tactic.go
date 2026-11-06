package bv

// Source: src/tactic/bv/elim_small_bv_tactic.cpp/.h.
// Christoph Winter, 2015-11-09.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ElimSmallBVTactic struct {
	self            *ElimSmallBVTactic
	manager         *ast.ASTManager
	context         *uparams.ParamsContext
	globals         *uparams.GlobalParamsService
	cooperation     *cooperate.Cooperation
	params          uparams.ParamsRef
	paramsReady     bool
	refs            uint32
	header          uint64
	closed, running bool
}

var _ tactic.Tactic = (*ElimSmallBVTactic)(nil)

func NewElimSmallBVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, coop *cooperate.Cooperation) (_ *ElimSmallBVTactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ElimSmallBVTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &ElimSmallBVTactic{manager: m, context: c, globals: g, cooperation: coop, header: n}
	t.self = t
	defer func() {
		if err != nil {
			err = errors.Join(err, t.Close())
		}
	}()
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	return t, err
}

func MakeElimSmallBVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, coop *cooperate.Cooperation) (tactic.Tactic, error) {
	t, err := NewElimSmallBVTactic(m, c, g, p, coop)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return w, nil
}
func (t *ElimSmallBVTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *ElimSmallBVTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *ElimSmallBVTactic) DecRef() error {
	if e := t.open(); e != nil {
		return e
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
func (t *ElimSmallBVTactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.paramsReady {
		err = t.context.Del(&t.params)
		if err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}
func (t *ElimSmallBVTactic) UpdateParams(p *uparams.ParamsRef) error {
	if e := t.open(); e != nil {
		return e
	}
	return t.context.Assign(&t.params, p)
}
func (t *ElimSmallBVTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if e := t.open(); e != nil {
		return e
	}
	a, b, c := "maximum amount of memory in megabytes", "maximum number of steps", "maximum bit-vector size of quantified bit-vectors to be eliminated"
	if e := d.InsertName("max_memory", uparams.ParamUInt, &a, nil, nil); e != nil {
		return e
	}
	if e := d.InsertName("max_steps", uparams.ParamUInt, &b, nil, nil); e != nil {
		return e
	}
	return d.InsertName("max_bits", uparams.ParamUInt, &c, nil, nil)
}
func (t *ElimSmallBVTactic) CollectStatistics() map[string]float64  { return nil }
func (t *ElimSmallBVTactic) ResetStatistics() error                 { return t.open() }
func (t *ElimSmallBVTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *ElimSmallBVTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *ElimSmallBVTactic) Cleanup() error                         { return t.open() }
func (t *ElimSmallBVTactic) Reset() error                           { return t.open() }
func (t *ElimSmallBVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewElimSmallBVTactic(m, t.context, t.globals, &t.params, t.cooperation)
}

type elimSmallBVConfig struct {
	rewriter.DefaultConfiguration
	owner             *ElimSmallBVTactic
	params            *uparams.ParamsRef
	simplify          *rewriter.TheoryRewriter
	substitute        *rewriter.VarSubstitution
	bindings          []ast.ASTHandle
	maxBits, maxSteps uint32
	maxMemory         uint64
	steps, eliminated uint32
}

func (c *elimSmallBVConfig) PreVisit(h ast.ASTHandle) (bool, error) {
	k, e := c.owner.manager.GetKind(h)
	if e != nil {
		return false, e
	}
	if k == ast.ASTQuantifier {
		n, e := c.owner.manager.GetNumDeclarations(h)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			s, e := c.owner.manager.GetDeclarationSort(h, i)
			if e != nil {
				return false, e
			}
			c.bindings = append(c.bindings, s)
		}
	}
	return true, nil
}
func (c *elimSmallBVConfig) checkpoint() (bool, error) {
	if c.owner.cooperation != nil {
		if e := c.owner.cooperation.Cooperate("elim-small-bv"); e != nil {
			return false, e
		}
	}
	if c.steps > c.maxSteps {
		return true, nil
	}
	if c.owner.manager.Budget().Used() > c.maxMemory {
		return false, tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	return false, nil
}
func (c *elimSmallBVConfig) ReduceQuantifier(q, body ast.ASTHandle, _, _ ast.AppArguments, out, _ ast.ASTRef) (changed bool, err error) {
	m := c.owner.manager
	n, err := m.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	old := len(c.bindings) - int(n)
	defer func() { c.bindings = c.bindings[:old] }()
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return false, err
	}
	if err = used.Collect(q); err != nil {
		return false, err
	}
	max := used.MaxFoundVarIndexPlusOne()
	current := body
	refs := []ast.ASTRef{}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
	}()
	hold := func(h ast.ASTHandle) error {
		r, e := m.NewRef(h)
		if e == nil {
			refs = append(refs, r)
		}
		return e
	}
	for i := int(n) - 1; i >= 0; i-- {
		sort, e := m.GetDeclarationSort(q, uint32(i))
		if e != nil {
			return false, e
		}
		is, e := m.IsBitVectorSort(sort)
		if e != nil {
			return false, e
		}
		if !is {
			continue
		}
		width, e := m.BitVectorSize(sort)
		if e != nil {
			return false, e
		}
		if width > c.maxBits {
			continue
		}
		bodies := []ast.ASTHandle{}
		for j := uint32(0); j < width; j++ {
			stop, e := c.checkpoint()
			if e != nil {
				return false, e
			}
			if stop {
				break
			}
			replacement, e := m.BitVectorNumeralUnsigned(j, width)
			if e != nil {
				return false, e
			}
			args := make([]ast.ASTHandle, n+max)
			args[n-uint32(i)-1] = replacement
			for l, r := 0, len(args)-1; l < r; l, r = l+1, r-1 {
				args[l], args[r] = args[r], args[l]
			}
			tmp, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return false, e
			}
			refs = append(refs, tmp)
			if e = c.substitute.Substitute(current, args, tmp); e != nil {
				return false, e
			}
			sub, e := m.RefGet(tmp)
			if e != nil {
				return false, e
			}
			simp, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return false, e
			}
			refs = append(refs, simp)
			if e = c.simplify.Rewrite(sub, simp); e != nil {
				return false, e
			}
			v, e := m.RefGet(simp)
			if e != nil {
				return false, e
			}
			bodies = append(bodies, v)
			c.steps++
		}
		forall, e := m.IsForall(q)
		if e != nil {
			return false, e
		}
		if forall {
			current, e = m.MkAndArray(bodies)
		} else {
			current, e = m.MkOrArray(bodies)
		}
		if e != nil {
			return false, e
		}
		simplified, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return false, e
		}
		refs = append(refs, simplified)
		if e = c.simplify.Rewrite(current, simplified); e != nil {
			return false, e
		}
		current, e = m.RefGet(simplified)
		if e != nil {
			return false, e
		}
		if e = hold(current); e != nil {
			return false, e
		}
		c.eliminated++
	}
	updated, e := m.UpdateQuantifierBody(q, current)
	if e != nil {
		return false, e
	}
	reduced, e := rewriter.ElimUnusedVarsWithParams(m, updated, c.owner.context, c.params)
	if e != nil {
		return false, e
	}
	refs = append(refs, reduced)
	v, e := m.RefGet(reduced)
	if e != nil {
		return false, e
	}
	return true, m.SetRef(out, v)
}

func (t *ElimSmallBVTactic) Apply(g *tactic.Goal, r *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || r == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	if err = r.Prepare(t.manager); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("elim-small-bv", g); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("elim-small-bv", g); err != nil {
		return err
	}
	key := "max_bits"
	bits, e := t.context.GetUIntName(&t.params, &key, 4)
	if e != nil {
		return e
	}
	key = "max_steps"
	steps, e := t.context.GetUIntName(&t.params, &key, math.MaxUint32)
	if e != nil {
		return e
	}
	key = "max_memory"
	mb, e := t.context.GetUIntName(&t.params, &key, math.MaxUint32)
	if e != nil {
		return e
	}
	cfg := &elimSmallBVConfig{owner: t, params: &t.params, maxBits: bits, maxSteps: steps, maxMemory: memory.MegabytesToBytes(mb)}
	cfg.substitute, e = rewriter.NewVarSubstitution(t.manager, true)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, cfg.substitute.Close()) }()
	cfg.simplify, e = rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, &t.params)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, cfg.simplify.Close()) }()
	trav, e := rewriter.NewTraversal(t.manager, cfg)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, trav.Close()) }()
	out, e := t.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	size, e := g.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		f, e := g.Form(i)
		if e != nil {
			return e
		}
		if e = trav.Rewrite(f, out); e != nil {
			return e
		}
		v, e := t.manager.RefGet(out)
		if e != nil {
			return e
		}
		d, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, v, ast.ASTHandle{}, d); e != nil {
			return e
		}
	}
	if e = g.IncDepth(); e != nil {
		return e
	}
	return r.AddGoal(g)
}
