package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// NLA2BVTactic stores parameters; all algorithm state is per invocation.
// A nonnil owner returned with an error must be closed.
type NLA2BVTactic struct {
	self            *NLA2BVTactic
	manager         *ast.ASTManager
	context         *uparams.ParamsContext
	globals         *uparams.GlobalParamsService
	services        model.ModelEvaluatorConfig
	params          uparams.ParamsRef
	paramsReady     bool
	active          *nla2BVInvocation
	pending         []*nla2BVInvocation
	header          uint64
	refs            uint32
	closed, closing bool
}

var _ tactic.Tactic = (*NLA2BVTactic)(nil)

func NewNLA2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*NLA2BVTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(NLA2BVTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &NLA2BVTactic{manager: m, context: c, globals: g, services: services, header: n}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	return t, err
}
func (t *NLA2BVTactic) owner() error {
	if t == nil || t.self != t || t.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (t *NLA2BVTactic) open() error {
	if err := t.owner(); err != nil {
		return err
	}
	if !t.paramsReady || t.closing || t.active != nil || len(t.pending) != 0 {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *NLA2BVTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *NLA2BVTactic) DecRef() error {
	if err := t.owner(); err != nil {
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
func (t *NLA2BVTactic) Cleanup() error {
	if err := t.owner(); err != nil {
		return err
	}
	if t.active != nil {
		return ast.ErrASTPending
	}
	for len(t.pending) > 0 {
		if err := t.pending[0].close(); err != nil {
			return err
		}
		t.pending = t.pending[1:]
	}
	return nil
}
func (t *NLA2BVTactic) Reset() error { return t.Cleanup() }
func (t *NLA2BVTactic) Close() error {
	if t != nil && t.self == t && t.closed {
		return nil
	}
	if err := t.owner(); err != nil {
		return err
	}
	if t.active != nil {
		return ast.ErrASTPending
	}
	t.closing = true
	if err := t.Cleanup(); err != nil {
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
func (t *NLA2BVTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.context.Assign(&t.params, p)
}
func (t *NLA2BVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewNLA2BVTactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *NLA2BVTactic) CollectStatistics() map[string]float64  { return nil }
func (t *NLA2BVTactic) ResetStatistics() error                 { return t.open() }
func (t *NLA2BVTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *NLA2BVTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *NLA2BVTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, item := range []struct{ name, text string }{
		{"nla2bv_max_bv_size", "(default: inf) maximum bit-vector size used by nla2bv tactic"},
		{"nla2bv_bv_size", "(default: 4) default bit-vector size used by nla2bv tactic."},
		{"nla2bv_root", "(default: 2) nla2bv tactic encodes reals into bit-vectors using expressions of the form a+b*sqrt(c), this parameter sets the value of c used in the encoding."},
		{"nla2bv_divisor", "(default: 2) nla2bv tactic parameter."},
	} {
		if err := out.InsertName(item.name, uparams.ParamUInt, &item.text, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
func nla2BVAssert(line int32, text string, ok bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/tactic/arith/nla2bv_tactic.cpp", line, text)
}
func nla2BVWellSorted(g *tactic.Goal, line int32) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	ok, err := g.IsWellSorted()
	if err != nil {
		return err
	}
	return nla2BVAssert(line, "g->is_well_sorted()", ok)
}
func (t *NLA2BVTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = nla2BVWellSorted(g, 449); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("nla2bv", g); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("nla2bv", g); err != nil {
		return err
	}
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	c, err := t.newInvocation()
	if c == nil {
		return err
	}
	defer func() {
		t.active = nil
		cleanup := c.close()
		if cleanup != nil {
			t.pending = append(t.pending, c)
		}
		err = errors.Join(err, cleanup)
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	if err != nil {
		return err
	}
	t.active = c
	if err = c.run(g, out); err != nil {
		return err
	}
	if err = out.AddGoal(g); err != nil {
		return err
	}
	return nla2BVWellSorted(g, 459)
}

type nla2BVInvocation struct {
	t                           *NLA2BVTactic
	real                        *BV2RealUtil
	integer                     *BV2IntContext
	bounds                      *BoundManager
	sub                         ast.ASTExprSubstitution
	subReady                    bool
	vars, defs, trail           []ast.ASTHandle
	local                       *bv2RealCall
	filter                      *tactic.FilterModelConverter
	extension                   *tactic.ExtensionModelConverter
	filterOwner, extensionOwner tactic.TacticResult
	looseConcat                 tactic.ModelConverter
	replacer                    *rewriter.DefaultExprReplacer
	intRewrite                  *BV2IntRewriter
	realRewrite                 *BV2RealRewriter
	report                      *tactic.TacticReport
	numBits                     uint32
	satPreserving               bool
	header                      uint64
}

func (t *NLA2BVTactic) newInvocation() (*nla2BVInvocation, error) {
	n := uint64(reflect.TypeOf(nla2BVInvocation{}).Size())
	if err := t.manager.Budget().Reserve(n); err != nil {
		return nil, err
	}
	c := &nla2BVInvocation{t: t, header: n, satPreserving: true}
	rootKey := "nla2bv_root"
	root, err := t.context.GetUIntName(&t.params, &rootKey, 2)
	if err != nil {
		return c, err
	}
	divisorKey, maxKey := "nla2bv_divisor", "nla2bv_max_bv_size"
	divisor, err := t.context.GetUIntName(&t.params, &divisorKey, 2)
	if err != nil {
		return c, err
	}
	max, err := t.context.GetUIntName(&t.params, &maxKey, ^uint32(0))
	if err != nil {
		return c, err
	}
	c.real, err = NewBV2RealUtil(t.manager, root, divisor, max)
	if err != nil {
		return c, err
	}
	c.local = &bv2RealCall{u: c.real}
	c.integer, err = NewBV2IntContext(t.manager, t.context, &t.params)
	if err != nil {
		return c, err
	}
	c.bounds, err = NewBoundManager(t.manager, t.context, t.globals)
	if err != nil {
		return c, err
	}
	c.sub, err = t.manager.NewExprSubstitution()
	c.subReady = err == nil
	if err != nil {
		return c, err
	}
	key := "nla2bv_bv_size"
	c.numBits, err = t.context.GetUIntName(&t.params, &key, 4)
	return c, err
}
func (c *nla2BVInvocation) close() error {
	if c.report != nil {
		if err := c.report.Close(); err != nil {
			return err
		}
		c.report = nil
	}
	if c.realRewrite != nil {
		if err := c.realRewrite.Close(); err != nil {
			return err
		}
		c.realRewrite = nil
	}
	if c.intRewrite != nil {
		if err := c.intRewrite.Close(); err != nil {
			return err
		}
		c.intRewrite = nil
	}
	if c.replacer != nil {
		if err := c.replacer.Close(); err != nil {
			return err
		}
		c.replacer = nil
	}
	if c.local != nil {
		if err := c.local.close(); err != nil {
			return err
		}
		c.local = nil
	}
	if c.looseConcat != nil {
		if err := c.looseConcat.Close(); err != nil {
			return err
		}
		c.looseConcat = nil
	}
	if err := c.extensionOwner.Close(); err != nil {
		return err
	}
	if err := c.filterOwner.Close(); err != nil {
		return err
	}
	if c.real != nil {
		for _, slice := range []*[]ast.ASTHandle{&c.trail, &c.defs, &c.vars} {
			if err := c.real.release(slice); err != nil {
				return err
			}
		}
	}
	if c.subReady {
		if err := c.t.manager.DelExprSubstitution(&c.sub); err != nil {
			return err
		}
		c.subReady = false
	}
	if c.bounds != nil {
		if err := c.bounds.Close(); err != nil {
			return err
		}
		c.bounds = nil
	}
	if c.integer != nil {
		if err := c.integer.Close(); err != nil {
			return err
		}
		c.integer = nil
	}
	if c.real != nil {
		if err := c.real.Close(); err != nil {
			return err
		}
		c.real = nil
	}
	c.t.manager.Budget().Release(c.header)
	c.header = 0
	return nil
}
