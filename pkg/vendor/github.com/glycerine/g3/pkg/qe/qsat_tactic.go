package qe

// Source: src/qe/qsat.cpp qsat tactic/factories.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type QSATTactic struct {
	self    *QSATTactic
	manager *ast.ASTManager
	config  Config
	mode    qsatMode
	search  *qsatSearch
	pending []*qsatTacticCall
	refs    uint32
	bytes   uint64
	closed  bool
}

var _ tactic.Tactic = (*QSATTactic)(nil)

type qsatConverterSlot struct {
	value          tactic.ModelConverter
	owned, cleanup bool
}
type qsatTacticCall struct {
	owner      *QSATTactic
	refs       []ast.ASTRef
	converters []qsatConverterSlot
	report     *tactic.TacticReport
	failures   []memory.ConstructionFailure
}
type QSATTacticFailure struct {
	owner          *QSATTactic
	cause, cleanup error
	constructor    bool
}

func (e *QSATTacticFailure) Error() string        { return e.Unwrap().Error() }
func (e *QSATTacticFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *QSATTacticFailure) OriginalCause() error { return e.cause }
func (e *QSATTacticFailure) CleanupCause() error  { return e.cleanup }
func (e *QSATTacticFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *QSATTacticFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}
func NewQSATTactic(m *ast.ASTManager, _ *uparams.ParamsRef, c Config) (*QSATTactic, error) {
	return newQSATTactic(m, qsatSAT, c)
}
func NewQE2Tactic(m *ast.ASTManager, _ *uparams.ParamsRef, c Config) (*QSATTactic, error) {
	return newQSATTactic(m, qsatQE, c)
}
func NewQERecursiveTactic(m *ast.ASTManager, _ *uparams.ParamsRef, c Config) (*QSATTactic, error) {
	return newQSATTactic(m, qsatRecursiveQE, c)
}
func newQSATTactic(m *ast.ASTManager, mode qsatMode, config Config) (result *QSATTactic, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Cooperation == nil || config.Globals.Context != config.Context {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(QSATTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &QSATTactic{manager: m, config: config, mode: mode, bytes: n}
	t.self = t
	defer func() {
		if err != nil {
			c := &qsatTacticCall{owner: t}
			c.retain(err)
			t.pending = append(t.pending, c)
			if cleanup := t.Close(); cleanup != nil {
				result = t
				err = &QSATTacticFailure{owner: t, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	t.search, err = newQSATSearch(m, mode, config)
	if err != nil {
		return nil, err
	}
	return t, nil
}
func (t *QSATTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.search == nil {
		return arena.ErrClosed
	}
	return t.search.open()
}
func (t *QSATTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *QSATTactic) DecRef() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		if err := t.Close(); err != nil {
			return &QSATTacticFailure{owner: t, cause: err, constructor: true}
		}
	}
	return nil
}
func (c *qsatTacticCall) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}
func (c *qsatTacticCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *qsatTacticCall) ownConverter(value tactic.ModelConverter, err error) (tactic.ModelConverter, error) {
	if value == nil {
		return nil, err
	}
	c.converters = append(c.converters, qsatConverterSlot{value: value})
	i := len(c.converters) - 1
	if err != nil {
		return value, err
	}
	if err = value.IncRef(); err != nil {
		return value, err
	}
	c.converters[i].owned = true
	return value, nil
}
func (c *qsatTacticCall) close() error {
	for len(c.failures) > 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.converters) > 0 {
		i := len(c.converters) - 1
		r := &c.converters[i]
		if r.cleanup || !r.owned {
			if err := r.value.Close(); err != nil {
				c.retain(err)
				return err
			}
		} else {
			consumed, err := r.value.DecRefConsumed()
			if consumed {
				r.owned = false
				r.cleanup = err != nil
			}
			if err != nil {
				c.retain(err)
				return err
			}
			if !consumed {
				return tactic.ErrModelConverterOwner
			}
		}
		c.converters = c.converters[:i]
	}
	for len(c.refs) > 0 {
		i := len(c.refs) - 1
		if c.refs[i] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.refs[i]); err != nil {
				c.retain(err)
				return err
			}
		}
		c.refs = c.refs[:i]
	}
	if c.report != nil {
		if err := c.report.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.report = nil
	}
	return nil
}
func (t *QSATTactic) drain() error {
	for len(t.pending) > 0 {
		i := len(t.pending) - 1
		if err := t.pending[i].close(); err != nil {
			return err
		}
		t.pending = t.pending[:i]
	}
	return nil
}
func (t *QSATTactic) finish(c *qsatTacticCall, err *error) {
	c.retain(*err)
	if cleanup := t.drain(); cleanup != nil {
		*err = &QSATTacticFailure{owner: t, cause: *err, cleanup: cleanup}
	}
}
func (t *QSATTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.drain(); err != nil {
		return err
	}
	c := &qsatTacticCall{owner: t}
	t.pending = append(t.pending, c)
	defer t.finish(c, &err)
	c.report, err = tactic.NewTacticReport("qsat-tactic", g)
	if err != nil {
		return err
	}
	if err = result.BindManager(t.manager); err != nil {
		return err
	}
	if err = result.SetModelConverter(nil); err != nil {
		return err
	}
	if err = result.SetProofConverter(nil); err != nil {
		return err
	}
	if err = result.SetCore(ast.ASTDependency{}); err != nil {
		return err
	}
	forms, err := g.AppendFormulas(nil)
	if err != nil {
		return err
	}
	h, err := t.manager.MkAndArray(forms)
	if err != nil {
		return err
	}
	fml, err := c.ref(h)
	if err != nil {
		return err
	}
	if t.mode == qsatRecursiveQE {
		if err = t.search.eliminateRecursive(h, fml); err != nil {
			return err
		}
		if err = g.Reset(); err != nil {
			return err
		}
		if err = g.IncDepth(); err != nil {
			return err
		}
		h, err = t.manager.RefGet(fml)
		if err != nil {
			return err
		}
		if err = g.Assert(h); err != nil {
			return err
		}
		return result.AddGoal(g)
	}
	if err = t.search.initialize(fml); err != nil {
		return err
	}
	status, err := t.search.check()
	if err != nil {
		return err
	}
	switch status {
	case lbool.False:
		if err = g.Reset(); err != nil {
			return err
		}
		if err = g.IncDepth(); err != nil {
			return err
		}
		if t.mode == qsatQE {
			hs := make([]ast.ASTHandle, len(t.search.answers))
			for i, r := range t.search.answers {
				hs[i], err = t.manager.RefGet(r)
				if err != nil {
					return err
				}
			}
			h, err = t.manager.MkAndArray(hs)
		} else {
			h, err = t.manager.MkFalse()
		}
		if err != nil {
			return err
		}
		if err = g.Assert(h); err != nil {
			return err
		}
		return result.AddGoal(g)
	case lbool.True:
		if err = g.Reset(); err != nil {
			return err
		}
		if err = g.IncDepth(); err != nil {
			return err
		}
		if err = result.AddGoal(g); err != nil {
			return err
		}
		models, err := g.ModelsEnabled()
		if err != nil || !models {
			return err
		}
		md, err := t.search.saved.Get()
		if err != nil {
			return err
		}
		stored, err := c.ownConverter(tactic.NewStoredModel(md))
		if err != nil {
			return err
		}
		filter, err := t.search.preparation.abstraction.FilterModelConverter()
		if err != nil {
			return err
		}
		combined, err := c.ownConverter(tactic.ConcatModels(t.manager.Budget(), filter, stored))
		if err != nil {
			return err
		}
		return result.SetModelConverter(combined)
	case lbool.Undef:
		if err = result.AddGoal(g); err != nil {
			return err
		}
		ex, e := t.search.exists.solver()
		if e != nil {
			return e
		}
		reason, e := ex.ReasonUnknown()
		if e != nil {
			return e
		}
		if reason == "ok" || reason == "unknown" {
			fa, e := t.search.forall.solver()
			if e != nil {
				return e
			}
			reason, e = fa.ReasonUnknown()
			if e != nil {
				return e
			}
		}
		return tactic.NewTacticException(reason)
	}
	return ast.ErrASTDomain
}

