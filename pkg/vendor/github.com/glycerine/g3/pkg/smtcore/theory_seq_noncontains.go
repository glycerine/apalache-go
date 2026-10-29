package smtcore

// Source: src/smt/theory_seq.h nc and scoped_vector<nc>.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSequenceNonContains struct {
	expression ast.ASTRef
	dependency *smtSequenceDependency
}

type smtSequenceNonContainsQueue struct {
	manager   *ast.ASTManager
	values    []smtSequenceNonContains
	snapshots [][]smtSequenceNonContains
	closed    bool
}

func newSMTSequenceNonContainsQueue(manager *ast.ASTManager) (*smtSequenceNonContainsQueue, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtSequenceNonContainsQueue{manager: manager}, nil
}

func (q *smtSequenceNonContainsQueue) open() error {
	if q == nil || q.manager == nil || q.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (q *smtSequenceNonContainsQueue) closeValues(values []smtSequenceNonContains) (result error) {
	for i := range values {
		result = errors.Join(result, q.manager.DelRef(&values[i].expression))
		values[i].dependency = nil
	}
	return result
}

func (q *smtSequenceNonContainsQueue) clone(values []smtSequenceNonContains) ([]smtSequenceNonContains, error) {
	result := make([]smtSequenceNonContains, 0, len(values))
	for i := range values {
		expression, err := q.manager.RefGet(values[i].expression)
		if err != nil {
			_ = q.closeValues(result)
			return nil, err
		}
		ref, err := q.manager.NewRef(expression)
		if err != nil {
			_ = q.closeValues(result)
			return nil, err
		}
		result = append(result, smtSequenceNonContains{expression: ref, dependency: values[i].dependency})
	}
	return result, nil
}

func (q *smtSequenceNonContainsQueue) Len() int {
	if q == nil || q.closed {
		return 0
	}
	return len(q.values)
}

func (q *smtSequenceNonContainsQueue) At(index int) (ast.ASTHandle, *smtSequenceDependency, error) {
	if err := q.open(); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if index < 0 || index >= len(q.values) {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	expression, err := q.manager.RefGet(q.values[index].expression)
	return expression, q.values[index].dependency, err
}

func (q *smtSequenceNonContainsQueue) Push(expression ast.ASTHandle, dependency *smtSequenceDependency) error {
	if err := q.open(); err != nil {
		return err
	}
	ref, err := q.manager.NewRef(expression)
	if err != nil {
		return err
	}
	q.values = append(q.values, smtSequenceNonContains{expression: ref, dependency: dependency})
	return nil
}

func (q *smtSequenceNonContainsQueue) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	snapshot, err := q.clone(q.values)
	if err != nil {
		return err
	}
	q.snapshots = append(q.snapshots, snapshot)
	return nil
}

func (q *smtSequenceNonContainsQueue) EraseSwapBack(index int) error {
	if err := q.open(); err != nil {
		return err
	}
	if index < 0 || index >= len(q.values) {
		return ast.ErrASTDomain
	}
	if err := q.manager.DelRef(&q.values[index].expression); err != nil {
		return err
	}
	last := len(q.values) - 1
	q.values[index] = q.values[last]
	q.values[last] = smtSequenceNonContains{}
	q.values = q.values[:last]
	return nil
}

// Source check_contains/solve_nc: retire solved entries by swap removal and
// immediately visit replacements, including disjunction children.
func (t *SMTSequenceTheory) checkNegativeContains() (bool, error) {
	before := t.state.searchProgress()
	for i := 0; !t.state.Inconsistent() && i < t.nonContains.Len(); {
		remove, err := t.solveNegativeContains(i)
		if err != nil {
			return false, err
		}
		if remove {
			if err = t.nonContains.EraseSwapBack(i); err != nil {
				return false, err
			}
			continue
		}
		i++
	}
	return t.state.Inconsistent() || t.state.searchProgress() != before, nil
}

// Source canonizes(false, e), called before exposing negative witnesses.
func (t *SMTSequenceTheory) canonizesNegativeSequence(expression ast.ASTHandle) (done bool, result error) {
	ref, dependency, err := t.canonizeSequenceExpression(expression, nil)
	if err != nil {
		return false, err
	}
	m := t.state.manager
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	canonical, err := m.RefGet(ref)
	if err != nil {
		return false, err
	}
	if m.IsTrue(canonical) {
		literal, err := t.state.Literal(expression)
		if err != nil {
			return false, err
		}
		_, err = t.propagateSequenceLiteral(literal, dependency)
		return true, err
	}
	return m.IsFalse(canonical), nil
}

func (t *SMTSequenceTheory) solveNegativeContains(index int) (remove bool, result error) {
	expression, dependency, err := t.nonContains.At(index)
	if err != nil {
		return false, err
	}
	ref, dependency, err := t.canonizeSequenceExpression(expression, dependency)
	if err != nil {
		return false, err
	}
	m := t.state.manager
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	canonical, err := m.RefGet(ref)
	if err != nil {
		return false, err
	}
	if m.IsTrue(canonical) {
		return true, t.setDependencyConflict(dependency)
	}
	if m.IsFalse(canonical) {
		return true, nil
	}
	if canonical != expression {
		if err = t.nonContains.Push(canonical, dependency); err != nil {
			return false, err
		}
		t.state.theoryProgress++
		return true, nil
	}
	family, err := m.GetFamilyID(canonical)
	if err != nil {
		return false, err
	}
	kind, err := m.GetDeclKind(canonical)
	if err != nil || family != 0 {
		return false, err
	}
	if kind == ast.ASTOpEq || kind == ast.ASTOpIff {
		if err = t.state.Internalize(canonical, true); err != nil {
			return false, err
		}
		literal, err := t.state.Literal(canonical)
		if err != nil {
			return false, err
		}
		_, err = t.propagateSequenceLiteral(literal.Negated(), dependency)
		return true, err
	}
	if kind == ast.ASTOpOr {
		count, err := m.GetNumArgs(canonical)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count; i++ {
			child, err := m.GetArg(canonical, i)
			if err != nil {
				return false, err
			}
			if err = t.nonContains.Push(child, dependency); err != nil {
				return false, err
			}
		}
		t.state.theoryProgress++
		return true, nil
	}
	return false, nil
}

func (q *smtSequenceNonContainsQueue) PopScopes(count uint32) (result error) {
	if err := q.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(q.snapshots)) {
		return ast.ErrASTDomain
	}
	level := len(q.snapshots) - int(count)
	result = q.closeValues(q.values)
	for i := level + 1; i < len(q.snapshots); i++ {
		result = errors.Join(result, q.closeValues(q.snapshots[i]))
	}
	q.values = q.snapshots[level]
	q.snapshots = q.snapshots[:level]
	return result
}

func (q *smtSequenceNonContainsQueue) Close() (result error) {
	if err := q.open(); err != nil {
		return err
	}
	result = q.closeValues(q.values)
	for i := range q.snapshots {
		result = errors.Join(result, q.closeValues(q.snapshots[i]))
	}
	q.values = nil
	q.snapshots = nil
	q.manager = nil
	q.closed = true
	return result
}
