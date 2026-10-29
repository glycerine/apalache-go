package pdr

// Source: src/muz/pdr/pdr_smt_context_manager.h and
// src/muz/pdr/pdr_smt_context_manager.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrSMTContextOwner = errors.New("muz/pdr: invalid or closed SMT context owner")

type SMTContextManager struct {
	manager    *ast.ASTManager
	params     *uparams.ParamsRef
	config     smtcore.SMTSolverConfig
	maximum    uint32
	created    uint32
	solvers    []*smtcore.SMTSolver
	predicates []ast.ASTRef
	auxiliary  map[ast.ASTHandle]bool
	active     uint32
	closed     bool
}

func NewSMTContextManager(manager *ast.ASTManager, params *uparams.ParamsRef, maximum uint32, config smtcore.SMTSolverConfig) (*SMTContextManager, error) {
	if manager == nil || config.Params == nil {
		return nil, ErrSMTContextOwner
	}
	return &SMTContextManager{manager: manager, params: params, config: config, maximum: maximum, auxiliary: make(map[ast.ASTHandle]bool)}, nil
}

func (m *SMTContextManager) open() error {
	if m == nil || m.closed || m.manager == nil || m.config.Params == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (m *SMTContextManager) newSolver() (*smtcore.SMTSolver, error) {
	solver, err := smtcore.NewSMTSolver(m.manager, m.params, "", m.config)
	if err != nil {
		return nil, err
	}
	if err = solver.SetProduceModels(true); err != nil {
		return nil, errors.Join(err, solver.Close())
	}
	m.solvers = append(m.solvers, solver)
	return solver, nil
}

func (m *SMTContextManager) Fresh() (_ *SMTContext, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	m.created++
	var solver *smtcore.SMTSolver
	var predicate ast.ASTHandle
	if m.maximum == 0 {
		var err error
		solver, err = m.newSolver()
		if err != nil {
			return nil, err
		}
		predicate, err = m.manager.MkTrue()
		if err != nil {
			return nil, err
		}
	} else {
		if uint32(len(m.solvers)) < m.maximum {
			var err error
			solver, err = m.newSolver()
			if err != nil {
				return nil, err
			}
		} else {
			solver = m.solvers[(m.created-1)%m.maximum]
		}
		boolean, err := m.manager.MkBoolSort()
		if err != nil {
			return nil, err
		}
		name, err := m.manager.Symbols.Intern(fmt.Sprintf("#context%d", m.created))
		if err != nil {
			return nil, err
		}
		predicate, err = m.manager.MkConst(name, boolean)
		if err != nil {
			return nil, err
		}
		reference, err := m.manager.NewRef(predicate)
		if err != nil {
			return nil, err
		}
		m.predicates = append(m.predicates, reference)
		declaration, err := m.manager.GetDecl(predicate)
		if err != nil {
			return nil, err
		}
		m.auxiliary[declaration] = true
	}
	m.active++
	return &SMTContext{parent: m, solver: solver, predicate: predicate}, nil
}

func (m *SMTContextManager) IsAuxPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.auxiliary[declaration], nil
}

func (m *SMTContextManager) CollectStatistics(destination *statistics.Statistics) error {
	if err := m.open(); err != nil {
		return err
	}
	for _, solver := range m.solvers {
		if err := solver.CollectStatistics(destination); err != nil {
			return err
		}
	}
	return nil
}

func (m *SMTContextManager) ResetStatistics() error {
	if err := m.open(); err != nil {
		return err
	}
	for _, solver := range m.solvers {
		if err := solver.ResetStatistics(); err != nil {
			return err
		}
	}
	return nil
}

func (m *SMTContextManager) Close() (result error) {
	if m == nil || m.manager == nil {
		return ErrSMTContextOwner
	}
	if m.closed {
		return nil
	}
	if m.active != 0 {
		return fmt.Errorf("%w: %d contexts remain active", ErrSMTContextOwner, m.active)
	}
	for index := len(m.solvers) - 1; index >= 0; index-- {
		result = errors.Join(result, m.solvers[index].Close())
	}
	for index := len(m.predicates) - 1; index >= 0; index-- {
		result = errors.Join(result, m.manager.DelRef(&m.predicates[index]))
	}
	if result == nil {
		m.solvers = nil
		m.predicates = nil
		m.auxiliary = nil
		m.manager = nil
		m.closed = true
	}
	return result
}

type SMTContext struct {
	parent    *SMTContextManager
	solver    *smtcore.SMTSolver
	predicate ast.ASTHandle
	delayed   bool
	pushed    bool
	closed    bool
}

func (c *SMTContext) open() error {
	if c == nil || c.closed || c.parent == nil || c.solver == nil {
		return ErrSMTContextOwner
	}
	return c.parent.open()
}

func (c *SMTContext) Predicate() (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.predicate, nil
}

func (c *SMTContext) IsAuxPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.parent.IsAuxPredicate(declaration)
}

func (c *SMTContext) Assert(expression ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	truth, err := c.parent.manager.MkTrue()
	if err != nil || expression == truth {
		return err
	}
	free, err := c.parent.manager.HasFreeVars(expression)
	if err != nil {
		return err
	}
	if free {
		return fmt.Errorf("muz/pdr: SMT context assertion has free variables")
	}
	if c.delayed && !c.pushed {
		if err = c.solver.Push(); err != nil {
			return err
		}
		c.pushed = true
	}
	if c.pushed || c.predicate == truth {
		return c.solver.Assert(expression)
	}
	guarded, err := c.parent.manager.MkImplies(c.predicate, expression)
	if err != nil {
		return err
	}
	return c.solver.Assert(guarded)
}

func (c *SMTContext) Check(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	truth, err := c.parent.manager.MkTrue()
	if err != nil {
		return lbool.Undef, err
	}
	values := append([]ast.ASTHandle(nil), assumptions...)
	if c.predicate != truth {
		values = append(values, c.predicate)
	}
	return c.solver.CheckSat(values)
}

func (c *SMTContext) Model() (*model.ModelCore, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.solver.AcquireModel()
}

func (c *SMTContext) Proof() (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.solver.Proof()
}

func (c *SMTContext) UnsatCore() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.solver.UnsatCore()
}

func (c *SMTContext) Push() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.solver.Push()
}

func (c *SMTContext) Pop() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.solver.Pop(1)
}

type SMTDelayScope struct {
	context *SMTContext
	closed  bool
}

func (c *SMTContext) BeginDelay() (*SMTDelayScope, error) {
	if err := c.open(); err != nil || c.delayed || c.pushed {
		return nil, errors.Join(err, ErrSMTContextOwner)
	}
	c.delayed = true
	return &SMTDelayScope{context: c}, nil
}

func (s *SMTDelayScope) Close() error {
	if s == nil || s.closed || s.context == nil {
		return ErrSMTContextOwner
	}
	c := s.context
	if err := c.open(); err != nil || !c.delayed {
		return errors.Join(err, ErrSMTContextOwner)
	}
	var result error
	if c.pushed {
		result = c.solver.Pop(1)
		if result == nil {
			c.pushed = false
		}
	}
	if result == nil {
		c.delayed = false
		s.context = nil
		s.closed = true
	}
	return result
}

func (c *SMTContext) Close() error {
	if c == nil || c.parent == nil {
		return ErrSMTContextOwner
	}
	if c.closed {
		return nil
	}
	if c.delayed || c.pushed {
		return ErrSMTContextOwner
	}
	c.parent.active--
	c.solver = nil
	c.parent = nil
	c.closed = true
	return nil
}
