package tactic

// Source: src/qe/nlqsat.cpp:181-223,383-400.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
)

// ModelProjector composes the retained NLSAT projection operation with the
// literal-to-AST converter used by nonlinear quantified search.
type ModelProjector struct {
	self          *ModelProjector
	manager       *ast.ASTManager
	solver        *nlsat.Solver
	converter     *GoalConverter
	first, second *nlsat.LiteralVector
	closed        bool
}

func NewModelProjector(manager *ast.ASTManager, solver *nlsat.Solver, atoms, terms *ast.Expr2Var) (projector *ModelProjector, err error) {
	converter, err := NewGoalConverter(manager, solver, atoms, terms)
	if err != nil {
		return nil, err
	}
	p := &ModelProjector{manager: manager, solver: solver, converter: converter}
	p.self = p
	defer func() {
		if err != nil {
			err = errors.Join(err, p.Close())
		}
	}()
	p.first, err = nlsat.NewLiteralVector(solver)
	if err != nil {
		return p, err
	}
	p.second, err = nlsat.NewLiteralVector(solver)
	if err != nil {
		return p, err
	}
	return p, nil
}

func (p *ModelProjector) open() error {
	if p == nil || p.self != p || p.manager == nil || p.solver == nil || p.converter == nil {
		return arena.ErrConfig
	}
	if p.closed {
		return arena.ErrClosed
	}
	return nil
}

func (p *ModelProjector) reset() error {
	if err := p.first.Reset(); err != nil {
		return err
	}
	return p.second.Reset()
}

// ProjectClause eliminates variables from a model-true literal cube in reverse
// order, then returns the owning AST disjunction that blocks that projected
// cube. This is nlqsat::mbp followed by negate_clause and clause2fml.
func (p *ModelProjector) ProjectClause(variables []uint32, literals []sat.Literal) (result ast.ASTRef, err error) {
	if err = p.open(); err != nil {
		return result, err
	}
	if err = p.reset(); err != nil {
		return result, err
	}
	for _, literal := range literals {
		if err = p.first.Push(literal); err != nil {
			return result, err
		}
	}
	current, next := p.first, p.second
	var values []sat.Literal
	for i := len(variables); i > 0; i-- {
		if err = current.Literals(&values); err != nil {
			return result, err
		}
		if err = p.solver.Project(variables[i-1], values, next); err != nil {
			return result, err
		}
		current, next = next, current
	}
	if err = current.Literals(&values); err != nil {
		return result, err
	}
	refs := make([]ast.ASTRef, 0, len(values))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, p.manager.DelRef(&refs[i]))
		}
	}()
	handles := make([]ast.ASTHandle, 0, len(values))
	for _, literal := range values {
		ref, cause := p.converter.ConvertLiteral(literal.Negated())
		if cause != nil {
			return result, cause
		}
		refs = append(refs, ref)
		handle, cause := p.manager.RefGet(ref)
		if cause != nil {
			return result, cause
		}
		handles = append(handles, handle)
	}
	formula, err := p.manager.MkOrArray(handles)
	if err != nil {
		return result, err
	}
	return p.manager.NewRef(formula)
}

func (p *ModelProjector) Close() (err error) {
	if p == nil || p.self != p {
		return arena.ErrConfig
	}
	if p.closed {
		return nil
	}
	if p.second != nil {
		if err = p.second.Close(); err != nil {
			return err
		}
		p.second = nil
	}
	if p.first != nil {
		if err = p.first.Close(); err != nil {
			return err
		}
		p.first = nil
	}
	p.converter = nil
	p.solver = nil
	p.manager = nil
	p.closed = true
	return nil
}
