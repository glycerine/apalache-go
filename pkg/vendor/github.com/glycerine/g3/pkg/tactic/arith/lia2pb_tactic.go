package arith

// Source: src/tactic/arith/lia2pb_tactic.cpp/.h.

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

type LIA2PBTactic struct {
	self     *LIA2PBTactic
	manager  *ast.ASTManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	services model.ModelEvaluatorConfig
	// Source updates both the outer parameter snapshot and live implementation.
	params                       uparams.ParamsRef
	bounds                       *BoundManager
	rewriter                     *rewriter.TheoryRewriter
	retired                      []*LIA2PBTactic
	trace                        *diagnostic.TraceTags
	ownerBytes                   uint64
	refs                         uint32
	partial, paramsReady, closed bool
	maxBits, totalBits           uint32
	newDeps                      []ast.ASTDependency
	releasedDeps                 int
}

var _ tactic.Tactic = (*LIA2PBTactic)(nil)

func NewLIA2PBTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*LIA2PBTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(LIA2PBTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &LIA2PBTactic{manager: m, context: c, globals: g, services: services, ownerBytes: bytes}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.bounds, err = NewBoundManager(m, c, g)
	}
	if err == nil {
		t.rewriter, err = rewriter.NewTheoryRewriter(m, c, g, &t.params)
	}
	if err == nil {
		err = t.updatePolicy(&t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeLIA2PBTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewLIA2PBTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *LIA2PBTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.bounds == nil || t.rewriter == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *LIA2PBTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *LIA2PBTactic) DecRef() error {
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
func (t *LIA2PBTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			return err
		}
		t.rewriter = nil
	}
	if err := t.resetNewDependencies(); err != nil {
		return err
	}
	if t.bounds != nil {
		if err := t.bounds.Close(); err != nil {
			return err
		}
		t.bounds = nil
	}
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
func (t *LIA2PBTactic) Cleanup() (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if err = t.drainRetired(); err != nil {
		return err
	}
	fresh, err := NewLIA2PBTactic(t.manager, t.context, t.globals, &t.params, t.services)
	if err != nil {
		return err
	}
	// Source publishes the fresh implementation before destroying the old.
	t.rewriter, fresh.rewriter = fresh.rewriter, t.rewriter
	t.bounds, fresh.bounds = fresh.bounds, t.bounds
	t.partial, fresh.partial = fresh.partial, t.partial
	t.maxBits, fresh.maxBits = fresh.maxBits, t.maxBits
	t.totalBits, fresh.totalBits = fresh.totalBits, t.totalBits
	t.newDeps, fresh.newDeps = fresh.newDeps, t.newDeps
	t.releasedDeps, fresh.releasedDeps = fresh.releasedDeps, t.releasedDeps
	t.bounds.trace = t.trace
	if err = fresh.Close(); err != nil {
		t.retired = append(t.retired, fresh)
	}
	return err
}
func (t *LIA2PBTactic) drainRetired() error {
	for len(t.retired) > 0 {
		if err := t.retired[0].Close(); err != nil {
			return err
		}
		t.retired = t.retired[1:]
	}
	return nil
}
func (t *LIA2PBTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	other, err := NewLIA2PBTactic(m, t.context, t.globals, &t.params, t.services)
	if err == nil {
		other.trace = t.trace
		other.bounds.trace = t.trace
	}
	return other, err
}
func (t *LIA2PBTactic) updatePolicy(p *uparams.ParamsRef) error {
	if err := t.rewriter.UpdateParams(p); err != nil {
		return err
	}
	key := "lia2pb_partial"
	var err error
	t.partial, err = t.context.GetBoolName(p, &key, false)
	if err == nil {
		key = "lia2pb_max_bits"
		t.maxBits, err = t.context.GetUIntName(p, &key, 32)
	}
	if err == nil {
		key = "lia2pb_total_bits"
		t.totalBits, err = t.context.GetUIntName(p, &key, 2048)
	}
	return err
}

func (t *LIA2PBTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.updatePolicy(p)
}
func (t *LIA2PBTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, p := range []struct {
		name        string
		kind        uparams.ParamKind
		description string
	}{
		{"lia2pb_partial", uparams.ParamBool, "(default: false) partial lia2pb conversion."},
		{"lia2pb_max_bits", uparams.ParamUInt, "(default: 32) maximum number of bits to be used (per variable) in lia2pb."},
		{"lia2pb_total_bits", uparams.ParamUInt, "(default: 2048) total number of bits to be used (per problem) in lia2pb."},
	} {
		if err := out.InsertName(p.name, p.kind, &p.description, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (t *LIA2PBTactic) CollectStatistics() map[string]float64  { return nil }
func (t *LIA2PBTactic) ResetStatistics() error                 { return t.open() }
func (t *LIA2PBTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *LIA2PBTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *LIA2PBTactic) Reset() error                           { return t.Cleanup() }

func (t *LIA2PBTactic) resetNewDependencies() error {
	for t.releasedDeps < len(t.newDeps) {
		consumed, err := t.manager.DependencyDecConsumed(t.newDeps[t.releasedDeps])
		if consumed {
			t.releasedDeps++
		}
		if err != nil {
			return err
		}
	}
	t.newDeps = nil
	t.releasedDeps = 0
	return nil
}
