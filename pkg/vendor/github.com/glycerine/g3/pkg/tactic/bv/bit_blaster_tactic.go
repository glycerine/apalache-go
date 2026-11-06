package bv

// Source: src/tactic/bv/bit_blaster_tactic.cpp/.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	bb "github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type bitBlasterImplementation struct {
	base, active *bb.BitBlasterRewriter
	out, proof   ast.ASTRef
	blastQuant   bool
	steps        uint32
}

// BitBlasterTactic owns the default rewriter even when its active rewriter is
// borrowed from an incremental SAT solver, as in the source overload.
type BitBlasterTactic struct {
	self        *BitBlasterTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	params      uparams.ParamsRef
	paramsReady bool
	external    *bb.BitBlasterRewriter
	cooperation *cooperate.Cooperation
	current     *bitBlasterImplementation
	retired     []*bitBlasterImplementation
	trace       *diagnostic.TraceTags
	ownerBytes  uint64
	refs        uint32
	closed      bool
}

var _ tactic.Tactic = (*BitBlasterTactic)(nil)

func NewBitBlasterTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (*BitBlasterTactic, error) {
	return NewBitBlasterTacticWithRewriter(m, nil, c, g, p, cooperation)
}
func NewBitBlasterTacticWithRewriter(m *ast.ASTManager, external *bb.BitBlasterRewriter, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (*BitBlasterTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if external != nil {
		owner, err := external.Manager()
		if err != nil {
			return nil, err
		}
		if owner != m {
			return nil, ast.ErrASTHandle
		}
	}
	bytes := uint64(reflect.TypeOf(BitBlasterTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &BitBlasterTactic{manager: m, context: c, globals: g, external: external, cooperation: cooperation, ownerBytes: bytes}
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
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeBitBlasterTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (tactic.Tactic, error) {
	return MakeBitBlasterTacticWithRewriter(m, nil, c, g, p, cooperation)
}
func MakeBitBlasterTacticWithRewriter(m *ast.ASTManager, rw *bb.BitBlasterRewriter, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (tactic.Tactic, error) {
	child, err := NewBitBlasterTacticWithRewriter(m, rw, c, g, p, cooperation)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *BitBlasterTactic) policy() (p bb.RewriterParams, err error) {
	p = bb.DefaultRewriterParams()
	for _, field := range []struct {
		name   string
		target *uint32
	}{{"max_memory", &p.MaxMemoryMB}, {"max_steps", &p.MaxSteps}} {
		*field.target, err = t.context.GetUIntName(&t.params, &field.name, ^uint32(0))
		if err != nil {
			return p, err
		}
	}
	for _, field := range []struct {
		name     string
		target   *bool
		fallback bool
	}{{"blast_mul", &p.BlastMul, true}, {"blast_add", &p.BlastAdd, true}, {"blast_quant", &p.BlastQuant, false}, {"blast_full", &p.BlastFull, false}} {
		*field.target, err = t.context.GetBoolName(&t.params, &field.name, field.fallback)
		if err != nil {
			return p, err
		}
	}
	return p, nil
}
func (t *BitBlasterTactic) build() (*bitBlasterImplementation, error) {
	p, err := t.policy()
	if err != nil {
		return nil, err
	}
	r := &bitBlasterImplementation{blastQuant: p.BlastQuant}
	r.base, err = bb.NewBitBlasterRewriter(t.manager, &p)
	if err == nil {
		r.active = r.base
		if t.external != nil {
			r.active = t.external
			err = r.active.UpdateParams(p)
		}
	}
	if err == nil && t.cooperation != nil {
		err = r.active.BindCooperation(t.cooperation)
	}
	if err == nil && t.trace != nil {
		err = r.active.BindTrace(t.trace)
	}
	if err == nil {
		r.out, err = t.manager.NewRef(ast.ASTHandle{})
	}
	if err == nil {
		r.proof, err = t.manager.NewRef(ast.ASTHandle{})
	}
	if err != nil {
		return nil, errors.Join(err, t.closeImplementation(r))
	}
	return r, nil
}
func (t *BitBlasterTactic) closeImplementation(r *bitBlasterImplementation) error {
	if r == nil {
		return nil
	}
	for _, ref := range []*ast.ASTRef{&r.proof, &r.out} {
		if *ref != (ast.ASTRef{}) {
			if err := t.manager.DelRef(ref); err != nil {
				return err
			}
		}
	}
	if r.base != nil {
		if err := r.base.Close(); err != nil {
			return err
		}
		r.base = nil
	}
	return nil
}
func (t *BitBlasterTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *BitBlasterTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *BitBlasterTactic) DecRef() error {
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
func (t *BitBlasterTactic) drainRetired() error {
	for len(t.retired) > 0 {
		if err := t.closeImplementation(t.retired[0]); err != nil {
			return err
		}
		t.retired = t.retired[1:]
	}
	return nil
}
func (t *BitBlasterTactic) Close() error {
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
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *BitBlasterTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.drainRetired(); err != nil {
		return err
	}
	fresh, err := t.build()
	if err != nil {
		return err
	}
	old := t.current
	t.current = fresh
	t.retired = append(t.retired, old)
	return t.drainRetired()
}
func (t *BitBlasterTactic) UpdateParams(p *uparams.ParamsRef) error {
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
	actual, err := t.policy()
	if err != nil {
		return err
	}
	if err = t.current.active.UpdateParams(actual); err != nil {
		return err
	}
	t.current.blastQuant = actual.BlastQuant
	return nil
}
func (t *BitBlasterTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if ast.ASTSourceDebug && t.external != nil {
		return nil, ast.ErrASTDomain
	}
	other, err := NewBitBlasterTactic(m, t.context, t.globals, &t.params, t.cooperation)
	if err == nil {
		other.trace = t.trace
		err = other.current.active.BindTrace(t.trace)
	}
	return other, err
}
func (t *BitBlasterTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	defer func() {
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	r := t.current
	if proofs && r.blastQuant {
		return tactic.NewTacticException("quantified variable blasting does not support proof generation")
	}
	report, err := tactic.NewTacticReport("bit-blaster", goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	if err = t.emit("before_bit_blaster", 66, func(w io.Writer) error { return goal.Display(w) }); err != nil {
		return err
	}
	r.steps = 0
	size, err := goal.Size()
	if err != nil {
		return err
	}
	generate, err := t.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	changed := false
	for i := uint32(0); i < size; i++ {
		bad, e := goal.Inconsistent()
		if e != nil {
			return e
		}
		if bad {
			break
		}
		curr, e := goal.Form(i)
		if e != nil {
			return e
		}
		if generate {
			e = r.active.RewriteWithProof(curr, r.out, r.proof)
		} else {
			e = t.manager.ResetRef(r.proof)
			if e == nil {
				e = r.active.Rewrite(curr, r.out)
			}
		}
		if e != nil {
			return e
		}
		steps, e := r.active.NumSteps()
		if e != nil {
			return e
		}
		r.steps += steps
		if proofs {
			original, e := goal.Proof(i)
			if e != nil {
				return e
			}
			rewrite, e := t.manager.RefGet(r.proof)
			if e != nil {
				return e
			}
			pr, e := t.manager.MkProofModusPonens(original, rewrite)
			if e != nil {
				return e
			}
			if e = t.manager.SetRef(r.proof, pr); e != nil {
				return e
			}
		}
		value, e := t.manager.RefGet(r.out)
		if e != nil {
			return e
		}
		if value == curr {
			continue
		}
		changed = true
		if e = t.emit("bit_blaster", 85, func(w io.Writer) error {
			if e := t.writeExpr(w, curr); e != nil {
				return e
			}
			if _, e := io.WriteString(w, " -> "); e != nil {
				return e
			}
			if e := t.writeExpr(w, value); e != nil {
				return e
			}
			_, e := io.WriteString(w, "\n")
			return e
		}); e != nil {
			return e
		}
		pr, e := t.manager.RefGet(r.proof)
		if e != nil {
			return e
		}
		dep, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if e = goal.Update(i, value, pr, dep); e != nil {
			return e
		}
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	if changed && models {
		cache, e := r.active.Const2Bits()
		if e != nil {
			return e
		}
		bindings := make([]BitBlasterBinding, 0, len(cache))
		for d, bits := range cache {
			bindings = append(bindings, BitBlasterBinding{d, bits})
		}
		mc, e := NewBitBlasterModelConverter(t.manager, t.context, t.globals, bindings)
		if e != nil {
			return e
		}
		mc.trace = t.trace
		if e = result.SetModelConverter(mc); e != nil {
			return errors.Join(e, mc.Close())
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if err = t.emit("after_bit_blaster", 96, func(w io.Writer) error {
		if e := goal.Display(w); e != nil {
			return e
		}
		if result.ModelConverter != nil {
			if e := result.ModelConverter.Display(w); e != nil {
				return e
			}
		}
		_, e := io.WriteString(w, "\n")
		return e
	}); err != nil {
		return err
	}
	return r.active.Cleanup()
}
func (t *BitBlasterTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	if err := uparams.InsertMaxSteps(out); err != nil {
		return err
	}
	for _, entry := range []struct{ name, description string }{{"blast_mul", "(default: true) bit-blast multipliers (and dividers, remainders)."}, {"blast_add", "(default: true) bit-blast adders."}, {"blast_quant", "(default: false) bit-blast quantified variables."}, {"blast_full", "(default: false) bit-blast any term with bit-vector sort, this option will make E-matching ineffective in any pattern containing bit-vector terms."}} {
		if err := out.InsertName(entry.name, uparams.ParamBool, &entry.description, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
func (t *BitBlasterTactic) NumSteps() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.current.steps, nil
}
func (t *BitBlasterTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return t.current.active.BindTrace(tags)
}
func (t *BitBlasterTactic) CollectStatistics() map[string]float64  { return nil }
func (t *BitBlasterTactic) ResetStatistics() error                 { return t.open() }
func (t *BitBlasterTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *BitBlasterTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *BitBlasterTactic) Reset() error                           { return t.Cleanup() }
