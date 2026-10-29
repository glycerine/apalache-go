package qe

// Source: src/qe/qsat.cpp qsat state, reset and alternating search.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// qsatSearch is the shared concrete engine of the QSAT/QE tactics and QMax.
// All solving uses the two actual retained SMT kernels.
type qsatSearch struct {
	self                                                     *qsatSearch
	manager                                                  *ast.ASTManager
	config                                                   Config
	mode                                                     qsatMode
	mbp                                                      *ModelBasedProjection
	forall, exists                                           *qsatKernel
	preparation                                              *qsatPreparation
	answers, assumptions, projectionVariables, freeVariables []ast.ASTRef
	current, saved                                           tactic.ModelRef
	level, rounds                                            uint32
	stats                                                    statistics.Statistics
	objective                                                ast.ASTHandle
	value                                                    *simplex.Optimum
	valueSaved                                               simplex.Optimum
	improvement                                              ast.ASTRef
	wasSat                                                   bool
	pending                                                  []*qsatSearchCall
	bytes                                                    uint64
	closed                                                   bool
}
type qsatSearchCall struct {
	owner         *qsatSearch
	refs          []ast.ASTRef
	vectors       []*[]ast.ASTRef
	evaluators    []*model.ModelEvaluator
	substitutions []*rewriter.VarSubstitution
	shifters      []*rewriter.VarShifter
	abstractors   []*ast.ExprAbstractor
	acquired      []*model.ModelCore
	modelCleanup  *model.ModelCore
	failures      []memory.ConstructionFailure
}
type qsatSearchFailure struct {
	owner          *qsatSearch
	cause, cleanup error
	constructor    bool
}

