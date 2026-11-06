package portfolio

// Source: src/tactic/portfolio/{bounded_int2bv,pb2bv,enum2bv}_solver.cpp
// translate methods and the concrete fd_solver wrapper chain.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	incsat "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateFiniteDomain copies the concrete finite-domain wrapper chain into
// target services. Like the source wrappers, it translates the underlying
// solver without flushing or copying pending wrapper assertions.
func (s *BoundedInt2BVSolver) TranslateFiniteDomain(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (solver.Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	source, ok := s.backend.(*PB2BVSolver)
	if !ok {
		return nil, fmt.Errorf("portfolio: unsupported finite-domain pseudo-Boolean backend")
	}
	backend, err := source.translateFiniteDomain(m, p, services)
	if err != nil {
		return nil, err
	}
	next, err := NewBoundedInt2BVSolver(m, services.Context, p, backend, services)
	if err != nil {
		var stopped *BoundedInt2BVConstructionFailure
		if errors.As(err, &stopped) {
			return nil, err
		}
		return nil, errors.Join(err, backend.Close())
	}
	return next, nil
}
func (s *PB2BVSolver) translateFiniteDomain(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (solver.Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	source, ok := s.backend.(*Enum2BVSolver)
	if !ok {
		return nil, fmt.Errorf("portfolio: unsupported finite-domain enumeration backend")
	}
	backend, err := source.translateFiniteDomain(m, p, services)
	if err != nil {
		return nil, err
	}
	next, err := NewPB2BVSolver(m, services.Context, p, backend)
	if err != nil {
		var stopped *PB2BVConstructionFailure
		if errors.As(err, &stopped) {
			return nil, err
		}
		return nil, errors.Join(err, backend.Close())
	}
	return next, nil
}
func (s *Enum2BVSolver) translateFiniteDomain(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (solver.Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	source, ok := s.backend.(*incsat.Solver)
	if !ok {
		return nil, fmt.Errorf("portfolio: unsupported finite-domain SAT backend")
	}
	backend, err := source.TranslateWithServices(m, p, services)
	if err != nil {
		return nil, err
	}
	next, err := NewEnum2BVSolver(m, services.Context, p, backend, services)
	if err != nil {
		var stopped *Enum2BVConstructionFailure
		if errors.As(err, &stopped) {
			return nil, err
		}
		return nil, errors.Join(err, backend.Close())
	}
	return next, nil
}
