package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticBoundsProcessor struct {
	self           *ArithmeticBoundsProcessor
	util           *ArithmeticQETermUtil
	manager        *ast.ASTManager
	marked         map[ast.ASTHandle]bool
	bounds         []*ArithmeticBound
	divisibilities []*ArithmeticDivisibility
	closed         bool
}

func NewArithmeticBoundsProcessor(util *ArithmeticQETermUtil) (*ArithmeticBoundsProcessor, error) {
	if util == nil {
		return nil, ast.ErrASTHandle
	}
	if err := util.open(); err != nil {
		return nil, err
	}
	p := &ArithmeticBoundsProcessor{util: util, manager: util.manager, marked: make(map[ast.ASTHandle]bool)}
	p.self = p
	return p, nil
}

func (p *ArithmeticBoundsProcessor) open() error {
	if p == nil || p.self != p || p.util == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return p.util.open()
}

func (p *ArithmeticBoundsProcessor) Reset() (result error) {
	if err := p.open(); err != nil {
		return err
	}
	for i := len(p.bounds) - 1; i >= 0; i-- {
		result = errors.Join(result, p.bounds[i].Close())
	}
	for i := len(p.divisibilities) - 1; i >= 0; i-- {
		result = errors.Join(result, p.divisibilities[i].Close())
	}
	p.bounds = nil
	p.divisibilities = nil
	p.marked = make(map[ast.ASTHandle]bool)
	return result
}

func (p *ArithmeticBoundsProcessor) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	err := p.Reset()
	if err == nil {
		p.closed = true
	}
	return err
}

func (p *ArithmeticBoundsProcessor) Bounds(strict bool, direction ArithmeticBoundDirection) []*ArithmeticBound {
	if p == nil || p.closed {
		return nil
	}
	result := make([]*ArithmeticBound, 0, len(p.bounds))
	for _, bound := range p.bounds {
		if bound.Strict == strict && bound.Direction == direction {
			result = append(result, bound)
		}
	}
	return result
}

func (p *ArithmeticBoundsProcessor) Divisibilities(nested bool) []*ArithmeticDivisibility {
	if p == nil || p.closed {
		return nil
	}
	result := make([]*ArithmeticDivisibility, 0, len(p.divisibilities))
	for _, constraint := range p.divisibilities {
		if constraint.Nested == nested {
			result = append(result, constraint)
		}
	}
	return result
}

func (p *ArithmeticBoundsProcessor) Add(variable *ast.ContainsApp, atom ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if variable == nil {
		return false, ast.ErrASTHandle
	}
	if p.marked[atom] {
		return true, nil
	}
	if matched, err := p.addNonStrict(variable, atom); err != nil || matched {
		if matched {
			p.marked[atom] = true
		}
		return matched, err
	}
	if matched, err := p.addStrict(variable, atom); err != nil || matched {
		if matched {
			p.marked[atom] = true
		}
		return matched, err
	}
	if matched, err := p.addDivides(variable, atom); err != nil || matched {
		if matched {
			p.marked[atom] = true
		}
		return matched, err
	}
	matched, err := p.addNestedDivides(variable, atom)
	if matched {
		p.marked[atom] = true
	}
	return matched, err
}
