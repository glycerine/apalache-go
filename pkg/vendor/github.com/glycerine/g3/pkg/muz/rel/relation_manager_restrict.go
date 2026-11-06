package rel

// Source: dl_relation_manager.cpp restrict_predicates (126-143), used by
// dl_context::scoped_query to remove query-local predicate relations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// RestrictPredicates retires relations outside keep and intersects saturation
// marks. Owners whose cleanup refuses remain attached for an explicit retry.
func (m *RelationManager) RestrictPredicates(keep map[ast.ASTHandle]bool) (result error) {
	if err := m.open(); err != nil {
		return err
	}
	for predicate, product := range m.explanations {
		if keep[predicate] {
			continue
		}
		if err := product.Close(); err != nil {
			result = errors.Join(result, err)
			continue
		}
		consumed, err := m.manager.DecRefConsumed(predicate)
		result = errors.Join(result, err)
		if consumed {
			delete(m.explanations, predicate)
		}
	}
	for predicate, product := range m.relationLevel {
		if keep[predicate] {
			continue
		}
		if err := product.Close(); err != nil {
			result = errors.Join(result, err)
			continue
		}
		consumed, err := m.manager.DecRefConsumed(predicate)
		result = errors.Join(result, err)
		if consumed {
			delete(m.relationLevel, predicate)
		}
	}
	for predicate, table := range m.finite {
		if keep[predicate] {
			continue
		}
		if err := table.Close(); err != nil {
			result = errors.Join(result, err)
			continue
		}
		consumed, err := m.manager.DecRefConsumed(predicate)
		result = errors.Join(result, err)
		if consumed {
			delete(m.finite, predicate)
		}
	}
	for predicate, table := range m.relations {
		if keep[predicate] {
			continue
		}
		if err := table.Close(); err != nil {
			result = errors.Join(result, err)
			continue
		}
		consumed, err := m.manager.DecRefConsumed(predicate)
		result = errors.Join(result, err)
		if consumed {
			delete(m.relations, predicate)
		}
	}
	for predicate := range m.saturated {
		if !keep[predicate] {
			delete(m.saturated, predicate)
		}
	}
	return result
}
