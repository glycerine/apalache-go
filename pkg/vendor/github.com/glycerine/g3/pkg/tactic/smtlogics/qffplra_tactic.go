package smtlogics

// Source: src/tactic/fpa/qffplra_tactic.cpp/.h.

import (
	"io"
	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewQFFPLRATactic preserves the source factory: QF_FPLRA uses the QF_FP
// strategy with the caller's parameters applied to it.
func NewQFFPLRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFFPLRATacticWithOutput(m, p, config, os.Stdout)
}

func NewQFFPLRATacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (tactic.Tactic, error) {
	return NewQFFPTacticWithOutput(m, p, config, output)
}

type isQFFPLRAProbe struct {
	self   *isQFFPLRAProbe
	budget *arena.Budget
	refs   uint32
	bytes  uint64
	closed bool
}

var _ tactic.ProbeEvaluator = (*isQFFPLRAProbe)(nil)

func newIsQFFPLRAProbe(budget *arena.Budget) (*isQFFPLRAProbe, error) {
	if budget == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(isQFFPLRAProbe{}).Size())
	if err := budget.Reserve(amount); err != nil {
		return nil, err
	}
	probe := &isQFFPLRAProbe{budget: budget, bytes: amount}
	probe.self = probe
	return probe, nil
}

// NewIsQFFPLRAProbe exposes the source default-portfolio probe.
func NewIsQFFPLRAProbe(budget *arena.Budget) (tactic.ProbeEvaluator, error) {
	return newIsQFFPLRAProbe(budget)
}

func (p *isQFFPLRAProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}

func (p *isQFFPLRAProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}

func (p *isQFFPLRAProbe) DecRef() error {
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

func (p *isQFFPLRAProbe) Close() error {
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

func (p *isQFFPLRAProbe) Eval(goal *tactic.Goal) (float64, error) {
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
		if family != m.BasicFamilyID() && family != fpa && family != bv && family != arith && !(family == ast.NullFamilyID && arguments == 0) && !realSort {
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
