package fp

// Source: dl_relation_manager.cpp empty_signature_relation_join_fn. A finite
// nullary input selects either the empty relation or an independent clone of
// the other operand; it does not convert the input's table representation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func udocFiniteNullary(state *predicateState) bool {
	return state != nil && len(state.sorts) == 0 && state.explicitFiniteRepresentation && state.table != nil
}

// This first mixed specialization consumes finite nullaries as extensional
// positive guards. Finite heads and negatives need their own generic dispatch.
func (e *PropositionalEngine) udocNullaryGuardsOnly() (bool, error) {
	count, err := e.set.Len()
	if err != nil {
		return false, err
	}
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return false, err
		}
		valid, cause := func() (bool, error) {
			head, err := rule.HeadDeclaration()
			if err != nil || udocFiniteNullary(e.predicates[head]) {
				return false, err
			}
			positive, err := rule.PositiveTailSize()
			if err != nil {
				return false, err
			}
			uninterpreted, err := rule.UninterpretedTailSize()
			if err != nil {
				return false, err
			}
			for index := positive; index < uninterpreted; index++ {
				tail, _, err := rule.Tail(index)
				if err != nil {
					return false, err
				}
				declaration, err := e.manager.GetDecl(tail)
				if err != nil || udocFiniteNullary(e.predicates[declaration]) {
					return false, err
				}
			}
			return true, nil
		}()
		closeErr := rule.Close()
		if cause != nil || closeErr != nil {
			return false, errors.Join(cause, closeErr)
		}
		if !valid {
			return false, nil
		}
	}
	return true, nil
}

func (c *datalogUDocContext) nullaryAnswer(declaration ast.ASTHandle) (ast.ASTRef, bool, error) {
	present, found := c.nullaryResult[declaration]
	if !found {
		return ast.ASTRef{}, false, nil
	}
	var value ast.ASTHandle
	var err error
	if present {
		value, err = c.manager.MkTrue()
	} else {
		value, err = c.manager.MkFalse()
	}
	if err != nil {
		return ast.ASTRef{}, true, err
	}
	result, err := c.manager.NewRef(value)
	return result, true, err
}
