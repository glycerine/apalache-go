package fp

// Source: src/muz/rel/dl_base.cpp table_base::to_formula. Each tuple's
// equalities and the final disjunction have owning expression scopes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *PropositionalEngine) finiteRelationAnswer(state *predicateState) (answer ast.ASTRef, result error) {
	attempt := &storedFactFormulaAttempt{manager: e.manager}
	defer func() {
		if result != nil {
			if answer != (ast.ASTRef{}) {
				attempt.outputs = append(attempt.outputs, answer)
				answer = ast.ASTRef{}
			}
			result = attempt.fail(result)
		}
	}()
	reserveCurrent := func() (ast.ASTRef, error) {
		ref, err := e.manager.NewRef(ast.ASTHandle{})
		if ref != (ast.ASTRef{}) {
			attempt.current = append(attempt.current, ref)
		}
		return ref, err
	}
	facts, err := state.table.Facts()
	if err != nil {
		return answer, err
	}
	rows := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		values, err := e.relations.DecodeFact(state.sorts, fact)
		attempt.current = values
		if err != nil {
			return answer, err
		}
		equalities := make([]ast.ASTHandle, len(fact))
		for i := range fact {
			root, err := reserveCurrent()
			if err != nil {
				return answer, err
			}
			variable, err := e.manager.MkVar(uint32(i), state.sorts[i])
			if !variable.IsNull() {
				err = errors.Join(err, e.manager.SetRef(root, variable))
			}
			if err != nil {
				return answer, err
			}
			value, err := e.manager.RefGet(values[i])
			if err != nil {
				return answer, err
			}
			root, err = reserveCurrent()
			if err != nil {
				return answer, err
			}
			equalities[i], err = e.manager.MkEq(variable, value)
			if !equalities[i].IsNull() {
				err = errors.Join(err, e.manager.SetRef(root, equalities[i]))
			}
			if err != nil {
				return answer, err
			}
		}
		root, err := e.manager.NewRef(ast.ASTHandle{})
		if root != (ast.ASTRef{}) {
			attempt.outputs = append(attempt.outputs, root)
		}
		if err != nil {
			return answer, err
		}
		row, err := e.manager.MkAndArray(equalities)
		if !row.IsNull() {
			err = errors.Join(err, e.manager.SetRef(root, row))
		}
		if err != nil {
			return answer, err
		}
		rows = append(rows, row)
		if err = attempt.clearCurrent(); err != nil {
			return answer, err
		}
	}
	answer, err = e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return answer, err
	}
	formula, err := e.manager.MkOrArray(rows)
	if !formula.IsNull() {
		err = errors.Join(err, e.manager.SetRef(answer, formula))
	}
	if err != nil {
		return answer, err
	}
	return answer, attempt.close()
}
