package fp

import "github.com/glycerine/g3/pkg/ast"

// rule_manager::mk_query chooses the first predicate in the flattened body;
// its requested kind is inherited by the fresh query head. Result precision
// belongs to the concrete relation produced during evaluation, not this
// declaration metadata.
func (e *PropositionalEngine) inheritDatalogQueryKind(query, head ast.ASTHandle) error {
	for {
		kind, err := e.manager.GetKind(query)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			break
		}
		query, err = e.manager.GetQuantifierBody(query)
		if err != nil {
			return err
		}
	}
	pending, err := e.manager.FlattenAnd([]ast.ASTHandle{query})
	if err != nil {
		return err
	}
	for _, current := range pending {
		kind, err := e.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			continue
		}
		declaration, err := e.manager.GetDecl(current)
		if err != nil {
			return err
		}
		state := e.predicates[declaration]
		if state == nil {
			continue
		}
		target := e.predicates[head]
		target.requestedRepresentations = append([]string(nil), state.requestedRepresentations...)
		if len(target.requestedRepresentations) == 1 {
			name := target.requestedRepresentations[0]
			switch name {
			case "tr_hashtable", "tr_sparse", "tr_lazy_sparse":
				supported := target.table != nil && (name == "tr_hashtable" || len(target.sorts) != 0)
				if supported {
					if err := e.selectFiniteRepresentation(head, name); err != nil {
						return err
					}
					target.explicitFiniteRepresentation = true
				}
			case "external_relation":
				relation, err := e.ExternalPredicateRelation(head)
				if err != nil {
					return err
				}
				target.external = relation
				target.table = nil
			}
		}
		return nil
	}
	return nil
}
