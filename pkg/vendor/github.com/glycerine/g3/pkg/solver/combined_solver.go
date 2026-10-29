package solver

// Source: src/solver/combined_solver.cpp; src/solver/combined_solver.h.

import (
	"errors"
	"io"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type IncrementalUnknownBehavior uint32

const (
	IncrementalReturnUnknown IncrementalUnknownBehavior = iota
	IncrementalUseFirstIfQuantifierFree
	IncrementalUseFirst
)

type CombinedSolverConfig struct {
	Params       *uparams.ParamsContext
	Globals      *uparams.GlobalParamsService
	TargetConfig func(*ast.ASTManager) (CombinedSolverConfig, error)
	RunTimed     func(uint32, *control.Reslimit, func() (lbool.LBool, error)) (lbool.LBool, bool, error)
}

type CombinedSolver struct {
	self                                      *CombinedSolver
	manager                                   *ast.ASTManager
	first, second                             Solver
	config                                    CombinedSolverConfig
	params                                    uparams.ParamsRef
	incMode, checked, useFirst, secondStarted bool
	ignoreFirst                               bool
	unknownBehavior                           IncrementalUnknownBehavior
	timeout                                   uint32
	references                                uint32
	ownerBytes                                uint64
	closed                                    bool
}

func NewCombinedSolver(first, second Solver, p *uparams.ParamsRef, config CombinedSolverConfig) (*CombinedSolver, error) {
	if first == nil || second == nil || config.Params == nil || config.Globals == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := first.Manager()
	if err != nil {
		return nil, err
	}
	m2, err := second.Manager()
	if err != nil || m2 != m {
		return nil, ast.ErrASTHandle
	}
	if err = config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	value := config.Params.EmptyValue()
	if p != nil {
		if err = config.Params.Copy(&value, p); err != nil {
			return nil, err
		}
	}
	amount := uint64(reflect.TypeOf(CombinedSolver{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		_ = config.Params.Del(&value)
		return nil, err
	}
	c := &CombinedSolver{manager: m, first: first, second: second, config: config, params: value, useFirst: true, ownerBytes: amount}
	c.self = c
	if err = first.IncRef(); err != nil {
		m.Budget().Release(amount)
		_ = config.Params.Del(&value)
		return nil, err
	}
	if err = second.IncRef(); err != nil {
		_ = first.DecRef()
		m.Budget().Release(amount)
		_ = config.Params.Del(&value)
		return nil, err
	}
	if err = c.updateLocalParams(&c.params); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *CombinedSolver) open() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return arena.ErrClosed
	}
	return c.config.Params.Check(&c.params)
}
func (c *CombinedSolver) IncRef() error {
	if err := c.open(); err != nil {
		return err
	}
	c.references++
	return nil
}
func (c *CombinedSolver) DecRef() error {
	if err := c.open(); err != nil {
		return err
	}
	if c.references == 0 {
		return arena.ErrBounds
	}
	c.references--
	if c.references == 0 {
		return c.Close()
	}
	return nil
}

func (c *CombinedSolver) updateLocalParams(p *uparams.ParamsRef) (err error) {
	v, err := NewCombinedSolverParams(c.config.Params, c.config.Globals, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, v.Close()) }()
	c.timeout, err = v.Solver2Timeout()
	if err != nil {
		return err
	}
	c.ignoreFirst, err = v.IgnoreSolver1()
	if err != nil {
		return err
	}
	n, err := v.Solver2Unknown()
	if err != nil {
		return err
	}
	if n > uint32(IncrementalUseFirst) {
		return ast.ErrASTDomain
	}
	c.unknownBehavior = IncrementalUnknownBehavior(n)
	return nil
}

func (c *CombinedSolver) initSecond() error {
	if c.secondStarted {
		return nil
	}
	n, err := c.first.NumAssertions()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		h, e := c.first.Assertion(i)
		if e != nil {
			return e
		}
		if e = c.second.Assert(h); e != nil {
			return e
		}
	}
	c.secondStarted = true
	return nil
}
func (c *CombinedSolver) switchIncremental() error {
	c.incMode = true
	return c.initSecond()
}
func (c *CombinedSolver) hasQuantifiers() (bool, error) {
	n, err := c.NumAssertions()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		h, e := c.Assertion(i)
		if e != nil {
			return false, e
		}
		yes, e := c.manager.HasQuantifiers(h)
		if e != nil || yes {
			return yes, e
		}
	}
	return false, nil
}
func (c *CombinedSolver) useFirstOnUnknown() (bool, error) {
	switch c.unknownBehavior {
	case IncrementalReturnUnknown:
		return false, nil
	case IncrementalUseFirstIfQuantifierFree:
		has, err := c.hasQuantifiers()
		return !has, err
	case IncrementalUseFirst:
		return true, nil
	default:
		return false, ast.ErrASTDomain
	}
}

