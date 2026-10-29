package smtlogics

// Source: src/tactic/smtlogics/qfufbv_tactic.cpp qfufbv_ackr_tactic.

import (
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	incsat "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ackrInvocation struct {
	backend      solver.Solver
	child        *qfliaConstruction
	lackr        *ackermannization.Lackr
	goal         *tactic.Goal
	info         *ackermannization.Info
	infoCleanup  bool
	model        *model.ModelCore
	modelCleanup bool
	converter    tactic.ModelConverter
	failures     []memory.ConstructionFailure
}

func (w *ackrInvocation) close() error {
	for len(w.failures) > 0 {
		i := len(w.failures) - 1
		if err := w.failures[i].Cleanup(); err != nil {
			return err
		}
		w.failures = w.failures[:i]
	}
	if w.converter != nil {
		if err := w.converter.Close(); err != nil {
			return err
		}
		w.converter = nil
	}
	if w.goal != nil {
		if err := w.goal.Close(); err != nil {
			return err
		}
		w.goal = nil
	}
	if w.model != nil {
		if w.modelCleanup {
			if err := w.model.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := w.model.DecRefConsumed()
			if consumed && err != nil {
				w.modelCleanup = true
			}
			if err != nil {
				return err
			}
		}
		w.model = nil
	}
	if w.info != nil {
		if w.infoCleanup {
			if err := w.info.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := w.info.DecRefConsumed()
			if consumed && err != nil {
				w.infoCleanup = true
			}
			if err != nil {
				return err
			}
		}
		w.info = nil
	}
	if w.lackr != nil {
		if err := w.lackr.Close(); err != nil {
			return err
		}
		w.lackr = nil
	}
	if w.backend != nil {
		if err := w.backend.Close(); err != nil {
			return err
		}
		w.backend = nil
	}
	if w.child != nil {
		if err := w.child.close(); err != nil {
			return err
		}
		w.child = nil
	}
	return nil
}

// QFUFBVAckrCoreTactic is the source raw tactic, before preamble/probe wrapping.
// Constructor parameters are copied; UpdateParams changes only backend flags.
type QFUFBVAckrCoreTactic struct {
	self                   *QFUFBVAckrCoreTactic
	manager                *ast.ASTManager
	config                 smtcore.SMTSolverConfig
	params                 uparams.ParamsRef
	paramsReady            bool
	output                 io.Writer
	stats                  ackermannization.Stats
	useSAT, incrementalSAT bool
	pending                []*ackrInvocation
	refs                   uint32
	header                 uint64
	closed                 bool
}

var _ tactic.Tactic = (*QFUFBVAckrCoreTactic)(nil)

func NewQFUFBVAckrCoreTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (*QFUFBVAckrCoreTactic, error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(QFUFBVAckrCoreTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	if output == nil {
		output = os.Stdout
	}
	t := &QFUFBVAckrCoreTactic{manager: m, config: config, output: output, header: n}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err != nil {
		if cleanup := t.Close(); cleanup != nil {
			return nil, &QFUFBVConstructionFailure{owner: &qfliaConstruction{c: c, m: m, pending: []io.Closer{t}}, cause: err, cleanup: cleanup}
		}
		return nil, err
	}
	return t, nil
}
func (t *QFUFBVAckrCoreTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if !t.paramsReady {
		return ast.ErrASTPending
	}
	return t.config.Params.Check(&t.params)
}
func (t *QFUFBVAckrCoreTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *QFUFBVAckrCoreTactic) DecRef() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
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
func (t *QFUFBVAckrCoreTactic) drain() error {
	for len(t.pending) > 0 {
		i := len(t.pending) - 1
		if err := t.pending[i].close(); err != nil {
			return err
		}
		t.pending = t.pending[:i]
	}
	return nil
}
func (t *QFUFBVAckrCoreTactic) Close() error {
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
		if err := t.config.Params.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}
func (t *QFUFBVAckrCoreTactic) UpdateParams(p *uparams.ParamsRef) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if p == nil {
		empty := t.config.Params.EmptyValue()
		p = &empty
	}
	global, err := t.config.Globals.SourceGetModuleName("ackermannization")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.config.Params.Del(&global)) }()
	key := "sat_backend"
	t.useSAT, err = t.config.Params.GetBoolFallbackName(p, &key, &global, false)
	if err != nil {
		return err
	}
	key = "inc_sat_backend"
	t.incrementalSAT, err = t.config.Params.GetBoolFallbackName(p, &key, &global, false)
	return err
}
func (t *QFUFBVAckrCoreTactic) setup(w *ackrInvocation) (err error) {
	if t.useSAT && t.incrementalSAT {
		var backend *incsat.Solver
		backend, err = incsat.NewIncrementalSolverWithConfig(t.manager, &t.params, incsat.IncrementalSolverConfig{Services: t.config.ModelEvaluator, Output: t.output})
		if backend != nil {
			w.backend = backend
		}
	} else {
		w.child = &qfliaConstruction{c: t.config.Params, m: t.manager}
		var child tactic.Tactic
		if t.useSAT {
			child = w.child.keep(NewQFBVTacticWithOutput(t.manager, &t.params, t.config, t.output))
		} else {
			child = w.child.keep(NewQFAUFBVTactic(t.manager, &t.params, t.config))
		}
		if w.child.err != nil {
			return w.child.err
		}
		var backend *solver.TacticSolver
		backend, err = solver.NewTacticSolver(t.manager, child, &t.params, false, true, false, "", solver.TacticSolverConfig{Params: t.config.Params, Trace: t.config.ModelEvaluator.Trace})
		if backend != nil {
			w.backend = backend
		}
	}
	if err != nil {
		return err
	}
	return w.backend.SetProduceModels(true)
}

func (t *QFUFBVAckrCoreTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
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
	report, err := tactic.NewTacticReport("qfufbv_ackr", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	if err = tactic.FailIfUnsatCoreGeneration("qfufbv_ackr", g); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("qfufbv_ackr", g); err != nil {
		return err
	}
	tags := t.config.ModelEvaluator.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err = tags.EmitConditional("qfufbv_ackr_tactic", "operator()", "src/tactic/smtlogics/qfufbv_tactic.cpp", 67, func() bool { return true }, func(out io.Writer) error {
		if _, e := io.WriteString(out, "goal:\n"); e != nil {
			return e
		}
		return g.Display(out)
	})
	if tags.IsStreamFailure(err) {
		err = nil
	}
	if err != nil {
		return err
	}
	w := &ackrInvocation{}
	t.pending = append(t.pending, w)
	defer func() {
		var failure memory.ConstructionFailure
		if errors.As(err, &failure) {
			w.failures = append(w.failures, failure)
		}
		err = errors.Join(err, t.drain())
	}()
	count, err := g.Size()
	if err != nil {
		return err
	}
	forms := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		h, e := g.Form(i)
		if e != nil {
			return e
		}
		forms = append(forms, h)
	}
	if err = t.setup(w); err != nil {
		return err
	}
	w.lackr, err = ackermannization.NewLackr(t.manager, &t.params, &t.stats, forms, w.backend, t.config.ModelEvaluator)
	if err != nil {
		return err
	}
	if err = w.lackr.BindTrace(t.config.ModelEvaluator.Trace); err != nil {
		return err
	}
	status, err := w.lackr.Solve()
	if err != nil {
		return err
	}
	w.goal, err = tactic.NewGoalConfigCopy(g)
	if err != nil {
		return err
	}
	if status == lbool.False {
		f, e := t.manager.MkFalse()
		if e != nil {
			return e
		}
		if e = w.goal.Assert(f); e != nil {
			return e
		}
	}
	if status != lbool.Undef {
		if err = result.TakeGoal(w.goal); err != nil {
			return err
		}
		w.goal = nil
	}
	models, err := g.ModelsEnabled()
	if err != nil {
		return err
	}
	if models && status == lbool.True {
		w.model, err = w.lackr.AcquireModel()
		if err != nil {
			return err
		}
		w.info, err = w.lackr.Info()
		if err != nil {
			return err
		}
		converter, e := NewQFUFBVAckrModelConverter(t.manager, w.info, w.model, t.config.ModelEvaluator)
		err = e
		if converter != nil {
			w.converter = converter
		}
		if err != nil {
			return err
		}
		if err = result.SetModelConverter(w.converter); err != nil {
			return err
		}
		w.converter = nil
	}
	return nil
}

func (t *QFUFBVAckrCoreTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.open() != nil {
		return nil
	}
	global, err := t.config.Globals.SourceGetModuleName("ackermannization")
	if err != nil {
		return nil
	}
	key := "eager"
	eager, err := t.config.Params.GetBoolFallbackName(&t.params, &key, &global, true)
	cleanup := t.config.Params.Del(&global)
	if err != nil || cleanup != nil {
		return nil
	}
	out := map[string]float64{"ackr-constraints": float64(t.stats.Constraints)}
	if !eager {
		out["lackr-its"] = float64(t.stats.Iterations)
	}
	return out
}
func (t *QFUFBVAckrCoreTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}
func (t *QFUFBVAckrCoreTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *QFUFBVAckrCoreTactic) Cleanup() error                                { return t.open() }
func (t *QFUFBVAckrCoreTactic) Reset() error                                  { return t.Cleanup() }
func (t *QFUFBVAckrCoreTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *QFUFBVAckrCoreTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *QFUFBVAckrCoreTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewQFUFBVAckrCoreTactic(m, &t.params, t.config, t.output)
}
