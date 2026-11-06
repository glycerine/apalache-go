package portfolio

// Source: src/tactic/portfolio/enum2bv_solver.cpp/.h.

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
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Enum2BVSolver adds source enum rewriting to an incremental backend.
// Consequence handles use Solver's borrowed-result convention; this owner
// retains the published result vector until the next consequence query/Close.
type Enum2BVSolver struct {
	status                       lbool.LBool
	self                         *Enum2BVSolver
	manager                      *ast.ASTManager
	context                      *uparams.ParamsContext
	services                     model.ModelEvaluatorConfig
	parameters                   uparams.ParamsRef
	decrement                    uparams.ParamsDecrementReceipt
	backend                      solver.Solver
	backendOwned, backendCleanup bool
	named                        *solver.NamedAssertions
	rw                           *rewriter.Enum2BVRewriter
	pending                      *enum2BVCall
	consequences                 []ast.ASTRef
	refs                         uint32
	header                       uint64
	closing, closed              bool
}

var _ solver.Solver = (*Enum2BVSolver)(nil)
var _ solver.NamedAssertionCore = (*Enum2BVSolver)(nil)

type Enum2BVConstructionFailure struct {
	owner          *Enum2BVSolver
	cause, cleanup error
}

func (e *Enum2BVConstructionFailure) Error() string                 { return e.Unwrap().Error() }
func (e *Enum2BVConstructionFailure) Unwrap() error                 { return errors.Join(e.cause, e.cleanup) }
func (e *Enum2BVConstructionFailure) OriginalCause() error          { return e.cause }
func (e *Enum2BVConstructionFailure) CleanupCause() error           { return e.cleanup }
func (e *Enum2BVConstructionFailure) RuntimeOwner() *memory.Runtime { return e.owner.services.Memory }
func (e *Enum2BVConstructionFailure) Cleanup() error                { return e.owner.Close() }

