package spacer

// Source: src/muz/spacer/spacer_smt_context_manager.h and
// src/muz/spacer/spacer_smt_context_manager.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrSMTContextManagerOwner = errors.New("muz/spacer: invalid or closed SMT context manager")

// SMTContextManager owns a bounded round-robin pool of shared virtual-solver
// factories. A maximum of zero preserves the source's unbounded behavior.
type SMTContextManager struct {
	manager   *ast.ASTManager
	params    *uparams.ParamsRef
	config    smtcore.SMTSolverConfig
	maximum   uint32
	created   uint32
	factories []*VirtualSolverFactory
	closed    bool
}

func NewSMTContextManager(manager *ast.ASTManager, params *uparams.ParamsRef, maximum uint32, config smtcore.SMTSolverConfig) (*SMTContextManager, error) {
	if manager == nil || config.Params == nil || config.Globals == nil {
		return nil, ErrSMTContextManagerOwner
	}
	return &SMTContextManager{manager: manager, params: params, config: config, maximum: maximum}, nil
}

func (m *SMTContextManager) open() error {
	if m == nil || m.closed || m.manager == nil || m.config.Params == nil || m.config.Globals == nil {
		return ErrSMTContextManagerOwner
	}
	return nil
}

func (m *SMTContextManager) newFactory() (*VirtualSolverFactory, error) {
	backend, err := smtcore.NewSMTSolver(m.manager, m.params, "", m.config)
	if err != nil {
		return nil, err
	}
	factory, err := NewVirtualSolverFactory(backend)
	if err != nil {
		return nil, errors.Join(err, backend.Close())
	}
	m.factories = append(m.factories, factory)
	return factory, nil
}

func (m *SMTContextManager) Fresh() (*VirtualSolver, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	m.created++
	var factory *VirtualSolverFactory
	if m.maximum == 0 || uint32(len(m.factories)) < m.maximum {
		var err error
		factory, err = m.newFactory()
		if err != nil {
			return nil, err
		}
	} else {
		factory = m.factories[(m.created-1)%m.maximum]
	}
	return factory.Fresh()
}

func (m *SMTContextManager) UpdateParams(params *uparams.ParamsRef) error {
	if err := m.open(); err != nil {
		return err
	}
	m.params = params
	for _, factory := range m.factories {
		if err := factory.backend.UpdateParams(params); err != nil {
			return err
		}
	}
	return nil
}

func (m *SMTContextManager) CollectStatistics(destination *statistics.Statistics) error {
	if err := m.open(); err != nil {
		return err
	}
	for _, factory := range m.factories {
		if err := factory.CollectStatistics(destination); err != nil {
			return err
		}
	}
	return nil
}

func (m *SMTContextManager) ResetStatistics() error {
	if err := m.open(); err != nil {
		return err
	}
	for _, factory := range m.factories {
		if err := factory.ResetStatistics(); err != nil {
			return err
		}
	}
	return nil
}

func (m *SMTContextManager) Close() (result error) {
	if m == nil {
		return ErrSMTContextManagerOwner
	}
	if m.closed {
		return nil
	}
	for _, factory := range m.factories {
		for _, virtual := range factory.solvers {
			if virtual != nil && !virtual.closed {
				return ErrSMTContextManagerOwner
			}
		}
	}
	for index := len(m.factories) - 1; index >= 0; index-- {
		if err := m.factories[index].Close(); err != nil {
			return errors.Join(result, err)
		}
	}
	m.factories = nil
	m.manager = nil
	m.params = nil
	m.closed = true
	return result
}