// The source constructor, updater, descriptors, logic and callback hooks
// deliberately ignore caller parameters; translation uses empty stored params.
func (t *QSATTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *QSATTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *QSATTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *QSATTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (s *qsatSearch) collectStatistics(out *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	out.Copy(s.stats)
	for _, k := range []*qsatKernel{s.forall, s.exists} {
		solver, err := k.solver()
		if err != nil {
			return err
		}
		if err = solver.CollectStatistics(out); err != nil {
			return err
		}
	}
	stats, err := s.preparation.abstraction.CollectStatistics()
	if err != nil {
		return err
	}
	for k, v := range stats {
		out.UpdateUint(statistics.Key(k), uint32(v))
	}
	out.UpdateUint("qsat num rounds", s.rounds)
	for k, v := range stats {
		out.UpdateUint(statistics.Key(k), uint32(v))
	}
	return nil
}
func (t *QSATTactic) CollectStatistics() map[string]float64 {
	if t.open() != nil {
		return nil
	}
	var st statistics.Statistics
	if t.search.collectStatistics(&st) != nil {
		return nil
	}
	out := make(map[string]float64)
	for i := 0; i < st.Size(); i++ {
		v := 0.0
		if st.IsUint(i) {
			v = float64(st.UintValue(i))
		} else {
			v = st.DoubleValue(i)
		}
		out[string(st.Key(i))] += v
	}
	return out
}
func (t *QSATTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.search.rounds = 0
	if err := t.search.forall.reset(); err != nil {
		return err
	}
	return t.search.exists.reset()
}
func (t *QSATTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.drain(); err != nil {
		return err
	}
	return t.search.reset()
}
func (t *QSATTactic) Reset() error { return t.Cleanup() }
func (t *QSATTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return newQSATTactic(m, t.mode, t.config)
}
func (t *QSATTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.drain(); err != nil {
		return err
	}
	if t.search != nil {
		if err := t.search.Close(); err != nil {
			return err
		}
		t.search = nil
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
