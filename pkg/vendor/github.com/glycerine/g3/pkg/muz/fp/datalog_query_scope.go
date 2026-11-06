package fp

// Source: rel_context.cpp scoped_query (58-91) and predicate query (232-241).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// beginDatalogQueryScope protects the source program before any relational
// transformation, including filter/simple-join and similarity normalization.
func (e *PropositionalEngine) beginDatalogQueryScope(outputs []ast.ASTHandle) (_ *datalogUnboundScope, resultErr error) {
	set, err := base.NewRuleSet(e.rules)
	if err != nil {
		return nil, err
	}
	scope := &datalogUnboundScope{engine: e, set: set, predicates: make(map[ast.ASTHandle]bool, len(e.predicates))}
	for declaration := range e.predicates {
		scope.predicates[declaration] = true
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, scope.dispose())
		}
	}()
	if err = set.AddRules(e.set); err != nil {
		return nil, err
	}
	for _, declaration := range outputs {
		if err = set.SetOutputPredicate(declaration); err != nil {
			return nil, err
		}
	}
	published = true
	return scope, nil
}
