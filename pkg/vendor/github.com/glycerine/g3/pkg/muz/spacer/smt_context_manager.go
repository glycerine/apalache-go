package spacer

// Source: src/muz/spacer/spacer_smt_context_manager.h and
// src/muz/spacer/spacer_smt_context_manager.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
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
	dump      VirtualSolverDumpConfig
	closed    bool
}

func NewSMTContextManager(manager *ast.ASTManager, params *uparams.ParamsRef, maximum uint32, config smtcore.SMTSolverConfig) (*SMTContextManager, error) {
	if manager == nil || config.Params == nil || config.Globals == nil {
		return nil, ErrSMTContextManagerOwner
	}
	result := &SMTContextManager{manager: manager, params: params, config: config, maximum: maximum}
	dump, err := result.readDumpConfig(params)
	if err != nil {
		return nil, err
	}
	result.dump = dump
	return result, nil
}

func (m *SMTContextManager) readDumpConfig(params *uparams.ParamsRef) (_ VirtualSolverDumpConfig, resultErr error) {
	view, err := newFixedpointParamView(params, m.config.Params, m.config.Globals)
	if err != nil {
		return VirtualSolverDumpConfig{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	enabled, err := view.Bool("spacer.vs.dump_benchmarks", false)
	if err != nil {
		return VirtualSolverDumpConfig{}, err
	}
	minimum, err := view.Double("spacer.vs.dump_min_time", 5.0)
	if err != nil {
		return VirtualSolverDumpConfig{}, err
	}
	recheck, err := view.Bool("spacer.vs.recheck", false)
	if err != nil {
		return VirtualSolverDumpConfig{}, err
	}
	result := VirtualSolverDumpConfig{Enabled: enabled, MinSeconds: minimum, Recheck: recheck}
	result.recheck = m.recheck
	return result, result.validate()
}

func (m *SMTContextManager) recheck(assertions, assumptions []ast.ASTHandle) (status lbool.LBool, resultErr error) {
	noProof, err := ast.NewScopedNoProof(m.manager)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, noProof.Close()) }()
	backend, err := smtcore.NewSMTSolver(m.manager, m.params, "", m.config)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, backend.Close()) }()
	for _, assertion := range assertions {
		if err = backend.Assert(assertion); err != nil {
			return lbool.Undef, err
		}
	}
	return backend.CheckSat(assumptions)
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
	factory, err := NewVirtualSolverFactoryWithDump(backend, m.dump)
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
	dump, err := m.readDumpConfig(params)
	if err != nil {
		return err
	}
	m.dump = dump
	for _, factory := range m.factories {
		if err := factory.backend.UpdateParams(params); err != nil {
			return err
		}
		if err := factory.UpdateDumpConfig(dump); err != nil {
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
