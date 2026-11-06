package smtlogics

// Source: src/tactic/fpa/qffp_tactic.cpp is_non_qffp_predicate,
// is_qffp_probe and mk_is_qffpbv_probe.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type isQFFPProbe struct {
	self   *isQFFPProbe
	budget *arena.Budget
	refs   uint32
	bytes  uint64
	closed bool
}

var _ tactic.ProbeEvaluator = (*isQFFPProbe)(nil)

func newIsQFFPProbe(budget *arena.Budget) (*isQFFPProbe, error) {
	if budget == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(isQFFPProbe{}).Size())
	if err := budget.Reserve(amount); err != nil {
		return nil, err
	}
	probe := &isQFFPProbe{budget: budget, bytes: amount}
	probe.self = probe
	return probe, nil
}

// NewIsQFFPProbe exposes the identical is-qffp/is-qffpbv source classifier to
// the default portfolio.
func NewIsQFFPProbe(budget *arena.Budget) (tactic.ProbeEvaluator, error) {
	return newIsQFFPProbe(budget)
}

func (p *isQFFPProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}

func (p *isQFFPProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}

func (p *isQFFPProbe) DecRef() error {
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

func (p *isQFFPProbe) Close() error {
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

func (p *isQFFPProbe) Eval(goal *tactic.Goal) (float64, error) {
	if err := p.Check(); err != nil {
		return 0, err
	}
	if goal == nil || goal.Manager() == nil {
		return 0, ast.ErrASTHandle
	}
	m := goal.Manager()
	fpa, err := m.ExistingFloatingPointFamilyID()
	if err != nil {
		return 0, err
	}
	bv, err := m.BitVectorUtilityFamilyID()
	if err != nil {
		return 0, err
	}
	arith, err := m.ArithmeticFamilyID()
	if err != nil {
		return 0, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return 0, err
	}
	size, err := goal.Size()
	if err != nil {
		return 0, err
	}
	stack := make([]ast.ASTHandle, 0, size)
	for i := size; i != 0; i-- {
		formula, getErr := goal.Form(i - 1)
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
		kind, getErr := m.GetKind(current)
		if getErr != nil {
			return 0, getErr
		}
		if kind != ast.ASTApp {
			return 0, nil
		}
		sort, getErr := m.GetSort(current)
		if getErr != nil {
			return 0, getErr
		}
		sortFamily, getErr := m.GetFamilyID(sort)
		if getErr != nil {
			return 0, getErr
		}
		sortKind, getErr := m.GetDeclKind(sort)
		if getErr != nil {
			return 0, getErr
		}
		_, _, floatSort, getErr := m.FloatingPointSortShape(sort)
		if getErr != nil {
			return 0, getErr
		}
		roundingModeSort, getErr := m.IsRoundingModeSort(sort)
		if getErr != nil {
			return 0, getErr
		}
		bitVectorSort, getErr := m.IsBitVectorSort(sort)
		if getErr != nil {
			return 0, getErr
		}
		realSort := sortFamily == arith && sortKind == 0
		if sort != boolean && !floatSort && !roundingModeSort && !bitVectorSort && !realSort {
			return 0, nil
		}
		family, getErr := m.GetFamilyID(current)
		if getErr != nil {
			return 0, getErr
		}
		arguments, getErr := m.GetNumArgs(current)
		if getErr != nil {
			return 0, getErr
		}
		allowed := family == m.BasicFamilyID() || family == fpa || family == bv || family == ast.NullFamilyID && arguments == 0
		if !allowed && realSort {
			_, _, allowed, getErr = m.ArithmeticNumeralText(current)
			if getErr != nil {
				return 0, getErr
			}
		}
		if !allowed {
			return 0, nil
		}
		for i := arguments; i != 0; i-- {
			argument, argErr := m.GetArg(current, i-1)
			if argErr != nil {
				return 0, argErr
			}
			stack = append(stack, argument)
		}
	}
	return 1, nil
}
