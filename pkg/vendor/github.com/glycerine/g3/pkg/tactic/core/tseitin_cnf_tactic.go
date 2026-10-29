package core

// Source: src/tactic/core/tseitin_cnf_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type TseitinCNFTactic struct {
	self       *TseitinCNFTactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	services   model.ModelEvaluatorConfig
	params     uparams.ParamsRef
	ownerBytes uint64
	maxMemory  uint64
	refs       uint32
	numAux     uint32
	closed     bool

	commonPatterns       bool
	distributivity       bool
	distributivityBlowup uint32
	iteChains            bool
	iteExtra             bool
}

var _ tactic.Tactic = (*TseitinCNFTactic)(nil)

func NewTseitinCNFCoreTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*TseitinCNFTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if err := m.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	params, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(TseitinCNFTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, c.Del(&params))
	}
	t := &TseitinCNFTactic{manager: m, context: c, globals: g, services: services, params: params, ownerBytes: amount}
	t.self = t
	if p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		err = t.readPolicy(&t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakeTseitinCNFCoreTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewTseitinCNFCoreTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

// MakeTseitinCNFTactic retains the source fallback: try the core encoder,
// otherwise simplify away AND/distinct gates and retry the core encoder.
func MakeTseitinCNFTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (result tactic.Tactic, err error) {
	if c == nil {
		return nil, uparams.ErrParamsOwner
	}
	settings, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&settings)) }()
	if p != nil {
		if err = c.Assign(&settings, p); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"elim_and", "blast_distinct"} {
		if err = c.SetBoolName(&settings, &name, true); err != nil {
			return nil, err
		}
	}
	var held []tactic.Tactic
	hold := func(x tactic.Tactic, e error) tactic.Tactic {
		if e != nil {
			if err == nil {
				err = e
			} else {
				err = errors.Join(err, e)
			}
			return nil
		}
		if e = x.IncRef(); e != nil {
			if err == nil {
				err = e
			} else {
				err = errors.Join(err, e)
			}
			return nil
		}
		held = append(held, x)
		return x
	}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			err = errors.Join(err, held[i].DecRef())
		}
	}()
	first := hold(MakeTseitinCNFCoreTactic(m, c, g, p, services))
	if err != nil {
		return nil, err
	}
	simplify := hold(MakeSimplifyTactic(m, c, g, &settings))
	if err != nil {
		return nil, err
	}
	second := hold(MakeTseitinCNFCoreTactic(m, c, g, p, services))
	if err != nil {
		return nil, err
	}
	retry := hold(tactic.NewAndThenTactic(simplify, second))
	if err != nil {
		return nil, err
	}
	return tactic.NewOrElseTactic(first, retry)
}

func (t *TseitinCNFTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *TseitinCNFTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *TseitinCNFTactic) DecRef() error {
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
func (t *TseitinCNFTactic) readPolicy(p *uparams.ParamsRef) (err error) {
	for _, f := range []struct {
		name string
		dst  *bool
		def  bool
	}{
		{"common_patterns", &t.commonPatterns, true}, {"distributivity", &t.distributivity, true},
		{"ite_chains", &t.iteChains, true}, {"ite_extra", &t.iteExtra, true},
	} {
		if *f.dst, err = t.context.GetBoolName(p, &f.name, f.def); err != nil {
			return err
		}
	}
	name := "distributivity_blowup"
	if t.distributivityBlowup, err = t.context.GetUIntName(p, &name, 32); err != nil {
		return err
	}
	name = "max_memory"
	mb, err := t.context.GetUIntName(p, &name, ^uint32(0))
	if err == nil {
		t.maxMemory = umemory.MegabytesToBytes(mb)
	}
	return err
}
func (t *TseitinCNFTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.readPolicy(&t.params)
}
func tseitinDescr(s string) *string { return &s }
func (t *TseitinCNFTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	for _, d := range []struct {
		name string
		kind uparams.ParamKind
		text string
	}{
		{"common_patterns", uparams.ParamBool, "minimize auxiliary variables by identifying common patterns"},
		{"distributivity", uparams.ParamBool, "apply distributivity over unshared subformulas"},
		{"distributivity_blowup", uparams.ParamUInt, "maximum overhead for distributive CNF encoding"},
		// Keep the source descriptor typo; parameter lookup itself uses ite_chains.
		{"ite_chaing", uparams.ParamBool, "identify if-then-else chains"},
		{"ite_extra", uparams.ParamBool, "add redundant clauses improving ITE unit propagation"},
	} {
		if err := out.InsertName(d.name, d.kind, tseitinDescr(d.text), nil, nil); err != nil {
			return err
		}
	}
	return nil
}
func (t *TseitinCNFTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.self != t || t.closed || t.numAux == 0 {
		return nil
	}
	return map[string]float64{"cnf encoding aux vars": float64(t.numAux)}
}
func (t *TseitinCNFTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.numAux = 0
	return nil
}
func (t *TseitinCNFTactic) Cleanup() error                         { return t.open() }
func (t *TseitinCNFTactic) Reset() error                           { return t.Cleanup() }
func (t *TseitinCNFTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *TseitinCNFTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *TseitinCNFTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	r, err := NewTseitinCNFCoreTactic(m, t.context, t.globals, &t.params, t.services)
	if err == nil {
		r.numAux = t.numAux
	}
	return r, err
}
func (t *TseitinCNFTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.context.Del(&t.params); err != nil {
		return err
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes, t.closed = 0, true
	return nil
}
