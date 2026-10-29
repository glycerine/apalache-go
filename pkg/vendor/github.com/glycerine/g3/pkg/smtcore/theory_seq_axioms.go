package smtcore

// Source: src/smt/theory_seq.cpp m_axioms, m_axiom_set, m_axioms_head and
// enque_axiom/deque_axiom propagation lifecycle.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSequenceAxiomScope struct{ limit, head int }

type smtSequenceAxiomQueue struct {
	manager *ast.ASTManager
	stats   *smtSequenceStatistics
	values  []ast.ASTRef
	set     map[ast.ASTHandle]struct{}
	head    int
	scopes  []smtSequenceAxiomScope
	closed  bool
}

func newSMTSequenceAxiomQueue(manager *ast.ASTManager, stats *smtSequenceStatistics) (*smtSequenceAxiomQueue, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtSequenceAxiomQueue{manager: manager, stats: stats, set: make(map[ast.ASTHandle]struct{})}, nil
}

func (q *smtSequenceAxiomQueue) open() error {
	if q == nil || q.manager == nil || q.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (q *smtSequenceAxiomQueue) Push(expression ast.ASTHandle) error {
	if err := q.open(); err != nil {
		return err
	}
	if expression.IsNull() {
		return ast.ErrASTDomain
	}
	if _, found := q.set[expression]; found {
		return nil
	}
	ref, err := q.manager.NewRef(expression)
	if err != nil {
		return err
	}
	q.values = append(q.values, ref)
	q.set[expression] = struct{}{}
	if q.stats != nil {
		q.stats.addAxiom++
	}
	return nil
}

func (q *smtSequenceAxiomQueue) Pending() bool {
	return q != nil && !q.closed && q.head < len(q.values)
}

func (q *smtSequenceAxiomQueue) Front() (ast.ASTHandle, error) {
	if err := q.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if !q.Pending() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return q.manager.RefGet(q.values[q.head])
}

func (q *smtSequenceAxiomQueue) Advance() error {
	if err := q.open(); err != nil {
		return err
	}
	if !q.Pending() {
		return ast.ErrASTDomain
	}
	q.head++
	return nil
}

func (q *smtSequenceAxiomQueue) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	q.scopes = append(q.scopes, smtSequenceAxiomScope{limit: len(q.values), head: q.head})
	return nil
}

func (q *smtSequenceAxiomQueue) PopScopes(count uint32) (result error) {
	if err := q.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(q.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(q.scopes) - int(count)
	scope := q.scopes[level]
	for i := len(q.values) - 1; i >= scope.limit; i-- {
		expression, err := q.manager.RefGet(q.values[i])
		if err == nil {
			delete(q.set, expression)
		}
		result = errors.Join(result, err, q.manager.DelRef(&q.values[i]))
	}
	q.values = q.values[:scope.limit]
	q.head = scope.head
	q.scopes = q.scopes[:level]
	return result
}

func (q *smtSequenceAxiomQueue) Close() (result error) {
	if err := q.open(); err != nil {
		return err
	}
	for i := range q.values {
		result = errors.Join(result, q.manager.DelRef(&q.values[i]))
	}
	q.values = nil
	q.set = nil
	q.scopes = nil
	q.manager = nil
	q.stats = nil
	q.closed = true
	return result
}
