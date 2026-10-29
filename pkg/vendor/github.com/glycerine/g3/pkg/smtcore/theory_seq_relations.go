package smtcore

// Source: src/smt/theory_seq.cpp m_atoms/m_atoms_lim and assign_eh for the
// retained prefix, suffix and contains atoms.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSequenceRelationAssignment struct {
	expression ast.ASTRef
	value      bool
	dependency *smtSequenceDependency
}

type smtSequenceRelationQueue struct {
	manager *ast.ASTManager
	values  []smtSequenceRelationAssignment
	limits  []int
	closed  bool
}

func newSMTSequenceRelationQueue(manager *ast.ASTManager) (*smtSequenceRelationQueue, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtSequenceRelationQueue{manager: manager}, nil
}

func (q *smtSequenceRelationQueue) open() error {
	if q == nil || q.manager == nil || q.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (q *smtSequenceRelationQueue) Len() int {
	if q == nil || q.closed {
		return 0
	}
	return len(q.values)
}

func (q *smtSequenceRelationQueue) Push(expression ast.ASTHandle, value bool, dependency *smtSequenceDependency) error {
	if err := q.open(); err != nil {
		return err
	}
	ref, err := q.manager.NewRef(expression)
	if err != nil {
		return err
	}
	q.values = append(q.values, smtSequenceRelationAssignment{expression: ref, value: value, dependency: dependency})
	return nil
}

func (q *smtSequenceRelationQueue) At(index int) (ast.ASTHandle, bool, *smtSequenceDependency, error) {
	if err := q.open(); err != nil {
		return ast.ASTHandle{}, false, nil, err
	}
	if index < 0 || index >= len(q.values) {
		return ast.ASTHandle{}, false, nil, ast.ErrASTDomain
	}
	value := &q.values[index]
	expression, err := q.manager.RefGet(value.expression)
	return expression, value.value, value.dependency, err
}

func (q *smtSequenceRelationQueue) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	q.limits = append(q.limits, len(q.values))
	return nil
}

func (q *smtSequenceRelationQueue) PopScopes(count uint32) (result error) {
	if err := q.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(q.limits)) {
		return ast.ErrASTDomain
	}
	level := len(q.limits) - int(count)
	limit := q.limits[level]
	for i := len(q.values) - 1; i >= limit; i-- {
		result = errors.Join(result, q.manager.DelRef(&q.values[i].expression))
		q.values[i].dependency = nil
	}
	q.values = q.values[:limit]
	q.limits = q.limits[:level]
	return result
}

func (q *smtSequenceRelationQueue) Close() (result error) {
	if err := q.open(); err != nil {
		return err
	}
	for i := range q.values {
		result = errors.Join(result, q.manager.DelRef(&q.values[i].expression))
	}
	q.values = nil
	q.limits = nil
	q.manager = nil
	q.closed = true
	return result
}