func runCombinedTimed(ms uint32, limit *control.Reslimit, fn func() (lbool.LBool, error)) (status lbool.LBool, timedOut bool, err error) {
	var fired atomic.Bool
	done := make(chan struct{})
	timer := time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
		fired.Store(true)
		limit.IncCancel()
		close(done)
	})
	status, err = fn()
	if !timer.Stop() {
		<-done
	}
	if fired.Load() {
		limit.DecCancel()
		timedOut = true
	}
	return
}

func (c *CombinedSolver) selected() Solver {
	if c.useFirst {
		return c.first
	}
	return c.second
}
func (c *CombinedSolver) Status() (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.selected().Status()
}
func (c *CombinedSolver) SetStatus(v lbool.LBool) error {
	if err := c.open(); err != nil {
		return err
	}
	return errors.Join(c.first.SetStatus(v), c.second.SetStatus(v))
}
func (c *CombinedSolver) Manager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.manager, nil
}
func (c *CombinedSolver) Params() (*uparams.ParamsRef, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	v := c.config.Params.EmptyValue()
	if err := c.config.Params.Copy(&v, &c.params); err != nil {
		return nil, err
	}
	return &v, nil
}
func (c *CombinedSolver) UpdateParams(p *uparams.ParamsRef) error {
	if err := c.open(); err != nil {
		return err
	}
	if p == nil {
		return uparams.ErrParamsOwner
	}
	if err := c.config.Params.Append(&c.params, p); err != nil {
		return err
	}
	if err := c.first.UpdateParams(p); err != nil {
		return err
	}
	if err := c.second.UpdateParams(p); err != nil {
		return err
	}
	return c.updateLocalParams(p)
}
func (c *CombinedSolver) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.first.CollectParamDescrs(d); err != nil {
		return err
	}
	if err := c.second.CollectParamDescrs(d); err != nil {
		return err
	}
	return CollectCombinedSolverParamDescrs(d)
}
func (c *CombinedSolver) SetProduceModels(v bool) error {
	if err := c.open(); err != nil {
		return err
	}
	return errors.Join(c.first.SetProduceModels(v), c.second.SetProduceModels(v))
}
func (c *CombinedSolver) Assert(h ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.checked {
		if err := c.switchIncremental(); err != nil {
			return err
		}
	}
	if err := c.first.Assert(h); err != nil {
		return err
	}
	if c.secondStarted {
		return c.second.Assert(h)
	}
	return nil
}
func (c *CombinedSolver) AssertTracked(h, a ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.checked {
		if err := c.switchIncremental(); err != nil {
			return err
		}
	}
	if err := c.first.AssertTracked(h, a); err != nil {
		return err
	}
	if err := c.initSecond(); err != nil {
		return err
	}
	return c.second.AssertTracked(h, a)
}
func (c *CombinedSolver) Push() error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.switchIncremental(); err != nil {
		return err
	}
	if err := c.first.Push(); err != nil {
		return err
	}
	return c.second.Push()
}
func (c *CombinedSolver) Pop(n uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.switchIncremental(); err != nil {
		return err
	}
	if err := c.first.Pop(n); err != nil {
		return err
	}
	return c.second.Pop(n)
}
func (c *CombinedSolver) ScopeLevel() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return c.first.ScopeLevel()
}

func (c *CombinedSolver) CheckSat(a []ast.ASTHandle) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	c.checked, c.useFirst = true, false
	n, err := c.NumAssumptions()
	if err != nil {
		return lbool.Undef, err
	}
	if n != 0 || len(a) != 0 || c.ignoreFirst {
		if err = c.switchIncremental(); err != nil {
			return lbool.Undef, err
		}
		return c.second.CheckSat(a)
	}
	if c.incMode {
		if c.timeout == ^uint32(0) {
			r, e := c.second.CheckSat(a)
			if e != nil {
				return lbool.Undef, e
			}
			if r != lbool.Undef {
				return r, nil
			}
			fallback, e := c.useFirstOnUnknown()
			if e != nil {
				return lbool.Undef, e
			}
			if !fallback {
				return r, nil
			}
			canceled, ce := c.manager.Canceled()
			if ce != nil {
				return lbool.Undef, ce
			}
			if canceled {
				return r, nil
			}
		} else {
			runner := c.config.RunTimed
			if runner == nil {
				runner = runCombinedTimed
			}
			r, timed, e := runner(c.timeout, c.manager.ResourceLimit(), func() (lbool.LBool, error) { return c.second.CheckSat(a) })
			if e != nil && !timed {
				return lbool.Undef, e
			}
			if !timed {
				if r != lbool.Undef {
					return r, nil
				}
				fallback, fe := c.useFirstOnUnknown()
				if fe != nil {
					return lbool.Undef, fe
				}
				if !fallback {
					return r, nil
				}
			}
		}
	}
	c.useFirst = true
	return c.first.CheckSat(a)
}

