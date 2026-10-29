package bv

// Source: src/tactic/bv/max_bv_sharing_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// MaxBVSharingTactic retains the source pair caches and rewriter between
// assertions. Successful application clears the pairs; Cleanup rebuilds the
// entire implementation from the owned parameter snapshot.
type MaxBVSharingTactic struct {
	self        *MaxBVSharingTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	params      uparams.ParamsRef
	current     *sharingImplementation
	retired     []*sharingImplementation
	cooperation *cooperate.Cooperation
	trace       *diagnostic.TraceTags
	ownerBytes  uint64
	refs        uint32
	paramsReady bool
	closed      bool
}

var _ tactic.Tactic = (*MaxBVSharingTactic)(nil)

func NewMaxBVSharingTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (*MaxBVSharingTactic, error) {
	if m == nil || c == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(MaxBVSharingTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &MaxBVSharingTactic{manager: m, context: c, cooperation: cooperation, ownerBytes: bytes}
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

func MakeMaxBVSharingTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, cooperation *cooperate.Cooperation) (tactic.Tactic, error) {
	child, err := NewMaxBVSharingTactic(m, c, p, cooperation)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

func (t *MaxBVSharingTactic) build() (*sharingImplementation, error) {
	p, err := NewSharingGoalPassWithParams(t.manager, t.context, &t.params)
	if err != nil {
		return nil, err
	}
	if t.cooperation != nil {
		if err = p.BindCooperation(t.cooperation); err != nil {
			return nil, err
		}
	}
	imp, err := newSharingImplementation(p)
	if err == nil {
		imp.configuration.reducer.trace = t.trace
	}
	return imp, err
}

func (t *MaxBVSharingTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *MaxBVSharingTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *MaxBVSharingTactic) DecRef() error {
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
func (t *MaxBVSharingTactic) drainRetired() error {
	for len(t.retired) != 0 {
		if err := t.retired[0].Close(); err != nil {
			return err
		}
		t.retired = t.retired[1:]
	}
	return nil
}
func (t *MaxBVSharingTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.current.Close(); err != nil {
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
func (t *MaxBVSharingTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.drainRetired(); err != nil {
		return err
	}
	// params_ref has a copy constructor and no move constructor in this
	// source tree, so std::move(m_params) still copies its snapshot.
	fresh, err := t.build()
	if err != nil {
		return err
	}
	old := t.current
	t.current = fresh
	t.retired = append(t.retired, old)
	return t.drainRetired()
}
func (t *MaxBVSharingTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err := result.Prepare(t.manager); err != nil {
		return err
	}
	if err := t.current.apply(goal); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
func (t *MaxBVSharingTactic) UpdateParams(p *uparams.ParamsRef) error {
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
	pass := t.current.configuration.pass
	if err := pass.UpdateParams(t.context, &t.params); err != nil {
		return err
	}
	t.current.configuration.reducer.maxArgs = pass.maxArgs
	return nil
}
func (t *MaxBVSharingTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	other, err := NewMaxBVSharingTactic(m, t.context, &t.params, t.cooperation)
	if err == nil {
		other.trace = t.trace
		other.current.configuration.reducer.trace = t.trace
	}
	return other, err
}
func (t *MaxBVSharingTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	if err := uparams.InsertMaxSteps(out); err != nil {
		return err
	}
	description := "(default: 128) maximum number of arguments (per application) that will be considered by the greedy (quadratic) heuristic."
	return out.InsertName("max_args", uparams.ParamUInt, &description, nil, nil)
}
func (t *MaxBVSharingTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	t.current.configuration.reducer.trace = tags
	return nil
}
func (t *MaxBVSharingTactic) CollectStatistics() map[string]float64  { return nil }
func (t *MaxBVSharingTactic) ResetStatistics() error                 { return t.open() }
func (t *MaxBVSharingTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *MaxBVSharingTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *MaxBVSharingTactic) Reset() error                           { return t.Cleanup() }
