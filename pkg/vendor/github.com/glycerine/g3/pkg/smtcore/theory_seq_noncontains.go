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
	values    []smtSequenceNonContains // physical entries
	index     []int
	size      int
	scopes    []smtSequenceNonContainsScope
	trail     []smtSequenceNonContainsIndex
	elemStart int
	closed    bool
}

type smtSequenceNonContainsScope struct{ size, trailStart, elemStart int }
type smtSequenceNonContainsIndex struct{ source, destination int }

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

func (q *smtSequenceNonContainsQueue) copyValue(value smtSequenceNonContains) (smtSequenceNonContains, error) {
	expression, err := q.manager.RefGet(value.expression)
	if err != nil {
		return smtSequenceNonContains{}, err
	}
	ref, err := q.manager.NewRef(expression)
	if err != nil {
		return smtSequenceNonContains{}, err
	}
	return smtSequenceNonContains{expression: ref, dependency: value.dependency}, nil
}

func (q *smtSequenceNonContainsQueue) Len() int {
	if q == nil || q.closed {
		return 0
	}
	return q.size
}

func (q *smtSequenceNonContainsQueue) At(index int) (ast.ASTHandle, *smtSequenceDependency, error) {
	if err := q.open(); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if index < 0 || index >= q.size {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	value := q.values[q.index[index]]
	expression, err := q.manager.RefGet(value.expression)
	return expression, value.dependency, err
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
	logical := q.size
	q.setIndex(logical)
	if logical == len(q.index) {
		q.index = append(q.index, len(q.values)-1)
	} else {
		q.index[logical] = len(q.values) - 1
	}
	q.size++
	return nil
}

func (q *smtSequenceNonContainsQueue) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	q.scopes = append(q.scopes, smtSequenceNonContainsScope{q.size, len(q.trail), len(q.values)})
	q.elemStart = len(q.values)
	return nil
}

func (q *smtSequenceNonContainsQueue) EraseSwapBack(index int) error {
	if err := q.open(); err != nil {
		return err
	}
	if index < 0 || index >= q.size {
		return ast.ErrASTDomain
	}
	last := q.size - 1
	if index != last {
		copy, err := q.copyValue(q.values[q.index[last]])
		if err != nil {
			return err
		}
		physical := q.index[index]
		if physical < q.elemStart {
			q.setIndex(index)
			q.values = append(q.values, copy)
			q.index[index] = len(q.values) - 1
		} else {
			if err = q.manager.DelRef(&q.values[physical].expression); err != nil {
				return errors.Join(err, q.manager.DelRef(&copy.expression))
			}
			q.values[physical] = copy
		}
	}
	return q.popBack()
}

func (q *smtSequenceNonContainsQueue) popBack() error {
	logical := q.size - 1
	physical := q.index[logical]
	if physical == len(q.values)-1 && physical >= q.elemStart {
		if err := q.manager.DelRef(&q.values[physical].expression); err != nil {
			return err
		}
		q.values = q.values[:physical]
	}
	q.size = logical
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
	if count == 0 || count > uint32(len(q.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(q.scopes) - int(count)
	scope := q.scopes[level]
	for i := len(q.trail) - 1; i >= scope.trailStart; i-- {
		q.index[q.trail[i].source] = q.trail[i].destination
	}
	q.trail = q.trail[:scope.trailStart]
	result = q.closeValues(q.values[scope.elemStart:])
	q.values = q.values[:scope.elemStart]
	q.size = scope.size
	q.scopes = q.scopes[:level]
	q.elemStart = scope.elemStart
	return result
}

func (q *smtSequenceNonContainsQueue) Close() (result error) {
	if err := q.open(); err != nil {
		return err
	}
	result = q.closeValues(q.values)
	q.values = nil
	q.index = nil
	q.scopes = nil
	q.trail = nil
	q.manager = nil
	q.closed = true
	return result
}

func (q *smtSequenceNonContainsQueue) setIndex(source int) {
	if source < len(q.index) && source < q.elemStart {
		q.trail = append(q.trail, smtSequenceNonContainsIndex{source, q.index[source]})
	}
}