func (e *qsatSearchFailure) Error() string        { return e.Unwrap().Error() }
func (e *qsatSearchFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *qsatSearchFailure) OriginalCause() error { return e.cause }
func (e *qsatSearchFailure) CleanupCause() error  { return e.cleanup }
func (e *qsatSearchFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *qsatSearchFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}
func newQSATSearch(m *ast.ASTManager, mode qsatMode, config Config) (result *qsatSearch, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if mode > qsatMaximize {
		return nil, ast.ErrASTDomain
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(qsatSearch{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	s := &qsatSearch{manager: m, config: config, mode: mode, bytes: n}
	s.self = s
	defer func() {
		if err != nil {
			var own *qsatSearchFailure
			if !errors.As(err, &own) || own.owner != s {
				c := s.call()
				c.retain(err)
			}
			if cleanup := s.Close(); cleanup != nil {
				result = s
				err = &qsatSearchFailure{owner: s, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	s.mbp, err = NewModelBasedProjection(m, config)
	if err != nil {
		return nil, err
	}
	s.forall, err = newQSATKernel(m, config)
	if err != nil {
		return nil, err
	}
	s.exists, err = newQSATKernel(m, config)
	if err != nil {
		return nil, err
	}
	s.preparation, err = newQSATPreparation(m, config)
	if err != nil {
		return nil, err
	}
	s.improvement, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	if err = s.reset(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *qsatSearch) open() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	_, err := s.manager.ProofsEnabled()
	return err
}
func (s *qsatSearch) call() *qsatSearchCall {
	c := &qsatSearchCall{owner: s}
	s.pending = append(s.pending, c)
	return c
}
func (c *qsatSearchCall) retain(err error) {
	var f memory.ConstructionFailure
	if errors.As(err, &f) {
		c.failures = append(c.failures, f)
	}
}
func (c *qsatSearchCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *qsatSearchCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, err = c.ref(h)
	return h, err
}
func (c *qsatSearchCall) vector() *[]ast.ASTRef {
	v := new([]ast.ASTRef)
	c.vectors = append(c.vectors, v)
	return v
}
func (c *qsatSearchCall) close() error {
	for len(c.failures) > 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.evaluators) > 0 {
		i := len(c.evaluators) - 1
		if err := c.evaluators[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.evaluators = c.evaluators[:i]
	}
	for len(c.shifters) > 0 {
		i := len(c.shifters) - 1
		if err := c.shifters[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.shifters = c.shifters[:i]
	}
	for len(c.substitutions) > 0 {
		i := len(c.substitutions) - 1
		if err := c.substitutions[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.substitutions = c.substitutions[:i]
	}
	for len(c.abstractors) > 0 {
		i := len(c.abstractors) - 1
		if err := c.abstractors[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.abstractors = c.abstractors[:i]
	}
	if c.modelCleanup != nil {
		if err := c.modelCleanup.Close(); err != nil {
			return err
		}
		c.modelCleanup = nil
	}
	for len(c.acquired) > 0 {
		i := len(c.acquired) - 1
		md := c.acquired[i]
		consumed, err := md.DecRefConsumed()
		if consumed {
			c.acquired = c.acquired[:i]
			if err != nil {
				c.modelCleanup = md
			}
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTHandle
		}
	}
	for len(c.vectors) > 0 {
		i := len(c.vectors) - 1
		if err := c.owner.clearRefs(c.vectors[i]); err != nil {
			c.retain(err)
			return err
		}
		c.vectors = c.vectors[:i]
	}
	return c.owner.clearRefs(&c.refs)
}
func (s *qsatSearch) clearRefs(v *[]ast.ASTRef) error {
	for len(*v) > 0 {
		i := len(*v) - 1
		if (*v)[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&(*v)[i]); err != nil {
				return err
			}
		}
		*v = (*v)[:i]
	}
	return nil
}
func (s *qsatSearch) appendRef(v *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := s.manager.NewRef(h)
	if err == nil {
		*v = append(*v, r)
	}
	return err
}
func (s *qsatSearch) appendRefs(out *[]ast.ASTRef, in []ast.ASTRef) error {
	for _, r := range in {
		h, err := s.manager.RefGet(r)
		if err != nil {
			return err
		}
		if err = s.appendRef(out, h); err != nil {
			return err
		}
	}
	return nil
}
func (s *qsatSearch) drain() error {
	for len(s.pending) > 0 {
		i := len(s.pending) - 1
		if err := s.pending[i].close(); err != nil {
			return err
		}
		s.pending = s.pending[:i]
	}
	return nil
}
func (s *qsatSearch) finish(c *qsatSearchCall, err *error) {
	c.retain(*err)
	if cleanup := s.drain(); cleanup != nil {
		*err = &qsatSearchFailure{owner: s, cause: *err, cleanup: cleanup}
	}
}
func (s *qsatSearch) begin() (*qsatSearchCall, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if err := s.drain(); err != nil {
		return nil, err
	}
	return s.call(), nil
}
func (s *qsatSearch) kernel(level uint32) *qsatKernel {
	if level%2 == 0 {
		return s.exists
	}
	return s.forall
}
func (s *qsatSearch) reset() (err error) {
	c, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(c, &err)
	return s.resetState()
}

// Nested recursive elimination resets source members while its caller's
// lexical frames remain on pending. The outer operation owns their cleanup.
func (s *qsatSearch) resetState() (err error) {
	s.stats.Reset()
	for _, k := range []*qsatKernel{s.forall, s.exists} {
		solver, e := k.solver()
		if e != nil {
			return e
		}
		if err = solver.CollectStatistics(&s.stats); err != nil {
			return err
		}
	}
	st, err := s.preparation.abstraction.CollectStatistics()
	if err != nil {
		return err
	}
	for k, v := range st {
		s.stats.UpdateUint(statistics.Key(k), uint32(v))
	}
	s.level = 0
	if err = s.clearRefs(&s.answers); err != nil {
		return err
	}
	if err = s.clearRefs(&s.assumptions); err != nil {
		return err
	}
	if err = s.preparation.reset(); err != nil {
		return err
	}
	if err = s.current.Set(nil); err != nil {
		return err
	}
	if err = s.forall.reset(); err != nil {
		return err
	}
	if err = s.exists.reset(); err != nil {
		return err
	}
	return s.clearRefs(&s.freeVariables)
}
func (s *qsatSearch) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if err := s.drain(); err != nil {
		return err
	}
	if err := s.current.Close(); err != nil {
		return err
	}
	if err := s.saved.Close(); err != nil {
		return err
	}
	for _, v := range []*[]ast.ASTRef{&s.answers, &s.assumptions, &s.projectionVariables, &s.freeVariables} {
		if err := s.clearRefs(v); err != nil {
			return err
		}
	}
	if s.improvement != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.improvement); err != nil {
			return err
		}
	}
	if s.preparation != nil {
		if err := s.preparation.Close(); err != nil {
			return err
		}
		s.preparation = nil
	}
	if s.exists != nil {
		if err := s.exists.Close(); err != nil {
			return err
		}
		s.exists = nil
	}
	if s.forall != nil {
		if err := s.forall.Close(); err != nil {
			return err
		}
		s.forall = nil
	}
	if s.mbp != nil {
		if err := s.mbp.Close(); err != nil {
			return err
		}
		s.mbp = nil
	}
	s.manager.Budget().Release(s.bytes)
	s.bytes = 0
	s.closed = true
	return nil
}
