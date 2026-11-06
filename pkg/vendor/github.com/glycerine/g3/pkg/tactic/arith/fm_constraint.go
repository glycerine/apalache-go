package arith

// Source: src/tactic/arith/fm_tactic.cpp:285-372,496-543.
import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
	"reflect"
)

type FMConstraint struct {
	ID                   uint32
	Literals             []int32
	Variables            []uint32
	Coefficients         []big.Rat
	Constant             big.Rat
	Dependency           ast.ASTDependency
	Strict, Dead, Marked bool
	bytes                uint64
}
type FMConstraintStore struct {
	index       *FMConstraintIndex
	diagnostics *fmConstraintDiagnostics
	self        *FMConstraintStore
	manager     *ast.ASTManager
	budget      *arena.Budget
	constraints []*FMConstraint
	freeIDs     []uint32
	nextID      uint32
	closed      bool
}

func NewFMConstraintStore(m *ast.ASTManager) (*FMConstraintStore, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	s := &FMConstraintStore{manager: m, budget: m.Budget()}
	s.self = s
	return s, nil
}
func (s *FMConstraintStore) open() error {
	if s == nil || s.self != s || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (s *FMConstraintStore) next() uint32 {
	n := len(s.freeIDs)
	if n > 0 {
		id := s.freeIDs[n-1]
		s.freeIDs = s.freeIDs[:n-1]
		return id
	}
	id := s.nextID
	s.nextID++
	return id
}
func (s *FMConstraintStore) New(lits []int32, vars []uint32, coeffs []big.Rat, constant *big.Rat, strict bool, dep ast.ASTDependency) (*FMConstraint, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if len(vars) != len(coeffs) || constant == nil {
		return nil, ast.ErrASTDomain
	}
	bytes := uint64(reflect.TypeOf(FMConstraint{}).Size()) + uint64(len(lits))*4 + uint64(len(vars))*(4+uint64(reflect.TypeOf(big.Rat{}).Size()))
	if e := s.budget.Reserve(bytes); e != nil {
		return nil, e
	}
	// Acquire the dependency before consuming an id.  This keeps both the id
	// free-list and the allocation budget unchanged when acquisition fails.
	if e := s.manager.DependencyInc(dep); e != nil {
		s.budget.Release(bytes)
		return nil, e
	}
	c := &FMConstraint{ID: s.next(), Literals: append([]int32(nil), lits...), Variables: append([]uint32(nil), vars...), Constant: *new(big.Rat).Set(constant), Strict: strict, Dependency: dep, bytes: bytes}
	c.Coefficients = make([]big.Rat, len(coeffs))
	for i := range coeffs {
		if err := s.diagnostics.allocatedVariable(i, vars[i]); err != nil {
			return nil, errors.Join(err, s.Delete(c))
		}
		c.Coefficients[i].Set(&coeffs[i])
	}
	s.constraints = append(s.constraints, c)
	return c, nil
}
func (s *FMConstraintStore) Delete(c *FMConstraint) error {
	if e := s.open(); e != nil {
		return e
	}
	if c == nil || c.bytes == 0 {
		return ast.ErrASTHandle
	}
	consumed, e := s.manager.DependencyDecConsumed(c.Dependency)
	if consumed {
		c.Dependency = ast.ASTDependency{}
	}
	if e != nil {
		return e
	}
	if s.index != nil {
		s.index.Todo.Erase(c)
	}
	s.freeIDs = append(s.freeIDs, c.ID)
	s.budget.Release(c.bytes)
	c.bytes = 0
	c.Dead = true
	c.Literals = nil
	c.Variables = nil
	c.Coefficients = nil
	return nil
}
func (s *FMConstraintStore) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	for _, c := range s.constraints {
		if c.bytes != 0 {
			if e := s.Delete(c); e != nil {
				return e
			}
		}
	}
	s.constraints = nil
	s.freeIDs = nil
	s.closed = true
	return nil
}
