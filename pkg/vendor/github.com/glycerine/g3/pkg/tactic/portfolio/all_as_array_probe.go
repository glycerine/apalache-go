package portfolio

// Source: src/tactic/portfolio/default_tactic.cpp initial simplification before
// the default SMT backend. This retained integration probe narrows the stronger
// Go preprocessing fallback to goals containing OP_AS_ARRAY.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type allAsArrayProbe struct {
	self       *allAsArrayProbe
	manager    *ast.ASTManager
	budget     *arena.Budget
	ownerBytes uint64
	references uint32
	closed     bool
}

func newALLAsArrayProbe(manager *ast.ASTManager) (*allAsArrayProbe, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(allAsArrayProbe{}).Size())
	if err := manager.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	p := &allAsArrayProbe{manager: manager, budget: manager.Budget(), ownerBytes: amount}
	p.self = p
	return p, nil
}

func (p *allAsArrayProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}

func (p *allAsArrayProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.references++
	return nil
}

func (p *allAsArrayProbe) DecRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	if p.references == 0 {
		return tactic.ErrRegistryOwner
	}
	p.references--
	if p.references == 0 {
		return p.Close()
	}
	return nil
}

func (p *allAsArrayProbe) Close() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return nil
	}
	p.budget.Release(p.ownerBytes)
	p.ownerBytes = 0
	p.manager = nil
	p.closed = true
	return nil
}

func (p *allAsArrayProbe) Eval(goal *tactic.Goal) (float64, error) {
	if err := p.Check(); err != nil {
		return 0, err
	}
	if goal == nil || goal.Manager() != p.manager {
		return 0, ast.ErrASTHandle
	}
	arrayFamily, err := p.manager.ArrayFamilyID()
	if err != nil {
		return 0, err
	}
	count, err := goal.Size()
	if err != nil {
		return 0, err
	}
	stack := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		formula, e := goal.Form(i)
		if e != nil {
			return 0, e
		}
		stack = append(stack, formula)
	}
	seen := make(map[uint32]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		id, e := p.manager.GetASTID(expression)
		if e != nil {
			return 0, e
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		kind, e := p.manager.GetKind(expression)
		if e != nil {
			return 0, e
		}
		switch kind {
		case ast.ASTApp:
			family, e := p.manager.GetFamilyID(expression)
			if e != nil {
				return 0, e
			}
			operation, e := p.manager.GetDeclKind(expression)
			if e != nil {
				return 0, e
			}
			if family == arrayFamily && operation == ast.ASTArrayOpAsArray {
				return 1, nil
			}
			arguments, e := p.manager.GetNumArgs(expression)
			if e != nil {
				return 0, e
			}
			for i := uint32(0); i < arguments; i++ {
				argument, x := p.manager.GetArg(expression, i)
				if x != nil {
					return 0, x
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, e := p.manager.GetQuantifierBody(expression)
			if e != nil {
				return 0, e
			}
			stack = append(stack, body)
			patterns, e := p.manager.GetNumPatterns(expression)
			if e != nil {
				return 0, e
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, x := p.manager.GetPattern(expression, i)
				if x != nil {
					return 0, x
				}
				stack = append(stack, pattern)
			}
		}
	}
	return 0, nil
}

var _ tactic.ProbeEvaluator = (*allAsArrayProbe)(nil)
