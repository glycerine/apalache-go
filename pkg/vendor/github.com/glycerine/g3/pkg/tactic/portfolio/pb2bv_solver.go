package portfolio

// Source: src/tactic/portfolio/pb2bv_solver.cpp/.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// PB2BVSolver buffers assertions and compiles pseudo-Booleans at the source
// flush boundaries. Base status and parameters are independent of its backend.
type PB2BVSolver struct {
	self                         *PB2BVSolver
	manager                      *ast.ASTManager
	context                      *uparams.ParamsContext
	parameters                   uparams.ParamsRef
	decrement                    uparams.ParamsDecrementReceipt
	backend                      solver.Solver
	backendOwned, backendCleanup bool
	named                        *solver.NamedAssertions
	rw                           *rewriter.PB2BVRewriter
	assertions                   []ast.ASTRef
	pending                      *pb2BVCall
	status                       lbool.LBool
	refs                         uint32
	header                       uint64
	closing, closed              bool
}

var _ solver.Solver = (*PB2BVSolver)(nil)
var _ solver.NamedAssertionCore = (*PB2BVSolver)(nil)

type PB2BVConstructionFailure struct {
	owner          *PB2BVSolver
	cause, cleanup error
}

func (e *PB2BVConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *PB2BVConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *PB2BVConstructionFailure) OriginalCause() error { return e.cause }
func (e *PB2BVConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *PB2BVConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *PB2BVConstructionFailure) Cleanup() error { return e.owner.Close() }

