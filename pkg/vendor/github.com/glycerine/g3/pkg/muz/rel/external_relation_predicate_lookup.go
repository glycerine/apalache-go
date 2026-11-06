package rel

import "github.com/glycerine/g3/pkg/ast"

// TryGetPredicateRelation observes an already published predicate owner. It
// never constructs a relation or invokes an external callback, including
// while callers are reporting a failed replacement cleanup.
func (p *ExternalRelationPlugin) TryGetPredicateRelation(predicate ast.ASTHandle) (*ExternalRelation, bool, error) {
	if p == nil || p.closed || p.manager == nil || p.manager.open() != nil || predicate.IsNull() {
		return nil, false, ErrTableRelation
	}
	relation, found := p.predicates[predicate]
	return relation, found, nil
}