func NewEnum2BVSolver(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, backend solver.Solver, services model.ModelEvaluatorConfig) (*Enum2BVSolver, error) {
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
	n := uint64(reflect.TypeOf(Enum2BVSolver{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	s := &Enum2BVSolver{status: lbool.Undef, manager: m, context: c, services: services, backend: backend, header: n}
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
		s.rw, err = rewriter.NewEnum2BVRewriter(m, c, local)
	}
	// The source constructor calls solver::updt_params, not its virtual override.
	if err == nil && p != nil {
		err = c.Copy(&s.parameters, p)
	}
	if err != nil {
		if cleanup := s.Close(); cleanup != nil {
			return nil, &Enum2BVConstructionFailure{owner: s, cause: err, cleanup: cleanup}
		}
		return nil, err
	}
	return s, nil
}
func (s *Enum2BVSolver) open() error {
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
func (s *Enum2BVSolver) IncRef() error {
	if err := s.open(); err != nil {
		return err
	}
	s.refs++
	return nil
}
func (s *Enum2BVSolver) DecRef() error {
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
func (s *Enum2BVSolver) Close() error {
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
	if err := s.clearConsequences(); err != nil {
		return err
	}
	if s.rw != nil {
		if err := s.rw.Close(); err != nil {
			return err
		}
		s.rw = nil
	}
	if s.backendOwned {
		// Under the retained valid-reference precondition, final DEC consumes
		// the backend reference before its destructor may report a Go error.
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
func (s *Enum2BVSolver) Assert(h ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.AssertFormula(h)
}
func (s *Enum2BVSolver) AssertTracked(h, tag ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.AssertTracked(h, tag)
}
func (s *Enum2BVSolver) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.Push()
}
func (s *Enum2BVSolver) Pop(n uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.named.Pop(n)
}
func (s *Enum2BVSolver) PushCore() error {
	if err := s.rw.Push(); err != nil {
		return err
	}
	return s.backend.Push()
}
func (s *Enum2BVSolver) PopCore(n uint32) error {
	if err := s.backend.Pop(n); err != nil {
		return err
	}
	return s.rw.Pop(n)
}
func (s *Enum2BVSolver) CheckSat(a []ast.ASTHandle) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.named.CheckSat(a)
}
func (s *Enum2BVSolver) CheckSatCore(a []ast.ASTHandle) (lbool.LBool, error) {
	return s.backend.CheckSat(a)
}
func (s *Enum2BVSolver) DisplayCore(w io.Writer) error { return solver.DefaultDisplay(s, w, nil) }
func (s *Enum2BVSolver) Display(w io.Writer, a []ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return solver.DefaultDisplay(s, w, a)
}
func (s *Enum2BVSolver) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.named.ScopeLevel()
}
func (s *Enum2BVSolver) NumAssumptions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.named.NumAssumptions()
}
func (s *Enum2BVSolver) Assumption(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.named.Assumption(i)
}
func (s *Enum2BVSolver) Params() (*uparams.ParamsRef, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return &s.parameters, nil
}
func (s *Enum2BVSolver) UpdateParams(p *uparams.ParamsRef) error {
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
func (s *Enum2BVSolver) PreferredSAT([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return solver.DefaultPreferredSAT(s)
}
func (s *Enum2BVSolver) Translate(m *ast.ASTManager, p *uparams.ParamsRef) (solver.Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	backend, err := s.backend.Translate(m, p)
	if err != nil {
		return nil, err
	}
	next, err := NewEnum2BVSolver(m, s.context, p, backend, s.services)
	if err != nil {
		var stopped *Enum2BVConstructionFailure
		if errors.As(err, &stopped) {
			return nil, err
		}
		return nil, errors.Join(err, backend.Close())
	}
	return next, nil
}

// Calls retain exact lexical owners on an interrupted Go cleanup.
type enum2BVCall struct {
	consequenceLocal          bool
	owner                     *Enum2BVSolver
	current, proof            ast.ASTRef
	bounds, bvars             []ast.ASTRef
	utility                   *ast.DatatypeUtil
	number                    unumeral.OwnedRational
	extension                 *tactic.ExtensionModelConverter
	filter                    *tactic.FilterModelConverter
	md                        tactic.ModelRef
	raw, output               *model.ModelCore
	rawCleanup, outputCleanup bool
}

func enum2BVReleaseModel(p **model.ModelCore, pending *bool) error {
	if *p == nil {
		return nil
	}
	if *pending {
		if err := (*p).Close(); err != nil {
			return err
		}
	} else {
		consumed, err := (*p).DecRefConsumed()
		if consumed && err != nil {
			*pending = true
		}
		if err != nil {
			return err
		}
		if !consumed {
			return tactic.ErrModelConverterOwner
		}
	}
	*p = nil
	return nil
}
func (c *enum2BVCall) close() error {
	m := c.owner.manager
	if c.consequenceLocal {
		if err := c.finishConsequence(); err != nil {
			return err
		}
	}
	if c.filter != nil {
		if err := c.filter.Close(); err != nil {
			return err
		}
		c.filter = nil
	}
	if c.extension != nil {
		if err := c.extension.Close(); err != nil {
			return err
		}
		c.extension = nil
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
	if c.number != (unumeral.OwnedRational{}) {
		if err := m.NumeralService().Del(&c.number); err != nil {
			return err
		}
	}
	if c.proof != (ast.ASTRef{}) {
		if err := m.DelRef(&c.proof); err != nil {
			return err
		}
	}
	for _, v := range [][]ast.ASTRef{c.bounds, c.bvars} {
		for i := range v {
			if v[i] != (ast.ASTRef{}) {
				if err := m.DelRef(&v[i]); err != nil {
					return err
				}
			}
		}
	}
	c.bounds, c.bvars = nil, nil
	if c.current != (ast.ASTRef{}) {
		if err := m.DelRef(&c.current); err != nil {
			return err
		}
	}
	if c.utility != nil {
		if err := c.utility.Close(); err != nil {
			return err
		}
		c.utility = nil
	}
	return nil
}
func (s *Enum2BVSolver) finishCall(c *enum2BVCall, err *error) {
	if cleanup := c.close(); cleanup != nil {
		s.pending = c
		*err = errors.Join(*err, cleanup)
	}
}
func (s *Enum2BVSolver) AssertFormula(h ast.ASTHandle) (err error) {
	c := &enum2BVCall{owner: s}
	defer s.finishCall(c, &err)
	c.current, err = s.manager.NewRef(h)
	if err != nil {
		return err
	}
	c.proof, err = s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = s.rw.Rewrite(h, c.current, c.proof); err != nil {
		return err
	}
	value, err := s.manager.RefGet(c.current)
	if err != nil {
		return err
	}
	if err = s.backend.Assert(value); err != nil {
		return err
	}
	if err = s.rw.FlushSideConstraints(&c.bounds); err != nil {
		return err
	}
	for _, r := range c.bounds {
		h, e := s.manager.RefGet(r)
		if e != nil {
			return e
		}
		if e = s.backend.Assert(h); e != nil {
			return e
		}
	}
	return nil
}
func (s *Enum2BVSolver) AcquireModel() (_ *model.ModelCore, err error) {
	if err = s.open(); err != nil {
		return nil, err
	}
	c := &enum2BVCall{owner: s}
	fail := func(cause error) (*model.ModelCore, error) { s.finishCall(c, &cause); return nil, cause }
	c.raw, err = s.backend.AcquireModel()
	if err != nil {
		return fail(err)
	}
	if c.raw == nil {
		return nil, nil
	}
	if err = c.md.Set(c.raw); err != nil {
		return fail(err)
	}
	if err = enum2BVReleaseModel(&c.raw, &c.rawCleanup); err != nil {
		return fail(err)
	}
	c.extension, err = tactic.NewExtensionModel(s.manager, s.services)
	if err != nil {
		return fail(err)
	}
	for decl, def := range s.rw.EnumDefinitions() {
		if err = c.extension.Insert(decl, def); err != nil {
			return fail(err)
		}
	}
	if err = c.extension.ApplyModel(&c.md, 0); err != nil {
		return fail(err)
	}
	if err = c.extension.Close(); err != nil {
		return fail(err)
	}
	c.extension = nil
	c.filter, err = tactic.NewFilterModel(s.manager)
	if err != nil {
		return fail(err)
	}
	for _, bv := range s.rw.EnumToBitVector() {
		if err = c.filter.Insert(bv); err != nil {
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
	// Hand off only after lexical retirement succeeds. On an error the pending
	// call continues owning the output instead of leaking an error-return value.
	out := c.output
	c.output = nil
	if err = c.close(); err != nil {
		c.output = out
		s.pending = c
		return nil, err
	}
	return out, nil
}

func (s *Enum2BVSolver) Status() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.status, nil
}

func (s *Enum2BVSolver) SetStatus(value lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.status = value
	return nil
}

func (s *Enum2BVSolver) CollectStatistics(value *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.CollectStatistics(value)
}

func (s *Enum2BVSolver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.UnsatCore()
}

func (s *Enum2BVSolver) Proof() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.backend.Proof()
}

func (s *Enum2BVSolver) ReasonUnknown() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	return s.backend.ReasonUnknown()
}

func (s *Enum2BVSolver) SetReasonUnknown(value string) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetReasonUnknown(value)
}

func (s *Enum2BVSolver) Labels() ([]usymbol.Symbol, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.backend.Labels()
}

func (s *Enum2BVSolver) Manager() (*ast.ASTManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.manager, nil
}

func (s *Enum2BVSolver) CollectParamDescrs(value *uparams.ParamDescrs) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.CollectParamDescrs(value)
}

func (s *Enum2BVSolver) SetProduceModels(value bool) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetProduceModels(value)
}

func (s *Enum2BVSolver) SetProgressCallback(value solver.ProgressCallback) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.backend.SetProgressCallback(value)
}

