package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type externalSimilarityQueryScope struct {
	plugin    *rel.ExternalRelationPlugin
	program   *rel.ExternalSimilarityProgram
	auxiliary []ast.ASTHandle
}

func (s *externalSimilarityQueryScope) close() error {
	if s == nil {
		return nil
	}
	for len(s.auxiliary) != 0 {
		last := len(s.auxiliary) - 1
		predicate := s.auxiliary[last]
		err := s.plugin.DropPredicateRelation(predicate)
		if err != nil {
			_, found, observeErr := s.plugin.TryGetPredicateRelation(predicate)
			if observeErr != nil || found {
				return errors.Join(err, observeErr)
			}
		}
		s.auxiliary = s.auxiliary[:last]
	}
	if s.program != nil {
		if err := s.program.Close(); err != nil {
			return err
		}
		s.program = nil
	}
	s.plugin = nil
	return nil
}

func (e *PropositionalEngine) discardExternalSimilarity() error {
	if e.externalSimilarity == nil {
		return nil
	}
	if err := e.externalSimilarity.close(); err != nil {
		return err
	}
	e.externalSimilarity = nil
	return nil
}

func (e *PropositionalEngine) prepareExternalSimilarity(declarations []ast.ASTHandle, threshold uint32) (_ *base.RuleSet, resultErr error) {
	if err := e.discardExternalSimilarity(); err != nil {
		return nil, err
	}
	program, err := rel.NewExternalSimilarityProgram(e.manager, e.set, threshold, declarations)
	if err != nil {
		return nil, err
	}
	scope := &externalSimilarityQueryScope{plugin: e.external, program: program}
	e.externalSimilarity = scope
	rules, err := program.Rules()
	if err != nil {
		return nil, err
	}
	for _, predicate := range program.AuxiliaryPredicates() {
		relation, cause := e.external.PredicateRelation(predicate)
		if cause != nil {
			return nil, cause
		}
		scope.auxiliary = append(scope.auxiliary, predicate)
		rows, found := program.SeedRows(predicate)
		if !found {
			return nil, ErrPropositionalHorn
		}
		for _, row := range rows {
			if cause = relation.AddFact(row); cause != nil {
				return nil, cause
			}
		}
	}
	return rules, nil
}
