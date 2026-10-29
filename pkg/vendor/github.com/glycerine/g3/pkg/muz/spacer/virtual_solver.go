package spacer

// Source: src/muz/spacer/spacer_virtual_solver.h and
// src/muz/spacer/spacer_virtual_solver.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrVirtualSolverOwner = errors.New("muz/spacer: invalid or closed virtual solver")

// VirtualSolverFactory gives several guarded solver views of one SMT backend.
// It owns the backend; every VirtualSolver must be closed before the factory.
type VirtualSolverFactory struct {
	backend solver.Solver
	manager *ast.ASTManager
	solvers []*VirtualSolver
	checks  uint32
	sat     uint32
	unknown uint32
	closed  bool
}

func NewVirtualSolverFactory(backend solver.Solver) (*VirtualSolverFactory, error) {
	if backend == nil {
		return nil, ErrVirtualSolverOwner
	}
	manager, err := backend.Manager()
	if err != nil || manager == nil {
		return nil, errors.Join(err, ErrVirtualSolverOwner)
	}
	return &VirtualSolverFactory{backend: backend, manager: manager}, nil
}

func (f *VirtualSolverFactory) open() error {
	if f == nil || f.closed || f.backend == nil || f.manager == nil {
		return ErrVirtualSolverOwner
	}
	return nil
}

func (f *VirtualSolverFactory) Fresh() (*VirtualSolver, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	boolean, err := f.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	name, err := f.manager.Symbols.Intern(fmt.Sprintf("vsolver#%d", len(f.solvers)))
	if err != nil {
		return nil, err
	}
	predicate, err := f.manager.MkConst(name, boolean)
	if err != nil {
		return nil, err
	}
	predicateRef, err := f.manager.NewRef(predicate)
	if err != nil {
		return nil, err
	}
	proofRef, err := f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, errors.Join(err, f.manager.DelRef(&predicateRef))
	}
	result := &VirtualSolver{Solver: f.backend, factory: f, predicate: predicateRef, proof: proofRef}
	f.solvers = append(f.solvers, result)
	return result, nil
}

func (f *VirtualSolverFactory) CollectStatistics(destination *statistics.Statistics) error {
	if err := f.open(); err != nil {
		return err
	}
	if destination == nil {
		return ast.ErrASTDomain
	}
	if err := f.backend.CollectStatistics(destination); err != nil {
		return err
	}
	destination.UpdateUint("virtual_solver.checks", f.checks)
	destination.UpdateUint("virtual_solver.checks.sat", f.sat)
	destination.UpdateUint("virtual_solver.checks.undef", f.unknown)
	return nil
}

func (f *VirtualSolverFactory) ResetStatistics() error {
	if err := f.open(); err != nil {
		return err
	}
	f.checks, f.sat, f.unknown = 0, 0, 0
	if resetter, yes := f.backend.(interface{ ResetStatistics() error }); yes {
		return resetter.ResetStatistics()
	}
	return nil
}

func (f *VirtualSolverFactory) Close() error {
	if f == nil {
		return ErrVirtualSolverOwner
	}
	if f.closed {
		return nil
	}
	for _, virtual := range f.solvers {
		if virtual != nil && !virtual.closed {
			return ErrVirtualSolverOwner
		}
	}
	if err := f.backend.Close(); err != nil {
		return err
	}
	f.closed = true
	f.backend = nil
	f.manager = nil
	f.solvers = nil
	return nil
}

// VirtualSolver is a guarded, independently scoped view of its factory's
// shared backend. The embedded methods are the source solver delegation
// surface; stateful methods below preserve the virtual view.
type VirtualSolver struct {
	solver.Solver
	factory         *VirtualSolverFactory
	predicate       ast.ASTRef
	proof           ast.ASTRef
	assertions      []ast.ASTRef
	scopeAssertions []uint32
	lastAssumptions []ast.ASTHandle
	head            uint32
	scopeLevel      uint32
	pushed          bool
	delayScope      bool
	closed          bool
}

func (s *VirtualSolver) open() error {
	if s == nil || s.closed || s.Solver == nil || s.factory == nil {
		return ErrVirtualSolverOwner
	}
	return s.factory.open()
}

func (s *VirtualSolver) predicateHandle() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.factory.manager.RefGet(s.predicate)
}

func (s *VirtualSolver) internalizeAssertions() error {
	predicate, err := s.predicateHandle()
	if err != nil {
		return err
	}
	for s.head < uint32(len(s.assertions)) {
		assertion, err := s.factory.manager.RefGet(s.assertions[s.head])
		if err != nil {
			return err
		}
		guarded, err := s.factory.manager.MkImplies(predicate, assertion)
		if err != nil {
			return err
		}
		if err = s.Solver.Assert(guarded); err != nil {
			return err
		}
		s.head++
	}
	return nil
}

func (s *VirtualSolver) Assert(expression ast.ASTHandle) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	truth, err := s.factory.manager.MkTrue()
	if err != nil || expression == truth {
		return err
	}
	if s.delayScope {
		if err = s.internalizeAssertions(); err != nil {
			return err
		}
		if err = s.Solver.Push(); err != nil {
			return err
		}
		s.pushed, s.delayScope = true, false
	}
	if s.pushed {
		return s.Solver.Assert(expression)
	}
	flattened, err := s.factory.manager.FlattenAnd([]ast.ASTHandle{expression})
	if err != nil {
		return err
	}
	for _, value := range flattened {
		reference, err := s.factory.manager.NewRef(value)
		if err != nil {
			return err
		}
		s.assertions = append(s.assertions, reference)
	}
	return nil
}

