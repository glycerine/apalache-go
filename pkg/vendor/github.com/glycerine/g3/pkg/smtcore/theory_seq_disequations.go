package smtcore

// Source: src/smt/theory_seq.h ne and scoped_vector<ne>.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSequenceDisequation struct {
	left, right ast.ASTRef
	ownLeft     bool
	ownRight    bool
	leftCases   [][]ast.ASTRef
	rightCases  [][]ast.ASTRef
	literals    []SMTLiteral
	dependency  *smtSequenceDependency
}

func newSMTSequenceDisequation(manager *ast.ASTManager, left, right ast.ASTHandle, dependency *smtSequenceDependency) (smtSequenceDisequation, error) {
	return newSMTSequenceDisequationCases(manager, left, right, [][]ast.ASTHandle{{left}}, [][]ast.ASTHandle{{right}}, nil, dependency)
}

func newSMTSequenceDisequationCases(manager *ast.ASTManager, left, right ast.ASTHandle, leftCases, rightCases [][]ast.ASTHandle, literals []SMTLiteral, dependency *smtSequenceDependency) (smtSequenceDisequation, error) {
	if manager == nil || left.IsNull() || right.IsNull() || len(leftCases) != len(rightCases) {
		return smtSequenceDisequation{}, ast.ErrASTDomain
	}
	result := smtSequenceDisequation{dependency: dependency, literals: append([]SMTLiteral(nil), literals...)}
	var err error
	result.left, err = manager.NewRef(left)
	if err == nil {
		result.ownLeft = true
		result.right, err = manager.NewRef(right)
		if err == nil {
			result.ownRight = true
		}
	}
	for i := 0; err == nil && i < len(leftCases); i++ {
		var leftRefs, rightRefs []ast.ASTRef
		for _, expression := range leftCases[i] {
			ref, refErr := manager.NewRef(expression)
			if refErr != nil {
				err = refErr
				break
			}
			leftRefs = append(leftRefs, ref)
		}
		for _, expression := range rightCases[i] {
			if err != nil {
				break
			}
			ref, refErr := manager.NewRef(expression)
			if refErr != nil {
				err = refErr
				break
			}
			rightRefs = append(rightRefs, ref)
		}
		result.leftCases = append(result.leftCases, leftRefs)
		result.rightCases = append(result.rightCases, rightRefs)
	}
	if err != nil {
		return smtSequenceDisequation{}, errors.Join(err, result.close(manager))
	}
	return result, nil
}

func (e *smtSequenceDisequation) close(manager *ast.ASTManager) (result error) {
	if e == nil || manager == nil {
		return ast.ErrASTHandle
	}
	if e.ownLeft {
		result = errors.Join(result, manager.DelRef(&e.left))
		e.ownLeft = false
	}
	if e.ownRight {
		result = errors.Join(result, manager.DelRef(&e.right))
		e.ownRight = false
	}
	for i := range e.leftCases {
		for j := range e.leftCases[i] {
			result = errors.Join(result, manager.DelRef(&e.leftCases[i][j]))
		}
	}
	for i := range e.rightCases {
		for j := range e.rightCases[i] {
			result = errors.Join(result, manager.DelRef(&e.rightCases[i][j]))
		}
	}
	e.leftCases = nil
	e.rightCases = nil
	e.literals = nil
	e.dependency = nil
	return result
}

