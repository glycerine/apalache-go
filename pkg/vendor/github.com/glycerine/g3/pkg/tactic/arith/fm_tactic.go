package arith

// Source: src/tactic/arith/fm_tactic.cpp:377-412,777-846,1534-1693.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fmTacticPolicy struct {
	config            FMEliminationConfig
	maxMemory         uint64
	limit             uint32
	realOnly, clauses bool
}

type FMTactic struct {
	fmTacticPolicy
	self                 *FMTactic
	manager              *ast.ASTManager
	context              *uparams.ParamsContext
	globals              *uparams.GlobalParamsService
	services             model.ModelEvaluatorConfig
	params               uparams.ParamsRef
	run                  *fmTacticRun
	refs                 uint32
	bytes                uint64
	paramsReady, running bool
	closed               bool
}

var _ tactic.Tactic = (*FMTactic)(nil)

func NewFMTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (_ *FMTactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(FMTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &FMTactic{manager: m, context: c, globals: g, services: services, bytes: n}
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
	if err == nil {
		_, err = m.RegisterArithmeticPlugin()
	}
	if err == nil {
		t.run, err = newFMTacticRun(m)
	}
	if err == nil {
		err = t.fmTacticPolicy.readPolicy(c, &t.params)
	}
	return t, err
}

func (t *FMTactic) open() error {
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
func (t *FMTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *FMTactic) DecRef() error {
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
func (t *FMTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.run != nil {
		if err := t.run.close(); err != nil {
			return err
		}
		t.run = nil
	}
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
func (t *FMTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	fresh, err := newFMTacticRun(t.manager)
	if err != nil {
		return err
	}
	policy := fmTacticPolicy{}
	if err = policy.readPolicy(t.context, &t.params); err != nil {
		return errors.Join(err, fresh.close())
	}
	if t.run != nil {
		if err = t.run.close(); err != nil {
			return errors.Join(err, fresh.close())
		}
	}
	t.run, t.fmTacticPolicy = fresh, policy
	return nil
}
func (t *FMTactic) Reset() error { return t.Cleanup() }
func (t *fmTacticPolicy) readPolicy(c *uparams.ParamsContext, p *uparams.ParamsRef) error {
	key := "max_memory"
	n, err := c.GetUIntName(p, &key, math.MaxUint32)
	if err != nil {
		return err
	}
	t.maxMemory = uint64(n) << 20
	if n == math.MaxUint32 || t.maxMemory > uint64(^uint(0)) {
		t.maxMemory = uint64(^uint(0))
	}
	key = "fm_real_only"
	realOnly, err := c.GetBoolName(p, &key, true)
	if err != nil {
		return err
	}
	t.realOnly = realOnly
	for _, field := range []struct {
		name     string
		value    *uint32
		fallback uint32
	}{
		{"fm_limit", &t.limit, 5000000}, {"fm_cutoff1", &t.config.CutoffSide, 8}, {"fm_cutoff2", &t.config.CutoffProduct, 256}, {"fm_extra", &t.config.Extra, 0},
	} {
		key = field.name
		value, err := c.GetUIntName(p, &key, field.fallback)
		if err != nil {
			return err
		}
		*field.value = value
	}
	key = "fm_occ"
	clauses, err := c.GetBoolName(p, &key, false)
	if err != nil {
		return err
	}
	t.clauses = clauses
	return nil
}
func (t *FMTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.fmTacticPolicy.readPolicy(t.context, p)
}
func (t *FMTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, field := range []struct {
		name        string
		kind        uparams.ParamKind
		description string
	}{
		{"produce_models", uparams.ParamBool, "(default: false) model generation."},
		{"max_memory", uparams.ParamUInt, "(default: infty) maximum amount of memory in megabytes."},
		{"fm_real_only", uparams.ParamBool, "(default: true) consider only real variables for fourier-motzkin elimination."},
		{"fm_occ", uparams.ParamBool, "(default: false) consider inequalities occurring in clauses for FM."},
		{"fm_limit", uparams.ParamUInt, "(default: 5000000) maximum number of constraints, monomials, clauses visited during FM."},
		{"fm_cutoff1", uparams.ParamUInt, "(default: 8) first cutoff for FM based on maximum number of lower/upper occurrences."},
		{"fm_cutoff2", uparams.ParamUInt, "(default: 256) second cutoff for FM based on num_lower * num_upper occurrences."},
		{"fm_extra", uparams.ParamUInt, "(default: 0) max. increase on the number of inequalities for each FM variable elimination step."},
	} {
		if err := d.InsertName(field.name, field.kind, &field.description, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
func (t *FMTactic) CollectStatistics() map[string]float64  { return nil }
func (t *FMTactic) ResetStatistics() error                 { return t.open() }
func (t *FMTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *FMTactic) SetProgressCallback(func(uint32)) error { return t.open() }
