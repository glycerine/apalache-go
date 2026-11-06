package fp

// Source: rel_context::add_fact(table_fact), non-table relation branch.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (e *PropositionalEngine) addArithmeticTableFact(declaration ast.ASTHandle, values []uint32) (resultErr error) {
	state := e.predicates[declaration]
	if state == nil || len(state.sorts) != len(values) {
		return ErrPropositionalHorn
	}
	if err := e.clearArithmeticTableFactCleanup(); err != nil {
		return err
	}
	attempt := &arithmeticTableFactAttempt{manager: e.manager, roots: make([]ast.ASTRef, 0, len(values)), handles: make([]ast.ASTHandle, len(values))}
	transferred := false
	defer func() {
		if !transferred {
			resultErr = errors.Join(resultErr, e.finishArithmeticTableFactAttempt(attempt))
		}
	}()
	for i, value := range values {
		h, err := e.manager.MkDatalogNumeral(uint64(value), state.sorts[i])
		if err != nil {
			return err
		}
		ref, err := e.manager.NewRef(h)
		if err != nil {
			return err
		}
		attempt.roots = append(attempt.roots, ref)
		attempt.handles[i] = h
	}
	transferred = true
	return attempt.addFact(e, declaration)
}
