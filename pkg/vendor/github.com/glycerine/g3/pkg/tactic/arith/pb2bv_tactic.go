package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type PB2BVTactic struct {
	self     *PB2BVTactic
	manager  *ast.ASTManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	services model.ModelEvaluatorConfig
	// Source updates both the outer parameter snapshot and live implementation.
	params                            uparams.ParamsRef
	bounds                            *BoundManager
	boolean                           *rewriter.BoolRewriter
	traversal                         *rewriter.Traversal
	configuration                     *pbTacticConfiguration
	saved                             ast.ASTRef
	retired                           []*PB2BVTactic
	trace                             *diagnostic.TraceTags
	ownerBytes                        uint64
	refs                              uint32
	paramsReady, closed               bool
	allClausesLimit, cardinalityLimit uint32
	maxMemory                         uint64
	bits                              map[ast.ASTHandle]int
	entries                           []pbBitBinding
	heldKeys, heldNots, temporaryInts []ast.ASTHandle
	used                              ast.ASTDependency
}

var _ tactic.Tactic = (*PB2BVTactic)(nil)

func NewPB2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*PB2BVTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(PB2BVTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &PB2BVTactic{manager: m, context: c, globals: g, services: services, ownerBytes: bytes}
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
		t.boolean, err = rewriter.NewBoolRewriter(m, c, g, &t.params)
	}
	if err == nil {
		t.saved, err = m.NewRef(ast.ASTHandle{})
	}
	if err == nil {
		t.configuration = &pbTacticConfiguration{owner: t}
		t.traversal, err = rewriter.NewTraversal(m, t.configuration)
	}
	if err == nil {
		err = t.updatePolicy(&t.params)
	}
	if err == nil {
		err = t.boolean.SetFlat(false)
	}
	if err == nil {
		err = t.boolean.SetEliminateAnd(true)
	}
	if err == nil {
		_, err = m.RegisterBitVectorPlugin()
	}
	if err == nil {
		t.bits = make(map[ast.ASTHandle]int)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakePB2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewPB2BVTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *PB2BVTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.bounds == nil || t.traversal == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *PB2BVTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *PB2BVTactic) DecRef() error {
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
func (t *PB2BVTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.releaseHandles(&t.heldKeys); err != nil {
		return err
	}
	if err := t.releaseHandles(&t.heldNots); err != nil {
		return err
	}
	t.entries = nil
	t.bits = nil
	if t.traversal != nil {
		if err := t.traversal.Close(); err != nil {
			return err
		}
		t.traversal = nil
	}
	if t.saved != (ast.ASTRef{}) {
		if err := t.manager.DelRef(&t.saved); err != nil {
			return err
		}
	}
	if t.bounds != nil {
		if err := t.bounds.Close(); err != nil {
			return err
		}
		t.bounds = nil
	}
	if err := t.releaseHandles(&t.temporaryInts); err != nil {
		return err
	}
	if err := t.resetUsed(); err != nil {
		return err
	}
	if t.boolean != nil {
		if err := t.boolean.Close(); err != nil {
			return err
		}
		t.boolean = nil
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
func (t *PB2BVTactic) Cleanup() (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if err = t.drainRetired(); err != nil {
		return err
	}
	fresh, err := NewPB2BVTactic(t.manager, t.context, t.globals, &t.params, t.services)
	if err != nil {
		return err
	}
	// Source publishes the fresh implementation before destroying the old.
	t.boolean, fresh.boolean = fresh.boolean, t.boolean
	t.traversal, fresh.traversal = fresh.traversal, t.traversal
	t.configuration, fresh.configuration = fresh.configuration, t.configuration
	t.configuration.owner = t
	fresh.configuration.owner = fresh
	t.saved, fresh.saved = fresh.saved, t.saved
	t.bounds, fresh.bounds = fresh.bounds, t.bounds
	t.allClausesLimit, fresh.allClausesLimit = fresh.allClausesLimit, t.allClausesLimit
	t.cardinalityLimit, fresh.cardinalityLimit = fresh.cardinalityLimit, t.cardinalityLimit
	t.maxMemory, fresh.maxMemory = fresh.maxMemory, t.maxMemory
	t.bits, fresh.bits = fresh.bits, t.bits
	t.entries, fresh.entries = fresh.entries, t.entries
	t.heldKeys, fresh.heldKeys = fresh.heldKeys, t.heldKeys
	t.heldNots, fresh.heldNots = fresh.heldNots, t.heldNots
	t.temporaryInts, fresh.temporaryInts = fresh.temporaryInts, t.temporaryInts
	t.used, fresh.used = fresh.used, t.used
	t.bounds.trace = t.trace
	if err = fresh.Close(); err != nil {
		t.retired = append(t.retired, fresh)
	}
	return err
}
func (t *PB2BVTactic) drainRetired() error {
	for len(t.retired) > 0 {
		if err := t.retired[0].Close(); err != nil {
			return err
		}
		t.retired = t.retired[1:]
	}
	return nil
}
func (t *PB2BVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	other, err := NewPB2BVTactic(m, t.context, t.globals, &t.params, t.services)
	if err == nil {
		other.trace = t.trace
		other.bounds.trace = t.trace
	}
	return other, err
}
func (t *PB2BVTactic) updatePolicy(p *uparams.ParamsRef) error {
	key := "max_memory"
	n, err := t.context.GetUIntName(p, &key, ^uint32(0))
	if err != nil {
		return err
	}
	t.maxMemory = umemory.MegabytesToBytes(n)
	key = "pb2bv_all_clauses_limit"
	t.allClausesLimit, err = t.context.GetUIntName(p, &key, 8)
	if err != nil {
		return err
	}
	key = "pb2bv_cardinality_limit"
	t.cardinalityLimit, err = t.context.GetUIntName(p, &key, ^uint32(0))
	if err != nil {
		return err
	}
	return t.boolean.UpdtParams(p)
}

func (t *PB2BVTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.updatePolicy(p)
}
func (t *PB2BVTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	for _, p := range []struct{ name, description string }{
		{"pb2bv_all_clauses_limit", "(default: 8) maximum number of literals for using equivalent CNF encoding of PB constraint."},
		{"pb2bv_cardinality_limit", "(default: inf) limit for using arc-consistent cardinality constraint encoding."},
	} {
		if err := out.InsertName(p.name, uparams.ParamUInt, &p.description, nil, nil); err != nil {
			return err
		}
	}
	if err := t.boolean.GetParamDescrs(out); err != nil {
		return err
	}
	if err := out.EraseName("flat"); err != nil {
		return err
	}
	return out.EraseName("elim_and")
}

func (t *PB2BVTactic) CollectStatistics() map[string]float64  { return nil }
func (t *PB2BVTactic) ResetStatistics() error                 { return t.open() }
func (t *PB2BVTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *PB2BVTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *PB2BVTactic) Reset() error                           { return t.Cleanup() }

func (t *PB2BVTactic) releaseHandles(values *[]ast.ASTHandle) error {
	for len(*values) != 0 {
		consumed, err := t.manager.DecRefConsumed((*values)[0])
		if consumed {
			*values = (*values)[1:]
		}
		if err != nil {
			return err
		}
	}
	*values = nil
	return nil
}
func (t *PB2BVTactic) resetUsed() error {
	if t.used.IsNull() {
		return nil
	}
	consumed, err := t.manager.DependencyDecConsumed(t.used)
	if consumed {
		t.used = ast.ASTDependency{}
	}
	return err
}
func (t *PB2BVTactic) checkpoint() error {
	if err := t.services.Cooperation.Cooperate("pb2bv"); err != nil {
		return err
	}
	n, err := t.services.Memory.AllocationSize()
	if err != nil {
		return err
	}
	if n > t.maxMemory {
		return tactic.NewTacticException("max. memory exceeded")
	}
	return nil
}
