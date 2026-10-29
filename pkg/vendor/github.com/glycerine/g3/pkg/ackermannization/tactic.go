package ackermannization

// Source: src/ackermannization/ackermannize_bv_tactic.cpp/.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// encodingState is the concrete state shared by the encoding tactic and the
// solver-backed lackr implementation. Services are borrowed by both owners.
type encodingState struct {
	manager  *ast.ASTManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	services model.ModelEvaluatorConfig
	params   uparams.ParamsRef // Constructor snapshot; updt_params does not replace it.
	stats    *Stats
	trace    *diagnostic.TraceTags
}
type Tactic struct {
	encodingState
	self        *Tactic
	paramsReady bool
	limit       float64 // Source has no defined pre-updt_params value; retained callers initialize it.
	pending     []*eagerEncoder
	refs        uint32
	header      uint64
	closed      bool
}

var _ tactic.Tactic = (*Tactic)(nil)

func NewTactic(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*Tactic, error) {
	c, g := services.Context, services.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(Tactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &Tactic{encodingState: encodingState{manager: m, context: c, globals: g, services: services, stats: &Stats{}}, header: n}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func (t *Tactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *Tactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *Tactic) DecRef() error {
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
func (t *Tactic) drain() error {
	for len(t.pending) > 0 {
		i := len(t.pending) - 1
		if err := t.pending[i].Close(); err != nil {
			return err
		}
		t.pending = t.pending[:i]
	}
	return nil
}
func (t *Tactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.drain(); err != nil {
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
func (t *encodingState) readEager(p *uparams.ParamsRef) (value bool, err error) {
	global, err := t.globals.SourceGetModuleName("ackermannization")
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, t.context.Del(&global)) }()
	key := "eager"
	return t.context.GetBoolFallbackName(p, &key, &global, true)
}
func (t *Tactic) UpdateParams(p *uparams.ParamsRef) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if p == nil {
		empty := t.context.EmptyValue()
		p = &empty
	}
	global, err := t.globals.SourceGetModuleName("rewriter")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.context.Del(&global)) }()
	key := "div0_ackermann_limit"
	n, err := t.context.GetUIntFallbackName(p, &key, &global, 1000)
	if err != nil {
		return err
	}
	t.limit = float64(n)
	return nil
}
func (t *Tactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.drain(); err != nil {
		return err
	}
	if err = result.BindManager(t.manager); err != nil {
		return err
	}
	if err = result.SetModelConverter(nil); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("ackermannize", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	if err = tactic.FailIfUnsatCoreGeneration("ackermannize", g); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("ackermannize", g); err != nil {
		return err
	}
	if err = t.emit(41, func(w io.Writer) error {
		if _, e := io.WriteString(w, "in\n"); e != nil {
			return e
		}
		return g.Display(w)
	}); err != nil {
		return err
	}
	r := &eagerEncoder{owner: &t.encodingState, groups: make(termGroups)}
	t.pending = append(t.pending, r)
	defer func() {
		var failure memory.ConstructionFailure
		if errors.As(err, &failure) {
			r.failures = append(r.failures, failure)
		}
		err = errors.Join(err, t.drain())
	}()
	if err = r.construct(g); err != nil {
		return err
	}
	goal, err := tactic.NewGoalConfigCopy(g)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, goal.Close())
		}
	}()
	success, err := r.mkAckermann(goal, t.limit)
	if err != nil {
		return err
	}
	if !success {
		if err = t.emit(52, func(w io.Writer) error { _, e := io.WriteString(w, "ackermannize not run due to limit\n"); return e }); err != nil {
			return err
		}
		if err = result.Prepare(t.manager); err != nil {
			return err
		}
		return result.AddGoal(g)
	}
	if err = result.TakeGoal(goal); err != nil {
		return err
	}
	published = true
	models, err := g.ModelsEnabled()
	if err != nil {
		return err
	}
	if models {
		mc, e := NewBVModelConverter(t.manager, r.info, t.services)
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
	if err = t.emit(67, func(w io.Writer) error {
		if _, e := io.WriteString(w, "out\n"); e != nil {
			return e
		}
		return goal.Display(w)
	}); err != nil {
		return err
	}
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil {
		return err
	}
	if enabled {
		good, err := goal.IsWellSorted()
		if err != nil {
			return err
		}
		if !good {
			return ast.ErrASTDomain
		}
	}
	return nil
}
func (t *Tactic) emit(line int, body func(io.Writer) error) error {
	return emitAck(t.trace, "ackermannize", "operator()", "src/ackermannization/ackermannize_bv_tactic.cpp", line, body)
}
func (t *Tactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *Tactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewTactic(m, &t.params, t.services)
}
func (t *Tactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return catalogCollectAckermannizeBvTacticParams(d)
}
func (t *Tactic) CollectStatistics() map[string]float64 {
	return map[string]float64{"ackr-constraints": float64(t.stats.Constraints)}
}
func (t *Tactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}
func (t *Tactic) Cleanup() error                         { return t.open() } // Source no-op; refused application destructors stay owned until Close.
func (t *Tactic) Reset() error                           { return t.Cleanup() }
func (t *Tactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *Tactic) SetProgressCallback(func(uint32)) error { return t.open() }
