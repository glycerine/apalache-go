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
	equations []smtSequenceEquation
	snapshots [][]smtSequenceEquation
	closed    bool
}

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

func (q *smtSequenceEquationQueue) clone(source []smtSequenceEquation) ([]smtSequenceEquation, error) {
	result := make([]smtSequenceEquation, 0, len(source))
	for i := range source {
		left, right, err := source[i].handles(q.manager)
		if err != nil {
			_ = q.closeEquations(result)
			return nil, err
		}
		copy, err := newSMTSequenceEquation(q.manager, source[i].id, left, right, source[i].dependency)
		if err != nil {
			_ = q.closeEquations(result)
			return nil, err
		}
		result = append(result, copy)
	}
	return result, nil
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
	return len(q.equations)
}

func (q *smtSequenceEquationQueue) At(index int) (uint32, []ast.ASTHandle, []ast.ASTHandle, *smtSequenceDependency, error) {
	if err := q.open(); err != nil {
		return 0, nil, nil, nil, err
	}
	if index < 0 || index >= len(q.equations) {
		return 0, nil, nil, nil, ast.ErrASTDomain
	}
	equation := &q.equations[index]
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
	return nil
}

func (q *smtSequenceEquationQueue) Set(index int, id uint32, left, right []ast.ASTHandle, dependency *smtSequenceDependency) error {
	if err := q.open(); err != nil {
		return err
	}
	if index < 0 || index >= len(q.equations) {
		return ast.ErrASTDomain
	}
	replacement, err := newSMTSequenceEquation(q.manager, id, left, right, dependency)
	if err != nil {
		return err
	}
	if err = q.equations[index].close(q.manager); err != nil {
		_ = replacement.close(q.manager)
		return err
	}
	q.equations[index] = replacement
	return nil
}

func (q *smtSequenceEquationQueue) PopBack() error {
	if err := q.open(); err != nil {
		return err
	}
	if len(q.equations) == 0 {
		return ast.ErrASTDomain
	}
	index := len(q.equations) - 1
	if err := q.equations[index].close(q.manager); err != nil {
		return err
	}
	q.equations = q.equations[:index]
	return nil
}

func (q *smtSequenceEquationQueue) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	snapshot, err := q.clone(q.equations)
	if err != nil {
		return err
	}
	q.snapshots = append(q.snapshots, snapshot)
	return nil
}

func (q *smtSequenceEquationQueue) PopScopes(count uint32) (result error) {
	if err := q.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(q.snapshots)) {
		return ast.ErrASTDomain
	}
	level := len(q.snapshots) - int(count)
	result = q.closeEquations(q.equations)
	for i := level + 1; i < len(q.snapshots); i++ {
		result = errors.Join(result, q.closeEquations(q.snapshots[i]))
	}
	q.equations = q.snapshots[level]
	q.snapshots = q.snapshots[:level]
	return result
}

func (q *smtSequenceEquationQueue) Close() (result error) {
	if err := q.open(); err != nil {
		return err
	}
	result = q.closeEquations(q.equations)
	for i := range q.snapshots {
		result = errors.Join(result, q.closeEquations(q.snapshots[i]))
	}
	q.equations = nil
	q.snapshots = nil
	q.manager = nil
	q.closed = true
	return result
}