func NewPB2BVSolver(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, backend solver.Solver) (*PB2BVSolver, error) {
	if m == nil || c == nil || backend == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bm, err := backend.Manager()
	if err != nil {
		return nil, err
	}
	if bm != m {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(PB2BVSolver{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	s := &PB2BVSolver{manager: m, context: c, backend: backend, status: lbool.Undef, header: n}
	s.self = s
	s.parameters, err = c.NewValue()
	if err == nil {
		s.named, err = solver.NewNamedAssertions(m, s)
	}
	if err == nil {
		err = backend.IncRef()
		s.backendOwned = err == nil
	}
	if err == nil {
		local := p
		if local == nil {
			local = &s.parameters
		}
		s.rw, err = rewriter.NewPB2BVRewriter(m, c, local)
	}
	if err == nil && p != nil {
		err = c.Copy(&s.parameters, p)
	}
	if err != nil {
		if cleanup := s.Close(); cleanup != nil {
			return nil, &PB2BVConstructionFailure{owner: s, cause: err, cleanup: cleanup}
		}
		return nil, err
	}
	return s, nil
}
func (s *PB2BVSolver) open() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	if s.closing || s.pending != nil {
		return ast.ErrASTPending
	}
	_, err := s.manager.ProofsEnabled()
	return err
}
func (s *PB2BVSolver) clearAssertions() error {
	for i := range s.assertions {
		if s.assertions[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.assertions[i]); err != nil {
				s.closing = true
				return err
			}
		}
	}
	s.assertions = nil
	return nil
}
func (s *PB2BVSolver) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	s.closing = true
	if s.pending != nil {
		if err := s.pending.close(); err != nil {
			return err
		}
		s.pending = nil
	}
	if s.rw != nil {
		if err := s.rw.Close(); err != nil {
			return err
		}
		s.rw = nil
	}
	if s.backendOwned {
		s.backendOwned = false
		s.backendCleanup = true
		if err := s.backend.DecRef(); err != nil {
			return err
		}
		s.backendCleanup = false
	}
	if s.backendCleanup {
		if err := s.backend.Close(); err != nil {
			return err
		}
		s.backendCleanup = false
	}
	s.backend = nil
	if err := s.clearAssertions(); err != nil {
		return err
	}
	if s.named != nil {
		if err := s.named.Close(); err != nil {
			return err
		}
		s.named = nil
	}
	if s.parameters != (uparams.ParamsRef{}) {
		var err error
		if s.decrement.Invocation() != 0 {
			err = s.context.CompleteDecrement(&s.parameters, s.decrement)
		} else {
			err = s.context.DelTracked(&s.parameters, &s.decrement)
		}
		if err != nil {
			return err
		}
	}
	s.manager.Budget().Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func (s *PB2BVSolver) AssertFormula(h ast.ASTHandle) error {
	ref, err := s.manager.NewRef(h)
	if err != nil {
		return err
	}
	s.assertions = append(s.assertions, ref)
	return nil
}
func (s *PB2BVSolver) PushCore() error {
	if err := s.flushAssertions(); err != nil {
		return err
	}
	if err := s.rw.Push(); err != nil {
		return err
	}
	return s.backend.Push()
}
func (s *PB2BVSolver) PopCore(n uint32) error {
	if err := s.clearAssertions(); err != nil {
		return err
	}
	if err := s.backend.Pop(n); err != nil {
		return err
	}
	return s.rw.Pop(n)
}
func (s *PB2BVSolver) CheckSatCore(a []ast.ASTHandle) (lbool.LBool, error) {
	if err := s.flushAssertions(); err != nil {
		return lbool.Undef, err
	}
	return s.backend.CheckSat(a)
}
func (s *PB2BVSolver) NumAssertions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if err := s.flushAssertions(); err != nil {
		return 0, err
	}
	return s.backend.NumAssertions()
}
func (s *PB2BVSolver) Assertion(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := s.flushAssertions(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.backend.Assertion(i)
}
func (s *PB2BVSolver) GetConsequences(a, v []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return s.named.WithAssumptions(a, func(asms []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
		if err := s.flushAssertions(); err != nil {
			return lbool.Undef, nil, err
		}
		return s.backend.GetConsequences(asms, v)
	})
}
func (s *PB2BVSolver) CollectStatistics(out *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	stats, err := s.rw.CollectStatistics()
	if err != nil {
		return err
	}
	out.UpdateUint("pb-aux-variables", stats.AuxiliaryVariables)
	out.UpdateUint("pb-aux-clauses", stats.AuxiliaryClauses)
	return s.backend.CollectStatistics(out)
}
func (s *PB2BVSolver) Translate(m *ast.ASTManager, p *uparams.ParamsRef) (solver.Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	// Source translates only the underlying solver, without flushing or
	// copying this wrapper's pending assertion buffer.
	backend, err := s.backend.Translate(m, p)
	if err != nil {
		return nil, err
	}
	next, err := NewPB2BVSolver(m, s.context, p, backend)
	if err != nil {
		var stopped *PB2BVConstructionFailure
		if errors.As(err, &stopped) {
			return nil, err
		}
		return nil, errors.Join(err, backend.Close())
	}
	return next, nil
}

type pb2BVCall struct {
	owner                     *PB2BVSolver
	proof, formula            ast.ASTRef
	clauses                   []ast.ASTHandle // actual owning references transferred by the rewriter
	filter                    *tactic.FilterModelConverter
	md                        tactic.ModelRef
	raw, output               *model.ModelCore
	rawCleanup, outputCleanup bool
}

func (c *pb2BVCall) close() error {
	m := c.owner.manager
	if c.filter != nil {
		if err := c.filter.Close(); err != nil {
			return err
		}
		c.filter = nil
	}
	if err := c.md.Close(); err != nil {
		return err
	}
	if err := enum2BVReleaseModel(&c.raw, &c.rawCleanup); err != nil {
		return err
	}
	if err := enum2BVReleaseModel(&c.output, &c.outputCleanup); err != nil {
		return err
	}
	for i, h := range c.clauses {
		if h.IsNull() {
			continue
		}
		consumed, err := m.DecRefConsumed(h)
		if consumed {
			c.clauses[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	c.clauses = nil
	for _, ref := range []*ast.ASTRef{&c.formula, &c.proof} {
		if *ref != (ast.ASTRef{}) {
			if err := m.DelRef(ref); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *PB2BVSolver) finishCall(c *pb2BVCall, err *error) {
	if cleanup := c.close(); cleanup != nil {
		s.pending = c
		*err = errors.Join(*err, cleanup)
	}
}
func (s *PB2BVSolver) flushAssertions() (err error) {
	c := &pb2BVCall{owner: s}
	defer s.finishCall(c, &err)
	c.proof, err = s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c.formula, err = s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	for _, ref := range s.assertions {
		h, e := s.manager.RefGet(ref)
		if e != nil {
			return e
		}
		if e = s.rw.Rewrite(h, c.formula, c.proof); e != nil {
			return e
		}
		h, e = s.manager.RefGet(c.formula)
		if e != nil {
			return e
		}
		if e = s.backend.Assert(h); e != nil {
			return e
		}
	}
	c.clauses, err = s.rw.FlushSideConstraints()
	if err != nil {
		return err
	}
	for _, h := range c.clauses {
		if err = s.backend.Assert(h); err != nil {
			return err
		}
	}
	return s.clearAssertions()
}
func (s *PB2BVSolver) AcquireModel() (_ *model.ModelCore, err error) {
	if err = s.open(); err != nil {
		return nil, err
	}
	c := &pb2BVCall{owner: s}
	fail := func(cause error) (*model.ModelCore, error) { s.finishCall(c, &cause); return nil, cause }
	c.raw, err = s.backend.AcquireModel()
	if err != nil {
		return fail(err)
	}
	if c.raw == nil {
		return nil, nil
	}
	fresh, err := s.rw.FreshConstants()
	if err != nil {
		return fail(err)
	}
	if len(fresh) == 0 {
		out := c.raw
		c.raw = nil
		return out, nil
	}
	if err = c.md.Set(c.raw); err != nil {
		return fail(err)
	}
	if err = enum2BVReleaseModel(&c.raw, &c.rawCleanup); err != nil {
		return fail(err)
	}
	c.filter, err = tactic.NewFilterModel(s.manager)
	if err != nil {
		return fail(err)
	}
	for _, d := range fresh {
		if err = c.filter.Insert(d); err != nil {
			return fail(err)
		}
	}
	if err = c.filter.ApplyModel(&c.md, 0); err != nil {
		return fail(err)
	}
	c.output, err = c.md.Get()
	if err != nil {
		return fail(err)
	}
	if c.output != nil {
		if err = c.output.IncRef(); err != nil {
			c.output = nil
			return fail(err)
		}
	}
	out := c.output
	c.output = nil
	if err = c.close(); err != nil {
		c.output = out
		s.pending = c
		return nil, err
	}
	return out, nil
}

func (s *PB2BVSolver) IncRef() error {
	if err := s.open(); err != nil {
		return err
	}
	s.refs++
	return nil
}

func (s *PB2BVSolver) DecRef() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.refs == 0 {
		return arena.ErrBounds
	}
	s.refs--
	if s.refs == 0 {
		return s.Close()
	}
	return nil
}

func (s *PB2BVSolver) Assert(h ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.AssertFormula(h)
}

func (s *PB2BVSolver) AssertTracked(h, tag ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.AssertTracked(h, tag)
}

func (s *PB2BVSolver) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.Push()
}

func (s *PB2BVSolver) Pop(n uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.Pop(n)
}

func (s *PB2BVSolver) CheckSat(a []ast.ASTHandle) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.named.CheckSat(a)
}

func (s *PB2BVSolver) DisplayCore(w io.Writer) error { return solver.DefaultDisplay(s, w, nil) }

func (s *PB2BVSolver) Display(w io.Writer, a []ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return solver.DefaultDisplay(s, w, a)
}

func (s *PB2BVSolver) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.named.ScopeLevel()
}

func (s *PB2BVSolver) NumAssumptions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.named.NumAssumptions()
}

func (s *PB2BVSolver) Assumption(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.named.Assumption(i)
}

func (s *PB2BVSolver) Params() (*uparams.ParamsRef, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return &s.parameters, nil
}

func (s *PB2BVSolver) UpdateParams(p *uparams.ParamsRef) error {
	if err := s.open(); err != nil {
		return err
	}
	if p == nil {
		empty := s.context.EmptyValue()
		p = &empty
	}
	if err := s.context.Copy(&s.parameters, p); err != nil {
		return err
	}
	return s.backend.UpdateParams(p)
}

func (s *PB2BVSolver) PreferredSAT([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return solver.DefaultPreferredSAT(s)
}

func (s *PB2BVSolver) Status() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.status, nil
}

func (s *PB2BVSolver) SetStatus(value lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.status = value
	return nil
}

func (s *PB2BVSolver) CollectParamDescrs(value *uparams.ParamDescrs) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.CollectParamDescrs(value)
}

func (s *PB2BVSolver) SetProduceModels(value bool) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetProduceModels(value)
}

func (s *PB2BVSolver) SetProgressCallback(value solver.ProgressCallback) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetProgressCallback(value)
}

func (s *PB2BVSolver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.UnsatCore()
}

func (s *PB2BVSolver) Proof() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.backend.Proof()
}

func (s *PB2BVSolver) ReasonUnknown() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	return s.backend.ReasonUnknown()
}

func (s *PB2BVSolver) SetReasonUnknown(value string) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetReasonUnknown(value)
}

func (s *PB2BVSolver) Labels() ([]usymbol.Symbol, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.Labels()
}

func (s *PB2BVSolver) Manager() (*ast.ASTManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.manager, nil
}

func (s *PB2BVSolver) FindMutexes(v []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return s.backend.FindMutexes(v)
}
