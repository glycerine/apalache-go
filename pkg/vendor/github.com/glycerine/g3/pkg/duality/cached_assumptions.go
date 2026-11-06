package duality

// Source: duality_rpfp.cpp:1006-1008,1018-1090,1247-1258,2787-2795.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// CachedAssumptions owns the active literal stack for a borrowed dedicated
// solver scope. Push/Pop change active assumptions, not the solver's guards.
type CachedAssumptions struct {
	scope    *EdgeSolverScope
	literals []*Expr
	limits   []int
	closed   bool
}

func NewCachedAssumptions(scope *EdgeSolverScope) (*CachedAssumptions, error) {
	if scope == nil || scope.Solver == nil || scope.Assumptions == nil {
		return nil, ErrWrapperOwner
	}
	return &CachedAssumptions{scope: scope}, nil
}
func (c *CachedAssumptions) open() error {
	if c == nil || c.closed || c.scope == nil || c.scope.Solver == nil || c.scope.Assumptions == nil {
		return ErrWrapperOwner
	}
	return nil
}
func (c *CachedAssumptions) Assert(formula *Expr) error {
	if err := c.open(); err != nil {
		return err
	}
	literals, err := c.scope.Assumptions.GetAssumptionLits(formula, nil)
	if err != nil {
		return err
	}
	c.literals = append(c.literals, literals...)
	return nil
}
func (c *CachedAssumptions) Push() error {
	if err := c.open(); err != nil {
		return err
	}
	c.limits = append(c.limits, len(c.literals))
	return nil
}
func (c *CachedAssumptions) Pop(count int) error {
	if err := c.open(); err != nil {
		return err
	}
	if count < 0 || count > len(c.limits) {
		return ast.ErrASTDomain
	}
	if count == 0 {
		return nil
	}
	target := c.limits[len(c.limits)-count]
	for len(c.literals) > target {
		n := len(c.literals) - 1
		if err := c.literals[n].Close(); err != nil {
			return err
		}
		c.literals = c.literals[:n]
	}
	c.limits = c.limits[:len(c.limits)-count]
	return nil
}

// Check adds transient assumptions without changing the active stack. A
// requested core is filtered to those transient assumptions in input order,
// preserving repeated inputs exactly as source FilterCore does.
func (c *CachedAssumptions) Check(extra []*Expr, wantCore bool) (status lbool.LBool, out []*Expr, result error) {
	if err := c.open(); err != nil {
		return lbool.Undef, nil, err
	}
	var owned []*Expr
	defer func() {
		if result != nil {
			for _, term := range owned {
				result = errors.Join(result, term.Close())
			}
			out = nil
		}
	}()
	context := c.scope.Assumptions.context
	handles := make([]ast.ASTHandle, 0, len(c.literals)+len(extra))
	for _, terms := range [][]*Expr{c.literals, extra} {
		for _, term := range terms {
			if err := term.open(); err != nil {
				return lbool.Undef, nil, err
			}
			if term.value.object.context != context {
				return lbool.Undef, nil, ErrContextMismatch
			}
			h, err := term.Raw()
			if err != nil {
				return lbool.Undef, nil, err
			}
			handles = append(handles, h)
		}
	}
	status, err := c.scope.Solver.CheckSat(handles)
	if err != nil {
		return status, nil, err
	}
	if status == lbool.False && wantCore {
		core, err := c.scope.Solver.UnsatCore()
		if err != nil {
			return status, nil, err
		}
		members := make(map[ast.ASTHandle]bool)
		for _, h := range core {
			members[h] = true
		}
		for _, term := range extra {
			h, err := term.Raw()
			if err != nil {
				return status, nil, err
			}
			if members[h] {
				copy, err := term.Clone()
				if err != nil {
					return status, nil, err
				}
				owned = append(owned, copy)
			}
		}
	}
	return status, owned, nil
}
func (c *CachedAssumptions) Close() error {
	if c == nil || c.closed {
		return nil
	}
	for len(c.literals) > 0 {
		n := len(c.literals) - 1
		if err := c.literals[n].Close(); err != nil {
			return err
		}
		c.literals = c.literals[:n]
	}
	c.limits = nil
	c.closed = true
	return nil
}

// CheckCore is the source direct assumption/core check, without a stack.
func (s *EdgeSolverScope) CheckCore(assumptions []*Expr) (status lbool.LBool, out []*Expr, result error) {
	if s == nil || s.Solver == nil || s.Assumptions == nil {
		return lbool.Undef, nil, ErrWrapperOwner
	}
	context := s.Assumptions.context
	handles := make([]ast.ASTHandle, len(assumptions))
	for n, term := range assumptions {
		if err := term.open(); err != nil {
			return lbool.Undef, nil, err
		}
		if term.value.object.context != context {
			return lbool.Undef, nil, ErrContextMismatch
		}
		h, err := term.Raw()
		if err != nil {
			return lbool.Undef, nil, err
		}
		handles[n] = h
	}
	status, err := s.Solver.CheckSat(handles)
	if err != nil {
		return status, nil, err
	}
	var owned []*Expr
	defer func() {
		if result != nil {
			for _, term := range owned {
				result = errors.Join(result, term.Close())
			}
			out = nil
		}
	}()
	if status == lbool.False {
		core, err := s.Solver.UnsatCore()
		if err != nil {
			return status, nil, err
		}
		for _, h := range core {
			term, err := NewExpr(context, h)
			if err != nil {
				return status, nil, err
			}
			owned = append(owned, term)
		}
	}
	return status, owned, nil
}
