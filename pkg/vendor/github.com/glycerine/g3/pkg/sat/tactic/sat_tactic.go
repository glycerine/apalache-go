package tactic

// Source: src/sat/tactic/sat_tactic.cpp/.h.

import (
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// SATTactic stores parameters and accumulated statistics; each application
// creates a fresh solver from its goal's manager, matching the source factory.
type SATTactic struct {
	self                        *SATTactic
	context                     *uparams.ParamsContext
	params                      uparams.ParamsRef
	services                    model.ModelEvaluatorConfig
	output                      io.Writer
	stats                       statistics.Statistics
	application                 *satTacticApplication
	ownerBytes                  uint64
	refs                        uint32
	paramsReady, active, closed bool
}
type satTacticApplication struct {
	manager                                      *ast.ASTManager
	runtime                                      *sat.Runtime
	solver                                       *sat.Solver
	translator                                   *GoalTranslator
	labels                                       []ast.ASTRef
	model                                        tactic.ModelRef
	pendingModel                                 *model.ModelCore
	converter                                    tactic.ModelConverter
	core, leaf                                   ast.ASTDependency
	coreHeld, leafHeld, corePending, leafPending bool
	dimacs                                       *utext.NumericTextStream
}

var _ tactic.Tactic = (*SATTactic)(nil)

// SATApplicationFailure retains the real owner if lexical destruction stops.
// Explicit Cleanup acknowledges an already observed SAT destructor guard;
// the original failure remains available through Unwrap after cleanup.
type SATApplicationFailure struct {
	Cause       error
	Owner       *SATTactic
	application *satTacticApplication
}

func (e *SATApplicationFailure) Error() string        { return e.Cause.Error() }
func (e *SATApplicationFailure) Unwrap() error        { return e.Cause }
func (e *SATApplicationFailure) OriginalCause() error { return e.Cause }
func (e *SATApplicationFailure) CleanupCause() error  { return e.Cause }
func (e *SATApplicationFailure) RuntimeOwner() *umemory.Runtime {
	if e.Owner == nil {
		return nil
	}
	return e.Owner.context.Numerals().RuntimeOwner()
}
func (e *SATApplicationFailure) Cleanup() error {
	t := e.Owner
	if t == nil || t.closed {
		return nil
	}
	if t.application != e.application {
		return nil
	}
	if t.active {
		return sat.ErrSATOwner
	}
	if a := t.application; a != nil && a.solver != nil && a.solver.StoppedCleanupCause() != nil {
		if err := a.solver.CleanupStopped(); err != nil {
			return err
		}
	}
	if err := t.releaseApplication(); err != nil {
		return err
	}
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}

func NewSATTactic(c *uparams.ParamsContext, p *uparams.ParamsRef, services model.ModelEvaluatorConfig, output io.Writer) (*SATTactic, error) {
	if c == nil || services.Context != c || services.Globals == nil || services.Globals.Context != c || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckOpen(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(SATTactic{}).Size())
	if err := c.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	if output == nil {
		output = os.Stdout
	}
	t := &SATTactic{context: c, services: services, output: output, ownerBytes: bytes}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err != nil {
		if cleanup := t.Close(); cleanup != nil {
			return t, errors.Join(err, cleanup)
		}
		return nil, err
	}
	return t, nil
}
func MakeSATTactic(c *uparams.ParamsContext, p *uparams.ParamsRef, services model.ModelEvaluatorConfig, output io.Writer) (tactic.Tactic, error) {
	child, err := NewSATTactic(c, p, services, output)
	if err != nil {
		if child == nil {
			return nil, err
		}
		return child, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func MakeSATPreprocessorTactic(c *uparams.ParamsContext, p *uparams.ParamsRef, services model.ModelEvaluatorConfig, output io.Writer) (result tactic.Tactic, err error) {
	aux, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&aux)) }()
	key := "max_conflicts"
	if err = c.SetUIntName(&aux, &key, 0); err != nil {
		return nil, err
	}
	child, err := MakeSATTactic(c, p, services, output)
	if err != nil {
		return child, err
	}
	if err = child.IncRef(); err != nil {
		return child, err
	}
	defer func() { err = errors.Join(err, child.DecRef()) }()
	using, err := tactic.NewUsingParamsTactic(c, child, &aux)
	if err != nil {
		return nil, err
	}
	if err = using.IncRef(); err != nil {
		return using, err
	}
	defer func() { err = errors.Join(err, using.DecRef()) }()
	wrapped, err := tactic.NewCleanTactic(using)
	if err != nil {
		return nil, err
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	if err = wrapped.UpdateParams(p); err != nil {
		if e := wrapped.IncRef(); e != nil {
			return wrapped, errors.Join(err, e)
		}
		return nil, errors.Join(err, wrapped.DecRef())
	}
	return wrapped, nil
}
func (t *SATTactic) open() error {
	if t == nil || t.self != t {
		return sat.ErrSATOwner
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *SATTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *SATTactic) DecRef() error {
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
func (t *SATTactic) Close() error {
	if t == nil || t.self != t {
		return sat.ErrSATOwner
	}
	if t.closed {
		return nil
	}
	if t.active {
		return sat.ErrSATAssertion
	}
	if err := t.releaseApplication(); err != nil {
		return err
	}
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.context.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *SATTactic) releaseApplication() error {
	a := t.application
	if a == nil {
		return nil
	}
	if a.converter != nil {
		if err := a.converter.Close(); err != nil {
			return err
		}
		a.converter = nil
	}
	if a.pendingModel != nil {
		if err := a.pendingModel.Close(); err != nil {
			return err
		}
		a.pendingModel = nil
	}
	if err := a.model.Close(); err != nil {
		return err
	}
	for _, dep := range []struct {
		value         *ast.ASTDependency
		held, pending *bool
	}{{&a.core, &a.coreHeld, &a.corePending}, {&a.leaf, &a.leafHeld, &a.leafPending}} {
		if dep.value.IsNull() {
			continue
		}
		if !*dep.held {
			if err := a.manager.DependencyInc(*dep.value); err != nil {
				return err
			}
			*dep.held = true
		}
		if *dep.pending {
			if err := a.manager.CompleteDependencyDec(*dep.value); err != nil {
				return err
			}
		} else {
			consumed, err := a.manager.DependencyDecConsumed(*dep.value)
			if consumed && err != nil {
				*dep.pending = true
			}
			if err != nil {
				return err
			}
		}
		*dep.value = ast.ASTDependency{}
	}
	for len(a.labels) > 0 {
		i := len(a.labels) - 1
		if err := a.manager.DelRef(&a.labels[i]); err != nil {
			return err
		}
		a.labels = a.labels[:i]
	}
	if a.translator != nil {
		if err := a.translator.Close(); err != nil {
			return err
		}
		a.translator = nil
	}
	if a.solver != nil {
		if err := a.solver.Close(); err != nil {
			return err
		}
		a.solver = nil
	}
	if a.runtime != nil {
		if err := a.runtime.Close(); err != nil {
			return err
		}
		a.runtime = nil
	}
	if a.dimacs != nil {
		if err := a.dimacs.Close(); err != nil {
			return err
		}
		a.dimacs = nil
	}
	t.application = nil
	return nil
}
func (t *SATTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		empty := t.context.EmptyValue()
		p = &empty
	}
	return t.context.Assign(&t.params, p)
}
func (t *SATTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	description := "(default: true) add redundant clauses (that improve unit propagation) when encoding if-then-else formulas"
	if err := out.InsertName("ite_extra", uparams.ParamBool, &description, nil, nil); err != nil {
		return err
	}
	description = "(default: false) collect also learned clauses."
	if err := out.InsertName("learned", uparams.ParamBool, &description, nil, nil); err != nil {
		return err
	}
	return sat.CollectSolverParamDescrs(out)
}
func (t *SATTactic) CollectStatistics() map[string]float64 {
	uints := make(map[string]uint32)
	values := make(map[string]float64)
	for i := 0; i < t.stats.Size(); i++ {
		k := string(t.stats.Key(i))
		if t.stats.IsUint(i) {
			uints[k] += t.stats.UintValue(i)
		} else {
			values[k] += t.stats.DoubleValue(i)
		}
	}
	for k, v := range uints {
		values[k] += float64(v)
	}
	return values
}
func (t *SATTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.stats.Reset()
	return nil
}
func (t *SATTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.active {
		enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
		if err != nil {
			return err
		}
		if enabled {
			return sat.ErrSATAssertion
		}
		return nil
	}
	return t.releaseApplication()
}
func (t *SATTactic) Reset() error                           { return t.Cleanup() }
func (t *SATTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *SATTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *SATTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if err := t.context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	other, err := NewSATTactic(t.context, &t.params, t.services, t.output)
	if other == nil {
		return nil, err
	}
	return other, err
}

func (t *SATTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	if err = g.CheckOwner(); err != nil {
		return err
	}
	m := g.Manager()
	if err = t.context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return err
	}
	if t.active {
		return sat.ErrSATAssertion
	}
	if err = t.releaseApplication(); err != nil {
		return err
	}
	a := &satTacticApplication{manager: m}
	t.application = a
	defer func() {
		t.active = false
		if cleanup := t.releaseApplication(); cleanup != nil {
			err = errors.Join(err, &SATApplicationFailure{Cause: cleanup, Owner: t, application: a})
		}
	}()
	a.runtime, err = sat.NewRuntime(sat.RuntimeConfig{Budget: t.context.Budget(), ParamsContext: t.context, Globals: t.services.Globals, Process: m.NumeralService().RuntimeOwner().ProcessOwner()})
	if err != nil {
		return err
	}
	a.solver, err = sat.NewSATSolver(sat.SolverConfig{ParamsContext: t.context, Params: t.params, RLimit: m.ResourceLimit(), Budget: t.context.Budget(), Runtime: a.runtime, TraceTags: t.services.Trace})
	if err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
		if e != nil {
			return e
		}
		proofs, e := m.ProofsEnabled()
		if e != nil {
			return e
		}
		if enabled && proofs {
			return sat.ErrSATAssertion
		}
	}
	t.active = true
	err = t.applySAT(g, out, a)
	var ex sat.SATSolverException
	var exp *sat.SATSolverException
	if err == nil || errors.As(err, &ex) || errors.As(err, &exp) {
		if e := a.solver.CollectStatistics(&t.stats); e != nil {
			return errors.Join(err, e)
		}
		if err != nil {
			return tactic.NewTacticException(err.Error())
		}
	} else {
		return err
	}
	return t.emit("sat_stats", "operator()", 198, func(w io.Writer) error { return t.stats.DisplaySMT2(w) })
}
func (t *SATTactic) applySAT(g *tactic.Goal, out *tactic.TacticResult, a *satTacticApplication) (err error) {
	m, s := a.manager, a.solver
	if err = out.Prepare(m); err != nil {
		return err
	}
	if err = tactic.FailIfProofGeneration("sat", g); err != nil {
		return err
	}
	models, err := g.ModelsEnabled()
	if err != nil {
		return err
	}
	cores, err := g.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	if err = t.emit("before_sat_solver", "operator()", 51, func(w io.Writer) error { return g.Display(w) }); err != nil {
		return err
	}
	if err = g.ElimRedundancies(); err != nil {
		return err
	}
	a.translator, err = NewGoalTranslator(m, s)
	if err != nil {
		return err
	}
	tr := a.translator
	tr.traceTags = t.services.Trace
	tr.printer = t.services.Globals
	if err = tr.BindCooperation(t.services.Cooperation); err != nil {
		return err
	}
	if err = tr.CompileGoal(g); err != nil {
		return err
	}
	interpreted, err := tr.HasInterpretedAtoms()
	if err != nil {
		return err
	}
	if err = t.traceInterpreted(tr, interpreted); err != nil {
		return err
	}
	for label := range tr.dependencies.All() {
		ref, e := m.NewRef(label)
		if e != nil {
			return e
		}
		a.labels = append(a.labels, ref)
	}
	if err = g.Reset(); err != nil {
		return err
	}
	// Source compacts allocator/hash-table capacity here. Go owns that storage;
	// no C++ allocator consolidation or hash-table capacity is reproduced.
	if err = s.AssertTacticInvariant(); err != nil {
		return err
	}
	verbose, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil {
		return err
	}
	if verbose {
		stream, e := diagnostic.BorrowProcessVerboseStream()
		if e != nil {
			return e
		}
		if e = s.DisplayStatus(stream); e != nil {
			return e
		}
		if e = stream.Flush(); e != nil {
			return e
		}
	}
	if err = t.traceDIMACS(s); err != nil {
		return err
	}
	assumptions, err := tr.AppendDependencyAssumptions(nil)
	if err != nil {
		return err
	}
	dimacs, err := s.DIMACSDisplayEnabled()
	if err != nil {
		return err
	}
	if dimacs {
		a.dimacs, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.context.Budget()})
		if err != nil {
			return err
		}
		if err = s.BindDIMACS(a.dimacs); err != nil {
			return err
		}
	}
	status, err := s.Check(assumptions)
	if err != nil {
		return err
	}
	if status == lbool.Undef && dimacs {
		text, e := a.dimacs.String()
		if e != nil {
			return e
		}
		if _, e = io.WriteString(t.output, text); e != nil {
			return e
		}
		if err = t.writeDIMACSAtoms(tr); err != nil {
			return err
		}
	}
	switch {
	case status == lbool.False:
		if cores {
			byLiteral := make(map[uint32]ast.ASTHandle)
			for dep, lit := range tr.dependencies.All() {
				byLiteral[lit] = dep
			}
			core, e := s.BorrowCore()
			if e != nil {
				return e
			}
			n, e := core.Size()
			if e != nil {
				return e
			}
			var labels []ast.ASTHandle
			for i := uint32(0); i < n; i++ {
				lit, e := core.Read(i)
				if e != nil {
					return e
				}
				dep, ok := byLiteral[lit.Index()]
				if !ok {
					return sat.ErrSATOwner
				}
				labels = append(labels, dep)
			}
			progress, e := m.DependencyJoinValues(labels)
			a.core, a.leaf = progress.Completed, progress.PendingLeaf
			if e != nil {
				return e
			}
		}
		f, e := m.MkFalse()
		if e != nil {
			return e
		}
		if err = g.AssertWithProof(f, ast.ASTHandle{}, a.core); err != nil {
			return err
		}
	case status == lbool.True && !interpreted:
		if models {
			values, e := s.AppendModel(nil)
			if e != nil {
				return e
			}
			md, e := BuildBooleanModel(tr, values)
			a.pendingModel = md
			if e != nil {
				return e
			}
			if err = a.model.Set(md); err != nil {
				return err
			}
			a.pendingModel = nil
			a.converter, err = tactic.NewStoredModel(md)
			if err != nil {
				return err
			}
			if err = out.SetModelConverter(a.converter); err != nil {
				return err
			}
			a.converter = nil
		}
	default:
		if err = s.PopToBaseLevel(); err != nil {
			return err
		}
		key := "learned"
		learned, e := s.BoolParameter(&key, false)
		if e != nil {
			return e
		}
		key = "max_memory"
		mb, e := s.UIntParameter(&key, ^uint32(0))
		if e != nil {
			return e
		}
		a.converter, err = RecoverSATGoal(tr, g, learned, umemory.MegabytesToBytes(mb), t.services)
		if a.converter != nil {
			if e := out.SetModelConverter(a.converter); e != nil {
				return errors.Join(err, e)
			}
			a.converter = nil
		}
		if err != nil {
			return err
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}
