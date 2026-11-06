package smtlogics

// Source: src/tactic/fpa/qffp_tactic.cpp is_non_fp_qfnra_predicate and
// is_fp_qfnra_probe.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type isFPQFNRAProbe struct {
	self   *isFPQFNRAProbe
	budget *arena.Budget
	refs   uint32
	bytes  uint64
	closed bool
}

var _ tactic.ProbeEvaluator = (*isFPQFNRAProbe)(nil)

func newIsFPQFNRAProbe(budget *arena.Budget) (*isFPQFNRAProbe, error) {
	if budget == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(isFPQFNRAProbe{}).Size())
	if err := budget.Reserve(amount); err != nil {
		return nil, err
	}
	probe := &isFPQFNRAProbe{budget: budget, bytes: amount}
	probe.self = probe
	return probe, nil
}

// NewIsFPQFNRAProbe exposes the source default-portfolio probe to the
// portfolio package while retaining its concrete ownership implementation.
func NewIsFPQFNRAProbe(budget *arena.Budget) (tactic.ProbeEvaluator, error) {
	return newIsFPQFNRAProbe(budget)
}

func (p *isFPQFNRAProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}

func (p *isFPQFNRAProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}

func (p *isFPQFNRAProbe) DecRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	if p.refs == 0 {
		return tactic.ErrRegistryOwner
	}
	p.refs--
	if p.refs == 0 {
		return p.Close()
	}
	return nil
}

func (p *isFPQFNRAProbe) Close() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return nil
	}
	p.budget.Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}

func (p *isFPQFNRAProbe) Eval(goal *tactic.Goal) (float64, error) {
	if err := p.Check(); err != nil {
		return 0, err
	}
	if goal == nil {
		return 0, ast.ErrASTHandle
	}
	manager := goal.Manager()
	if manager == nil {
		return 0, ast.ErrASTHandle
	}
	floatingPoint, err := manager.ExistingFloatingPointFamilyID()
	if err != nil {
		return 0, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return 0, err
	}
	size, err := goal.Size()
	if err != nil {
		return 0, err
	}
	stack := make([]ast.ASTHandle, 0, size)
	for index := size; index != 0; index-- {
		formula, getErr := goal.Form(index - 1)
		if getErr != nil {
			return 0, getErr
		}
		stack = append(stack, formula)
	}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, getErr := manager.GetKind(current)
		if getErr != nil {
			return 0, getErr
		}
		if kind != ast.ASTApp {
			return 0, nil
		}
		family, getErr := manager.GetFamilyID(current)
		if getErr != nil {
			return 0, getErr
		}
		// Preserve the source predicate's ordering. In particular, basic-family
		// applications are rejected by this first guard before its later,
		// unreachable basic-family branch.
		if family != ast.NullFamilyID && family != floatingPoint {
			return 0, nil
		}
		sort, getErr := manager.GetSort(current)
		if getErr != nil {
			return 0, getErr
		}
		_, _, isFloat, getErr := manager.FloatingPointSortShape(sort)
		if getErr != nil {
			return 0, getErr
		}
		isRoundingMode, getErr := manager.IsRoundingModeSort(sort)
		if getErr != nil {
			return 0, getErr
		}
		if family == floatingPoint {
			operator, kindErr := manager.GetDeclKind(current)
			if kindErr != nil {
				return 0, kindErr
			}
			if !isFloat && !isRoundingMode && operator != ast.FPAOpToReal {
				return 0, nil
			}
		} else if !isFloat && !isRoundingMode && sort != boolean {
			return 0, nil
		}
		arguments, getErr := manager.GetNumArgs(current)
		if getErr != nil {
			return 0, getErr
		}
		for index := arguments; index != 0; index-- {
			argument, argErr := manager.GetArg(current, index-1)
			if argErr != nil {
				return 0, argErr
			}
			stack = append(stack, argument)
		}
	}
	return 1, nil
}
