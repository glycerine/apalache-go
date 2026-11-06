package fp

// Source: src/muz/rel/rel_context.cpp similarity-compressor registration.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) datalogSimilarityPlugin(outputs []ast.ASTHandle) (*rel.FiniteSimilarityCompressor, error) {
	if e.backend != "datalog" {
		return nil, nil
	}
	enabled, threshold := true, uint32(11)
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return nil, err
		}
		enabled, err = view.Bool("datalog.similarity_compressor", true)
		if err == nil && enabled {
			threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return nil, err
		}
	}
	if !enabled {
		return nil, nil
	}
	var requested []ast.ASTHandle
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state != nil && state.explicitFiniteRepresentation {
			requested = append(requested, declaration)
		}
	}
	return rel.NewFiniteSimilarityCompressorForRulesWithRequestedKinds(e.rules, e.relations, threshold, outputs, requested)
}

func (e *PropositionalEngine) installDatalogSimilarityPredicates(plugin *rel.FiniteSimilarityCompressor) error {
	if plugin == nil {
		return nil
	}
	for _, decl := range plugin.FreshPredicates() {
		state, err := e.predicateState(decl)
		if err != nil {
			return err
		}
		if e.predicates[decl] == nil {
			e.predicateOrder = append(e.predicateOrder, decl)
		}
		e.predicates[decl] = state
		if origin, found := plugin.FreshPredicateOrigin(decl); found {
			if err = e.inheritDatalogFiniteRepresentation(decl, origin); err != nil {
				return err
			}
		}
		e.auxiliary[decl] = true
	}
	return nil
}

func (e *PropositionalEngine) applyDatalogSimilarity(outputs []ast.ASTHandle) (resultErr error) {
	plugin, err := e.datalogSimilarityPlugin(outputs)
	if err != nil || plugin == nil {
		return err
	}
	transformer := base.NewRuleTransformer()
	if err = transformer.Register(plugin); err != nil {
		return errors.Join(err, plugin.Close(), transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.Close()) }()
	modified, err := transformer.Apply(e.set)
	if err != nil || !modified {
		return err
	}
	if err = e.installDatalogSimilarityPredicates(plugin); err != nil {
		return err
	}
	e.revision++
	return nil
}
