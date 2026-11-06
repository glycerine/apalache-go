package core

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type PBPreprocessTactic struct {
	self     *PBPreprocessTactic
	manager  *ast.ASTManager
	services model.ModelEvaluatorConfig
	refs     uint32
	bytes    uint64
	closed   bool
}

var _ tactic.Tactic = (*PBPreprocessTactic)(nil)

func NewPBPreprocessTactic(m *ast.ASTManager, services model.ModelEvaluatorConfig) (*PBPreprocessTactic, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(PBPreprocessTactic{}).Size())
	if e := m.Budget().Reserve(n); e != nil {
		return nil, e
	}
	t := &PBPreprocessTactic{manager: m, services: services, bytes: n}
	t.self = t
	if _, e := m.RegisterPseudoBooleanPlugin(); e != nil {
		m.Budget().Release(n)
		return nil, e
	}
	return t, nil
}
func (t *PBPreprocessTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *PBPreprocessTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *PBPreprocessTactic) DecRef() error {
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
func (t *PBPreprocessTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
func (t *PBPreprocessTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = out.Prepare(t.manager); err != nil {
		return
	}
	if err = tactic.FailIfProofGeneration("pb-preprocess", g); err != nil {
		return
	}
	report, e := tactic.NewTacticReport("pb-preprocess", g)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	converter, e := tactic.NewExtensionModel(t.manager, t.services)
	if e != nil {
		return e
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, converter.Close())
		}
	}()
	for {
		progress, x := t.simplify(g, converter)
		if x != nil {
			return x
		}
		if !progress {
			break
		}
	}
	if err = g.IncDepth(); err != nil {
		return
	}
	if err = out.SetModelConverter(converter); err != nil {
		return
	}
	published = true
	if err = out.AddGoal(g); err != nil {
		return
	}
	return nil
}
func (t *PBPreprocessTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *PBPreprocessTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *PBPreprocessTactic) CollectStatistics() map[string]float64         { return nil }
func (t *PBPreprocessTactic) ResetStatistics() error                        { return t.open() }
func (t *PBPreprocessTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *PBPreprocessTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *PBPreprocessTactic) Cleanup() error                                { return t.open() }
func (t *PBPreprocessTactic) Reset() error                                  { return t.open() }
func (t *PBPreprocessTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewPBPreprocessTactic(m, t.services)
}