func (s *Enum2BVSolver) NumAssertions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.backend.NumAssertions()
}

func (s *Enum2BVSolver) Assertion(value uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.backend.Assertion(value)
}

func (s *Enum2BVSolver) FindMutexes(v []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return s.backend.FindMutexes(v)
}

func (s *Enum2BVSolver) clearConsequences() error {
	for i := range s.consequences {
		if s.consequences[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.consequences[i]); err != nil {
				return err
			}
		}
	}
	s.consequences = nil
	return nil
}
func enum2BVConstant(m *ast.ASTManager, h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 0 {
		return ast.ASTHandle{}, false, err
	}
	d, err := m.GetDecl(h)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	family, err := m.GetFamilyID(d)
	return d, family == ast.NullFamilyID, err
}
func enum2BVOperation(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	d, err := m.GetDecl(h)
	if err != nil {
		return false, err
	}
	f, err := m.GetFamilyID(d)
	if err != nil || f != 0 {
		return false, err
	}
	k, err := m.GetDeclKind(d)
	return k == op, err
}
func (s *Enum2BVSolver) GetConsequences(a, v []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	if err := s.clearConsequences(); err != nil {
		return lbool.Undef, nil, err
	}
	return s.named.WithAssumptions(a, func(asms []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) { return s.consequencesCore(asms, v) })
}
func (s *Enum2BVSolver) consequencesCore(a, v []ast.ASTHandle) (status lbool.LBool, out []ast.ASTHandle, err error) {
	m := s.manager
	c := &enum2BVCall{owner: s}
	defer s.finishCall(c, &err)
	c.utility, err = ast.NewDatatypeUtil(m)
	if err != nil {
		return lbool.Undef, nil, err
	}
	// Internalize even variables absent from the current assertions. Each
	// source iteration owns its tmp/proof until the next iteration begins.
	for _, h := range v {
		eq, e := m.MkEq(h, h)
		if e != nil {
			return lbool.Undef, nil, e
		}
		c.current, e = m.NewRef(eq)
		if e != nil {
			return lbool.Undef, nil, e
		}
		c.proof, e = m.NewRef(ast.ASTHandle{})
		if e != nil {
			return lbool.Undef, nil, e
		}
		if e = s.rw.Rewrite(eq, c.current, c.proof); e != nil {
			return lbool.Undef, nil, e
		}
		if e = m.DelRef(&c.proof); e != nil {
			return lbool.Undef, nil, e
		}
		if e = m.DelRef(&c.current); e != nil {
			return lbool.Undef, nil, e
		}
	}
	if err = s.rw.FlushSideConstraints(&c.bounds); err != nil {
		return lbool.Undef, nil, err
	}
	for _, ref := range c.bounds {
		h, e := m.RefGet(ref)
		if e != nil {
			return lbool.Undef, nil, e
		}
		if e = s.backend.Assert(h); e != nil {
			return lbool.Undef, nil, e
		}
	}
	vars := make([]ast.ASTHandle, 0, len(v))
	mapping := s.rw.EnumToBitVector()
	for _, h := range v {
		d, yes, e := enum2BVConstant(m, h)
		if e != nil {
			return lbool.Undef, nil, e
		}
		if bv, found := mapping[d]; yes && found {
			h, e = m.MkApp(bv, nil)
			if e != nil {
				return lbool.Undef, nil, e
			}
		}
		ref, e := m.NewRef(h)
		if e != nil {
			return lbool.Undef, nil, e
		}
		c.bvars = append(c.bvars, ref)
		vars = append(vars, h)
	}
	status, out, err = s.backend.GetConsequences(a, vars)
	// Publish concrete owning result roots before retiring the borrowed-query
	// arguments; subsequent conversion uses SetRef's INC-new/DEC-old order.
	for _, h := range out {
		ref, e := m.NewRef(h)
		if e != nil {
			return status, out, errors.Join(err, e)
		}
		s.consequences = append(s.consequences, ref)
	}
	if err != nil {
		return status, out, err
	}
	for i := range out {
		value, e := s.translateConsequence(c, out[i], s.consequences[i])
		if e != nil {
			return status, out, e
		}
		out[i] = value
	}
	return status, out, nil
}
func (s *Enum2BVSolver) translateConsequence(c *enum2BVCall, h ast.ASTHandle, slot ast.ASTRef) (result ast.ASTHandle, err error) {
	m := s.manager
	c.consequenceLocal = true
	c.number, err = m.NumeralService().NewValue()
	if err != nil {
		return h, err
	}
	defer func() { err = errors.Join(err, c.finishConsequence()) }()
	yes, err := enum2BVOperation(m, h, ast.ASTOpImplies)
	if err != nil {
		return h, err
	}
	if !yes {
		return h, ast.ErrASTDomain
	} // source VERIFY defined precondition
	antecedent, err := m.GetArg(h, 0)
	if err != nil {
		return h, err
	}
	b, err := m.GetArg(h, 1)
	if err != nil {
		return h, err
	}
	yes, err = enum2BVOperation(m, b, ast.ASTOpEq)
	if err != nil || !yes {
		return h, err
	}
	u, err := m.GetArg(b, 0)
	if err != nil {
		return h, err
	}
	d, yes, err := enum2BVConstant(m, u)
	if err != nil || !yes {
		return h, err
	}
	original, found := s.rw.BitVectorToEnum()[d]
	if !found {
		return h, nil
	}
	v, err := m.GetArg(b, 1)
	if err != nil {
		return h, err
	}
	var width uint32
	yes, err = m.BitVectorNumeralValue(v, &c.number, &width)
	if err != nil || !yes {
		return h, err
	}
	if diagnostic.DebugBuild {
		debug := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(debug)
		if e != nil {
			return h, e
		}
		if enabled {
			unsigned, e := m.NumeralService().IsUnsigned(&c.number)
			if e != nil {
				return h, e
			}
			if !unsigned {
				return h, diagnostic.DebugAssertionFailure(debug, "src/tactic/portfolio/enum2bv_solver.cpp", 135, "num.is_unsigned()")
			}
		}
	}
	sort, err := m.GetRange(original)
	if err != nil {
		return h, err
	}
	constructors, err := c.utility.Constructors(sort)
	if err != nil {
		return h, err
	}
	index, err := m.NumeralService().Unsigned(&c.number)
	if err != nil {
		return h, err
	}
	if uint64(index) >= uint64(len(constructors)) {
		return h, nil
	}
	x, err := m.MkApp(original, nil)
	if err != nil {
		return h, err
	}
	value, err := m.MkApp(constructors[index], nil)
	if err != nil {
		return h, err
	}
	head, err := m.MkEq(x, value)
	if err != nil {
		return h, err
	}
	c.current, err = m.NewRef(head)
	if err != nil {
		return h, err
	}
	result, err = m.MkImplies(antecedent, head)
	if err != nil {
		return h, err
	}
	// Assign the caller-result root while head is still owned by this local.
	if err = m.SetRef(slot, result); err != nil {
		return h, err
	}
	return result, nil
}

func (c *enum2BVCall) finishConsequence() error {
	if c.current != (ast.ASTRef{}) {
		if err := c.owner.manager.DelRef(&c.current); err != nil {
			return err
		}
	}
	if c.number != (unumeral.OwnedRational{}) {
		if err := c.owner.manager.NumeralService().Del(&c.number); err != nil {
			return err
		}
	}
	c.consequenceLocal = false
	return nil
}
