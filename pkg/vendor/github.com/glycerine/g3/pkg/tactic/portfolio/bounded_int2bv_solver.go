package portfolio

// Source: src/tactic/portfolio/bounded_int2bv_solver.cpp/.h.

import (
	"errors"
	"io"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// BoundedInt2BVSolver buffers and lowers bounded integer assertions at the source
// flush boundaries. Base status and parameters are independent of its backend.
type BoundedInt2BVSolver struct {
	self                         *BoundedInt2BVSolver
	manager                      *ast.ASTManager
	context                      *uparams.ParamsContext
	parameters                   uparams.ParamsRef
	decrement                    uparams.ParamsDecrementReceipt
	backend                      solver.Solver
	backendOwned, backendCleanup bool
	named                        *solver.NamedAssertions
	rw                           *arith.BV2IntRewriter
	assertions                   []ast.ASTRef
	services                     model.ModelEvaluatorConfig
	rewriteContext               *arith.BV2IntContext
	bounds                       []*arith.BoundManager
	bvDecls, intDecls            []ast.ASTHandle
	limits                       []uint32
	int2bv, bv2int               map[ast.ASTHandle]ast.ASTHandle
	offsets                      map[ast.ASTHandle]big.Rat
	consequences                 []ast.ASTRef
	pending                      *boundedInt2BVCall
	status                       lbool.LBool
	refs                         uint32
	header                       uint64
	closing, closed              bool
}

var _ solver.Solver = (*BoundedInt2BVSolver)(nil)
var _ solver.NamedAssertionCore = (*BoundedInt2BVSolver)(nil)

type BoundedInt2BVConstructionFailure struct {
	owner          *BoundedInt2BVSolver
	cause, cleanup error
}

func (e *BoundedInt2BVConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *BoundedInt2BVConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *BoundedInt2BVConstructionFailure) OriginalCause() error { return e.cause }
func (e *BoundedInt2BVConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *BoundedInt2BVConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *BoundedInt2BVConstructionFailure) Cleanup() error { return e.owner.Close() }

func NewBoundedInt2BVSolver(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, backend solver.Solver, services model.ModelEvaluatorConfig) (*BoundedInt2BVSolver, error) {
	if m == nil || c == nil || backend == nil || services.Context != c || services.Globals == nil || services.Globals.Context != c || services.Memory == nil || services.Cooperation == nil {
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
	n := uint64(reflect.TypeOf(BoundedInt2BVSolver{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	s := &BoundedInt2BVSolver{manager: m, context: c, backend: backend, status: lbool.Undef, header: n, services: services, int2bv: make(map[ast.ASTHandle]ast.ASTHandle), bv2int: make(map[ast.ASTHandle]ast.ASTHandle), offsets: make(map[ast.ASTHandle]big.Rat)}
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
		s.rewriteContext, err = arith.NewBV2IntContext(m, c, local)
		if err == nil {
			s.rw, err = arith.NewBV2IntRewriter(s.rewriteContext)
		}
	}
	if err == nil && p != nil {
		err = c.Copy(&s.parameters, p)
	}
	if err == nil {
		var bm *arith.BoundManager
		bm, err = arith.NewBoundManager(m, c, services.Globals)
		if bm != nil {
			s.bounds = append(s.bounds, bm)
		}
	}
	if err != nil {
		if cleanup := s.Close(); cleanup != nil {
			return nil, &BoundedInt2BVConstructionFailure{owner: s, cause: err, cleanup: cleanup}
		}
		return nil, err
	}
	return s, nil
}
func (s *BoundedInt2BVSolver) open() error {
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
func (s *BoundedInt2BVSolver) clearAssertions() error {
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
func (s *BoundedInt2BVSolver) Close() error {
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
	for len(s.bounds) > 0 {
		i := len(s.bounds) - 1
		if err := s.bounds[i].Close(); err != nil {
			return err
		}
		s.bounds = s.bounds[:i]
	}
	if s.rw != nil {
		if err := s.rw.Close(); err != nil {
			return err
		}
		s.rw = nil
	}
	if s.rewriteContext != nil {
		if err := s.rewriteContext.Close(); err != nil {
			return err
		}
		s.rewriteContext = nil
	}
	clear(s.offsets)
	clear(s.int2bv)
	clear(s.bv2int)
	if err := s.releaseDecls(&s.intDecls, 0); err != nil {
		return err
	}
	if err := s.releaseDecls(&s.bvDecls, 0); err != nil {
		return err
	}
	if err := s.clearConsequences(); err != nil {
		return err
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
func (s *BoundedInt2BVSolver) AssertFormula(h ast.ASTHandle) error {
	i := len(s.assertions)
	if err := s.appendAssertion(h); err != nil {
		return err
	}
	for i < len(s.assertions) {
		h, err := s.manager.RefGet(s.assertions[i])
		if err != nil {
			return err
		}
		yes, err := enum2BVOperation(s.manager, h, ast.ASTOpAnd)
		if err != nil {
			return err
		}
		if !yes {
			i++
			continue
		}
		n, err := s.manager.GetNumArgs(h)
		if err != nil {
			return err
		}
		for j := uint32(0); j < n; j++ {
			a, err := s.manager.GetArg(h, j)
			if err != nil {
				return err
			}
			if err = s.appendAssertion(a); err != nil {
				return err
			}
		}
		last := len(s.assertions) - 1
		a, err := s.manager.RefGet(s.assertions[last])
		if err != nil {
			return err
		}
		if err = s.manager.SetRef(s.assertions[i], a); err != nil {
			return err
		}
		if err = s.manager.DelRef(&s.assertions[last]); err != nil {
			return err
		}
		s.assertions = s.assertions[:last]
	}
	return nil
}
func (s *BoundedInt2BVSolver) PushCore() error {
	if err := s.flushAssertions(); err != nil {
		return err
	}
	if err := s.backend.Push(); err != nil {
		return err
	}
	s.limits = append(s.limits, uint32(len(s.bvDecls)))
	bm, err := arith.NewBoundManager(s.manager, s.context, s.services.Globals)
	if bm != nil {
		s.bounds = append(s.bounds, bm)
	}
	return err
}
func (s *BoundedInt2BVSolver) PopCore(n uint32) error {
	if err := s.clearAssertions(); err != nil {
		return err
	}
	if err := s.backend.Pop(n); err != nil {
		return err
	}
	if n > 0 {
		if n > uint32(len(s.limits)) {
			return arena.ErrBounds
		}
		size := len(s.limits) - int(n)
		lim := s.limits[size]
		for i := len(s.intDecls) - 1; i >= int(lim); i-- {
			delete(s.int2bv, s.intDecls[i])
			delete(s.bv2int, s.bvDecls[i])
			delete(s.offsets, s.bvDecls[i])
		}
		s.limits = s.limits[:size]
		if err := s.releaseDecls(&s.bvDecls, lim); err != nil {
			s.closing = true
			return err
		}
		if err := s.releaseDecls(&s.intDecls, lim); err != nil {
			s.closing = true
			return err
		}
	}
	for ; n > 0; n-- {
		i := len(s.bounds) - 1
		if err := s.bounds[i].Close(); err != nil {
			s.closing = true
			return err
		}
		s.bounds = s.bounds[:i]
	}
	return nil
}
func (s *BoundedInt2BVSolver) CheckSatCore(a []ast.ASTHandle) (lbool.LBool, error) {
	if err := s.flushAssertions(); err != nil {
		return lbool.Undef, err
	}
	return s.backend.CheckSat(a)
}
func (s *BoundedInt2BVSolver) NumAssertions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if err := s.flushAssertions(); err != nil {
		return 0, err
	}
	return s.backend.NumAssertions()
}
func (s *BoundedInt2BVSolver) Assertion(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := s.flushAssertions(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.backend.Assertion(i)
}
func (s *BoundedInt2BVSolver) GetConsequences(a, v []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	if err := s.clearConsequences(); err != nil {
		return lbool.Undef, nil, err
	}
	return s.named.WithAssumptions(a, func(asms []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) { return s.consequencesCore(asms, v) })
}
func (s *BoundedInt2BVSolver) Translate(m *ast.ASTManager, p *uparams.ParamsRef) (solver.Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	// Source translates only the underlying solver, without flushing or
	// copying this wrapper's pending assertion buffer.
	backend, err := s.backend.Translate(m, p)
	if err != nil {
		return nil, err
	}
	next, err := NewBoundedInt2BVSolver(m, s.context, p, backend, s.services)
	if err != nil {
		var stopped *BoundedInt2BVConstructionFailure
		if errors.As(err, &stopped) {
			return nil, err
		}
		return nil, errors.Join(err, backend.Close())
	}
	return next, nil
}

func (s *BoundedInt2BVSolver) IncRef() error {
	if err := s.open(); err != nil {
		return err
	}
	s.refs++
	return nil
}

func (s *BoundedInt2BVSolver) DecRef() error {
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

func (s *BoundedInt2BVSolver) Assert(h ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.AssertFormula(h)
}

func (s *BoundedInt2BVSolver) AssertTracked(h, tag ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.AssertTracked(h, tag)
}

func (s *BoundedInt2BVSolver) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.Push()
}

func (s *BoundedInt2BVSolver) Pop(n uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.Pop(n)
}

func (s *BoundedInt2BVSolver) CheckSat(a []ast.ASTHandle) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.named.CheckSat(a)
}

func (s *BoundedInt2BVSolver) DisplayCore(w io.Writer) error { return solver.DefaultDisplay(s, w, nil) }

func (s *BoundedInt2BVSolver) Display(w io.Writer, a []ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return solver.DefaultDisplay(s, w, a)
}

func (s *BoundedInt2BVSolver) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.named.ScopeLevel()
}

func (s *BoundedInt2BVSolver) NumAssumptions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.named.NumAssumptions()
}

func (s *BoundedInt2BVSolver) Assumption(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.named.Assumption(i)
}

func (s *BoundedInt2BVSolver) Params() (*uparams.ParamsRef, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return &s.parameters, nil
}

func (s *BoundedInt2BVSolver) UpdateParams(p *uparams.ParamsRef) error {
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

func (s *BoundedInt2BVSolver) PreferredSAT([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return solver.DefaultPreferredSAT(s)
}

func (s *BoundedInt2BVSolver) Status() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.status, nil
}

func (s *BoundedInt2BVSolver) SetStatus(value lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.status = value
	return nil
}

func (s *BoundedInt2BVSolver) CollectParamDescrs(value *uparams.ParamDescrs) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.CollectParamDescrs(value)
}

func (s *BoundedInt2BVSolver) SetProduceModels(value bool) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetProduceModels(value)
}

func (s *BoundedInt2BVSolver) SetProgressCallback(value solver.ProgressCallback) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetProgressCallback(value)
}

func (s *BoundedInt2BVSolver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.UnsatCore()
}

func (s *BoundedInt2BVSolver) Proof() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.backend.Proof()
}

func (s *BoundedInt2BVSolver) ReasonUnknown() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	return s.backend.ReasonUnknown()
}

func (s *BoundedInt2BVSolver) SetReasonUnknown(value string) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetReasonUnknown(value)
}

func (s *BoundedInt2BVSolver) Labels() ([]usymbol.Symbol, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.Labels()
}

func (s *BoundedInt2BVSolver) Manager() (*ast.ASTManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.manager, nil
}

func (s *BoundedInt2BVSolver) FindMutexes(v []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return s.backend.FindMutexes(v)
}

func (s *BoundedInt2BVSolver) CollectStatistics(out *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.CollectStatistics(out)
}