func (s *VirtualSolver) AssertTracked(expression, tracking ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	implication, err := s.factory.manager.MkImplies(tracking, expression)
	if err != nil {
		return err
	}
	return s.Assert(implication)
}

// PrepareCandidateModel forwards the optional incomplete-check model protocol
// while preserving the active virtual-solver view.
func (s *VirtualSolver) PrepareCandidateModel() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	provider, ok := s.Solver.(solver.CandidateModelProvider)
	if !ok {
		return false, nil
	}
	return provider.PrepareCandidateModel()
}

func (s *VirtualSolver) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	s.scopeAssertions = append(s.scopeAssertions, uint32(len(s.assertions)))
	s.scopeLevel++
	if s.delayScope {
		if err := s.internalizeAssertions(); err != nil {
			return err
		}
		if err := s.Solver.Push(); err != nil {
			return err
		}
		s.pushed, s.delayScope = true, false
	}
	if !s.pushed {
		s.delayScope = true
		return nil
	}
	return s.Solver.Push()
}

func (s *VirtualSolver) Pop(count uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if count > s.scopeLevel {
		return arena.ErrBounds
	}
	if count == 0 {
		return nil
	}
	if s.pushed {
		if err := s.Solver.Pop(count); err != nil {
			return err
		}
	}
	target := s.scopeLevel - count
	if !s.pushed {
		first := s.scopeAssertions[target]
		for len(s.assertions) > int(first) {
			last := len(s.assertions) - 1
			if err := s.factory.manager.DelRef(&s.assertions[last]); err != nil {
				return err
			}
			s.assertions = s.assertions[:last]
		}
		if s.head > uint32(len(s.assertions)) {
			s.head = uint32(len(s.assertions))
		}
	}
	s.scopeAssertions = s.scopeAssertions[:target]
	s.scopeLevel = target
	s.pushed = s.pushed && target != 0
	s.delayScope = !s.pushed && target != 0
	return nil
}

func (s *VirtualSolver) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.scopeLevel, nil
}

func (s *VirtualSolver) CheckSat(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if err := s.internalizeAssertions(); err != nil {
		return lbool.Undef, err
	}
	if err := s.factory.manager.SetRef(s.proof, ast.ASTHandle{}); err != nil {
		return lbool.Undef, err
	}
	predicate, err := s.predicateHandle()
	if err != nil {
		return lbool.Undef, err
	}
	values := make([]ast.ASTHandle, 0, len(assumptions)+1)
	values = append(values, predicate)
	values = append(values, assumptions...)
	s.lastAssumptions = append(s.lastAssumptions[:0], assumptions...)
	status, err := s.Solver.CheckSat(values)
	if err != nil {
		return status, err
	}
	s.factory.checks++
	if status == lbool.True {
		s.factory.sat++
	} else if status == lbool.Undef {
		s.factory.unknown++
	}
	return status, nil
}

func (s *VirtualSolver) NumAssertions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.assertions)), nil
}

func (s *VirtualSolver) Assertion(index uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= uint32(len(s.assertions)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return s.factory.manager.RefGet(s.assertions[index])
}

func (s *VirtualSolver) NumAssumptions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.lastAssumptions)), nil
}

func (s *VirtualSolver) Assumption(index uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= uint32(len(s.lastAssumptions)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return s.lastAssumptions[index], nil
}

func (s *VirtualSolver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	core, err := s.Solver.UnsatCore()
	if err != nil {
		return nil, err
	}
	predicate, err := s.predicateHandle()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, 0, len(core))
	for _, value := range core {
		if value != predicate {
			result = append(result, value)
		}
	}
	return result, nil
}

func (s *VirtualSolver) Proof() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	root, err := s.Solver.Proof()
	if err != nil || root.IsNull() {
		return root, err
	}
	predicate, err := s.predicateHandle()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = proofs.EliminateAuxAssertions(s.factory.manager, predicate, root, s.proof); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.factory.manager.RefGet(s.proof)
}

func (s *VirtualSolver) Close() (result error) {
	if s == nil || s.factory == nil {
		return ErrVirtualSolverOwner
	}
	if s.closed {
		return nil
	}
	if s.pushed {
		result = errors.Join(result, s.Solver.Pop(s.scopeLevel))
	}
	predicate, err := s.factory.manager.RefGet(s.predicate)
	if err == nil {
		var negated ast.ASTHandle
		negated, err = s.factory.manager.MkNot(predicate)
		if err == nil {
			err = s.Solver.Assert(negated)
		}
	}
	result = errors.Join(result, err)
	for index := len(s.assertions) - 1; index >= 0; index-- {
		result = errors.Join(result, s.factory.manager.DelRef(&s.assertions[index]))
	}
	result = errors.Join(result, s.factory.manager.DelRef(&s.proof), s.factory.manager.DelRef(&s.predicate))
	if result == nil {
		s.assertions = nil
		s.scopeAssertions = nil
		s.lastAssumptions = nil
		s.Solver = nil
		s.closed = true
	}
	return result
}