func (c *CombinedSolver) GetConsequences(a, v []ast.ASTHandle) (status lbool.LBool, out []ast.ASTHandle, err error) {
	if err = c.open(); err != nil {
		return lbool.Undef, nil, err
	}
	if err = c.switchIncremental(); err != nil {
		return lbool.Undef, nil, err
	}
	c.useFirst = false
	status, out, err = c.second.GetConsequences(a, v)
	if err != nil {
		canceled, e := c.manager.Canceled()
		if e != nil {
			return lbool.Undef, nil, e
		}
		reason := err.Error()
		if canceled {
			reason = messages.Canceled
		}
		_ = c.SetReasonUnknown(reason)
		return lbool.Undef, nil, nil
	}
	return
}
func (c *CombinedSolver) SetProgressCallback(v ProgressCallback) error {
	if err := c.open(); err != nil {
		return err
	}
	return errors.Join(c.first.SetProgressCallback(v), c.second.SetProgressCallback(v))
}
func (c *CombinedSolver) NumAssertions() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return c.first.NumAssertions()
}
func (c *CombinedSolver) Assertion(i uint32) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.first.Assertion(i)
}
func (c *CombinedSolver) NumAssumptions() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	a, e := c.first.NumAssumptions()
	if e != nil {
		return 0, e
	}
	b, e := c.second.NumAssumptions()
	return a + b, e
}
func (c *CombinedSolver) Assumption(i uint32) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	n, e := c.first.NumAssumptions()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if i < n {
		return c.first.Assumption(i)
	}
	return c.second.Assumption(i - n)
}
func (c *CombinedSolver) FindMutexes(v []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return DefaultFindMutexes(v)
}
func (c *CombinedSolver) PreferredSAT(a []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return DefaultPreferredSAT(c)
}
func (c *CombinedSolver) Display(w io.Writer, a []ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.first.Display(w, a)
}
func (c *CombinedSolver) CollectStatistics(s *statistics.Statistics) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.second.CollectStatistics(s); err != nil {
		return err
	}
	if c.useFirst {
		return c.first.CollectStatistics(s)
	}
	return nil
}
func (c *CombinedSolver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.selected().UnsatCore()
}
func (c *CombinedSolver) AcquireModel() (*model.ModelCore, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.selected().AcquireModel()
}
func (c *CombinedSolver) Proof() (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.selected().Proof()
}
func (c *CombinedSolver) ReasonUnknown() (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	return c.selected().ReasonUnknown()
}
func (c *CombinedSolver) SetReasonUnknown(v string) error {
	if err := c.open(); err != nil {
		return err
	}
	return errors.Join(c.first.SetReasonUnknown(v), c.second.SetReasonUnknown(v))
}
func (c *CombinedSolver) Labels() ([]usymbol.Symbol, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.selected().Labels()
}

func (c *CombinedSolver) Translate(m *ast.ASTManager, p *uparams.ParamsRef) (Solver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	config := c.config
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		if config.TargetConfig == nil {
			return nil, err
		}
		var e error
		config, e = config.TargetConfig(m)
		if e != nil {
			return nil, e
		}
	}
	a, err := c.first.Translate(m, p)
	if err != nil {
		return nil, err
	}
	b, err := c.second.Translate(m, p)
	if err != nil {
		_ = a.Close()
		return nil, err
	}
	next, err := NewCombinedSolver(a, b, p, config)
	if err != nil {
		_ = a.Close()
		_ = b.Close()
		return nil, err
	}
	next.secondStarted, next.incMode, next.checked, next.useFirst = c.secondStarted, c.incMode, c.checked, c.useFirst
	return next, nil
}
func (c *CombinedSolver) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if err := c.config.Params.Del(&c.params); err != nil {
		return err
	}
	if err := c.second.DecRef(); err != nil {
		return err
	}
	if err := c.first.DecRef(); err != nil {
		return err
	}
	c.manager.Budget().Release(c.ownerBytes)
	c.ownerBytes = 0
	c.first, c.second = nil, nil
	c.closed = true
	return nil
}

type combinedSolverFactory struct {
	first, second Factory
	config        CombinedSolverConfig
	closed        bool
}

func NewCombinedSolverFactory(first, second Factory, config CombinedSolverConfig) (Factory, error) {
	if first == nil || second == nil {
		return nil, ast.ErrASTHandle
	}
	return &combinedSolverFactory{first: first, second: second, config: config}, nil
}
func (f *combinedSolverFactory) New(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (Solver, error) {
	if f == nil || f.closed {
		return nil, arena.ErrClosed
	}
	a, e := f.first.New(m, p, proofs, models, cores, logic)
	if e != nil {
		return nil, e
	}
	b, e := f.second.New(m, p, proofs, models, cores, logic)
	if e != nil {
		_ = a.Close()
		return nil, e
	}
	c, e := NewCombinedSolver(a, b, p, f.config)
	if e != nil {
		_ = a.Close()
		_ = b.Close()
	}
	return c, e
}
func (f *combinedSolverFactory) Close() error {
	if f == nil || f.closed {
		return nil
	}
	if err := f.second.Close(); err != nil {
		return err
	}
	if err := f.first.Close(); err != nil {
		return err
	}
	f.first, f.second = nil, nil
	f.closed = true
	return nil
}

var _ Solver = (*CombinedSolver)(nil)