func smtSequenceHandles(manager *ast.ASTManager, refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		value, err := manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

func (e *smtSequenceDisequation) values(manager *ast.ASTManager) (left, right ast.ASTHandle, leftCases, rightCases [][]ast.ASTHandle, literals []SMTLiteral, dependency *smtSequenceDependency, err error) {
	left, err = manager.RefGet(e.left)
	if err != nil {
		return
	}
	right, err = manager.RefGet(e.right)
	if err != nil {
		return
	}
	leftCases = make([][]ast.ASTHandle, len(e.leftCases))
	rightCases = make([][]ast.ASTHandle, len(e.rightCases))
	for i := range e.leftCases {
		leftCases[i], err = smtSequenceHandles(manager, e.leftCases[i])
		if err != nil {
			return
		}
		rightCases[i], err = smtSequenceHandles(manager, e.rightCases[i])
		if err != nil {
			return
		}
	}
	literals = append([]SMTLiteral(nil), e.literals...)
	dependency = e.dependency
	return
}

type smtSequenceDisequationQueue struct {
	manager      *ast.ASTManager
	disequations []smtSequenceDisequation
	snapshots    [][]smtSequenceDisequation
	closed       bool
}

func newSMTSequenceDisequationQueue(manager *ast.ASTManager) (*smtSequenceDisequationQueue, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtSequenceDisequationQueue{manager: manager}, nil
}

func (q *smtSequenceDisequationQueue) open() error {
	if q == nil || q.manager == nil || q.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (q *smtSequenceDisequationQueue) closeValues(values []smtSequenceDisequation) (result error) {
	for i := range values {
		result = errors.Join(result, values[i].close(q.manager))
	}
	return result
}

func (q *smtSequenceDisequationQueue) clone(source []smtSequenceDisequation) ([]smtSequenceDisequation, error) {
	result := make([]smtSequenceDisequation, 0, len(source))
	for i := range source {
		left, right, leftCases, rightCases, literals, dependency, err := source[i].values(q.manager)
		if err != nil {
			_ = q.closeValues(result)
			return nil, err
		}
		copy, err := newSMTSequenceDisequationCases(q.manager, left, right, leftCases, rightCases, literals, dependency)
		if err != nil {
			_ = q.closeValues(result)
			return nil, err
		}
		result = append(result, copy)
	}
	return result, nil
}

func (q *smtSequenceDisequationQueue) Len() int {
	if q == nil || q.closed {
		return 0
	}
	return len(q.disequations)
}

func (q *smtSequenceDisequationQueue) At(index int) (left, right ast.ASTHandle, leftCases, rightCases [][]ast.ASTHandle, literals []SMTLiteral, dependency *smtSequenceDependency, err error) {
	if err = q.open(); err != nil {
		return
	}
	if index < 0 || index >= len(q.disequations) {
		err = ast.ErrASTDomain
		return
	}
	return q.disequations[index].values(q.manager)
}

func (q *smtSequenceDisequationQueue) Push(left, right ast.ASTHandle, dependency *smtSequenceDependency) error {
	if err := q.open(); err != nil {
		return err
	}
	value, err := newSMTSequenceDisequation(q.manager, left, right, dependency)
	if err != nil {
		return err
	}
	q.disequations = append(q.disequations, value)
	return nil
}

func (q *smtSequenceDisequationQueue) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	snapshot, err := q.clone(q.disequations)
	if err != nil {
		return err
	}
	q.snapshots = append(q.snapshots, snapshot)
	return nil
}

func (q *smtSequenceDisequationQueue) Set(index int, left, right ast.ASTHandle, leftCases, rightCases [][]ast.ASTHandle, literals []SMTLiteral, dependency *smtSequenceDependency) error {
	if err := q.open(); err != nil {
		return err
	}
	if index < 0 || index >= len(q.disequations) {
		return ast.ErrASTDomain
	}
	replacement, err := newSMTSequenceDisequationCases(q.manager, left, right, leftCases, rightCases, literals, dependency)
	if err != nil {
		return err
	}
	if err = q.disequations[index].close(q.manager); err != nil {
		return errors.Join(err, replacement.close(q.manager))
	}
	q.disequations[index] = replacement
	return nil
}

// Source solve_nqs replaces a removed slot by the last record and revisits it.
func (q *smtSequenceDisequationQueue) EraseSwapBack(index int) error {
	if err := q.open(); err != nil {
		return err
	}
	if index < 0 || index >= len(q.disequations) {
		return ast.ErrASTDomain
	}
	if err := q.disequations[index].close(q.manager); err != nil {
		return err
	}
	last := len(q.disequations) - 1
	q.disequations[index] = q.disequations[last]
	q.disequations[last] = smtSequenceDisequation{}
	q.disequations = q.disequations[:last]
	return nil
}

func (q *smtSequenceDisequationQueue) PopScopes(count uint32) (result error) {
	if err := q.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(q.snapshots)) {
		return ast.ErrASTDomain
	}
	level := len(q.snapshots) - int(count)
	result = q.closeValues(q.disequations)
	for i := level + 1; i < len(q.snapshots); i++ {
		result = errors.Join(result, q.closeValues(q.snapshots[i]))
	}
	q.disequations = q.snapshots[level]
	q.snapshots = q.snapshots[:level]
	return result
}

func (q *smtSequenceDisequationQueue) Close() (result error) {
	if err := q.open(); err != nil {
		return err
	}
	result = q.closeValues(q.disequations)
	for i := range q.snapshots {
		result = errors.Join(result, q.closeValues(q.snapshots[i]))
	}
	q.disequations = nil
	q.snapshots = nil
	q.manager = nil
	q.closed = true
	return result
}
