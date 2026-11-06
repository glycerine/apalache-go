package smtcore

// Source: src/smt/theory_seq.h eq and scoped_vector<eq>; the retained
// sequence instantiation uses concrete Go storage with explicit AST roots.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSequenceEquation struct {
	id         uint32
	left       []ast.ASTRef
	right      []ast.ASTRef
	dependency *smtSequenceDependency
}

func newSMTSequenceEquation(manager *ast.ASTManager, id uint32, left, right []ast.ASTHandle, dependency *smtSequenceDependency) (smtSequenceEquation, error) {
	if manager == nil {
		return smtSequenceEquation{}, ast.ErrASTHandle
	}
	equation := smtSequenceEquation{id: id, dependency: dependency}
	var err error
	for _, expression := range left {
		ref, refErr := manager.NewRef(expression)
		if refErr != nil {
			err = refErr
			break
		}
		equation.left = append(equation.left, ref)
	}
	if err == nil {
		for _, expression := range right {
			ref, refErr := manager.NewRef(expression)
			if refErr != nil {
				err = refErr
				break
			}
			equation.right = append(equation.right, ref)
		}
	}
	if err != nil {
		return smtSequenceEquation{}, errors.Join(err, equation.close(manager))
	}
	return equation, nil
}

func (e *smtSequenceEquation) close(manager *ast.ASTManager) (result error) {
	if e == nil || manager == nil {
		return ast.ErrASTHandle
	}
	for i := range e.left {
		result = errors.Join(result, manager.DelRef(&e.left[i]))
	}
	for i := range e.right {
		result = errors.Join(result, manager.DelRef(&e.right[i]))
	}
	e.left = nil
	e.right = nil
	e.dependency = nil
	return result
}

func (e *smtSequenceEquation) handles(manager *ast.ASTManager) (left, right []ast.ASTHandle, err error) {
	if e == nil || manager == nil {
		return nil, nil, ast.ErrASTHandle
	}
	left = make([]ast.ASTHandle, len(e.left))
	right = make([]ast.ASTHandle, len(e.right))
	for i := range e.left {
		left[i], err = manager.RefGet(e.left[i])
		if err != nil {
			return nil, nil, err
		}
	}
	for i := range e.right {
		right[i], err = manager.RefGet(e.right[i])
		if err != nil {
			return nil, nil, err
		}
	}
	return left, right, nil
}

type smtSequenceEquationQueue struct {
	manager   *ast.ASTManager
	equations []smtSequenceEquation // physical elements, including entries hidden by scopes
	index     []int                 // logical index to physical element
	size      int
	scopes    []smtSequenceEquationScope
	trail     []smtSequenceEquationIndex
	elemStart int
	closed    bool
}

type smtSequenceEquationScope struct{ size, trailStart, elemStart int }
type smtSequenceEquationIndex struct{ source, destination int }

func newSMTSequenceEquationQueue(manager *ast.ASTManager) (*smtSequenceEquationQueue, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtSequenceEquationQueue{manager: manager}, nil
}

func (q *smtSequenceEquationQueue) open() error {
	if q == nil || q.manager == nil || q.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (q *smtSequenceEquationQueue) closeEquations(equations []smtSequenceEquation) (result error) {
	for i := range equations {
		result = errors.Join(result, equations[i].close(q.manager))
	}
	return result
}

func (q *smtSequenceEquationQueue) Len() int {
	if q == nil || q.closed {
		return 0
	}
	return q.size
}

func (q *smtSequenceEquationQueue) At(index int) (uint32, []ast.ASTHandle, []ast.ASTHandle, *smtSequenceDependency, error) {
	if err := q.open(); err != nil {
		return 0, nil, nil, nil, err
	}
	if index < 0 || index >= q.size {
		return 0, nil, nil, nil, ast.ErrASTDomain
	}
	equation := &q.equations[q.index[index]]
	left, right, err := equation.handles(q.manager)
	return equation.id, left, right, equation.dependency, err
}

func (q *smtSequenceEquationQueue) Push(id uint32, left, right []ast.ASTHandle, dependency *smtSequenceDependency) error {
	if err := q.open(); err != nil {
		return err
	}
	equation, err := newSMTSequenceEquation(q.manager, id, left, right, dependency)
	if err != nil {
		return err
	}
	q.equations = append(q.equations, equation)
	logical := q.size
	q.setIndex(logical)
	if logical == len(q.index) {
		q.index = append(q.index, len(q.equations)-1)
	} else {
		q.index[logical] = len(q.equations) - 1
	}
	q.size++
	return nil
}

func (q *smtSequenceEquationQueue) Set(index int, id uint32, left, right []ast.ASTHandle, dependency *smtSequenceDependency) error {
	if err := q.open(); err != nil {
		return err
	}
	if index < 0 || index >= q.size {
		return ast.ErrASTDomain
	}
	replacement, err := newSMTSequenceEquation(q.manager, id, left, right, dependency)
	if err != nil {
		return err
	}
	physical := q.index[index]
	if physical < q.elemStart {
		q.setIndex(index)
		q.equations = append(q.equations, replacement)
		q.index[index] = len(q.equations) - 1
	} else {
		if err = q.equations[physical].close(q.manager); err != nil {
			_ = replacement.close(q.manager)
			return err
		}
		q.equations[physical] = replacement
	}
	return nil
}

func (q *smtSequenceEquationQueue) PopBack() error {
	if err := q.open(); err != nil {
		return err
	}
	if q.size == 0 {
		return ast.ErrASTDomain
	}
	logical := q.size - 1
	physical := q.index[logical]
	if physical == len(q.equations)-1 && physical >= q.elemStart {
		if err := q.equations[physical].close(q.manager); err != nil {
			return err
		}
		q.equations = q.equations[:physical]
	}
	q.size = logical
	return nil
}

func (q *smtSequenceEquationQueue) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	q.scopes = append(q.scopes, smtSequenceEquationScope{q.size, len(q.trail), len(q.equations)})
	q.elemStart = len(q.equations)
	return nil
}

func (q *smtSequenceEquationQueue) PopScopes(count uint32) (result error) {
	if err := q.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(q.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(q.scopes) - int(count)
	scope := q.scopes[level]
	for i := len(q.trail) - 1; i >= scope.trailStart; i-- {
		q.index[q.trail[i].source] = q.trail[i].destination
	}
	q.trail = q.trail[:scope.trailStart]
	result = q.closeEquations(q.equations[scope.elemStart:])
	q.equations = q.equations[:scope.elemStart]
	q.size = scope.size
	q.scopes = q.scopes[:level]
	q.elemStart = scope.elemStart
	return result
}

func (q *smtSequenceEquationQueue) Close() (result error) {
	if err := q.open(); err != nil {
		return err
	}
	result = q.closeEquations(q.equations)
	q.equations = nil
	q.index = nil
	q.scopes = nil
	q.trail = nil
	q.manager = nil
	q.closed = true
	return result
}

func (q *smtSequenceEquationQueue) setIndex(source int) {
	if source < len(q.index) && source < q.elemStart {
		q.trail = append(q.trail, smtSequenceEquationIndex{source, q.index[source]})
	}
}
